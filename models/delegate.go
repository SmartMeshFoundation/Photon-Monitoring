package models

import (
	"bytes"
	"encoding/gob"
	"math/big"

	"github.com/SmartMeshFoundation/Photon-Monitoring/params"

	"github.com/SmartMeshFoundation/Photon-Monitoring/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/jinzhu/gorm"
)

// DelegateStatus :
type DelegateStatus int

/*
第三方服务应该保持长期在线,所以他提供的服务总是为最新的 channel,
所以 openblockNumber 并不关键,在一个 channel 被 settle 以后,他应该自动清除这个 channel 的所有信息.
*/
const (
	//DelegateStatusInit init
	DelegateStatusInit = iota
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
)

// Delegate 一次photon委托的信息
type Delegate struct {
	Key                        []byte         `json:"key" gorm:"primary_key"`          // append(channelIdentifier,address)
	ChannelIdentifierStr       string         `json:"channel_identifier" gorm:"index"` //委托 channel
	OpenBlockNumber            int64          `json:"open_block_number"`               // open block number of this channel
	TokenAddressStr            string         `json:"token_address"`
	DelegatorAddressStr        string         `json:"delegator_address"` //delegator
	PartnerAddressStr          string         `json:"partner_address"`
	SettleBlockNumber          int64          `json:"settle_block_number"`   // closed block number+settle_timeout
	DelegateTimestamp          int64          `json:"delegate_timestamp"`    //委托时间
	DelegateBlockNumber        int64          `json:"delegate_block_number"` // 委托块
	Status                     DelegateStatus `json:"status" gorm:"index"`
	Error                      string         `json:"error"`
	NeedSMTStr                 string         `json:"need_smt_str"`
	UpdateBalanceProofGobBytes []byte         `json:"-"`
	UnlocksGobBytes            []byte         `json:"-"`
}

// ChannelIdentifier getter
func (d *Delegate) ChannelIdentifier() common.Hash {
	return common.HexToHash(d.ChannelIdentifierStr)
}

// TokenAddress getter
func (d *Delegate) TokenAddress() common.Address {
	return common.HexToAddress(d.TokenAddressStr)
}

// DelegatorAddress getter
func (d *Delegate) DelegatorAddress() common.Address {
	return common.HexToAddress(d.DelegatorAddressStr)
}

// PartnerAddress getter
func (d *Delegate) PartnerAddress() common.Address {
	return common.HexToAddress(d.PartnerAddressStr)
}

// SetUpdateBalanceProof setter
func (d *Delegate) SetUpdateBalanceProof(dubp *DelegateUpdateBalanceProof) {
	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)
	err := encoder.Encode(dubp)
	if err != nil {
		panic(err)
	}
	d.UpdateBalanceProofGobBytes = buf.Bytes()
}

// UpdateBalanceProof getter
func (d *Delegate) UpdateBalanceProof() *DelegateUpdateBalanceProof {
	decoder := gob.NewDecoder(bytes.NewBuffer(d.UpdateBalanceProofGobBytes))
	dubp := &DelegateUpdateBalanceProof{}
	err := decoder.Decode(dubp)
	if err != nil {
		panic(err)
	}
	return dubp
}

// SetUnlocks setter
func (d *Delegate) SetUnlocks(unlocks []*DelegateUnlock) {
	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)
	err := encoder.Encode(&unlocks)
	if err != nil {
		panic(err)
	}
	d.UnlocksGobBytes = buf.Bytes()
}

// Unlocks getter
func (d *Delegate) Unlocks() []*DelegateUnlock {
	decoder := gob.NewDecoder(bytes.NewBuffer(d.UnlocksGobBytes))
	var das []*DelegateUnlock
	err := decoder.Decode(&das)
	if err != nil {
		panic(err)
	}
	return das
}

// NeedSMT getter
func (d *Delegate) NeedSMT() *big.Int {
	return utils.StringToBigInt(d.NeedSMTStr)
}

// CalcNeedSMT 计算一次委托需要的总花费
func (d *Delegate) CalcNeedSMT(smt4Punish *big.Int) {
	needSMT := big.NewInt(0)
	// 计算一次
	if d.UpdateBalanceProof().Nonce > 0 {
		needSMT = needSMT.Add(needSMT, params.SmtUpdateTransfer)
	}
	// 按数量
	smt4Unlock := new(big.Int).Mul(big.NewInt(int64(len(d.Unlocks()))), params.SmtUnlock)
	needSMT = needSMT.Add(needSMT, smt4Unlock)

	// 直接add参数
	needSMT = needSMT.Add(needSMT, smt4Punish)
	d.NeedSMTStr = utils.BigIntToString(needSMT)
}

/*
dao
*/

// GetDelegateByKey  query by primary_key
func (model *ModelDB) GetDelegateByKey(key []byte) (d *Delegate, err error) {
	d = &Delegate{}
	err = model.db.Where(&Delegate{
		Key: key,
	}).First(d).Error
	return
}

/*
GetDelegateListByChannelIdentifier returns the delegate about this channel and not run
*/
func (model *ModelDB) GetDelegateListByChannelIdentifier(channelIdentifier common.Hash) (ds []*Delegate, err error) {
	err = model.db.Where(&Delegate{
		ChannelIdentifierStr: channelIdentifier.String(),
	}).Find(&ds).Error
	if err == gorm.ErrRecordNotFound {
		err = nil
	}
	return
}

//UpdateDelegateStatus change delegate status
func (model *ModelDB) UpdateDelegateStatus(d *Delegate, status DelegateStatus) error {
	d.Status = status
	return model.db.Model(d).UpdateColumn("Status", status).Error
}

// DeleteDelegate delete all records about a delegate
func (model *ModelDB) DeleteDelegate(key []byte) (err error) {
	tx := model.db.Begin()
	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()
	err = tx.Delete(&Delegate{
		Key: key,
	}).Error
	err = tx.Delete(&DelegatePunish{
		DelegateKey: key,
	}).Error
	err = tx.Delete(&DelegateAnnounceDispose{
		DelegateKey: key,
	}).Error
	return
}

// BuildDelegateKey 根据channelIdentifier及delegator地址构造key
func BuildDelegateKey(cAddr common.Hash, delegator common.Address) []byte {
	var key []byte
	key = append(key, cAddr[:]...)
	key = append(key, delegator[:]...)
	return key
}

// markDelegateRunning for test
func (model *ModelDB) markDelegateRunning(cAddr common.Hash, addr common.Address) (err error) {
	key := BuildDelegateKey(cAddr, addr)
	err = model.db.Model(&Delegate{
		Key: key,
	}).UpdateColumn("Status", DelegateStatusRunning).Error
	return
}

// getDelegateByOriginKey for test
func (model *ModelDB) getDelegateByOriginKey(cAddr common.Hash, addr common.Address) (d *Delegate) {
	d, err := model.GetDelegateByKey(BuildDelegateKey(cAddr, addr))
	if err != nil {
		panic(err)
	}
	return
}
