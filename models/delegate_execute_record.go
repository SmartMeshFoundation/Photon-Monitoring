package models

import (
	"bytes"
	"encoding/gob"
	"time"

	"github.com/SmartMeshFoundation/Photon/utils"
	"github.com/ethereum/go-ethereum/common"
)

// ExecuteStatus :
type ExecuteStatus int

const (
	//ExecuteStatusNotExecute Tx not start
	ExecuteStatusNotExecute = iota
	//ExecuteStatusSuccessFinished this tx success finished
	ExecuteStatusSuccessFinished
	//ExecuteStatusErrorFinished this tx finished with error
	ExecuteStatusErrorFinished
)

// DelegateType :
type DelegateType int

// #nosec
const (
	DelegateTypeUpdateBalanceProof = iota
	DelegateTypeUnlock
	DelegateTypePunish
)

/*
DelegateExecuteRecord 保存委托的合约调用执行情况相关信息
*/
type DelegateExecuteRecord struct {
	Key                  string        `json:"key" gorm:"primary_key"` // 主键,随机生成
	ChannelIdentifierStr string        `json:"channel_identifier"`
	OpenBlockNumber      int64         `json:"open_block_number"`
	DelegatorStr         string        `json:"delegator"`
	Type                 DelegateType  `json:"type"`
	Status               ExecuteStatus `json:"status"`
	Error                string        `json:"error"`
	ExecuteTimestamp     int64         `json:"execute_timestamp"` // 执行时间
	TxHashStr            string        `json:"tx_hash_str"`
	TxCreateBlockNumber  int64         `json:"tx_create_block_number"` //生成高度
	TxCreateTimestamp    int64         `json:"tx_create_timestamp"`    // 生成时间
	TxPackBlockNumber    int64         `json:"tx_pack_block_number"`   // 打包高度
	TxPackTimestamp      int64         `json:"tx_pack_timestamp"`      // 打包时间
	GobParams            []byte        `json:"params"`                 // 相关参数,gob编码,根据类型不同对应DelegateUpdateBalanceProof,DelegateUnlock,DelegatePunish三个结构体
}

// ChannelIdentifier :
func (r *DelegateExecuteRecord) ChannelIdentifier() common.Hash {
	return common.HexToHash(r.ChannelIdentifierStr)
}

// Delegator :
func (r *DelegateExecuteRecord) Delegator() common.Address {
	return common.HexToAddress(r.DelegatorStr)
}

// TxHash :
func (r *DelegateExecuteRecord) TxHash() common.Hash {
	return common.HexToHash(r.TxHashStr)
}

// NewDelegateExecuteRecord :
func NewDelegateExecuteRecord(d *Delegate, executeType DelegateType, params interface{}) *DelegateExecuteRecord {
	var buf bytes.Buffer
	e := gob.NewEncoder(&buf)
	err := e.Encode(params)
	if err != nil {
		panic(err)
	}
	return &DelegateExecuteRecord{
		Key:                  utils.NewRandomAddress().String(),
		ChannelIdentifierStr: d.ChannelIdentifierStr,
		OpenBlockNumber:      d.OpenBlockNumber,
		DelegatorStr:         d.DelegatorAddressStr,
		Type:                 executeType,
		Status:               ExecuteStatusNotExecute,
		Error:                "",
		ExecuteTimestamp:     time.Now().Unix(), // 执行时间
		TxHashStr:            "",
		TxCreateBlockNumber:  0,
		TxCreateTimestamp:    0,
		TxPackBlockNumber:    0,
		TxPackTimestamp:      0,
		GobParams:            buf.Bytes(),
	}
}

/*
dao
*/

// SaveDelegateExecuteRecord :
func (model *ModelDB) SaveDelegateExecuteRecord(r *DelegateExecuteRecord) {
	err := model.db.Save(r).Error
	if err != nil {
		panic(err)
	}
}
