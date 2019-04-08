package models

import (
	"bytes"
	"encoding/binary"
	"math/big"

	"github.com/SmartMeshFoundation/Photon/transfer/mtree"

	"fmt"

	"time"

	"github.com/SmartMeshFoundation/Photon-Monitoring/params"
	"github.com/SmartMeshFoundation/Photon/log"
	"github.com/SmartMeshFoundation/Photon/utils"
	"github.com/asdine/storm"
	"github.com/ethereum/go-ethereum/common"
)

/*
第三方服务应该保持长期在线,所以他提供的服务总是为最新的 channel,
所以 openblockNumber 并不关键,在一个 channel 被 settle 以后,他应该自动清除这个 channel 的所有信息.
*/
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
	//DelegateStatusCooperativeSettled this channel is cooperative settled
	DelegateStatusCooperativeSettled = 6
	//DelegateStatusWithdrawed this channel is withdrawed
	DelegateStatusWithdrawed = 7
	//TxStatusNotExecute Tx not start
	TxStatusNotExecute = 0
	//TxStatusExecuteSuccessFinished this tx success finished
	TxStatusExecuteSuccessFinished = 1
	//TxStatusExecueteErrorFinished this tx finished with error
	TxStatusExecueteErrorFinished = 2
)

//Delegate is from app's request and it's tx result
type Delegate struct {
	Key               []byte         `storm:"id"`
	Address           common.Address //delegator
	PartnerAddress    common.Address
	ChannelIdentifier []byte `storm:"index"` //委托 channel
	OpenBlockNumber   int64  // open block number of this channel
	SettleBlockNumber int64  // closed block number+settle_timeout
	TokenAddress      common.Address
	Time              time.Time //委托时间
	TxTime            time.Time //执行时间
	TxBlockNumber     int64     //执行开始块
	MinBlockNumber    int64     //Tx最早开始块
	MaxBlockNumber    int64     //Tx 最晚开始块
	Status            int       `storm:"index"`
	Error             string
	Content           *ChannelFor3rd
}

//RemovedDelegate represents a finished delegate, when a channel is settled? or withdrawed?
type RemovedDelegate struct {
	Key []byte `storm:"id"`
	D   Delegate
}

//UpdateTransfer arguments need to call contract updatetransfer
type UpdateTransfer struct {
	Nonce               int64       `json:"nonce"`
	TransferAmount      *big.Int    `json:"transfer_amount"`
	Locksroot           common.Hash `json:"locksroot"`
	ExtraHash           common.Hash `json:"extra_hash"`
	ClosingSignature    []byte      `json:"closing_signature"`
	NonClosingSignature []byte      `json:"non_closing_signature"`
	TxStatus            int
	TxError             string
	TxHash              common.Hash
}

//Unlock arguments need to call contract Withdraw
type Unlock struct {
	Lock        *mtree.Lock `json:"lock"`
	MerkleProof []byte      `json:"merkle_proof"`
	Secret      common.Hash `json:"secret"`
	Signature   []byte      `json:"signature"`
	TxStatus    int
	TxError     string
	TxHash      common.Hash
}

//ChannelFor3rd is for 3rd party to call update transfer
type ChannelFor3rd struct {
	ChannelIdentifier common.Hash    `json:"channel_identifier"`
	OpenBlockNumber   int64          `json:"open_block_number"`
	TokenAddress      common.Address `json:"token_address"`
	PartnerAddress    common.Address `json:"partner_address"`
	UpdateTransfer    UpdateTransfer `json:"update_transfer"`
	Unlocks           []*Unlock      `json:"unlocks"`
	Punishes          []*Punish      `json:"punishes"`
}

//Punish 需要委托给第三方的 punish证据
type Punish struct {
	LockHash       common.Hash `json:"lock_hash"` //the whole lock's hash,not lock secret hash
	AdditionalHash common.Hash `json:"additional_hash"`
	Signature      []byte      `json:"signature"`
	TxStatus       int
	TxError        string
	TxHash         common.Hash
}

//DelegateDeleteDelegate move delegate from bucket[Delegate] to bucket[RemovedDelegate]
func (model *ModelDB) DelegateDeleteDelegate(d *Delegate) error {
	rd := &RemovedDelegate{
		D: *d,
	}
	buf := new(bytes.Buffer)
	_, err := buf.Write(d.ChannelIdentifier)
	_, err = buf.Write(d.Address[:])
	err = binary.Write(buf, binary.LittleEndian, d.OpenBlockNumber)
	rd.Key = buf.Bytes()
	err = model.db.DeleteStruct(d)
	if err != nil {
		return err
	}
	err = model.db.Save(rd)
	return err
}

//将d中的punish合并到c中
func mergePunish(c *ChannelFor3rd, d *Delegate) {
	m := make(map[common.Hash]*Punish)
	for _, p := range c.Punishes {
		m[p.LockHash] = p
	}
	for _, p := range d.Content.Punishes {
		if m[p.LockHash] != nil {
			continue
		}
		c.Punishes = append(c.Punishes, p)
	}
}

