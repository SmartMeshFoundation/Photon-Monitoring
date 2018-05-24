package main

import (
	"crypto/ecdsa"
	"encoding/hex"

	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/crypto"
)

func GetPrivateKey(addr string, password string, KeyPath string) (privKeyBin []byte, err error) {
	var filename string
	addrhex := strings.ToLower(addr)
	if len(addr) == 42 {
		filename = fmt.Sprintf("UTC--*%s", addrhex[2:]) //skip 0x
	} else {
		filename = fmt.Sprintf("UTC--*%s", addrhex[:])
	}
	path := filepath.Join(KeyPath, filename)
	files, err := filepath.Glob(path)
	if err != nil {
		return
	}
	keyjson, _ := ioutil.ReadFile(files[0])
	key, err := keystore.DecryptKey(keyjson, password)
	if err != nil {
		return
	}
	privKeyBin = crypto.FromECDSA(key.PrivateKey)
	return
}
func GetPrivKeyb(key []byte) *ecdsa.PrivateKey {
	privkey, _ := crypto.ToECDSA(key)
	return privkey
}
func SignData(privKey *ecdsa.PrivateKey, data []byte) (sig []byte, err error) {
	hash := Sha3(data)
	//why add 27 for the last byte?
	sig, err = crypto.Sign(hash, privKey)
	if err == nil {
		sig[len(sig)-1] += byte(27)
	}
	return
}
func Sha3(data []byte) (hash []byte) {
	h := crypto.Keccak256Hash(data)
	hash = h.Bytes()
	return
}
func StrSha3(str string) (result string) {
	data := []byte(str)
	hash := crypto.Keccak256Hash(data)
	result = "0x" + hex.EncodeToString(hash[:])
	return
}
func Sigtest(nodeaddress string) {
	privKeyBin, _ := GetPrivateKey(nodeaddress, "123", `C:\smtwork\privnet3\data\keystore\`)
	PK := GetPrivKeyb(privKeyBin)
	baddress, _ := hex.DecodeString("00")
	Sig, _ := SignData(PK, baddress)
	Sigstr := hex.EncodeToString(Sig)
	fmt.Println("Sigtest:", Sigstr)
}
