package chainservice

import (
	"crypto/ecdsa"
	"errors"
	"math/big"

	"github.com/SmartMeshFoundation/SmartRaiden/transfer"

	"fmt"

	"sync/atomic"

	"time"

	"bytes"

	"encoding/binary"

	"github.com/SmartMeshFoundation/SmartRaiden-Monitoring/models"
	"github.com/SmartMeshFoundation/SmartRaiden-Monitoring/params"
	"github.com/SmartMeshFoundation/SmartRaiden/blockchain"
	"github.com/SmartMeshFoundation/SmartRaiden/log"
	"github.com/SmartMeshFoundation/SmartRaiden/network/helper"
	"github.com/SmartMeshFoundation/SmartRaiden/network/rpc"
	smparams "github.com/SmartMeshFoundation/SmartRaiden/params"
	"github.com/SmartMeshFoundation/SmartRaiden/transfer/mediatedtransfer"
	"github.com/SmartMeshFoundation/SmartRaiden/utils"
	"github.com/asdine/storm"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

/*
ChainEvents block chain operations
*/
type ChainEvents struct {
	client          *helper.SafeEthClient
	be              *blockchain.Events
	bcs             *rpc.BlockChainService
	key             *ecdsa.PrivateKey
	db              *models.ModelDB
	quitChan        chan struct{}
	alarm           *blockchain.AlarmTask
	stopped         bool
	BlockNumberChan chan int64
	blockNumber     *atomic.Value
}

//NewChainEvents create chain events
func NewChainEvents(key *ecdsa.PrivateKey, client *helper.SafeEthClient, tokenNetworkRegistryAddress common.Address, db *models.ModelDB) *ChainEvents {
	log.Trace(fmt.Sprintf("tokenNetworkRegistryAddress %s", tokenNetworkRegistryAddress.String()))
	bcs, err := rpc.NewBlockChainService(key, tokenNetworkRegistryAddress, client)
	if err != nil {
		panic(err)
	}
	registry := bcs.Registry(tokenNetworkRegistryAddress, true)
	if registry == nil {
		panic("startup error : cannot get registry")
	}
	return &ChainEvents{
		client:          client,
		be:              blockchain.NewBlockChainEvents(client, bcs, nil),
		bcs:             bcs,
		key:             key,
		db:              db,
		quitChan:        make(chan struct{}),
		alarm:           blockchain.NewAlarmTask(client),
		BlockNumberChan: make(chan int64, 1),
		blockNumber:     new(atomic.Value),
	}
}

//Start moniter blockchain
func (ce *ChainEvents) Start() error {
	err := ce.alarm.Start()
	if err != nil {
		return err
	}
	go func() {
		for {
			select {
			case blockNumber := <-ce.alarm.LastBlockNumberChan:
				ce.db.SaveLatestBlockNumber(blockNumber)
				ce.setBlockNumber(blockNumber)
			}
		}
	}()
	err = ce.be.Start(ce.db.GetLatestBlockNumber())
	if err != nil {
		log.Error(fmt.Sprintf("blockchain events start err %s", err))
	}
	go ce.loop()
	return nil
}

//Stop service
func (ce *ChainEvents) Stop() {
	ce.alarm.Stop()
	ce.be.Stop()
	close(ce.quitChan)
}
func (ce *ChainEvents) setBlockNumber(blocknumber int64) {
	if ce.stopped {
		log.Info(fmt.Sprintf("new block number arrived %d,but has stopped", blocknumber))
		return
	}
	ce.BlockNumberChan <- blocknumber
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
		case n, ok := <-ce.BlockNumberChan:
			if !ok {
				log.Info("BlockNumberChan closed")
				return
			}
			ce.handleBlockNumber(n)
		case <-ce.quitChan:
			return
		}
	}
}

