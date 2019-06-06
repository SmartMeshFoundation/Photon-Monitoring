package models

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/jinzhu/gorm"
)

/*
DelegateAnnounceDispose 保存一次AnnounceDispose委托的相关数据
*/
type DelegateAnnounceDispose struct {
	ID                int    `json:"-" gorm:"id"` // 自增ID,确保不冲突
	LockSecretHashStr string `json:"lock_secret_hash"`
	DelegateKey       []byte `json:"delegate_key" gorm:"index"` // 对应的photonDelegateKey
}

// LockSecretHash getter
func (da *DelegateAnnounceDispose) LockSecretHash() common.Hash {
	return common.HexToHash(da.LockSecretHashStr)
}

/*
dao
*/

// GetDelegateAnnounceDisposeListByDelegateKey query by primary_key
func (model *ModelDB) GetDelegateAnnounceDisposeListByDelegateKey(delegateKey []byte) (das []*DelegateAnnounceDispose, err error) {
	err = model.db.Where(&DelegateAnnounceDispose{
		DelegateKey: delegateKey,
	}).Find(&das).Error
	if err == gorm.ErrRecordNotFound {
		err = nil
	}
	return
}
