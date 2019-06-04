package models

import (
	"fmt"

	"github.com/SmartMeshFoundation/Spectrum/log"

	"github.com/SmartMeshFoundation/Photon-Monitoring/params"
	"github.com/SmartMeshFoundation/Photon/utils"
	"github.com/jinzhu/gorm"
)

// MonitorType 监视器类型
type MonitorType int

// #nosec
const (
	MonitorTypeUnlockAndUpdateBalanceProof = iota // updateBalanceProof及unlock
	MonitorTypePunish                             // 惩罚
)

// DelegateMonitor 存储一次委托的触发时间点
type DelegateMonitor struct {
	Key         []byte `gorm:"primary_key"` // 随机生成,唯一
	BlockNumber int64  `gorm:"index"`
	Type        MonitorType
	DelegateKey []byte
}

//GetDelegateMonitorList return all delegates which should be executed at `blockNumber`
func (model *ModelDB) GetDelegateMonitorList(blockNumber int64) (dms []*DelegateMonitor, err error) {
	dm := &DelegateMonitor{
		BlockNumber: blockNumber,
	}
	err = model.db.Where(dm).Find(&dms).Error
	if err == gorm.ErrRecordNotFound {
		err = nil
	}
	return
}

// AddDelegateMonitor 为一次委托添加Monitor
func (model *ModelDB) AddDelegateMonitor(d *Delegate) {
	// 复用
	AddDelegateMonitorInTx(model.db, d)
}

// AddDelegateMonitorInTx 为一次委托添加Monitor
func AddDelegateMonitorInTx(tx *gorm.DB, d *Delegate) {
	/*
		PMS在RevealTimeout时进行updateBalanceProof
		主要基于如下考虑:
			1. 避免PMS和photon自身balanceProof提交产生的冲突问题
			2. 在主链上RevealTimeout非常长,足够应付各种情况.
			3. 如果photon保持无网到RevealTimeout这么久,出现安全问题,photon自己担责.
	*/
	updateBalanceProofTime := d.SettleBlockNumber - int64(params.RevealTimeout)
	err := tx.Save(&DelegateMonitor{
		Key:         utils.NewRandomAddress().Bytes(),
		BlockNumber: updateBalanceProofTime,
		Type:        MonitorTypeUnlockAndUpdateBalanceProof,
		DelegateKey: d.Key,
	}).Error
	if err != nil {
		panic(fmt.Sprintf("db err %s", err))
	}
	/*
		代理惩罚部分统一在通道 settle time out 以后进行,避免和真正的参与方发生冲突.
	*/
	err = tx.Save(&DelegateMonitor{
		Key:         utils.NewRandomAddress().Bytes(),
		BlockNumber: d.SettleBlockNumber,
		Type:        MonitorTypePunish,
		DelegateKey: d.Key,
	}).Error
	if err != nil {
		panic(fmt.Sprintf("db err %s", err))
	}
	log.Info("delegate [channel=%s delegator=%s] will try to UpdateTransfer and Unlock at %d and Punish at %d ",
		d.ChannelIdentifierStr, d.DelegatorAddressStr, updateBalanceProofTime, d.SettleBlockNumber)
}
