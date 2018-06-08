package chainservice

import (
	"crypto/ecdsa"
	"errors"

	"fmt"

	"sync/atomic"

	"time"

	"bytes"

	"encoding/binary"

	"github.com/SmartMeshFoundation/SmartRaiden-Monitoring/models"
	"github.com/SmartMeshFoundation/SmartRaiden-Monitoring/params"
	"github.com/SmartMeshFoundation/SmartRaiden/blockchain"
	"github.com/SmartMeshFoundation/SmartRaiden/encoding"
	"github.com/SmartMeshFoundation/SmartRaiden/log"
	"github.com/SmartMeshFoundation/SmartRaiden/network/helper"
	"github.com/SmartMeshFoundation/SmartRaiden/network/rpc"
	"github.com/SmartMeshFoundation/SmartRaiden/transfer"
	"github.com/SmartMeshFoundation/SmartRaiden/transfer/mediatedtransfer"
	"github.com/SmartMeshFoundation/SmartRaiden/utils"
	"github.com/asdine/storm"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
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
func NewChainEvents(key *ecdsa.PrivateKey, client *helper.SafeEthClient, registryAddress common.Address, db *models.ModelDB) *ChainEvents {
	return &ChainEvents{
		client:          client,
		be:              blockchain.NewBlockChainEvents(client, registryAddress),
		bcs:             rpc.NewBlockChainService(key, registryAddress, client),
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
	ce.alarm.Start()
	ce.alarm.RegisterCallback(func(number int64) error {
		ce.db.SaveLatestBlockNumber(number)
		ce.setBlockNumber(number)
		return nil
	})
	err := ce.be.Start(ce.db.GetLatestBlockNumber())
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

func (ce *ChainEvents) handleStateChange(st transfer.StateChange) {
	switch st2 := st.(type) {
	case *mediatedtransfer.ContractReceiveClosedStateChange:
		log.Info(fmt.Sprintf("channel closed %s", st2.ChannelAddress.String()))
		ds, err := ce.db.DelegateGetByChannelAddress(st2.ChannelAddress)
		if err != nil {
			if err != storm.ErrNotFound {
				log.Error(fmt.Sprintf("DelegateGetByChannelAddress for ContractReceiveClosedStateChange err=%s st=%s",
					err, utils.StringInterface(st, 2)))
				return
			}
		} else {
			for _, d := range ds {
				ch, err := ce.bcs.NettingChannel(st2.ChannelAddress)
				if err != nil {
					log.Error(fmt.Sprintf("recevied close event ,but channel %s not exist", st2.ChannelAddress.String()))
					continue
				}
				n, err := ch.SettleTimeout()
				if err != nil {
					log.Error(fmt.Sprintf("channel %s get settle timeout err %s", st2.ChannelAddress.String(), err))
					continue
				}
				closedBlock, err := ch.Closed()
				if err != nil || closedBlock == 0 {
					log.Error(fmt.Sprintf("get closed err %s,closedblock=%d,channel=%s", err, closedBlock, st2.ChannelAddress.String()))
					continue
				}
				blockNumber := int64(n/2) + 2 + closedBlock
				err = ce.db.DelegateMonitorAdd(blockNumber, d.Key)
				if err != nil {
					log.Error(fmt.Sprintf("DelegateMonitorAdd err %s", err))
				}
				log.Info(fmt.Sprintf("%s will updatedTransfer @ %d,closedBlock=%d", d.Key, blockNumber, closedBlock))
			}
		}
		//处理 channel 关闭事件
	case *mediatedtransfer.ContractTransferUpdatedStateChange:
		//处理TransferUpdate事件
		log.Info(fmt.Sprintf("recevie transfer update %s %s", st2.ChannelAddress.String(), st2.Participant.String()))
		d := ce.db.DelegatetGet(st2.ChannelAddress.String(), st2.Participant)
		if d.Content != nil && d.Status == models.DelegateStatusInit {
			//have recevied delegate
			d.Status = models.DelegateStatusSuccessFinishedByOther
			ce.db.DelegateSave(d)
		}
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

/*
updateTransfer 主要完成以下任务:
1. 调用 updateTransfer,
2. 调用 Withdraw
3. 计费
4. 将每个 tx 结果记录保存,供以后查询
todo 如何处理在执行过程中,程序要求退出,等待?计费?如何解决
*/
func (ce *ChainEvents) updateTransfer(d *models.Delegate) {
	needsmt := models.CalcNeedSmtForThisChannel(d.Content)
	addr := common.HexToAddress(d.Address)
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
		err = ce.db.AccountUseSmt(addr, params.SmtUpdatTransfer)
		if err != nil {
			log.Error(fmt.Sprintf("AccountUseSmt err %s", err))
		}
		d.Content.UpdateTransfer.TxStatus = models.TxStatusExecuteSuccessFinished
		ce.db.DelegateSave(d)
		hasErr := false
		for _, w := range d.Content.Withdraws {
			err := ce.dowithdraw(w, addr, common.HexToAddress(d.ChannelAddress))
			if err != nil {
				log.Error(fmt.Sprintf("dowithdraw %s %s err %s", d.Key, w.Secret, err))
				hasErr = true
				w.TxStatus = models.TxStatusExecueteErrorFinished
				w.TxError = err.Error()
				err2 := ce.db.AccountUnlockSmt(addr, params.SmtWithdraw)
				if err2 != nil {
					log.Error(fmt.Sprintf("AccountUnlockSmt err %s", err))
				}
			} else {
				w.TxStatus = models.TxStatusExecuteSuccessFinished
				err2 := ce.db.AccountUseSmt(addr, params.SmtWithdraw)
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
func (ce *ChainEvents) doUpdateTransfer(d *models.Delegate) error {
	channelAddr := common.HexToAddress(d.ChannelAddress)
	log.Info(fmt.Sprintf("UpdateTransfer %s called ,updateTransfer=%s",
		d.ChannelAddress, utils.StringInterface(d.Content.UpdateTransfer, 3)))
	ch, err := ce.bcs.NettingChannel(channelAddr)
	if err != nil {
		return err
	}
	u := &d.Content.UpdateTransfer
	closingSignatur := common.Hex2Bytes(u.ClosingSignature)
	nonClosingSignature := common.Hex2Bytes(u.NonClosingSignature)
	log.Trace(fmt.Sprintf("signer=%s, updateTransfer=%s", utils.APex(ce.bcs.Auth.From), utils.StringInterface(&u, 4)))
	tx, err := ch.GetContract().UpdateTransferDelegate(ce.bcs.Auth, uint64(u.Nonce), u.TransferAmount, common.HexToHash(u.Locksroot),
		common.HexToHash(u.ExtraHash), closingSignatur, nonClosingSignature)
	if err != nil {
		return err
	}
	d.Content.UpdateTransfer.TxHash = tx.Hash()
	receipt, err := bind.WaitMined(rpc.GetCallContext(), ce.bcs.Client, tx)
	if err != nil {
		return err
	}
	if receipt.Status != types.ReceiptStatusSuccessful {
		log.Info(fmt.Sprintf("updatetransfer failed %s,receipt=%s", utils.APex(channelAddr), receipt))
		return errors.New("tx execution failed")
	}
	log.Info(fmt.Sprintf("updatetransfer success %s ", utils.APex(channelAddr)))

	return nil

}
func (ce *ChainEvents) dowithdraw(w *models.Withdraw, participant, channelAddr common.Address) error {
	log.Info(fmt.Sprintf("withdraw %s on %s for %s", w.Secret, utils.APex(channelAddr), utils.APex(participant)))
	ch, err := ce.bcs.NettingChannel(channelAddr)
	if err != nil {
		return err
	}
	lockEncoded := common.Hex2Bytes(w.LockedEncoded)
	lock := new(encoding.Lock)
	lock.FromBytes(lockEncoded)
	if lock.Expiration <= ce.GetBlockNumber() {
		return fmt.Errorf("lock has expired, expration=%d,currentBlockNumber=%d", lock.Expiration, ce.GetBlockNumber())
	}
	tx, err := ch.GetContract().Withdraw(ce.bcs.Auth, participant, lockEncoded, common.Hex2Bytes(w.MerkleProof), common.HexToHash(w.Secret))
	if err != nil {
		return fmt.Errorf("withdraw failed %s on channel %s,lock=%s", err, utils.APex2(channelAddr), utils.StringInterface(lock, 3))
	}
	w.TxHash = tx.Hash()
	receipt, err := bind.WaitMined(rpc.GetCallContext(), ce.bcs.Client, tx)
	if err != nil {
		return fmt.Errorf("%s WithDraw failed with error:%s", w.Secret, err)
	}
	if receipt.Status != types.ReceiptStatusSuccessful {
		return fmt.Errorf("withdraw failed %s,receipt=%s", utils.APex2(channelAddr), receipt)
	}
	log.Info(fmt.Sprintf("withdraw success %s ", utils.APex2(channelAddr)))
	return nil
}

//VerifyDelegate verify delegate from app is valid or not,should be thread safe
func (ce *ChainEvents) VerifyDelegate(c *models.ChannelFor3rd, delegater common.Address) error {
	chAddr := common.HexToAddress(c.ChannelAddress)
	ch, err := ce.bcs.NettingChannel(chAddr)
	if err != nil {
		return err
	}
	participant1, _, participant2, _, err := ch.AddressAndBalance()
	if err != nil {
		return err
	}
	if delegater != participant1 && delegater != participant2 {
		return errors.New("not a participant")
	}
	closingAddr, err := recoverClosingSignature(c)
	if err != nil {
		return err
	}
	if (closingAddr != participant2 && closingAddr != participant1) || closingAddr == delegater {
		return errors.New("participant error")
	}
	if !recoverNonClosingSignature(c, delegater) {
		return errors.New("non closing error")
	}
	return nil
}

func recoverClosingSignature(c *models.ChannelFor3rd) (signer common.Address, err error) {
	var h common.Hash
	chAddr := common.HexToAddress(c.ChannelAddress)
	u := c.UpdateTransfer
	buf := new(bytes.Buffer)
	err = binary.Write(buf, binary.BigEndian, u.Nonce)
	_, err = buf.Write(utils.BigIntTo32Bytes(u.TransferAmount))
	h = common.HexToHash(u.Locksroot)
	_, err = buf.Write(h[:])
	_, err = buf.Write(chAddr[:])
	h = common.HexToHash(u.ExtraHash)
	_, err = buf.Write(h[:])
	hash := utils.Sha3(buf.Bytes())
	sig := common.Hex2Bytes(u.ClosingSignature)
	if len(sig) != 65 {
		err = errors.New("signature length error")
		return
	}
	sig[len(sig)-1] -= 27
	pubkey, err := crypto.Ecrecover(hash[:], sig)
	if err != nil {
		return
	}
	signer = utils.PubkeyToAddress(pubkey)
	return
}
func recoverNonClosingSignature(c *models.ChannelFor3rd, delegater common.Address) bool {
	var h common.Hash
	var err error
	chAddr := common.HexToAddress(c.ChannelAddress)
	u := c.UpdateTransfer
	buf := new(bytes.Buffer)
	err = binary.Write(buf, binary.BigEndian, u.Nonce)
	_, err = buf.Write(utils.BigIntTo32Bytes(u.TransferAmount))
	h = common.HexToHash(u.Locksroot)
	_, err = buf.Write(h[:])
	_, err = buf.Write(chAddr[:])
	h = common.HexToHash(u.ExtraHash)
	_, err = buf.Write(h[:])
	_, err = buf.Write(common.Hex2Bytes(u.ClosingSignature))
	_, err = buf.Write(params.Address[:])
	hash := utils.Sha3(buf.Bytes())
	sig := common.Hex2Bytes(u.NonClosingSignature)
	if len(sig) != 65 {
		log.Error("signature length error")
		return false
	}
	sig[len(sig)-1] -= 27
	pubkey, err := crypto.Ecrecover(hash[:], sig)
	if err != nil {
		log.Error(fmt.Sprintf("Ecrecover err %s", err))
		return false
	}
	return delegater == utils.PubkeyToAddress(pubkey)
}

//GetBlockNumber return latest blocknumber of ethereum
func (ce *ChainEvents) GetBlockNumber() int64 {
	return ce.blockNumber.Load().(int64)
}
