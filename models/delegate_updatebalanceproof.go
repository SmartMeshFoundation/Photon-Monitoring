package models

import (
	"encoding/gob"
	"math/big"

	"github.com/SmartMeshFoundation/Photon-Monitoring/utils"

	"github.com/ethereum/go-ethereum/common"
)

/*
DelegateUpdateBalanceProof 保存一次updateBalanceProof委托的相关数据
*/
type DelegateUpdateBalanceProof struct {
	Nonce               int64  `json:"nonce"`
	TransferAmountStr   string `json:"transfer_amount"`
	LocksrootStr        string `json:"locksroot"`
	ExtraHashStr        string `json:"extra_hash"`
	ClosingSignature    []byte `json:"closing_signature"`
	NonClosingSignature []byte `json:"non_closing_signature"`
}

// TransferAmount :
func (dubp *DelegateUpdateBalanceProof) TransferAmount() *big.Int {
	return utils.StringToBigInt(dubp.TransferAmountStr)
}

// Locksroot :
func (dubp *DelegateUpdateBalanceProof) Locksroot() common.Hash {
	return common.HexToHash(dubp.LocksrootStr)
}

// ExtraHash :
func (dubp *DelegateUpdateBalanceProof) ExtraHash() common.Hash {
	return common.HexToHash(dubp.ExtraHashStr)
}

func init() {
	gob.Register(&DelegateUpdateBalanceProof{})
}
