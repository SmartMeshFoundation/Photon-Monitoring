package main

import (
	"fmt"

	"encoding/hex"
	"io/ioutil"
	"os"
	"os/signal"
	"path"
	"path/filepath"

	debug2 "runtime/debug"

	"github.com/SmartMeshFoundation/SmartRaiden"
	"github.com/SmartMeshFoundation/SmartRaiden-Monitoring/chainservice"
	"github.com/SmartMeshFoundation/SmartRaiden-Monitoring/internal/debug"
	"github.com/SmartMeshFoundation/SmartRaiden-Monitoring/models"
	"github.com/SmartMeshFoundation/SmartRaiden-Monitoring/params"
	"github.com/SmartMeshFoundation/SmartRaiden-Monitoring/rest"
	"github.com/SmartMeshFoundation/SmartRaiden-Monitoring/smt"
	"github.com/SmartMeshFoundation/SmartRaiden/log"
	"github.com/SmartMeshFoundation/SmartRaiden/network/helper"
	"github.com/SmartMeshFoundation/SmartRaiden/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/node"
	"gopkg.in/urfave/cli.v1"
)

func main() {
	StartMain()
}

func init() {
	debug2.SetTraceback("crash")
}

func panicOnNullValue() {
	var c []int
	c[0] = 0
}

//StartMain entry point of raiden app
func StartMain() {
	fmt.Printf("os.args=%q\n", os.Args)
	app := cli.NewApp()
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "address",
			Usage: "The ethereum address you would like smartraiden monitoring to use sign transaction on ethereum",
		},
		cli.StringFlag{
			Name:  "keystore-path",
			Usage: "If you have a non-standard path for the ethereum keystore directory provide it using this argument. ",
			Value: params.DefaultKeyStoreDir(),
		},
		cli.StringFlag{
			Name: "eth-rpc-endpoint",
			Usage: `"host:port" address of ethereum JSON-RPC server.\n'
	           'Also accepts a protocol prefix (ws:// or ipc channel) with optional port',`,
			Value: node.DefaultIPCEndpoint("geth"),
		},
		cli.StringFlag{
			Name:  "registry-contract-address",
			Usage: `hex encoded address of the registry contract.`,
			Value: params.RegistryAddress.String(),
		},
		cli.IntFlag{
			Name:  "api-port",
			Usage: ` port  for the RPC server to listen on.`,
			Value: 6000,
		},
		cli.StringFlag{
			Name:  "datadir",
			Usage: "Directory for storing raiden data.",
			Value: params.DefaultDataDir(),
		},
		cli.StringFlag{
			Name:  "password-file",
			Usage: "Text file containing password for provided account",
		},
		cli.StringFlag{
			Name:  "smt",
			Usage: "smt address",
			Value: params.SmtAddress.String(),
		},
	}
	app.Flags = append(app.Flags, debug.Flags...)
	app.Action = mainCtx
	app.Name = "smartraidenmonitoring"
	app.Version = "0.1"
	app.Before = func(ctx *cli.Context) error {
		if err := debug.Setup(ctx); err != nil {
			return err
		}
		return nil
	}

	app.After = func(ctx *cli.Context) error {
		debug.Exit()
		return nil
	}
	app.Run(os.Args)
}

