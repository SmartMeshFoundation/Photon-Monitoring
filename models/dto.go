package models

import (
	"math/big"

	"github.com/SmartMeshFoundation/Photon-Monitoring/utils"
	"github.com/SmartMeshFoundation/Photon/transfer/mtree"
	"github.com/ethereum/go-ethereum/common"
)

//UpdateTransfer arguments need to call contract updatetransfer
type UpdateTransfer struct {
	Nonce               int64       `json:"nonce"`
	TransferAmount      *big.Int    `json:"transfer_amount"`
	Locksroot           common.Hash `json:"locksroot"`
	ExtraHash           common.Hash `json:"extra_hash"`
	ClosingSignature    []byte      `json:"closing_signature"`
	NonClosingSignature []byte      `json:"non_closing_signature"`
	TxStatus            int
	TxError             string
	TxHash              common.Hash
}

//Unlock arguments need to call contract Withdraw
type Unlock struct {
	Lock        *mtree.Lock `json:"lock"`
	MerkleProof []byte      `json:"merkle_proof"`
	Signature   []byte      `json:"signature"`
	TxStatus    int
	TxError     string
	TxHash      common.Hash
}

// Secret 需要注册的密码委托
type Secret struct {
	Secret        common.Hash `json:"secret"`
	RegisterBlock int64       `json:"register_block"` // 委托注册的时间
	TxStatus      int
	TxError       string
	TxHash        common.Hash
}

//ChannelFor3rd is for 3rd party to call update transfer
type ChannelFor3rd struct {
	ChannelIdentifier common.Hash        `json:"channel_identifier"`
	OpenBlockNumber   int64              `json:"open_block_number"`
	TokenAddress      common.Address     `json:"token_address"`
	PartnerAddress    common.Address     `json:"partner_address"`
	UpdateTransfer    UpdateTransfer     `json:"update_transfer"`
	Unlocks           []*Unlock          `json:"unlocks"`
	Punishes          []*Punish          `json:"punishes"`
	AnnouceDisposed   []*AnnouceDisposed `json:"annouce_disposed"`
	Secrets           []*Secret          `json:"secrets"`
	settleBlockNumber int64              //for internal use,
}

//SetSettleBlockNumber 设置blockNumber,主要用于解决用户委托的时候通道已经关闭的情形.
func (c *ChannelFor3rd) SetSettleBlockNumber(blockNumber int64) {
	c.settleBlockNumber = blockNumber
}

// GetDelegateUpdateBalanceProof change UpdateTransfer to DelegateUpdateBalanceProof
func (c *ChannelFor3rd) GetDelegateUpdateBalanceProof() *DelegateUpdateBalanceProof {
	return &DelegateUpdateBalanceProof{
		Nonce:               c.UpdateTransfer.Nonce,
		TransferAmountStr:   utils.BigIntToString(c.UpdateTransfer.TransferAmount),
		LocksrootStr:        c.UpdateTransfer.Locksroot.String(),
		ExtraHashStr:        c.UpdateTransfer.ExtraHash.String(),
		ClosingSignature:    c.UpdateTransfer.ClosingSignature,
		NonClosingSignature: c.UpdateTransfer.NonClosingSignature,
	}
}

// GetDelegateUnlocks change Unlock to DelegateUnlock
func (c *ChannelFor3rd) GetDelegateUnlocks() (dus []*DelegateUnlock) {
	for _, u := range c.Unlocks {
		dus = append(dus, &DelegateUnlock{
			LockSecretHashStr: u.Lock.LockSecretHash.String(),
			AmountStr:         utils.BigIntToString(u.Lock.Amount),
			Expiration:        u.Lock.Expiration,
			MerkleProof:       u.MerkleProof,
			Signature:         u.Signature})
	}
	return
}

// GetDeleteSecrets change Secret to DelegateSecret
func (c *ChannelFor3rd) GetDeleteSecrets() (ds []*DelegateSecret) {
	for _, s := range c.Secrets {
		ds = append(ds, &DelegateSecret{
			Secret:        s.Secret.String(),
			RegisterBlock: s.RegisterBlock,
		})
	}
	return
}

//Punish 需要委托给第三方的 punish证据
type Punish struct {
	LockHash       common.Hash `json:"lock_hash"` //the whole lock's hash,not lock secret hash
	AdditionalHash common.Hash `json:"additional_hash"`
	Signature      []byte      `json:"signature"`
	TxStatus       int
	TxError        string
	TxHash         common.Hash
}

//AnnouceDisposed 确保unlock的时候不要去unlock那些声明放弃的锁
type AnnouceDisposed struct {
	LockSecretHash common.Hash `json:"secret_hash"`
}
