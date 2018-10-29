package main

import (
	"fmt"
	"math/big"

	"github.com/SmartMeshFoundation/Photon/accounts"

	"encoding/hex"
	"os"
	"os/signal"
	"path"
	"path/filepath"

	debug2 "runtime/debug"

	"github.com/SmartMeshFoundation/Photon-Monitoring/chainservice"
	"github.com/SmartMeshFoundation/Photon-Monitoring/internal/debug"
	"github.com/SmartMeshFoundation/Photon-Monitoring/models"
	"github.com/SmartMeshFoundation/Photon-Monitoring/params"
	"github.com/SmartMeshFoundation/Photon-Monitoring/rest"
	"github.com/SmartMeshFoundation/Photon-Monitoring/smt"
	"github.com/SmartMeshFoundation/Photon/log"
	"github.com/SmartMeshFoundation/Photon/network/helper"
	"github.com/SmartMeshFoundation/Photon/utils"
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
	log.LocationTrims = append(log.LocationTrims,
		"github.com/SmartMeshFoundation/Photon-Monitoring/vendor/github.com/SmartMeshFoundation/Photon/",
		"github.com/SmartMeshFoundation/Photon-Monitoring",
	)

}

func panicOnNullValue() {
	var c []int
	c[0] = 0
}

//StartMain entry point of Photon app
func StartMain() {
	fmt.Printf("os.args=%q\n", os.Args)
	app := cli.NewApp()
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "address",
			Usage: "The ethereum address you would like Photon monitoring to use sign transaction on ethereum",
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
			Usage: "Directory for storing photon data.",
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
		cli.StringFlag{
			Name:  "unlock-fee",
			Usage: "fee for a unlock Transaction",
			Value: "1000000000000000000",
		},
		cli.StringFlag{
			Name:  "punish-fee",
			Usage: "fee for a punish Transaction",
			Value: "2000000000000000000",
		},
		cli.StringFlag{
			Name:  "update-transfer-fee",
			Usage: "fee for update transfer Transaction ",
			Value: "3000000000000000000",
		},
	}
	app.Flags = append(app.Flags, debug.Flags...)
	app.Action = mainCtx
	app.Name = "Photonmonitoring"
	app.Version = "0.5"
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
	err := app.Run(os.Args)
	if err != nil {
		log.Error(fmt.Sprintf("run err %s", err))
	}
}

func mainCtx(ctx *cli.Context) error {

	var err error
	fmt.Printf("Welcom to Photon monitoring,version %s\n", ctx.App.Version)
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
	sq := smt.NewSmtQuery(params.PhotonURL, db, 0)
	sq.Start()
	ce := chainservice.NewChainEvents(params.PrivKey, client, params.RegistryAddress, db)
	err = ce.Start()
	if err != nil {
		log.Error(fmt.Sprintf("ce start err =%s ", err))
		utils.SystemExit(3)
	}
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
		db.CloseDB()
		utils.SystemExit(0)
	}()
	restful.Start(db, ce)
	return nil
}
func config(ctx *cli.Context) {
	var err error
	params.APIPort = ctx.Int("api-port")
	address := common.HexToAddress(ctx.String("address"))
	address, privkeyBin, err := accounts.PromptAccount(address, ctx.String("keystore-path"), ctx.String("password-file"))
	if err != nil {
		panic(err)
	}
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
		dataDir = path.Join(utils.GetHomePath(), ".Photonmonitoring")
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
	bi, b := new(big.Int).SetString(ctx.String("unlock-fee"), 10)
	if !b {
		log.Error(fmt.Sprintf("unlock-fee arg err %s", ctx.String("unlock-fee")))
		utils.SystemExit(1)
	}
	params.SmtUnlock = bi
	bi, b = new(big.Int).SetString(ctx.String("punish-fee"), 10)
	if !b {
		log.Error(fmt.Sprintf("punish-fee arg err %s", ctx.String("unlock-fee")))
		utils.SystemExit(1)
	}
	params.SmtPunish = bi
	bi, b = new(big.Int).SetString(ctx.String("update-transfer-fee"), 10)
	if !b {
		log.Error(fmt.Sprintf("update-transfer-fee arg err %s", ctx.String("unlock-fee")))
		utils.SystemExit(1)
	}
	params.SmtUpdateTransfer = bi
	log.Info(fmt.Sprintf("unlockfee=%s,punishfee=%s,updatetransferfee=%s,smtaddress=%s",
		params.SmtUnlock, params.SmtPunish, params.SmtUpdateTransfer, params.SmtAddress.String(),
	))
}