/*
一个通道关闭了,主要进行的就是设置
todo 限制锁的数量不能超过settle_timeout/2或者这里的 unlock 以及updatebalanceproof 用并行模式
1. updateBalanceProof以及/ unlock, 这些发生在 settle time out 的后半段
2. 设置 punish 发生在 settle time out 到期时间
*/
func (ce *ChainEvents) handleClosedStateChange(st2 *mediatedtransfer.ContractClosedStateChange) {
	log.Info(fmt.Sprintf("channel closed %s", st2.ChannelIdentifier.String()))
	ds, err := ce.db.DelegateGetByChannelIdentifier(st2.ChannelIdentifier)
	if err != nil {
		if err != storm.ErrNotFound {
			log.Error(fmt.Sprintf("DelegateGetByChannelIdentifier for ContractReceiveClosedStateChange err=%s st=%s",
				err, utils.StringInterface(st2, 2)))
			return
		}
	} else {
		tokenNetwork, err := ce.bcs.TokenNetwork(st2.TokenNetworkAddress)
		if err != nil {
			log.Error(fmt.Sprintf("recevied close event ,but toke network %s not exist", st2.TokenNetworkAddress.String()))
			return
		}
		_, settleBlockNumber, _, _, settleTimeout, err := tokenNetwork.GetContract().GetChannelInfoByChannelIdentifier(nil, st2.ChannelIdentifier)
		if err != nil {
			log.Error(fmt.Sprintf("channel %s get settle timeout err %s", st2.ChannelIdentifier.String(), err))
			return
		}
		for _, d := range ds {
			/*
				代理惩罚部分统一在通道 settle time out 以后进行,避免和真正的参与方发生冲突.
			*/
			d.SettleBlockNumber = int64(settleBlockNumber)
			if len(d.Content.Punishes) > 0 {
				err = ce.db.DelegateMonitorAdd(d.SettleBlockNumber, d.Key)
				ce.db.DelegateSave(d)
			}
			/*
				close 一方是不需要第三方代理来 UpdateBalanceProof 和 Withdraw 的,他自己会做.
			*/
			if d.Address == st2.ClosingAddress {
				//if the delegator closed this channel, dont need update
				d.Status = models.DelegateStatusSuccessFinishedByOther
				d.Error = "delegator closed channel"
				ce.db.DelegateSave(d)
				continue
			}
			//只有 punish, 没有 BalanceProof 的委托也是允许的
			if d.Content.UpdateTransfer.Nonce > 0 {
				/*
					代理只能在过settle time out 过半进行
				*/
				blockNumber := settleBlockNumber - settleTimeout/2 + 1
				err = ce.db.DelegateMonitorAdd(int64(blockNumber), d.Key)
				if err != nil {
					log.Error(fmt.Sprintf("DelegateMonitorAdd err %s", err))
				}

				log.Info(fmt.Sprintf("%s will updatedTransfer @ %d,closedBlock=%d", d.Key, blockNumber, st2.ClosedBlock))
			}
		}
	}
}

//如果发生了 balance proof update, 第三方服务就不用进行了,有可能是委托方自己做了
func (ce *ChainEvents) handleBalanceProofUpdatedStateChange(st2 *mediatedtransfer.ContractBalanceProofUpdatedStateChange) {
	log.Info(fmt.Sprintf("recevie transfer update %s %s", st2.ChannelIdentifier.String(), st2.Participant.String()))
	d := ce.db.DelegatetGet(st2.ChannelIdentifier, st2.Participant)
	if d.Content != nil && d.Status == models.DelegateStatusInit {
		//have recevied delegate
		d.Status = models.DelegateStatusSuccessFinishedByOther
		ce.db.DelegateSave(d)
	}
}

/*
通道settle
暂时先删除记录,后续根据需要再做修改
*/
func (ce *ChainEvents) handleSettledStateChange(st2 *mediatedtransfer.ContractSettledStateChange) {
	ds, err := ce.db.DelegateGetByChannelIdentifier(st2.ChannelIdentifier)
	if err != nil {
		log.Error(fmt.Sprintf("DelegateGetByChannelIdentifier err %s", err))
	}
	for _, d := range ds {
		err = ce.db.DelegateDeleteDelegate(d)
		if err != nil {
			log.Error(fmt.Sprintf("DelegateDeleteDelegate err %s", err))
		}
	}

}

