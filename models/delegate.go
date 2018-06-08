package models

import (
	"math/big"

	"fmt"

	"time"

	"github.com/SmartMeshFoundation/SmartRaiden-Monitoring/params"
	"github.com/SmartMeshFoundation/SmartRaiden/log"
	"github.com/SmartMeshFoundation/SmartRaiden/utils"
	"github.com/asdine/storm"
	"github.com/ethereum/go-ethereum/common"
)

const (
	//DelegateStatusInit init
	DelegateStatusInit = 0
	//DelegateStatusRunning is running
	DelegateStatusRunning = 1
	//DelegateStatusSuccessFinished all success finished
	DelegateStatusSuccessFinished = 2
	//DelegateStatusPartialSuccess only partial status
	DelegateStatusPartialSuccess = 3
	//DelegateStatusSuccessFinishedByOther not me call update transfer
	DelegateStatusSuccessFinishedByOther = 4
	//DelegateStatusFailed fail call tx,not engough smt,etc.
	DelegateStatusFailed = 5
	//TxStatusNotExecute Tx not start
	TxStatusNotExecute = 0
	//TxStatusExecuteSuccessFinished this tx success finished
	TxStatusExecuteSuccessFinished = 1
	//TxStatusExecueteErrorFinished this tx finished with error
	TxStatusExecueteErrorFinished = 2
)

//Delegate is from app's request and it's tx result
type Delegate struct {
	Key            string    `storm:"id"`
	Address        string    //delegator
	ChannelAddress string    `storm:"index"` //委托 channel
	Time           time.Time //委托时间
	TxTime         time.Time //执行时间
	TxBlockNumber  int64     //执行开始块
	MinBlockNumber int64     //Tx最早开始块
	MaxBlockNumber int64     //Tx 最晚开始块
	Status         int       `storm:"index"`
	Error          string
	Content        *ChannelFor3rd
}

//UpdateTransfer arguments need to call contract updatetransfer
type UpdateTransfer struct {
	Nonce               int64    `json:"nonce"`
	TransferAmount      *big.Int `json:"transfer_amount"`
	Locksroot           string   `json:"locksroot"`
	ExtraHash           string   `json:"extra_hash"`
	ClosingSignature    string   `json:"closing_signature"`
	NonClosingSignature string   `json:"non_closing_signature"`
	TxStatus            int
	TxError             string
	TxHash              common.Hash
}

//Withdraw arguments need to call contract Withdraw
type Withdraw struct {
	LockedEncoded string `json:"locked_encoded"`
	MerkleProof   string `json:"merkle_proof"`
	Secret        string `json:"secret"`
	TxStatus      int
	TxError       string
	TxHash        common.Hash
}

//ChannelFor3rd is for 3rd party to call update transfer
type ChannelFor3rd struct {
	ChannelAddress string         `json:"channel_address"`
	UpdateTransfer UpdateTransfer `json:"update_transfer"`
	Withdraws      []*Withdraw    `json:"withdraws"`
}

//DelegateNewDelegate  accept a new delegate,error if the previous version of this delegate is running.
func (m *ModelDB) DelegateNewDelegate(c *ChannelFor3rd, addr common.Address) error {
	var newsmt, oldsmt *big.Int
	if !m.delegateCanCreateOrUpdate(c, addr) {
		return fmt.Errorf("%s is running tx,cannot be replaced", m.delegateKey(c.ChannelAddress, addr))
	}
	d := m.DelegatetGet(c.ChannelAddress, addr)
	if d.Content != nil && d.Status == DelegateStatusInit && d.Content.UpdateTransfer.Nonce >= c.UpdateTransfer.Nonce {
		return fmt.Errorf("only delegate newer nonce ,old nonce=%d,new=%d", d.Content.UpdateTransfer.Nonce, c.UpdateTransfer.Nonce)
	}
	if d.Status != DelegateStatusInit {
		log.Warn(fmt.Sprintf("old delegate will be replaced, a channle was settled and re create? d=\n%s", utils.StringInterface(d, 4)))
	}
	newsmt = CalcNeedSmtForThisChannel(c)
	if d.Content != nil {
		oldsmt = CalcNeedSmtForThisChannel(d.Content)
	} else {
		oldsmt = big.NewInt(0)
	}
	d.Time = time.Now()
	d.Key = fmt.Sprintf("%s-%s", c.ChannelAddress, addr.String())
	d.ChannelAddress = c.ChannelAddress
	d.Address = addr.String()
	d.Content = c

	m.lock.Lock()
	a := m.AccountGetAccount(addr)
	log.Trace(fmt.Sprintf("newsmt=%s,oldsmt=%s", newsmt, oldsmt))
	a.NeedSmt.Add(a.NeedSmt, newsmt)
	a.NeedSmt.Sub(a.NeedSmt, oldsmt)
	log.Trace(fmt.Sprintf("account=%s", a))
	err := m.db.Save(a)
	if err != nil {
		panic(fmt.Sprintf("db err %s", err))
	}
	m.lock.Unlock()
	err = m.db.Save(d)
	if err != nil {
		if err != nil {
			panic(fmt.Sprintf("db err %s", err))
		}
	}
	return nil
}

//DelegatetGet return the lastest delegate status
func (m *ModelDB) DelegatetGet(cAddr string, addr common.Address) *Delegate {
	return m.DelegatetGetByKey(m.delegateKey(cAddr, addr))
}

//DelegatetGetByKey return the lastest delegate status
func (m *ModelDB) DelegatetGetByKey(key string) *Delegate {
	var d Delegate
	err := m.db.One("Key", key, &d)
	if err == storm.ErrNotFound {
		return &d
	}
	if err != nil {
		panic(fmt.Sprintf("db err %s", err))
	}
	return &d
}
func (m *ModelDB) delegateCanCreateOrUpdate(c *ChannelFor3rd, addr common.Address) bool {
	var d Delegate
	err := m.db.One("Key", m.delegateKey(c.ChannelAddress, addr), &d)
	if err == storm.ErrNotFound {
		return true
	}
	if err != nil {
		panic(fmt.Sprintf("db err %s", err))
	}
	return d.Status != DelegateStatusRunning
}
func (m *ModelDB) delegateKey(cAddr string, addr common.Address) string {
	return fmt.Sprintf("%s-%s", cAddr, addr.String())
}

//MarkDelegateRunning mark this delegate is running ,deny new version
func (m *ModelDB) MarkDelegateRunning(cAddr string, addr common.Address) error {
	d := m.DelegatetGet(cAddr, addr)
	d.Status = DelegateStatusRunning
	return m.db.Save(d)
}

//DelegateSave call when finish a delegate
func (m *ModelDB) DelegateSave(d *Delegate) {
	err := m.db.Save(d)
	if err != nil {
		panic(err)
	}
}

//DelegateSetStatus change delegate status
func (m *ModelDB) DelegateSetStatus(status int, d *Delegate) error {
	return m.db.UpdateField(d, "Status", status)
}

/*
DelegateGetByChannelAddress returns the delegate about this channel and not run
*/
func (m *ModelDB) DelegateGetByChannelAddress(ch common.Address) (ds []*Delegate, err error) {
	err = m.db.Find("ChannelAddress", ch.String(), &ds)
	return
}

//CalcNeedSmtForThisChannel returns how much smt need to run this tx
func CalcNeedSmtForThisChannel(c *ChannelFor3rd) *big.Int {
	n := new(big.Int)
	if c.UpdateTransfer.Nonce > 0 {
		n = n.Add(n, params.SmtUpdatTransfer)
	}
	for range c.Withdraws {
		n = n.Add(n, params.SmtWithdraw)
	}
	return n
}
