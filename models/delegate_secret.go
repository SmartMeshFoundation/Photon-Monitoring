package models

import (
	"encoding/gob"
	"github.com/ethereum/go-ethereum/common"
)

/*
DelegateSecret 保存一次secret委托的相关数据
*/
type DelegateSecret struct {
	Secret        string `json:"secret"`
	RegisterBlock int64  `json:"register_block"`
}

// Secret getter
func (ds *DelegateSecret) GetSecret() common.Hash {
	return common.HexToHash(ds.Secret)
}

func init() {
	gob.Register(&DelegateSecret{})
}