/*
合作关闭通道,以前的委托都可以删掉了,因为是合作 settle,
说明没有纠纷,因此也不需要提交证明了,并且因为通道 open block number 改变,原来的委托肯定也作废了
*/
func (ce *ChainEvents) handleCooperativeSettledStateChange(st2 *mediatedtransfer.ContractCooperativeSettledStateChange) {
	ds, err := ce.db.DelegateGetByChannelIdentifier(st2.ChannelIdentifier)
	if err != nil {
		log.Error(fmt.Sprintf("DelegateGetByChannelIdentifier err %s", err))
	}
	for _, d := range ds {
		err = ce.db.DelegateDeleteDelegate(d)
		if err != nil {
			log.Error(fmt.Sprintf("DelegateDeleteDelegate err %s", err))
		}
	}
}

/*
合作 withdraw,以前的委托都可以删掉了,因为是合作 withdraw,
说明没有纠纷,因此也不需要提交证明了,并且因为通道 open block number 改变,原来的委托肯定也作废了
*/
func (ce *ChainEvents) handleWithdrawStateChange(st2 *mediatedtransfer.ContractChannelWithdrawStateChange) {
	ds, err := ce.db.DelegateGetByChannelIdentifier(st2.ChannelIdentifier.ChannelIdentifier)
	if err != nil {
		log.Error(fmt.Sprintf("DelegateGetByChannelIdentifier err %s", err))
	}
	for _, d := range ds {
		err = ce.db.DelegateDeleteDelegate(d)
		if err != nil {
			log.Error(fmt.Sprintf("DelegateDeleteDelegate err %s", err))
		}
	}
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
	}
}
func (ce *ChainEvents) handleBlockNumber(n int64) {
	ce.blockNumber.Store(n)
	keys, err := ce.db.DelegateMonitorGet(n)
	if err != nil {
		log.Error(fmt.Sprintf("DelegateMonitorGet err %s", err))
		return
	}
	if len(keys) > 0 {
		for _, key := range keys {
			d := ce.db.DelegatetGetByKey(key)
			//发生在通道可以 settle 时,说明是留给 punish的
			if d.SettleBlockNumber == n {
				go ce.doPunishes(d)
			} else {
				if d.Status == models.DelegateStatusSuccessFinishedByOther {
					log.Info(fmt.Sprintf("handle delegate ,but it's status=%d, delegate=%s", d.Status, utils.StringInterface(d, 4)))
					continue
				}
				if d.Status != models.DelegateStatusInit {
					log.Error(fmt.Sprintf("handle delegate error,it's status=%d,delegate=%s", d.Status, utils.StringInterface(d, 4)))
					continue
				}
				d.Status = models.DelegateStatusRunning
				err = ce.db.DelegateSetStatus(d.Status, d)
				if err != nil {
					log.Error(fmt.Sprintf("DelegateSetStatus  %s err %s", d.Key, err))
					continue
				}
				go ce.updateTransfer(d)
			}
		}
	}
}