func mainCtx(ctx *cli.Context) error {

	var err error
	fmt.Printf("Welcom to smartraiden monitoring,version %s\n", ctx.App.Version)
	config(ctx)
	//log.Debug(fmt.Sprintf("Config:%s", utils.StringInterface(cfg, 2)))
	ethEndpoint := ctx.String("eth-rpc-endpoint")
	client, err := helper.NewSafeClient(ethEndpoint)
	if err != nil {
		log.Error(fmt.Sprintf("cannot connect to geth :%s err=%s", ethEndpoint, err))
		utils.SystemExit(1)
	}

	db, err := models.OpenDb(params.DataBasePath)
	if err != nil {
		log.Error(fmt.Sprintf("err=%s", err))
		utils.SystemExit(2)
	}
	sq := smt.NewSmtQuery(params.RaidenURL, db, 0)
	sq.Start()
	ce := chainservice.NewChainEvents(params.PrivKey, client, params.RegistryAddress, db)
	ce.Start()
	/*
		quit handler
	*/
	go func() {
		quitSignal := make(chan os.Signal, 1)
		signal.Notify(quitSignal, os.Interrupt, os.Kill)
		<-quitSignal
		signal.Stop(quitSignal)
		sq.Stop()
		ce.Stop()
		utils.SystemExit(0)
	}()
	restful.Start(db, ce)
	return nil
}
func promptAccount(adviceAddress common.Address, keystorePath, passwordfile string) (addr common.Address, keybin []byte) {
	am := smartraiden.NewAccountManager(keystorePath)
	if len(am.Accounts) == 0 {
		log.Error(fmt.Sprintf("No Ethereum accounts found in the directory %s", keystorePath))
		utils.SystemExit(1)
	}
	if !am.AddressInKeyStore(adviceAddress) {
		if adviceAddress != utils.EmptyAddress {
			log.Error(fmt.Sprintf("account %s could not be found on the sytstem. aborting...", adviceAddress))
			utils.SystemExit(1)
		}
		shouldPromt := true
		fmt.Println("The following accounts were found in your machine:")
		for i := 0; i < len(am.Accounts); i++ {
			fmt.Printf("%3d -  %s\n", i, am.Accounts[i].Address.String())
		}
		fmt.Println("")
		for shouldPromt {
			fmt.Printf("Select one of them by index to continue:\n")
			idx := -1
			fmt.Scanf("%d", &idx)
			if idx >= 0 && idx < len(am.Accounts) {
				shouldPromt = false
				addr = am.Accounts[idx].Address
			} else {
				fmt.Printf("Error: Provided index %d is out of bounds", idx)
			}
		}
	} else {
		addr = adviceAddress
	}
	if len(passwordfile) > 0 {
		data, err := ioutil.ReadFile(passwordfile)
		if err != nil {
			data = []byte(passwordfile)
		}
		password := string(data)
		log.Trace(fmt.Sprintf("password is %s", password))
		keybin, err = am.GetPrivateKey(addr, password)
		if err != nil {
			log.Error(fmt.Sprintf("Incorrect password for %s in file. Aborting ... %s", addr.String(), err))
			utils.SystemExit(1)
		}
	} else {
		panic("must specified password")
	}
	return
}
func config(ctx *cli.Context) {
	var err error
	params.APIPort = ctx.Int("api-port")
	address := common.HexToAddress(ctx.String("address"))
	address, privkeyBin := promptAccount(address, ctx.String("keystore-path"), ctx.String("password-file"))
	log.Trace(fmt.Sprintf("privkey=%s", common.Bytes2Hex(privkeyBin)))
	params.Address = address
	params.PrivKey, err = crypto.ToECDSA(privkeyBin)
	if err != nil {
		log.Error("privkey error:", err)
		utils.SystemExit(1)
	}
	registAddrStr := ctx.String("registry-contract-address")
	if len(registAddrStr) > 0 {
		params.RegistryAddress = common.HexToAddress(registAddrStr)
	}
	dataDir := ctx.String("datadir")
	if len(dataDir) == 0 {
		dataDir = path.Join(utils.GetHomePath(), ".smartraidenmonitoring")
	}
	params.DataDir = dataDir
	if !utils.Exists(params.DataDir) {
		err = os.MkdirAll(params.DataDir, os.ModePerm)
		if err != nil {
			log.Error(fmt.Sprintf("Datadir:%s doesn't exist and cannot create %v", params.DataDir, err))
			utils.SystemExit(1)
		}
	}
	userDbPath := hex.EncodeToString(params.Address[:])
	userDbPath = userDbPath[:8]
	userDbPath = filepath.Join(params.DataDir, userDbPath)
	if !utils.Exists(userDbPath) {
		err = os.MkdirAll(userDbPath, os.ModePerm)
		if err != nil {
			log.Error(fmt.Sprintf("Datadir:%s doesn't exist and cannot create %v", userDbPath, err))
			utils.SystemExit(1)
		}
	}
	databasePath := filepath.Join(userDbPath, "log.db")
	params.DataBasePath = databasePath
	params.SmtAddress = common.HexToAddress(ctx.String("smt"))
}
