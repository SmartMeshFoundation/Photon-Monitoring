package models

import (
	"math/big"

	"fmt"

	"github.com/SmartMeshFoundation/Photon/utils"
	"github.com/asdine/storm"
	"github.com/ethereum/go-ethereum/common"
	"github.com/labstack/gommon/log"
)

//Account save available smt
type Account struct {
	Address          []byte   `storm:"id"`
	TotalReceivedSmt *big.Int //单增
	UsedSmt          *big.Int //单增
	LockedSmt        *big.Int //执行之前先锁定,成功的话,则减去响应的 smt,否则应该退还.
	NeedSmt          *big.Int //还需要多少 smt, 才能执行所有提交的 tx,供查询,不是计费的依据
}

func (a *Account) String() string {
	return fmt.Sprintf("addr=%s,total=%s,used=%s,locked=%s,need=%s", common.BytesToAddress(a.Address).String(),
		a.TotalReceivedSmt, a.UsedSmt, a.LockedSmt, a.NeedSmt)
}

/*
AccountAddSmt account receive new deposit,amount must be positive
*/
func (model *ModelDB) AccountAddSmt(addr common.Address, amount *big.Int) {
	var err error
	var a = &Account{}
	err = model.db.One("Address", addr[:], a)
	if err == storm.ErrNotFound {
		a = &Account{
			Address:          addr[:],
			TotalReceivedSmt: big.NewInt(0),
			UsedSmt:          big.NewInt(0),
			LockedSmt:        big.NewInt(0),
			NeedSmt:          big.NewInt(0),
		}
		model.lock.Lock()
		err = model.db.Save(a)
		if err != nil {
			log.Error(fmt.Sprintf("AccountAddSmt new account err %s", err))
		}
		model.lock.Unlock()
	}
	a.TotalReceivedSmt.Add(a.TotalReceivedSmt, amount)
	model.accountSanity(a)
	err = model.db.UpdateField(a, "TotalReceivedSmt", a.TotalReceivedSmt)
	if err != nil {
		panic(fmt.Sprintf("save account err %s", err))
	}
	log.Info(fmt.Sprintf("receive smt %s,now total=%s", amount, a.TotalReceivedSmt))
}
func (model *ModelDB) accountUpdateNeedSmt(addr common.Address, amount *big.Int) {
	a := model.AccountGetAccount(addr)
	a.NeedSmt = new(big.Int).Set(amount)
	model.accountSanity(a)
	err := model.db.Save(a)
	if err != nil {
		panic(fmt.Sprintf("save account err %s", err))
	}
}

//AccountAvailable  helper func to calc availabe
func AccountAvailable(a *Account) *big.Int {
	n := new(big.Int)
	n = n.Sub(a.TotalReceivedSmt, a.UsedSmt)
	n = n.Sub(n, a.LockedSmt)
	return n
}

//AccountIsBalanceEnough returns account has enough balance?
func (model *ModelDB) AccountIsBalanceEnough(addr common.Address) bool {
	a := model.AccountGetAccount(addr)
	model.accountSanity(a)
	av := AccountAvailable(a)
	return av.Cmp(a.NeedSmt) >= 0
}

//AccountLockSmt returns account Locked smt,which means there are tx running
func (model *ModelDB) AccountLockSmt(addr common.Address, amount *big.Int) error {
	a := model.AccountGetAccount(addr)
	av := AccountAvailable(a)
	if av.Cmp(amount) < 0 {
		return fmt.Errorf("balance not enough,availabe=%s,amount=%s", av, amount)
	}
	if a.NeedSmt.Cmp(amount) < 0 {
		return fmt.Errorf("need smt smaller, need=%s,amount=%s", a.NeedSmt, amount)
	}
	a.LockedSmt.Add(a.LockedSmt, amount)
	a.NeedSmt.Sub(a.NeedSmt, amount)
	model.accountSanity(a)
	err := model.db.Save(a)
	return err
}

//AccountUnlockSmt tx failed
func (model *ModelDB) AccountUnlockSmt(addr common.Address, amount *big.Int) error {
	a := model.AccountGetAccount(addr)
	if a.LockedSmt.Cmp(amount) < 0 {
		return fmt.Errorf("error unlock smt ,unlock amount=%s,locked=%s", amount, a.LockedSmt)
	}
	a.LockedSmt.Sub(a.LockedSmt, amount)
	model.accountSanity(a)
	err := model.db.Save(a)
	return err
}

//AccountUseSmt tx success
func (model *ModelDB) AccountUseSmt(addr common.Address, amount *big.Int) error {
	a := model.AccountGetAccount(addr)
	if a.LockedSmt.Cmp(amount) < 0 {
		return fmt.Errorf("error unlock smt ,unlock amount=%s,locked=%s", amount, a.LockedSmt)
	}
	a.LockedSmt.Sub(a.LockedSmt, amount)
	a.UsedSmt.Add(a.UsedSmt, amount)
	model.accountSanity(a)
	err := model.db.Save(a)
	return err
}

//AccountGetAccount returns account info
func (model *ModelDB) AccountGetAccount(addr common.Address) *Account {
	var a = &Account{}
	err := model.db.One("Address", addr[:], a)
	if err == storm.ErrNotFound {
		a = &Account{
			Address:          addr[:],
			TotalReceivedSmt: big.NewInt(0),
			UsedSmt:          big.NewInt(0),
			LockedSmt:        big.NewInt(0),
			NeedSmt:          big.NewInt(0),
		}
		return a
	}
	if err != nil {
		panic(fmt.Sprintf("getAccount addr %s err %s", utils.APex(addr), err))
	}
	return a
}

func (model *ModelDB) accountSanity(a *Account) {
	if a.TotalReceivedSmt.Cmp(utils.BigInt0) < 0 {
		panic(fmt.Sprintf("totalReceive negative=%s, account=%s\n", a.TotalReceivedSmt, utils.StringInterface(a, 2)))
	}
	if a.UsedSmt.Cmp(utils.BigInt0) < 0 {
		panic(fmt.Sprintf("UsedSmt negative=%s, account=%s\n", a.UsedSmt, utils.StringInterface(a, 2)))
	}
	if a.LockedSmt.Cmp(utils.BigInt0) < 0 {
		panic(fmt.Sprintf("LockedSmt negative=%s, account=%s\n", a.LockedSmt, utils.StringInterface(a, 2)))
	}
	if a.NeedSmt.Cmp(utils.BigInt0) < 0 {
		panic(fmt.Sprintf("NeedSmt negative=%s, account=%s\n", a.NeedSmt, utils.StringInterface(a, 2)))
	}
	if a.UsedSmt.Cmp(a.TotalReceivedSmt) > 0 {
		panic(fmt.Sprintf("UsedSmt>TotalReceivedSmt used=%s,total=%s account=%s\n", a.UsedSmt, a.TotalReceivedSmt, utils.StringInterface(a, 2)))
	}
}
