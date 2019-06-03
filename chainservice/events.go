package chainservice

import (
	"crypto/ecdsa"
	"math/big"
	"time"

	"github.com/jinzhu/gorm"

	"github.com/SmartMeshFoundation/Photon/notify"

	"github.com/SmartMeshFoundation/Photon/transfer"

	"fmt"

	"sync/atomic"

	"github.com/SmartMeshFoundation/Photon-Monitoring/models"
	"github.com/SmartMeshFoundation/Photon-Monitoring/params"
	"github.com/SmartMeshFoundation/Photon/blockchain"
	"github.com/SmartMeshFoundation/Photon/log"
	"github.com/SmartMeshFoundation/Photon/network/helper"
	"github.com/SmartMeshFoundation/Photon/network/rpc"
	"github.com/SmartMeshFoundation/Photon/transfer/mediatedtransfer"
	"github.com/SmartMeshFoundation/Photon/utils"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

/*
ChainEvents block chain operations
*/
type ChainEvents struct {
	client      *helper.SafeEthClient
	be          *blockchain.Events
	bcs         *rpc.BlockChainService
	key         *ecdsa.PrivateKey
	db          *models.ModelDB
	quitChan    chan struct{}
	stopped     bool
	blockNumber *atomic.Value
}

//NewChainEvents create chain events
func NewChainEvents(key *ecdsa.PrivateKey, client *helper.SafeEthClient, tokenNetworkRegistryAddress common.Address, db *models.ModelDB) *ChainEvents {
	log.Trace(fmt.Sprintf("tokenNetworkRegistryAddress %s", tokenNetworkRegistryAddress.String()))
	bcs, err := rpc.NewBlockChainService(key, tokenNetworkRegistryAddress, client, &notify.Handler{}, &mockTxInfoDao{})
	if err != nil {
		panic(err)
	}
	_, err = bcs.Registry(tokenNetworkRegistryAddress, true)
	if err != nil {
		panic("startup error : cannot get registry")
	}
	return &ChainEvents{
		client:      client,
		be:          blockchain.NewBlockChainEvents(client, bcs, &mockChainEventRecordDao{}),
		bcs:         bcs,
		key:         key,
		db:          db,
		quitChan:    make(chan struct{}),
		blockNumber: new(atomic.Value),
	}
}

//Start moniter blockchain
func (ce *ChainEvents) Start() error {
	ce.be.Start(ce.db.GetLatestBlockNumber())
	go ce.loop()
	return nil
}

//Stop service
func (ce *ChainEvents) Stop() {
	ce.be.Stop()
	close(ce.quitChan)
}

func (ce *ChainEvents) loop() {
	for {
		select {
		case st, ok := <-ce.be.StateChangeChannel:
			if !ok {
				log.Info("StateChangeChannel closed")
				return
			}
			ce.handleStateChange(st)
		case <-ce.quitChan:
			return
		}
	}
}

/*
一个通道关闭了,主要进行的就是设置
todo 限制锁的数量不能超过settle_timeout/2或者这里的 unlock 以及updatebalanceproof 用并行模式
考虑到实际交易过程中,App可能会随时切换到无网状态,为了保证利益,委托应该设计成这样.
1. 通道关闭的时候,记录下需要在settleBlockNumber减去RevealTimeout进行进行updateBalanceProof,
2. 如果需要,同时记录下,需要在settleBlockNumber减去RevealTimeout进行unlock
3. 如果需要,同时记录下,需要在settleBlockNumber进行punish

根据从链上获取的settleBlockNumber和settleTimeout进行判断,当前委托是要进行updatebalanceproof还是进行unlock和punish

考虑到App有可能在通道关闭以后,settleBlockNumber之前随时更新unlock和punish,因此委托处理必须允许这种情况.
同时如果委托时发现通道已经关闭,那么应该根据情况更新步骤2,3中的记录

//todo 如何测试呢?

*/
func (ce *ChainEvents) handleClosedStateChange(st2 *mediatedtransfer.ContractClosedStateChange) {
	ds, err := ce.db.GetDelegateListByChannelIdentifier(st2.ChannelIdentifier)
	if err != nil {
		log.Error(fmt.Sprintf("GetDelegateListByChannelIdentifier for ContractReceiveClosedStateChange err=%s st=%s",
			err, utils.StringInterface(st2, 2)))
		return
	}
	if ds == nil || len(ds) == 0 { //没有什么要做的
		log.Trace(fmt.Sprintf("%s closed, but no related delegate", st2.ChannelIdentifier.String()))
		return
	}
	log.Info(fmt.Sprintf("channel closed %s", utils.StringInterface(st2, 3)))
	tokenNetwork, err := ce.bcs.TokenNetwork(ds[0].TokenAddress())
	if err != nil {
		log.Error(fmt.Sprintf("create token network  error for token %s", ds[0].TokenAddressStr))
		return
	}
	settleBlockNumber, _, _, _, err := tokenNetwork.GetContract().GetChannelInfoByChannelIdentifier(nil, st2.ChannelIdentifier)
	if err != nil {
		log.Error(fmt.Sprintf("channel %s get settle timeout err %s", st2.ChannelIdentifier.String(), err))
		return
	}
	for _, d := range ds {
		// 1.  更新委托的信息
		d.SettleBlockNumber = int64(settleBlockNumber)
		if d.DelegatorAddress() == st2.ClosingAddress {
			// 自己关闭的话,不需要第三方代理来 UpdateBalanceProof 和 Withdraw 的,他自己会做.
			d.Status = models.DelegateStatusSuccessFinishedByOther
			d.Error = "delegator closed channel"
		}
		ce.db.UpdateObject(d)
		// 2. 添加monitor
		ce.db.AddDelegateMonitor(d)
	}
}

//如果发生了 balance proof update, 第三方服务就不用进行了,有可能是委托方自己做了
// 自己close对方update的话,已经在close中处理了,这里都不用做
// 对方close自己update的话,需要在这里标记状态
func (ce *ChainEvents) handleBalanceProofUpdatedStateChange(st2 *mediatedtransfer.ContractBalanceProofUpdatedStateChange) {
	key := models.BuildDelegateKey(st2.ChannelIdentifier, st2.Participant)
	d, err := ce.db.GetDelegateByKey(key)
	if err == gorm.ErrRecordNotFound {
		// 无需处理
		return
	}
	if err != nil {
		log.Error(fmt.Sprintf("db GetDelegateByKey err : %s", err.Error()))
		return
	}
	if d.Status == models.DelegateStatusInit && d.DelegatorAddress() == st2.Participant {
		log.Info(fmt.Sprintf("recevie transfer update by delegotor %s %s", st2.ChannelIdentifier.String(), st2.Participant.String()))
		d.Status = models.DelegateStatusSuccessFinishedByOther // TODO 这里服用对方upadte的状态,是否需要一个独立的状态
		d.Error = "delegator update balance proof"
		ce.db.UpdateObject(d)
	}
}

/*
通道settle
暂时先删除记录,后续根据需要再做修改
*/
func (ce *ChainEvents) handleSettledStateChange(st2 *mediatedtransfer.ContractSettledStateChange) {
	ds, err := ce.db.GetDelegateListByChannelIdentifier(st2.ChannelIdentifier)
	if err != nil {
		log.Error(fmt.Sprintf("db GetDelegateListByChannelIdentifier err : %s", err.Error()))
	}
	if len(ds) > 0 {
		log.Info(fmt.Sprintf("recevied ContractSettledStateChange %s", utils.StringInterface(st2, 3)))
		// 1. 标记删除
		for _, d := range ds {
			err = ce.db.DeleteDelegate(d.Key)
			if err != nil {
				log.Error(fmt.Sprintf("DeleteDelegate err %s", err.Error()))
			}
		}
		log.Info(fmt.Sprintf("%d delegate of channel %s deleted", len(ds), st2.ChannelIdentifier.String()))
	}
}

/*
合作关闭通道,以前的委托都可以删掉了,因为是合作 settle,
说明没有纠纷,因此也不需要提交证明了,并且因为通道 open block number 改变,原来的委托肯定也作废了
*/
func (ce *ChainEvents) handleCooperativeSettledStateChange(st2 *mediatedtransfer.ContractCooperativeSettledStateChange) {
	ds, err := ce.db.GetDelegateListByChannelIdentifier(st2.ChannelIdentifier)
	if err != nil {
		log.Error(fmt.Sprintf("db GetDelegateListByChannelIdentifier err : %s", err.Error()))
	}
	if len(ds) > 0 {
		log.Info(fmt.Sprintf("recevied ContractCooperativeSettledStateChange %s", utils.StringInterface(st2, 3)))
		// 1. 标记删除
		for _, d := range ds {
			err = ce.db.DeleteDelegate(d.Key)
			if err != nil {
				log.Error(fmt.Sprintf("DeleteDelegate err %s", err.Error()))
			}
		}
		log.Info(fmt.Sprintf("%d delegate of channel %s deleted", len(ds), st2.ChannelIdentifier.String()))
	}
}

/*
合作 withdraw,以前的委托都可以删掉了,因为是合作 withdraw,
说明没有纠纷,因此也不需要提交证明了,并且因为通道 open block number 改变,原来的委托肯定也作废了
*/
func (ce *ChainEvents) handleWithdrawStateChange(st2 *mediatedtransfer.ContractChannelWithdrawStateChange) {
	ds, err := ce.db.GetDelegateListByChannelIdentifier(st2.ChannelIdentifier.ChannelIdentifier)
	if err != nil {
		log.Error(fmt.Sprintf("db GetDelegateListByChannelIdentifier err : %s", err.Error()))
	}
	if len(ds) > 0 {
		log.Info(fmt.Sprintf("recevied ContractChannelWithdrawStateChange %s", utils.StringInterface(st2, 3)))
		// 1. 标记删除
		for _, d := range ds {
			err = ce.db.DeleteDelegate(d.Key)
			if err != nil {
				log.Error(fmt.Sprintf("DeleteDelegate err %s", err.Error()))
			}
		}
		log.Info(fmt.Sprintf("%d delegate of channel %s deleted", len(ds), st2.ChannelIdentifier.ChannelIdentifier.String()))
	}
}

/*
无法从连上直接获取当前注册了哪些token,只能按照事件检索.
*/
func (ce *ChainEvents) handleTokenAddedStateChange(st *mediatedtransfer.ContractTokenAddedStateChange) {
	//log.Trace(fmt.Sprintf("recevied ContractTokenAddedStateChange %s", utils.StringInterface(st, 3)))
	//err := ce.db.AddToken(st.TokenAddress, utils.EmptyAddress)
	//if err != nil {
	//	log.Error(fmt.Sprintf("handleTokenAddedStateChange err=%s, st=%s", err, utils.StringInterface1(st)))
	//}
}

/*
第三方监控服务目前只做三件事情:
1. update balance proof
2. unlock
3. punish
如果发生了通道关闭事件,那么第三方监控服务就需要
1. 如果不是关闭方,标记触发update balance proof 以及 withdraw 的时间
2. 如果需要 punish,那需要标记 punish 的触发时间
如果发生了coperative settle/withdraw, 说明委托已经是历史了,直接删除就可以了.
*/
func (ce *ChainEvents) handleStateChange(st transfer.StateChange) {
	switch st2 := st.(type) {
	case *transfer.BlockStateChange:
		ce.handleBlockNumber(st2.BlockNumber)
	case *mediatedtransfer.ContractClosedStateChange:
		//处理 channel 关闭事件
		ce.handleClosedStateChange(st2)
	case *mediatedtransfer.ContractBalanceProofUpdatedStateChange:
		//处理TransferUpdate事件
		ce.handleBalanceProofUpdatedStateChange(st2)
	case *mediatedtransfer.ContractCooperativeSettledStateChange:
		ce.handleCooperativeSettledStateChange(st2)
	case *mediatedtransfer.ContractChannelWithdrawStateChange:
		ce.handleWithdrawStateChange(st2)
	case *mediatedtransfer.ContractSettledStateChange:
		ce.handleSettledStateChange(st2)
	case *mediatedtransfer.ContractTokenAddedStateChange:
		ce.handleTokenAddedStateChange(st2)
		//punish不是在收到unlock的时候发生,而是在通道可以settle的时候发生,
		//default:
		//	log.Trace(fmt.Sprintf("receive state change: %s", utils.StringInterface(st2, 3)))
	}
}

func (ce *ChainEvents) handleBlockNumber(n int64) {
	ce.blockNumber.Store(n)
	monitors, err := ce.db.GetDelegateMonitorList(n)
	if err != nil {
		log.Error(fmt.Sprintf("GetDelegateMonitorList err %s", err))
		return
	}
	if len(monitors) > 0 {
		for _, monitor := range monitors {
			d, err := ce.db.GetDelegateByKey(monitor.DelegateKey)
			if err == gorm.ErrRecordNotFound {
				// 已经删除
				continue
			}
			if err != nil {
				log.Error(fmt.Sprintf("GetDelegateByKey err %s", err))
				return
			}
			switch monitor.Type {
			case models.MonitorTypeUnlockAndUpdateBalanceProof:
				//unlock 以及 updateBalanceProof
				if d.Status == models.DelegateStatusSuccessFinishedByOther {
					log.Info(fmt.Sprintf("handle delegate ,but it's status=%d, delegate=%s", d.Status, utils.StringInterface(d, 4)))
					//无论委托人是关闭方还是因为用户自己做了updateBalanceProof,解锁都会重新做一遍,大不了都失败而已.
					go ce.doDelegateUnlocks(d)
					continue
				} else {
					if d.Status != models.DelegateStatusInit {
						log.Error(fmt.Sprintf("handle delegate error,it's status=%d,delegate=%s", d.Status, utils.StringInterface(d, 4)))
						continue
					}
					err = ce.db.UpdateDelegateStatus(d, models.DelegateStatusRunning)
					if err != nil {
						log.Error(fmt.Sprintf("UpdateDelegateStatus  %s err %s", d.Key, err))
						continue
					}
					go func() {
						//先updateBalanceProof,无论成功与否都尝试进行unlock,就算是unlock尝试全部失败也要尝试.
						ce.doDelegateUpdateBalanceProof(d)
						ce.doDelegateUnlocks(d)
					}()
				}
			case models.MonitorTypePunish:
				// punish
				go ce.doDelegatePunishes(d)
			}
		}
	}
	ce.db.SaveLatestBlockNumber(n)
}

/*
必须先扣费,后执行
doDelegateUpdateBalanceProof 主要完成以下任务:
1. 调用 doDelegateUpdateBalanceProof,
2. 调用 Withdraw
3. 计费
4. 将每个 tx 结果记录保存,供以后查询
todo 如何处理在执行过程中,程序要求退出,等待?计费?如何解决
*/
func (ce *ChainEvents) doDelegateUpdateBalanceProof(d *models.Delegate) {
	// TODO 需要事务么
	// 0. 获取DelegateUpdateBalanceProof
	du := d.UpdateBalanceProof()
	if du.Nonce <= 0 {
		log.Info("delegate [channel=%s delegator=%s] UpdateTransfer no need because nonce = 0 ", d.ChannelIdentifierStr, d.DelegatorAddressStr)
		return
	}
	// 1. 锁定费用
	err := ce.db.AccountLockSmt(d.DelegatorAddress(), params.SmtUpdateTransfer)
	if err != nil {
		d.Status = models.DelegateStatusFailed
		d.Error = fmt.Sprintf("smt not enough,err=%s", err)
		ce.db.UpdateObject(d)
		return
	}
	// 2. 执行UpdateBalanceProof
	r := ce.doUpdateBalanceProof(d, du)
	// 3. 如果失败更新delegate,不扣费
	if r.Status != models.ExecuteStatusSuccessFinished {
		err = ce.db.AccountUnlockSmt(d.DelegatorAddress(), params.SmtUpdateTransfer)
		if err != nil {
			log.Error(fmt.Sprintf("AccountUnlockSmt err %s", err))
		}
		d.Status = models.DelegateStatusFailed
		d.Error = r.Error
		ce.db.UpdateObject(d)
		log.Error(fmt.Sprintf("delegate [channel=%s delegator=%s] UpdateTransfer called Failed : %s", d.ChannelIdentifierStr, d.DelegatorAddressStr, utils.StringInterface(du, 3)))
		return
	}
	log.Info(fmt.Sprintf("delegate [channel=%s delegator=%s] UpdateTransfer called SUCCESS", d.ChannelIdentifierStr, d.DelegatorAddressStr))
	// 3. 扣除
	err = ce.db.AccountUseSmt(d.DelegatorAddress(), params.SmtUpdateTransfer)
	if err != nil {
		log.Error(fmt.Sprintf("AccountUseSmt err %s", err))
	}
}

func (ce *ChainEvents) doUpdateBalanceProof(d *models.Delegate, du *models.DelegateUpdateBalanceProof) (r *models.DelegateExecuteRecord) {
	r = models.NewDelegateExecuteRecord(d, models.DelegateTypeUpdateBalanceProof, du)
	defer ce.db.SaveDelegateExecuteRecord(r)

	channelAddr := d.ChannelIdentifier()
	tokenNetwork, err := ce.bcs.TokenNetwork(d.TokenAddress())
	if err != nil {
		r.Error = fmt.Sprintf("TokenNetwork err : %s", err.Error())
		return
	}
	closingSignature := du.ClosingSignature
	nonClosingSignature := du.NonClosingSignature
	log.Trace(fmt.Sprintf("signer=%s, doDelegateUpdateBalanceProof=%s", utils.APex(ce.bcs.Auth.From), utils.StringInterface(&du, 4)))
	tx, err := tokenNetwork.GetContract().UpdateBalanceProofDelegate(ce.bcs.Auth, d.TokenAddress(), d.PartnerAddress(), d.DelegatorAddress(), du.TransferAmount(), du.Locksroot(), uint64(du.Nonce), du.ExtraHash(), closingSignature, nonClosingSignature)
	if err != nil {
		r.Error = fmt.Sprintf("create tx err : %s", err.Error())
		return
	}
	r.Status = models.ExecuteStatusErrorFinished // 默认失败
	r.TxHashStr = tx.Hash().String()
	r.TxCreateBlockNumber = ce.blockNumber.Load().(int64)
	r.TxCreateTimestamp = time.Now().Unix()
	txReceipt, err := bind.WaitMined(rpc.GetCallContext(), ce.bcs.Client, tx)
	if err != nil {
		r.Error = fmt.Sprintf("tx WaitMined err : %s", err.Error())
		return
	}
	r.TxPackBlockNumber = ce.blockNumber.Load().(int64)
	r.TxPackTimestamp = time.Now().Unix()
	if txReceipt.Status != types.ReceiptStatusSuccessful {
		log.Info(fmt.Sprintf("updatetransfer failed %s,receipt=%s", utils.HPex(channelAddr), utils.StringInterface(txReceipt, 3)))
		r.Error = "tx execution err "
		return
	}
	r.Status = models.ExecuteStatusSuccessFinished
	return
}

/*
为了简化起见,PMS并不考虑合适注册密码,以及密码是否注册,
当可以unlock的时间到来以后,PMS会尝试去unlock委托的每一个锁,如果成功则扣费,不成功则不计费.
todo 存在问题:
有可能对手在setteblockNumber-RevealTimeout之后去注册密码.
比如如下情形:
A-B-C交易,C收到MedaitedTransfer以后,A立即关闭A,B通道,C选择在AB通道settle时间临近时注册密码.
这样由于B来不及unlock,将会造成B的损失,而C不当得利.
因此要求使用PMS的photon节点,交易中采用的RevealTimeout一定要大于等于PMS中的RevealTimeout,否则
有可能造成损失
*/
func (ce *ChainEvents) doDelegateUnlocks(d *models.Delegate) {
	// TODO 需要事务么
	// 0. 获取DelegateUpdateBalanceProof
	dUpdateBalanceProof := d.UpdateBalanceProof()
	// 0. 获取DelegateUpdateBalanceProof及DelegateUnlocks及DelegateAnnounceDispose
	dus := d.Unlocks()
	if len(dus) == 0 {
		log.Info(fmt.Sprintf("delegate [channel=%s delegator=%s] Unlock no need ", d.ChannelIdentifierStr, d.DelegatorAddressStr))
		return
	}
	das, err := ce.db.GetDelegateAnnounceDisposeListByDelegateKey(d.Key)
	if err != nil {
		panic(err)
	}
	costSMT := new(big.Int).Mul(params.SmtUnlock, big.NewInt(int64(len(dus))))
	// 1. 锁定费用
	err = ce.db.AccountLockSmt(d.DelegatorAddress(), costSMT)
	if err != nil {
		d.Status = models.DelegateStatusFailed
		d.Error = fmt.Sprintf("smt not enough,err=%s", err)
		ce.db.UpdateObject(d)
		return
	}
	// 2. 执行Unlock
	hasErr := false
	hasSuccess := false
	for _, du := range dus {
		r := ce.doUnlock(d, du, dUpdateBalanceProof.TransferAmount(), das)
		if r.Status != models.ExecuteStatusSuccessFinished {
			hasErr = true
			err = ce.db.AccountUnlockSmt(d.DelegatorAddress(), params.SmtUnlock)
			if err != nil {
				log.Error(fmt.Sprintf("db AccountUnlockSmt err : %s", err.Error()))
			}
		} else {
			hasSuccess = true
			err = ce.db.AccountUseSmt(d.DelegatorAddress(), params.SmtUnlock)
			if err != nil {
				log.Error(fmt.Sprintf("db AccountUseSmt err : %s", err.Error()))
			}
		}
	}
	// 4. 更新成功/部分成功/失败 结果到delegate
	if hasErr {
		if hasSuccess {
			d.Status = models.DelegateStatusPartialSuccess
			d.Error = "unlock partial success"
		} else {
			d.Status = models.DelegateStatusFailed
			d.Error = "unlock all failed"
		}
		log.Error(fmt.Sprintf("delegate [channel=%s delegator=%s] Unlock called Failed : %s", d.ChannelIdentifierStr, d.DelegatorAddressStr, utils.StringInterface(dus, 3)))
	} else {
		d.Status = models.DelegateStatusSuccessFinished
		log.Info(fmt.Sprintf("delegate [channel=%s delegator=%s] Unlock called SUCCESS", d.ChannelIdentifierStr, d.DelegatorAddressStr))
	}
	ce.db.UpdateObject(d)
}

func (ce *ChainEvents) doUnlock(d *models.Delegate, du *models.DelegateUnlock, transferAmount *big.Int, das []*models.DelegateAnnounceDispose) (r *models.DelegateExecuteRecord) {
	r = models.NewDelegateExecuteRecord(d, models.DelegateTypeUnlock, du)
	defer ce.db.SaveDelegateExecuteRecord(r)

	for _, da := range das {
		if da.LockSecretHash() == du.LockSecretHash() {
			r.Status = models.ExecuteStatusSuccessFinished
			r.Error = fmt.Sprintf("give up by AnnounceDispose")
			return
		}
	}
	channelAddr := d.ChannelIdentifier()
	tokenNetwork, err := ce.bcs.TokenNetwork(d.TokenAddress())
	if err != nil {
		r.Error = fmt.Sprintf("TokenNetwork err : %s", err.Error())
		return
	}
	//不需要检测,密码必须在链上注册才有可能成功,
	//if lock.Expiration <= ce.GetBlockNumber() {
	//	return fmt.Errorf("lock has expired, expration=%d,currentBlockNumber=%d", lock.Expiration, ce.GetBlockNumber())
	//}
	tx, err := tokenNetwork.GetContract().UnlockDelegate(ce.bcs.Auth, d.TokenAddress(), d.PartnerAddress(), d.DelegatorAddress(), transferAmount, big.NewInt(du.Expiration), du.Amount(), du.LockSecretHash(), du.MerkleProof, du.Signature)
	if err != nil {
		r.Error = fmt.Sprintf("create tx err : %s", err.Error())
		return
	}
	r.Status = models.ExecuteStatusErrorFinished // 默认失败
	r.TxHashStr = tx.Hash().String()
	r.TxCreateBlockNumber = ce.blockNumber.Load().(int64)
	r.TxCreateTimestamp = time.Now().Unix()
	txReceipt, err := bind.WaitMined(rpc.GetCallContext(), ce.bcs.Client, tx)
	if err != nil {
		r.Error = fmt.Sprintf("tx WaitMined err : %s", err.Error())
		return
	}
	r.TxPackBlockNumber = ce.blockNumber.Load().(int64)
	r.TxPackTimestamp = time.Now().Unix()
	if txReceipt.Status != types.ReceiptStatusSuccessful {
		log.Info(fmt.Sprintf("unlock failed %s,receipt=%s", utils.HPex(channelAddr), utils.StringInterface(txReceipt, 3)))
		r.Error = "tx execution err "
		return
	}
	r.Status = models.ExecuteStatusSuccessFinished
	return
}

/*
在可以代理惩罚的时间(closedBlockNumber+settleTimeout)到来时,
逐个执行惩罚委托,只要有一个成功,就可以立即结束,因为多余的惩罚也没有任何意义.
务必此时先扣费,后执行
如果委托方自己进行了 punish, 监控服务多做一遍也没什么成本,简化处理流程.
*/
func (ce *ChainEvents) doDelegatePunishes(d *models.Delegate) {
	// TODO 需要事务么
	// 0. 获取DelegatePunishes
	dps, err := ce.db.GetDelegatePunishListByDelegateKey(d.Key)
	if err != nil {
		panic(err)
	}
	if len(dps) == 0 {
		log.Info(fmt.Sprintf("delegate [channel=%s delegator=%s] Punish no need ", d.ChannelIdentifierStr, d.DelegatorAddressStr))
		return
	}
	// 1. 锁定费用
	err = ce.db.AccountLockSmt(d.DelegatorAddress(), params.SmtPunish)
	if err != nil {
		d.Status = models.DelegateStatusFailed
		d.Error = fmt.Sprintf("smt not enough,err=%s", err)
		ce.db.UpdateObject(d)
		return
	}
	// 2. 执行Punish
	hasSuccess := false
	for _, dp := range dps {
		if hasSuccess {
			//无需继续执行,保存记录
			r := models.NewDelegateExecuteRecord(d, models.DelegateTypePunish, dp)
			r.Status = models.ExecuteStatusSuccessFinished
			r.Error = "no need because already punish success"
			ce.db.SaveDelegateExecuteRecord(r)
			continue
		}
		r := ce.doPunish(d, dp)
		if r.Status == models.ExecuteStatusSuccessFinished {
			hasSuccess = true
		}
	}
	// 4. 结果处理
	if hasSuccess {
		//成功则计费
		err = ce.db.AccountUseSmt(d.DelegatorAddress(), params.SmtPunish)
		if err != nil {
			log.Error(fmt.Sprintf("db AccountUseSmt err : %s", err.Error()))
		}
		log.Info(fmt.Sprintf("delegate [channel=%s delegator=%s] Punish called SUCCESS", d.ChannelIdentifierStr, d.DelegatorAddressStr))
	} else {
		// 失败解锁费用并更新delegate
		err = ce.db.AccountUnlockSmt(d.DelegatorAddress(), params.SmtPunish)
		if err != nil {
			log.Error(fmt.Sprintf("db AccountUnlockSmt err : %s", err.Error()))
		}
		d.Status = models.DelegateStatusFailed
		d.Error = "punish all failed"
		ce.db.UpdateObject(d)
		log.Error(fmt.Sprintf("delegate [channel=%s delegator=%s] Unlock called Failed : %s", d.ChannelIdentifierStr, d.DelegatorAddressStr, utils.StringInterface(dps, 3)))
	}
}

func (ce *ChainEvents) doPunish(d *models.Delegate, dp *models.DelegatePunish) (r *models.DelegateExecuteRecord) {
	r = models.NewDelegateExecuteRecord(d, models.DelegateTypePunish, dp)
	defer ce.db.SaveDelegateExecuteRecord(r)

	channelAddr := d.ChannelIdentifier()
	tokenNetwork, err := ce.bcs.TokenNetwork(d.TokenAddress())
	if err != nil {
		r.Error = fmt.Sprintf("TokenNetwork err : %s", err.Error())
		return
	}
	tx, err := tokenNetwork.GetContract().PunishObsoleteUnlock(ce.bcs.Auth, d.TokenAddress(), d.DelegatorAddress(), d.PartnerAddress(), dp.LockHash(), dp.AdditionalHash(), dp.Signature)
	if err != nil {
		r.Error = fmt.Sprintf("create tx err : %s", err.Error())
		return
	}
	r.Status = models.ExecuteStatusErrorFinished // 默认失败
	r.TxHashStr = tx.Hash().String()
	r.TxCreateBlockNumber = ce.blockNumber.Load().(int64)
	r.TxCreateTimestamp = time.Now().Unix()
	txReceipt, err := bind.WaitMined(rpc.GetCallContext(), ce.bcs.Client, tx)
	if err != nil {
		r.Error = fmt.Sprintf("tx WaitMined err : %s", err.Error())
		return
	}
	r.TxPackBlockNumber = ce.blockNumber.Load().(int64)
	r.TxPackTimestamp = time.Now().Unix()
	if txReceipt.Status != types.ReceiptStatusSuccessful {
		log.Info(fmt.Sprintf("punish failed %s,receipt=%s", utils.HPex(channelAddr), utils.StringInterface(txReceipt, 3)))
		r.Error = "tx execution err "
		return
	}
	r.Status = models.ExecuteStatusSuccessFinished
	return
}

//GetBlockNumber return latest blocknumber of ethereum
func (ce *ChainEvents) GetBlockNumber() int64 {
	return ce.blockNumber.Load().(int64)
}
