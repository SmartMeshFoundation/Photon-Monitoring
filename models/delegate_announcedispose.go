package models

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/jinzhu/gorm"
)

/*
DelegateAnnounceDispose 保存一次AnnounceDispose委托的相关数据
*/
type DelegateAnnounceDispose struct {
	/*
	todo 直接以locksecrethash做为key是不合理的,比如
	A尝试经过BC给D转账,失败以后,A走E-F给D转成功.
		A-B-C-B-A-E-F-D
	那么C给B的,B给A的AnnounceDisposed Key是相同的
	punish也存在同样的情况
	 */
	LockSecretHashStr string `json:"secret_hash" gorm:"primary_key"`
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
