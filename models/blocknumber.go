package models

import (
	"fmt"
	"github.com/SmartMeshFoundation/Photon/log"
)

const lastBlockNumberKey = "lastBlockNumberKey"

type lastBlockNumber struct {
	Key         string `gorm:"primary_key"`
	BlockNumber int64
}

//GetLatestBlockNumber lastest block number
func (model *ModelDB) GetLatestBlockNumber() int64 {
	var number int64
	lastBlockNumber := &lastBlockNumber{
		Key: lastBlockNumberKey,
	}
	err := model.db.Where(lastBlockNumber).First(lastBlockNumber).Error
	if err != nil {
		log.Error(fmt.Sprintf("models GetLatestBlockNumber err=%s", err))
	}
	return number
}

//SaveLatestBlockNumber block numer has been processed
func (model *ModelDB) SaveLatestBlockNumber(blockNumber int64) {
	err := model.db.Save(&lastBlockNumber{
		Key:         lastBlockNumberKey,
		BlockNumber: blockNumber,
	}).Error
	if err != nil {
		log.Error(fmt.Sprintf("models SaveLatestBlockNumber err=%s", err))
	}
}
