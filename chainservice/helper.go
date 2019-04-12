package chainservice

import (
	"github.com/SmartMeshFoundation/Photon/models"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)
//以下都只是为了配合photon接口需要,
type mockTxInfoDao struct{}

func (m*mockTxInfoDao)NewPendingTXInfo(tx *types.Transaction, txType models.TXInfoType, channelIdentifier common.Hash, openBlockNumber int64, txParams models.TXParams) (txInfo *models.TXInfo, err error) {
	return &models.TXInfo{},nil
}
func (m*mockTxInfoDao) SaveEventToTXInfo(event interface{}) (txInfo *models.TXInfo, err error) {
	return &models.TXInfo{},nil
}
func (m*mockTxInfoDao) UpdateTXInfoStatus(txHash common.Hash, status models.TXInfoStatus, pendingBlockNumber int64, gasUsed uint64) (txInfo *models.TXInfo, err error) {
	return &models.TXInfo{},nil
}
func (m*mockTxInfoDao) GetTXInfoList(channelIdentifier common.Hash, openBlockNumber int64, tokenAddress common.Address, txType models.TXInfoType, status models.TXInfoStatus) (list []*models.TXInfo, err error) {
	return nil ,nil
}


type mockChainEventRecordDao struct {

}

func (m*mockChainEventRecordDao) NewDeliveredChainEvent(id models.ChainEventID, blockNumber uint64) {
	return
}
func (m*mockChainEventRecordDao)CheckChainEventDelivered(id models.ChainEventID) (blockNumber uint64, delivered bool){
	return
}
func (m*mockChainEventRecordDao)ClearOldChainEventRecord(blockNumber uint64){

}
func (m*mockChainEventRecordDao) 	MakeChainEventID(l *types.Log) models.ChainEventID{
	return  ""
}