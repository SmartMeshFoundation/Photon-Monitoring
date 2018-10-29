package models

import (
	"encoding/gob"

	"github.com/SmartMeshFoundation/Photon/log"
	"github.com/SmartMeshFoundation/Photon/utils"
	"github.com/asdine/storm"
	"github.com/ethereum/go-ethereum/common"
)

//AddressMap is token address to mananger address
type AddressMap map[common.Address]common.Address

const bucketToken = "bucketToken"
const keyToken = "tokens"
const bucketTokenNodes = "bucketTokenNodes"

func init() {
	gob.Register(common.Address{})
	gob.Register(make(AddressMap))
}

//GetAllTokens returna all tokens on this registry contract
func (model *ModelDB) GetAllTokens() (tokens AddressMap, err error) {
	err = model.db.Get(bucketToken, keyToken, &tokens)
	if err != nil {
		if err == storm.ErrNotFound {
			tokens = make(AddressMap)
		}
	}
	return
}

//AddToken add a new token to db,
func (model *ModelDB) AddToken(token common.Address, tokenNetworkAddress common.Address) error {
	var m AddressMap
	err := model.db.Get(bucketToken, keyToken, &m)
	if err != nil {
		return err
	}
	if m[token] != utils.EmptyAddress {
		//startup ...
		log.Info("AddToken ,but already exists,should be ignored when startup...")
		return nil
	}
	m[token] = tokenNetworkAddress
	err = model.db.Set(bucketToken, keyToken, m)
	return err
}
