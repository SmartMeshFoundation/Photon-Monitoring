package models

import (
	"encoding/gob"

	"github.com/ethereum/go-ethereum/common"
	"github.com/jinzhu/gorm"
)

/*
DelegatePunish 保存一次punish委托的相关数据
*/
type DelegatePunish struct {
	LockHashStr       string `json:"lock_hash" gorm:"primary_key"` //the whole lock's hash,not lock secret hash
	DelegateKey       []byte `json:"delegate_key" gorm:"index"`    // 对应的photonDelegateKey
	AdditionalHashStr string `json:"additional_hash"`
	Signature         []byte `json:"signature"`
}

// LockHash :
func (dp *DelegatePunish) LockHash() common.Hash {
	return common.HexToHash(dp.LockHashStr)
}

// AdditionalHash :
func (dp *DelegatePunish) AdditionalHash() common.Hash {
	return common.HexToHash(dp.AdditionalHashStr)
}

/*
dao
*/

// GetDelegatePunishListByDelegateKey :
func (model *ModelDB) GetDelegatePunishListByDelegateKey(delegateKey []byte) (dus []*DelegatePunish, err error) {
	err = model.db.Where(&DelegatePunish{
		DelegateKey: delegateKey,
	}).Find(&dus).Error
	if err == gorm.ErrRecordNotFound {
		err = nil
	}
	return
}

func init() {
	gob.Register(&DelegatePunish{})
}
