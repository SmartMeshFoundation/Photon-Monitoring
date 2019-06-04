package models

import (
	"encoding/gob"
	"math/big"

	"github.com/SmartMeshFoundation/Photon-Monitoring/utils"
	"github.com/ethereum/go-ethereum/common"
)

/*
DelegateUnlock 保存一次unlock委托的相关数据
*/
type DelegateUnlock struct {
	LockSecretHashStr string `json:"lock_secret_hash"`
	AmountStr         string `json:"amount"`
	Expiration        int64  // expiration block number
	MerkleProof       []byte `json:"merkle_proof"`
	Signature         []byte `json:"signature"`
}

// LockSecretHash getter
func (du *DelegateUnlock) LockSecretHash() common.Hash {
	return common.HexToHash(du.LockSecretHashStr)
}

// Amount getter
func (du *DelegateUnlock) Amount() *big.Int {
	return utils.StringToBigInt(du.AmountStr)
}

func init() {
	gob.Register(&DelegateUnlock{})
}
