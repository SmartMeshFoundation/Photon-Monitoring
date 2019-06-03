package models

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/jinzhu/gorm"
)

/*
DelegateAnnounceDispose 保存一次AnnounceDispose委托的相关数据
*/
type DelegateAnnounceDispose struct {
	LockSecretHashStr string `json:"secret_hash" gorm:"primary_key"`
	DelegateKey       []byte `json:"delegate_key" gorm:"index"` // 对应的photonDelegateKey
}

// LockSecretHash :
func (da *DelegateAnnounceDispose) LockSecretHash() common.Hash {
	return common.HexToHash(da.LockSecretHashStr)
}

/*
dao
*/

// GetDelegateAnnounceDisposeListByDelegateKey :
func (model *ModelDB) GetDelegateAnnounceDisposeListByDelegateKey(delegateKey []byte) (das []*DelegateAnnounceDispose, err error) {
	err = model.db.Where(&DelegateAnnounceDispose{
		DelegateKey: delegateKey,
	}).Find(&das).Error
	if err == gorm.ErrRecordNotFound {
		err = nil
	}
	return
}