/*
必须先扣费,后执行
updateTransfer 主要完成以下任务:
1. 调用 updateTransfer,
2. 调用 Withdraw
3. 计费
4. 将每个 tx 结果记录保存,供以后查询
todo 如何处理在执行过程中,程序要求退出,等待?计费?如何解决
*/
func (ce *ChainEvents) updateTransfer(d *models.Delegate) {
	needsmt := models.CalceNeedSmtForUpdateBalanceProofAndUnlock(d.Content)
	addr := d.Address
	d.TxBlockNumber = ce.GetBlockNumber()
	d.TxTime = time.Now()
	err := ce.db.AccountLockSmt(addr, needsmt)
	if err != nil {
		d.Status = models.DelegateStatusFailed
		d.Error = fmt.Sprintf("smt not enough,err=%s", err)
		ce.db.DelegateSave(d)
		return
	}
	err = ce.doUpdateTransfer(d)
	if err != nil {
		log.Error(fmt.Sprintf("doUpdateTransfer %s failed %s", d.Key, err))
		err2 := ce.db.AccountUnlockSmt(addr, needsmt)
		if err2 != nil {
			log.Error(fmt.Sprintf("AccountUnlockSmt err %s", err))
		}
		d.Content.UpdateTransfer.TxStatus = models.TxStatusExecueteErrorFinished
		d.Content.UpdateTransfer.TxError = err.Error()
		d.Status = models.DelegateStatusFailed
		d.Error = err.Error()
		ce.db.DelegateSave(d)
	} else {
		err = ce.db.AccountUseSmt(addr, params.SmtUpdateTransfer)
		if err != nil {
			log.Error(fmt.Sprintf("AccountUseSmt err %s", err))
		}
		d.Content.UpdateTransfer.TxStatus = models.TxStatusExecuteSuccessFinished
		ce.db.DelegateSave(d)
		hasErr := false
		for _, w := range d.Content.Unlocks {
			err := ce.doUnlock(w, d.PartnerAddress, d.Address, d.TokenNetworkAddress, common.BytesToHash(d.ChannelIdentifier), d.Content.UpdateTransfer.TransferAmount)
			if err != nil {
				log.Error(fmt.Sprintf("doUnlock %s %s err %s", d.Key, w.Secret, err))
				hasErr = true
				w.TxStatus = models.TxStatusExecueteErrorFinished
				w.TxError = err.Error()
				err2 := ce.db.AccountUnlockSmt(addr, params.SmtUnlock)
				if err2 != nil {
					log.Error(fmt.Sprintf("AccountUnlockSmt err %s", err))
				}
			} else {
				w.TxStatus = models.TxStatusExecuteSuccessFinished
				err2 := ce.db.AccountUseSmt(addr, params.SmtUnlock)
				if err2 != nil {
					log.Error(fmt.Sprintf("AccountUseSmt err %s", err))
				}
			}
		}
		if hasErr {
			d.Status = models.DelegateStatusPartialSuccess
		} else {
			d.Status = models.DelegateStatusSuccessFinished
		}
		ce.db.DelegateSave(d)
	}
}

