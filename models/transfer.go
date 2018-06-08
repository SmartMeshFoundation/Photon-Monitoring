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
	Key            string         `storm:"id"`
	BlockNumber    int64          `json:"block_number" storm:"index"`
	ChannelAddress common.Address `json:"channel_address"`
	TokenAddress   common.Address `json:"token_address"`
	FromAddress    common.Address `json:"from_address"`
	Nonce          int64          `json:"nonce"`
	Amount         *big.Int       `json:"amount"`
}

//NewReceivedTransfer save a new received transfer to db
func (m *ModelDB) NewReceivedTransfer(blockNumber int64, channelAddr, tokenAddr, fromAddr common.Address, nonce int64, amount *big.Int) {
	key := fmt.Sprintf("%s-%d", channelAddr.String(), nonce)
	st := &ReceivedTransfer{
		Key:            key,
		BlockNumber:    blockNumber,
		ChannelAddress: channelAddr,
		TokenAddress:   tokenAddr,
		FromAddress:    fromAddr,
		Nonce:          nonce,
		Amount:         amount,
	}
	if ost, err := m.GetReceivedTransfer(key); err == nil {
		log.Error(fmt.Sprintf("NewReceivedTransfer, but already exist, old=\n%s,new=\n%s",
			utils.StringInterface(ost, 2), utils.StringInterface(st, 2)))
		return
	}
	err := m.db.Save(st)
	if err != nil {
		log.Error(fmt.Sprintf("save ReceivedTransfer err %s", err))
	}
}

//NewReceiveTransferFromReceiveTransfer save a new received transfer to db
func (m *ModelDB) NewReceiveTransferFromReceiveTransfer(tr *ReceivedTransfer) bool {

	if _, err := m.GetReceivedTransfer(tr.Key); err == nil {
		return false
	}
	err := m.db.Save(tr)
	if err != nil {
		log.Error(fmt.Sprintf("save ReceivedTransfer err %s", err))
	}
	return true
}

//GetReceivedTransfer return the received transfer by key
func (m *ModelDB) GetReceivedTransfer(key string) (*ReceivedTransfer, error) {
	var r ReceivedTransfer
	err := m.db.One("Key", key, &r)
	return &r, err
}

//IsReceivedTransferExist return  true if the received transfer already exists
func (m *ModelDB) IsReceivedTransferExist(key string) bool {
	var r ReceivedTransfer
	err := m.db.One("Key", key, &r)
	return err == nil
}

//GetReceivedTransferInBlockRange returns the received transfer between from and to blocks
func (m *ModelDB) GetReceivedTransferInBlockRange(fromBlock, toBlock int64) (transfers []*ReceivedTransfer, err error) {
	if fromBlock < 0 {
		fromBlock = 0
	}
	if toBlock < 0 {
		toBlock = math.MaxInt64
	}
	err = m.db.Range("BlockNumber", fromBlock, toBlock, &transfers)
	if err == storm.ErrNotFound { //ingore not found error
		err = nil
	}
	return
}
