package models

import (
	"math"
	"math/big"

	"fmt"

	"github.com/SmartMeshFoundation/SmartRaiden/log"
	"github.com/SmartMeshFoundation/SmartRaiden/utils"
	"github.com/asdine/storm"
	"github.com/ethereum/go-ethereum/common"
)

//ReceivedTransfer tokens I have received and where it comes from
type ReceivedTransfer struct {
	Key               string         `storm:"id"`
	BlockNumber       int64          `json:"block_number" storm:"index"`
	ChannelIdentifier common.Hash    `json:"channel_identifier"`
	OpenBlockNumber   int64          `json:"open_block_number"`
	TokenAddress      common.Address `json:"token_address"`
	FromAddress       common.Address `json:"from_address"`
	Nonce             int64          `json:"nonce"`
	Amount            *big.Int       `json:"amount"`
}

//NewReceivedTransfer save a new received transfer to db
func (model *ModelDB) NewReceivedTransfer(blockNumber int64, channelIdentifier common.Hash, openBlockNumber int64, tokenAddr, fromAddr common.Address, nonce int64, amount *big.Int) {
	key := fmt.Sprintf("%s-%d-%d", channelIdentifier.String(), openBlockNumber, nonce)
	st := &ReceivedTransfer{
		Key:               key,
		BlockNumber:       blockNumber,
		ChannelIdentifier: channelIdentifier,
		OpenBlockNumber:   openBlockNumber,
		TokenAddress:      tokenAddr,
		FromAddress:       fromAddr,
		Nonce:             nonce,
		Amount:            amount,
	}
	if ost, err := model.GetReceivedTransfer(key); err == nil {
		log.Error(fmt.Sprintf("NewReceivedTransfer, but already exist, old=\n%s,new=\n%s",
			utils.StringInterface(ost, 2), utils.StringInterface(st, 2)))
		return
	}
	err := model.db.Save(st)
	if err != nil {
		log.Error(fmt.Sprintf("save ReceivedTransfer err %s", err))
	}
}

//NewReceiveTransferFromReceiveTransfer save a new received transfer to db
func (model *ModelDB) NewReceiveTransferFromReceiveTransfer(tr *ReceivedTransfer) bool {

	if _, err := model.GetReceivedTransfer(tr.Key); err == nil {
		return false
	}
	err := model.db.Save(tr)
	if err != nil {
		log.Error(fmt.Sprintf("save ReceivedTransfer err %s", err))
	}
	return true
}

//GetReceivedTransfer return the received transfer by key
func (model *ModelDB) GetReceivedTransfer(key string) (*ReceivedTransfer, error) {
	var r ReceivedTransfer
	err := model.db.One("Key", key, &r)
	return &r, err
}

//IsReceivedTransferExist return  true if the received transfer already exists
func (model *ModelDB) IsReceivedTransferExist(key string) bool {
	var r ReceivedTransfer
	err := model.db.One("Key", key, &r)
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
	err = model.db.Range("BlockNumber", fromBlock, toBlock, &transfers)
	if err == storm.ErrNotFound { //ingore not found error
		err = nil
	}
	return
}