/*
在可以代理惩罚的时间(closedBlockNumber+settleTimeout)到来时,
逐个执行惩罚委托,只要有一个成功,就可以立即结束,因为多余的惩罚也没有任何意义.
务必此时先扣费,后执行
如果委托方自己进行了 punish, 监控服务多做一遍也没什么成本,简化处理流程.
*/
func (ce *ChainEvents) doPunishes(d *models.Delegate) error {
	hasSuccess := false
	needsmt := models.CalceNeedSmtForPunish(d.Content)
	err := ce.db.AccountLockSmt(d.Address, needsmt)
	if err != nil {
		d.Status = models.DelegateStatusFailed
		d.Error = fmt.Sprintf("smt not enough,err=%s", err)
		ce.db.DelegateSave(d)
		return err
	}
	for _, p := range d.Content.Punishes {
		//只要有一个成功的,后续就不用惩罚了,再惩罚也没有意义了.
		if hasSuccess {
			p.TxStatus = models.TxStatusExecuteSuccessFinished
			continue
		}
		err := ce.doPunish(p, d)
		if err != nil {
			p.TxStatus = models.TxStatusExecueteErrorFinished
			p.TxError = err.Error()
			err2 := ce.db.AccountUnlockSmt(d.Address, params.SmtPunish)
			if err2 != nil {
				log.Error(fmt.Sprintf("account unlock smt error %s", err))
			}
		} else {
			hasSuccess = true
			p.TxStatus = models.TxStatusExecuteSuccessFinished
			err2 := ce.db.AccountUseSmt(d.Address, params.SmtPunish)
			if err2 != nil {
				log.Error(fmt.Sprintf("account use smt err %s", err))
			}
			break
		}
	}
	ce.db.DelegateSave(d)
	return nil
}
func (ce *ChainEvents) doUpdateTransfer(d *models.Delegate) error {
	channelAddr := common.BytesToHash(d.ChannelIdentifier)
	log.Info(fmt.Sprintf("UpdateTransfer %s called ,updateTransfer=%s",
		d.ChannelIdentifier, utils.StringInterface(d.Content.UpdateTransfer, 3)))
	tokenNetwork, err := ce.bcs.TokenNetwork(d.TokenNetworkAddress)
	if err != nil {
		return err
	}
	u := &d.Content.UpdateTransfer
	closingSignatur := u.ClosingSignature
	nonClosingSignature := u.NonClosingSignature
	log.Trace(fmt.Sprintf("signer=%s, updateTransfer=%s", utils.APex(ce.bcs.Auth.From), utils.StringInterface(&u, 4)))
	tx, err := tokenNetwork.GetContract().UpdateBalanceProofDelegate(ce.bcs.Auth, d.PartnerAddress, d.Address, u.TransferAmount, u.Locksroot, uint64(u.Nonce), u.ExtraHash, closingSignatur, nonClosingSignature)
	if err != nil {
		return err
	}
	d.Content.UpdateTransfer.TxHash = tx.Hash()
	txReceipt, err := bind.WaitMined(rpc.GetCallContext(), ce.bcs.Client, tx)
	if err != nil {
		return err
	}
	if txReceipt.Status != types.ReceiptStatusSuccessful {
		log.Info(fmt.Sprintf("updatetransfer failed %s,receipt=%s", utils.HPex(channelAddr), utils.StringInterface(txReceipt, 3)))
		return errors.New("tx execution failed")
	}
	log.Info(fmt.Sprintf("updatetransfer success %s ", utils.HPex(channelAddr)))

	return nil

}
func (ce *ChainEvents) doUnlock(w *models.Unlock, participant, partner, tokenNetworkAddress common.Address, channelAddr common.Hash, transferAmount *big.Int) error {
	log.Info(fmt.Sprintf("unlock %s on %s for %s", w.Secret, utils.HPex(channelAddr), utils.APex(participant)))
	tokenNetwork, err := ce.bcs.TokenNetwork(tokenNetworkAddress)
	if err != nil {
		return err
	}
	lock := w.Lock
	if lock.Expiration <= ce.GetBlockNumber() {
		return fmt.Errorf("lock has expired, expration=%d,currentBlockNumber=%d", lock.Expiration, ce.GetBlockNumber())
	}
	tx, err := tokenNetwork.GetContract().UnlockDelegate(ce.bcs.Auth, partner, participant, transferAmount, big.NewInt(lock.Expiration), lock.Amount, lock.LockSecretHash, w.MerkleProof, w.Signature)
	if err != nil {
		return fmt.Errorf("withdraw failed %s on channel %s,lock=%s", err, utils.HPex(channelAddr), utils.StringInterface(lock, 3))
	}
	w.TxHash = tx.Hash()
	txReceipt, err := bind.WaitMined(rpc.GetCallContext(), ce.bcs.Client, tx)
	if err != nil {
		return fmt.Errorf("%s WithDraw failed with error:%s", w.Secret, err)
	}
	if txReceipt.Status != types.ReceiptStatusSuccessful {
		return fmt.Errorf("withdraw failed %s,receipt=%s", utils.HPex(channelAddr), utils.StringInterface(txReceipt.Status, 3))
	}
	log.Info(fmt.Sprintf("withdraw success %s ", utils.HPex(channelAddr)))
	return nil
}

