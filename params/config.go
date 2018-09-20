package params

import (
	"math/big"
	"os"
	"os/user"
	"path/filepath"
	"runtime"

	"crypto/ecdsa"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/node"
)

//SmtUnlock needs to call one withdraw
var SmtUnlock *big.Int

//SmtUpdateTransfer needs to call update transfer
var SmtUpdateTransfer *big.Int

//SmtPunish needs to do punish Tx
var SmtPunish *big.Int

//SmtAddress the token payed for service
var SmtAddress common.Address

//APIPort listening request from app
var APIPort = 6000

//RaidenURL for cantact with raiden
var RaidenURL = "http://127.0.0.1:5001/api/1/queryreceivedtransfer"

//RegistryAddress chain events where comes from
var RegistryAddress = common.HexToAddress("0xd66d3719E89358e0790636b8586b539467EDa596")

//PrivKey used for sign tx
var PrivKey *ecdsa.PrivateKey

//Address used for sign tx
var Address common.Address

//DataDir where to store smartmonitoring data
var DataDir string

//DataBasePath where db stored
var DataBasePath string

func init() {
	SmtUnlock = big.NewInt(1)
	SmtPunish = big.NewInt(2)
	SmtUpdateTransfer = big.NewInt(3)
	SmtAddress = common.HexToAddress("0x292650fee408320D888e06ed89D938294Ea42f99")
}

//DefaultDataDir default work directory
func DefaultDataDir() string {
	// Try to place the data folder in the user's home dir
	home := homeDir()
	if home != "" {
		if runtime.GOOS == "darwin" {
			return filepath.Join(home, "Library", "smartraidenmonitoring")
		} else if runtime.GOOS == "windows" {
			return filepath.Join(home, "AppData", "Roaming", "smartraidenmonitoring")
		} else {
			return filepath.Join(home, ".smartraidenmonitoring")
		}
	}
	// As we cannot guess a stable location, return empty and handle later
	return ""
}

func homeDir() string {
	if home := os.Getenv("HOME"); home != "" {
		return home
	}
	if usr, err := user.Current(); err == nil {
		return usr.HomeDir
	}
	return ""
}

//DefaultKeyStoreDir keystore path of ethereum
func DefaultKeyStoreDir() string {
	return filepath.Join(node.DefaultDataDir(), "keystore")
}
