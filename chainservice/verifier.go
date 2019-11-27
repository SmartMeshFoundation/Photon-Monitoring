package chainservice

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"

	"github.com/SmartMeshFoundation/Photon/params"

	"github.com/SmartMeshFoundation/Photon-Monitoring/models"
	"github.com/SmartMeshFoundation/Photon/log"
	"github.com/SmartMeshFoundation/Photon/utils"
	"github.com/ethereum/go-ethereum/common"
)

//VerifyDelegate verify delegate from app is valid or not,should be thread safe
//todo 为了解决用户进行委托的时候通道已经关闭的问题,这里对c做了修改,后续应该重构解决这个问题
func (ce *ChainEvents) VerifyDelegate(c *models.ChannelFor3rd, delegater common.Address) error {
	partner := c.PartnerAddress
	haveValidData := false
	tokenNetwork, err := ce.bcs.TokenNetwork(c.TokenAddress)
	if err != nil {
		return err
	}
	settleBlockNumber, openBlockNumber, _, _, err := tokenNetwork.GetContract().GetChannelInfoByChannelIdentifier(nil, c.ChannelIdentifier)
	if err != nil {
		return fmt.Errorf("channel %s get channel info err %s", c.ChannelIdentifier.String(), err)
	}
	//openBlockNumber 表示通道不存在,
	if openBlockNumber == 0 || c.OpenBlockNumber != int64(openBlockNumber) {
		return fmt.Errorf("channel %s open blocknumber not match on chain, chain's openblocknumber=%d,delegate's=%d",
			c.ChannelIdentifier.String(), openBlockNumber, c.OpenBlockNumber,
		)
	}
	if c.UpdateTransfer.Nonce > 0 {
		closingAddr, err := verifyClosingSignature(c)
		if err != nil {
			return err
		}
		if closingAddr != partner || closingAddr == delegater {
			return fmt.Errorf("participant error,closingAddr=%s,partner=%s,delegater=%s",
				utils.APex(closingAddr), utils.APex(partner),
				utils.APex(delegater))
		}
		if !verifyNonClosingSignature(c, delegater) {
			return errors.New("non closing error")
		}
		err = ce.verifyUnlocks(c, delegater)
		if err != nil {
			return err
		}
		haveValidData = true
	}
	if len(c.Punishes) > 0 {
		err := ce.verifyPunishes(c)
		if err != nil {
			return err
		}
		haveValidData = true
	}
	if len(c.AnnouceDisposed) > 0 {
		for _, a := range c.AnnouceDisposed {
			if a == nil || a.LockSecretHash == utils.EmptyHash {
				return fmt.Errorf("AnnouceDisposed error, AnnouceDisposed must be valid")
			}
		}
		haveValidData = true
	}
	if len(c.Secrets) > 0 {
		for _, s := range c.Secrets {
			if s == nil || s.Secret == utils.EmptyHash {
				return fmt.Errorf("secret error, Secret must be valid")
			}
		}
		haveValidData = true
	}
	if !haveValidData {
		return fmt.Errorf("invalid delegate,it's empty")
	}
	c.SetSettleBlockNumber(int64(settleBlockNumber))
	return nil
}
func (ce *ChainEvents) verifyUnlocks(c *models.ChannelFor3rd, delegater common.Address) error {
	for _, l := range c.Unlocks {
		if l.Lock == nil || l.Lock.Amount == nil {
			return fmt.Errorf("lock error %s", l.Lock)
		}
		signer, err := ce.verifyUnlockSignature(l, c)
		if err != nil {
			return err
		}
		if signer != delegater {
			return fmt.Errorf("unlock's signature error, signer=%s,delegater=%s", signer.String(), delegater.String())
		}
	}
	return nil
}
func (ce *ChainEvents) verifyPunishes(c *models.ChannelFor3rd) error {
	for _, p := range c.Punishes {
		signer, err := ce.verifyPunishSignature(p, c)
		if err != nil {
			return err
		}
		if signer != c.PartnerAddress {
			return fmt.Errorf("punish signature error,signer=%s,partner=%s", signer.String(), c.PartnerAddress.String())
		}
	}
	return nil
}
func (ce *ChainEvents) verifyUnlockSignature(u *models.Unlock, c *models.ChannelFor3rd) (signer common.Address, err error) {
	buf := new(bytes.Buffer)
	_, err = buf.Write(params.ContractSignaturePrefix)
	_, err = buf.Write([]byte(params.ContractUnlockDelegateProofMessageLength))
	//_, err = buf.Write(utils.BigIntTo32Bytes(c.UpdateTransfer.TransferAmount))
	_, err = buf.Write(ce.bcs.NodeAddress[:])
	_, err = buf.Write(utils.BigIntTo32Bytes(big.NewInt(u.Lock.Expiration)))
	_, err = buf.Write(utils.BigIntTo32Bytes(u.Lock.Amount))
	_, err = buf.Write(u.Lock.LockSecretHash[:])
	_, err = buf.Write(c.ChannelIdentifier[:])
	err = binary.Write(buf, binary.BigEndian, c.OpenBlockNumber)
	_, err = buf.Write(utils.BigIntTo32Bytes(params.ChainID))
	if err != nil {
		log.Error(fmt.Sprintf("buf write error %s", err))
		return
	}
	hash := utils.Sha3(buf.Bytes())
	sig := u.Signature
	return utils.Ecrecover(hash, sig)
}
func (ce *ChainEvents) verifyPunishSignature(p *models.Punish, c *models.ChannelFor3rd) (signer common.Address, err error) {
	buf := new(bytes.Buffer)
	_, err = buf.Write(params.ContractSignaturePrefix)
	_, err = buf.Write([]byte(params.ContractDisposedProofMessageLength))
	_, err = buf.Write(p.LockHash[:])
	_, err = buf.Write(c.ChannelIdentifier[:])
	err = binary.Write(buf, binary.BigEndian, c.OpenBlockNumber)
	_, err = buf.Write(utils.BigIntTo32Bytes(params.ChainID))
	_, err = buf.Write(p.AdditionalHash[:])
	if err != nil {
		return
	}
	hash := utils.Sha3(buf.Bytes())
	return utils.Ecrecover(hash, p.Signature)
}
func verifyClosingSignature(c *models.ChannelFor3rd) (signer common.Address, err error) {
	u := c.UpdateTransfer
	buf := new(bytes.Buffer)
	if c.UpdateTransfer.Nonce <= 0 {
		err = fmt.Errorf("delegate with nonce <=0")
		return
	}
	log.Trace(fmt.Sprintf("c=%s", utils.StringInterface(c, 5)))
	log.Trace(fmt.Sprintf("chaind=%s", params.ChainID.String()))
	_, err = buf.Write(params.ContractSignaturePrefix)
	_, err = buf.Write([]byte(params.ContractBalanceProofMessageLength))
	_, err = buf.Write(utils.BigIntTo32Bytes(u.TransferAmount))
	_, err = buf.Write(u.Locksroot[:])
	err = binary.Write(buf, binary.BigEndian, u.Nonce)
	_, err = buf.Write(u.ExtraHash[:])
	_, err = buf.Write(c.ChannelIdentifier[:])
	err = binary.Write(buf, binary.BigEndian, c.OpenBlockNumber)
	_, err = buf.Write(utils.BigIntTo32Bytes(params.ChainID))
	hash := utils.Sha3(buf.Bytes())
	sig := u.ClosingSignature
	return utils.Ecrecover(hash, sig)
}
func verifyNonClosingSignature(c *models.ChannelFor3rd, delegater common.Address) bool {
	var err error
	u := c.UpdateTransfer
	buf := new(bytes.Buffer)
	_, err = buf.Write(params.ContractSignaturePrefix)
	_, err = buf.Write([]byte(params.ContractBalanceProofDelegateMessageLength))
	_, err = buf.Write(utils.BigIntTo32Bytes(u.TransferAmount))
	_, err = buf.Write(u.Locksroot[:])
	err = binary.Write(buf, binary.BigEndian, u.Nonce)
	_, err = buf.Write(c.ChannelIdentifier[:])
	err = binary.Write(buf, binary.BigEndian, c.OpenBlockNumber)
	_, err = buf.Write(utils.BigIntTo32Bytes(params.ChainID))
	hash := utils.Sha3(buf.Bytes())
	sig := u.NonClosingSignature
	signer, err := utils.Ecrecover(hash, sig)
	return err == nil && signer == delegater
}