func (ce *ChainEvents) doPunish(p *models.Punish, d *models.Delegate) error {
	log.Info(fmt.Sprintf("punish %s on lockhash %s for channel %s", d.PartnerAddress, p.LockHash.String(), utils.BPex(d.ChannelIdentifier)))
	tokenNetwork, err := ce.bcs.TokenNetwork(d.TokenNetworkAddress)
	if err != nil {
		return err
	}
	tx, err := tokenNetwork.GetContract().PunishObsoleteUnlock(ce.bcs.Auth, d.Address, d.PartnerAddress, p.LockHash, p.AdditionalHash, p.Signature)
	if err != nil {
		return fmt.Errorf("punish faied %s,on channel %s,lockhash=%s", err, utils.BPex(d.ChannelIdentifier), p.LockHash.String())
	}
	p.TxHash = tx.Hash()
	txReceipt, err := bind.WaitMined(rpc.GetCallContext(), ce.bcs.Client, tx)
	if err != nil {
		return fmt.Errorf("%s punish failed with error:%s", p.LockHash.String(), err)
	}
	if txReceipt.Status != types.ReceiptStatusSuccessful {
		return fmt.Errorf("punish failed %s,receipt=%s", utils.BPex(d.ChannelIdentifier), utils.StringInterface(txReceipt.Status, 3))
	}
	log.Info(fmt.Sprintf("punish success %s", utils.HPex(p.LockHash)))
	return nil
}

