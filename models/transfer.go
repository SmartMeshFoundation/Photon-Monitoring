package models

import (
	"math"
	"math/big"

	utils2 "github.com/SmartMeshFoundation/Photon-Monitoring/utils"

	"github.com/jinzhu/gorm"

	"fmt"

	"github.com/SmartMeshFoundation/Photon/log"
	"github.com/SmartMeshFoundation/Photon/utils"
	"github.com/ethereum/go-ethereum/common"
)

//ReceivedTransfer tokens I have received and where it comes from
type ReceivedTransfer struct {
	Key                  string `gorm:"primary_key"`
	BlockNumber          int64  `json:"block_number" gorm:"index"`
	ChannelIdentifierStr string `json:"channel_identifier"`
	OpenBlockNumber      int64  `json:"open_block_number"`
	TokenAddressStr      string `json:"token_address"`
	FromAddressStr       string `json:"from_address"`
	Nonce                int64  `json:"nonce"`
	AmountStr            string `json:"amount"`
}

// ChannelIdentifier :
func (rt *ReceivedTransfer) ChannelIdentifier() common.Hash {
	return common.HexToHash(rt.ChannelIdentifierStr)
}

// TokenAddress :
func (rt *ReceivedTransfer) TokenAddress() common.Address {
	return common.HexToAddress(rt.TokenAddressStr)
}

// FromAddress :
func (rt *ReceivedTransfer) FromAddress() common.Address {
	return common.HexToAddress(rt.FromAddressStr)
}

// Amount :
func (rt *ReceivedTransfer) Amount() *big.Int {
	return utils2.StringToBigInt(rt.AmountStr)
}

/*
dao
*/

//NewReceivedTransfer save a new received transfer to db
func (model *ModelDB) NewReceivedTransfer(blockNumber int64, channelIdentifier common.Hash, openBlockNumber int64, tokenAddr, fromAddr common.Address, nonce int64, amount *big.Int) {
	key := fmt.Sprintf("%s-%d-%d", channelIdentifier.String(), openBlockNumber, nonce)
	st := &ReceivedTransfer{
		Key:                  key,
		BlockNumber:          blockNumber,
		ChannelIdentifierStr: channelIdentifier.String(),
		OpenBlockNumber:      openBlockNumber,
		TokenAddressStr:      tokenAddr.String(),
		FromAddressStr:       fromAddr.String(),
		Nonce:                nonce,
		AmountStr:            amount.String(),
	}
	if ost, err := model.GetReceivedTransfer(key); err == nil {
		log.Error(fmt.Sprintf("NewReceivedTransfer, but already exist, old=\n%s,new=\n%s",
			utils.StringInterface(ost, 2), utils.StringInterface(st, 2)))
		return
	}
	err := model.db.Save(st).Error
	if err != nil {
		log.Error(fmt.Sprintf("save ReceivedTransfer err %s", err))
	}
}

//NewReceiveTransferFromReceiveTransfer save a new received transfer to db
func (model *ModelDB) NewReceiveTransferFromReceiveTransfer(tr *ReceivedTransfer) bool {

	if _, err := model.GetReceivedTransfer(tr.Key); err == nil {
		return false
	}
	err := model.db.Save(tr).Error
	if err != nil {
		log.Error(fmt.Sprintf("save ReceivedTransfer err %s", err))
	}
	return true
}

//GetReceivedTransfer return the received transfer by key
func (model *ModelDB) GetReceivedTransfer(key string) (*ReceivedTransfer, error) {
	var r ReceivedTransfer
	r.Key = key
	err := model.db.Where(&r).Find(&r).Error
	return &r, err
}

//IsReceivedTransferExist return  true if the received transfer already exists
func (model *ModelDB) IsReceivedTransferExist(key string) bool {
	var r ReceivedTransfer
	r.Key = key
	err := model.db.Where(&r).Find(&r).Error
	return err == nil
}

//GetReceivedTransferInBlockRange returns the received transfer between from and to blocks
func (model *ModelDB) GetReceivedTransferInBlockRange(fromBlock, toBlock int64) (transfers []*ReceivedTransfer, err error) {
	if fromBlock < 0 {
		fromBlock = 0
	}
	if toBlock < 0 {
		toBlock = math.MaxInt64
	}
	d := model.db.Where("1=1")
	if fromBlock > 0 {
		d = d.Where("block_number >= ?", fromBlock)
	}
	if toBlock > 0 {
		d = d.Where("block_number < ?", toBlock)
	}
	err = d.Find(&transfers).Error
	if err == gorm.ErrRecordNotFound {
		err = nil
	}
	return
}