//DelegateNewOrUpdateDelegate  accept a new delegate,error if the previous version of this delegate is running.
func (model *ModelDB) DelegateNewOrUpdateDelegate(c *ChannelFor3rd, addr common.Address) error {
	channelIdentifier := c.ChannelIdentifier
	var newsmt, oldsmt *big.Int
	if !model.delegateCanCreateOrUpdate(c, addr) {
		return fmt.Errorf("%s is running tx,cannot be replaced", model.delegateKey(c.ChannelIdentifier, addr))
	}
	d := model.DelegatetGet(c.ChannelIdentifier, addr)
	/*
		考虑到测试网上用户可能删除数据,从而导致nonce重新开始,因此允许旧的balanceProof覆盖新的
	*/
	if !params.DebugMode && d.Content != nil && d.Status == DelegateStatusInit && d.Content.UpdateTransfer.Nonce >= c.UpdateTransfer.Nonce {
		return fmt.Errorf("only delegate newer nonce ,old nonce=%d,new=%d", d.Content.UpdateTransfer.Nonce, c.UpdateTransfer.Nonce)
	}
	if d.Status != DelegateStatusInit {
		log.Warn(fmt.Sprintf("old delegate will be replaced, a channle was settled and re create? d=\n%s", utils.StringInterface(d, 4)))
	}
	newsmt = CalcNeedSmtForThisChannel(c)
	if d.Content != nil {
		mergePunish(c, d)
		oldsmt = CalcNeedSmtForThisChannel(d.Content)
	} else {
		oldsmt = big.NewInt(0)
	}
	d.Time = time.Now()
	d.Key = model.delegateKey(channelIdentifier, addr)
	d.ChannelIdentifier = channelIdentifier[:]
	d.OpenBlockNumber = c.OpenBlockNumber
	d.Address = addr
	d.Content = c
	d.TokenAddress = c.TokenAddress
	d.PartnerAddress = c.PartnerAddress

	model.lock.Lock()
	a := model.AccountGetAccount(addr)
	log.Trace(fmt.Sprintf("newsmt=%s,oldsmt=%s", newsmt, oldsmt))
	a.NeedSmt.Add(a.NeedSmt, newsmt)
	a.NeedSmt.Sub(a.NeedSmt, oldsmt)
	log.Trace(fmt.Sprintf("account=%s", a))
	err := model.db.Save(a)
	if err != nil {
		panic(fmt.Sprintf("db err %s", err))
	}
	model.lock.Unlock()
	err = model.db.Save(d)
	if err != nil {
		if err != nil {
			panic(fmt.Sprintf("db err %s", err))
		}
	}
	return nil
}

//DelegatetGet return the lastest delegate status
func (model *ModelDB) DelegatetGet(cAddr common.Hash, addr common.Address) *Delegate {
	return model.DelegatetGetByKey(model.delegateKey(cAddr, addr))
}

//DelegatetGetByKey return the lastest delegate status
func (model *ModelDB) DelegatetGetByKey(key []byte) *Delegate {
	var d Delegate
	err := model.db.One("Key", key, &d)
	if err == storm.ErrNotFound {
		return &d
	}
	if err != nil {
		panic(fmt.Sprintf("db err %s", err))
	}
	return &d
}
func (model *ModelDB) delegateCanCreateOrUpdate(c *ChannelFor3rd, addr common.Address) bool {
	var d Delegate
	err := model.db.One("Key", model.delegateKey(c.ChannelIdentifier, addr), &d)
	if err == storm.ErrNotFound {
		return true
	}
	if err != nil {
		panic(fmt.Sprintf("db err %s", err))
	}
	return d.Status != DelegateStatusRunning
}
func (model *ModelDB) delegateKey(cAddr common.Hash, addr common.Address) []byte {
	var key []byte
	key = append(key, cAddr[:]...)
	key = append(key, addr[:]...)
	return key
}

//MarkDelegateRunning mark this delegate is running ,deny new version
func (model *ModelDB) MarkDelegateRunning(cAddr common.Hash, addr common.Address) error {
	d := model.DelegatetGet(cAddr, addr)
	d.Status = DelegateStatusRunning
	return model.db.Save(d)
}

//DelegateSave call when finish a delegate
func (model *ModelDB) DelegateSave(d *Delegate) {
	err := model.db.Save(d)
	if err != nil {
		panic(err)
	}
}

//DelegateSetStatus change delegate status
func (model *ModelDB) DelegateSetStatus(status int, d *Delegate) error {
	return model.db.UpdateField(d, "Status", status)
}

/*
DelegateGetByChannelIdentifier returns the delegate about this channel and not run
*/
func (model *ModelDB) DelegateGetByChannelIdentifier(channelIdentifier common.Hash) (ds []*Delegate, err error) {
	err = model.db.Find("ChannelIdentifier", channelIdentifier[:], &ds)
	if err == storm.ErrNotFound {
		err = nil
	}
	return
}

//CalcNeedSmtForThisChannel returns how much smt need to run this tx
func CalcNeedSmtForThisChannel(c *ChannelFor3rd) *big.Int {
	n := new(big.Int)
	if c.UpdateTransfer.Nonce > 0 {
		n = n.Add(n, params.SmtUpdateTransfer)
	}
	for range c.Unlocks {
		n = n.Add(n, params.SmtUnlock)
	}
	//惩罚只需成功一次,即可.
	if len(c.Punishes) > 0 {
		n = n.Add(n, params.SmtPunish)
	}
	return n
}

//CalceNeedSmtForUpdateBalanceProofAndUnlock returns how much smt need to update balance proof and unlock
func CalceNeedSmtForUpdateBalanceProofAndUnlock(c *ChannelFor3rd) *big.Int {
	n := new(big.Int)
	if c.UpdateTransfer.Nonce > 0 {
		n = n.Add(n, params.SmtUpdateTransfer)
	}
	for range c.Unlocks {
		n = n.Add(n, params.SmtUnlock)
	}
	return n
}

//CalceNeedSmtForPunish returns how much smt need to punish
func CalceNeedSmtForPunish(c *ChannelFor3rd) *big.Int {
	n := new(big.Int)
	if len(c.Punishes) > 0 {
		n = n.Add(n, params.SmtPunish)
	}
	return n
}