//VerifyDelegate verify delegate from app is valid or not,should be thread safe
func (ce *ChainEvents) VerifyDelegate(c *models.ChannelFor3rd, delegater common.Address) error {
	partner := c.PartnerAddress
	if c.UpdateTransfer.Nonce > 0 {
		closingAddr, err := verifyClosingSignature(c)
		if err != nil {
			return err
		}
		if closingAddr != partner || closingAddr == delegater {
			return fmt.Errorf("participant error,closingAddr=%s,partner=%s,delegater=%s",
				utils.APex(closingAddr), utils.APex(partner),
				utils.APex(delegater))
		}
		if !verifyNonClosingSignature(c, delegater) {
			return errors.New("non closing error")
		}
		err = ce.verifyUnlocks(c, delegater)
		if err != nil {
			return err
		}
	} else {
		if len(c.Punishes) > 0 {
			err := ce.verifyPunishes(c)
			return err
		}
		return fmt.Errorf("no punishes and balance proof")
	}
	return nil
}
func (ce *ChainEvents) verifyUnlocks(c *models.ChannelFor3rd, delegater common.Address) error {
	for _, l := range c.Unlocks {
		if l.Lock == nil || l.Lock.Amount == nil {
			return fmt.Errorf("lock error %s", l.Lock)
		}
		signer, err := ce.verifyUnlockSignature(l, c)
		if err != nil {
			return err
		}
		if signer != delegater {
			return fmt.Errorf("unlock's signature error, signer=%s,delegater=%s", signer.String(), delegater.String())
		}
	}
	return nil
}
func (ce *ChainEvents) verifyPunishes(c *models.ChannelFor3rd) error {
	for _, p := range c.Punishes {
		signer, err := ce.verifyPunishSignature(p, c)
		if err != nil {
			return err
		}
		if signer != c.PartnerAddress {
			return fmt.Errorf("punish signature error,signer=%s,partner=%s", signer.String(), c.PartnerAddress.String())
		}
	}
	return nil
}
func (ce *ChainEvents) verifyUnlockSignature(u *models.Unlock, c *models.ChannelFor3rd) (signer common.Address, err error) {
	buf := new(bytes.Buffer)
	_, err = buf.Write(smparams.ContractSignaturePrefix)
	_, err = buf.Write([]byte(smparams.ContractUnlockDelegateProofMessageLength))
	_, err = buf.Write(utils.BigIntTo32Bytes(c.UpdateTransfer.TransferAmount))
	_, err = buf.Write(ce.bcs.NodeAddress[:])
	_, err = buf.Write(utils.BigIntTo32Bytes(big.NewInt(u.Lock.Expiration)))
	_, err = buf.Write(utils.BigIntTo32Bytes(u.Lock.Amount))
	_, err = buf.Write(u.Lock.LockSecretHash[:])
	_, err = buf.Write(c.ChannelIdentifier[:])
	err = binary.Write(buf, binary.BigEndian, c.OpenBlockNumber)
	_, err = buf.Write(utils.BigIntTo32Bytes(smparams.ChainID))
	if err != nil {
		log.Error(fmt.Sprintf("buf write error %s", err))
		return
	}
	hash := utils.Sha3(buf.Bytes())
	sig := u.Signature
	return utils.Ecrecover(hash, sig)
}
func (ce *ChainEvents) verifyPunishSignature(p *models.Punish, c *models.ChannelFor3rd) (signer common.Address, err error) {
	buf := new(bytes.Buffer)
	_, err = buf.Write(smparams.ContractSignaturePrefix)
	_, err = buf.Write([]byte(smparams.ContractDisposedProofMessageLength))
	_, err = buf.Write(p.LockHash[:])
	_, err = buf.Write(c.ChannelIdentifier[:])
	err = binary.Write(buf, binary.BigEndian, c.OpenBlockNumber)
	_, err = buf.Write(utils.BigIntTo32Bytes(smparams.ChainID))
	_, err = buf.Write(p.AdditionalHash[:])
	if err != nil {
		return
	}
	hash := utils.Sha3(buf.Bytes())
	return utils.Ecrecover(hash, p.Signature)
}
func verifyClosingSignature(c *models.ChannelFor3rd) (signer common.Address, err error) {
	u := c.UpdateTransfer
	buf := new(bytes.Buffer)
	if c.UpdateTransfer.Nonce <= 0 {
		err = fmt.Errorf("delegate with nonce <=0")
		return
	}
	_, err = buf.Write(smparams.ContractSignaturePrefix)
	_, err = buf.Write([]byte(smparams.ContractBalanceProofMessageLength))
	_, err = buf.Write(utils.BigIntTo32Bytes(u.TransferAmount))
	_, err = buf.Write(u.Locksroot[:])
	err = binary.Write(buf, binary.BigEndian, u.Nonce)
	_, err = buf.Write(u.ExtraHash[:])
	_, err = buf.Write(c.ChannelIdentifier[:])
	err = binary.Write(buf, binary.BigEndian, c.OpenBlockNumber)
	_, err = buf.Write(utils.BigIntTo32Bytes(smparams.ChainID))
	hash := utils.Sha3(buf.Bytes())
	sig := u.ClosingSignature
	return utils.Ecrecover(hash, sig)
}
func verifyNonClosingSignature(c *models.ChannelFor3rd, delegater common.Address) bool {
	var err error
	u := c.UpdateTransfer
	buf := new(bytes.Buffer)
	_, err = buf.Write(smparams.ContractSignaturePrefix)
	_, err = buf.Write([]byte(smparams.ContractBalanceProofDelegateMessageLength))
	_, err = buf.Write(utils.BigIntTo32Bytes(u.TransferAmount))
	_, err = buf.Write(u.Locksroot[:])
	err = binary.Write(buf, binary.BigEndian, u.Nonce)
	_, err = buf.Write(c.ChannelIdentifier[:])
	err = binary.Write(buf, binary.BigEndian, c.OpenBlockNumber)
	_, err = buf.Write(utils.BigIntTo32Bytes(smparams.ChainID))
	hash := utils.Sha3(buf.Bytes())
	sig := u.NonClosingSignature
	signer, err := utils.Ecrecover(hash, sig)
	return err == nil && signer == delegater
}

//GetBlockNumber return latest blocknumber of ethereum
func (ce *ChainEvents) GetBlockNumber() int64 {
	return ce.blockNumber.Load().(int64)
}
