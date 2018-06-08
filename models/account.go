package models

import (
	"math/big"

	"fmt"

	"github.com/SmartMeshFoundation/SmartRaiden/utils"
	"github.com/asdine/storm"
	"github.com/ethereum/go-ethereum/common"
	"github.com/labstack/gommon/log"
)

//Account save available smt
type Account struct {
	Address          string   `storm:"id"`
	TotalReceivedSmt *big.Int //单增
	UsedSmt          *big.Int //单增
	LockedSmt        *big.Int //执行之前先锁定,成功的话,则减去响应的 smt,否则应该退还.
	NeedSmt          *big.Int //还需要多少 smt, 才能执行所有提交的 tx,供查询,不是计费的依据
}

func (a *Account) String() string {
	return fmt.Sprintf("addr=%s,total=%s,used=%s,locked=%s,need=%s", a.Address[:6],
		a.TotalReceivedSmt, a.UsedSmt, a.LockedSmt, a.NeedSmt)
}

/*
AccountAddSmt account receive new deposit,amount must be positive
*/
func (m *ModelDB) AccountAddSmt(addr common.Address, amount *big.Int) {
	var a = &Account{}
	err := m.db.One("Address", addr.String(), a)
	if err == storm.ErrNotFound {
		a = &Account{
			Address:          addr.String(),
			TotalReceivedSmt: big.NewInt(0),
			UsedSmt:          big.NewInt(0),
			LockedSmt:        big.NewInt(0),
			NeedSmt:          big.NewInt(0),
		}
		m.lock.Lock()
		m.db.Save(a)
		m.lock.Unlock()
	}
	a.TotalReceivedSmt.Add(a.TotalReceivedSmt, amount)
	m.accountSanity(a)
	err = m.db.UpdateField(a, "TotalReceivedSmt", a.TotalReceivedSmt)
	if err != nil {
		panic(fmt.Sprintf("save account err %s", err))
	}
	log.Info(fmt.Sprintf("receive smt %s,now total=%s", amount, a.TotalReceivedSmt))
}
func (m *ModelDB) accountUpdateNeedSmt(addr common.Address, amount *big.Int) {
	a := m.AccountGetAccount(addr)
	a.NeedSmt = new(big.Int).Set(amount)
	m.accountSanity(a)
	err := m.db.Save(a)
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
func (m *ModelDB) AccountIsBalanceEnough(addr common.Address) bool {
	a := m.AccountGetAccount(addr)
	m.accountSanity(a)
	av := AccountAvailable(a)
	return av.Cmp(a.NeedSmt) >= 0
}

//AccountLockSmt returns account Locked smt,which means there are tx running
func (m *ModelDB) AccountLockSmt(addr common.Address, amount *big.Int) error {
	a := m.AccountGetAccount(addr)
	av := AccountAvailable(a)
	if av.Cmp(amount) < 0 {
		return fmt.Errorf("balance not enough,availabe=%s,amount=%s", av, amount)
	}
	if a.NeedSmt.Cmp(amount) < 0 {
		return fmt.Errorf("need smt smaller, need=%s,amount=%s", a.NeedSmt, amount)
	}
	a.LockedSmt.Add(a.LockedSmt, amount)
	a.NeedSmt.Sub(a.NeedSmt, amount)
	m.accountSanity(a)
	err := m.db.Save(a)
	return err
}

//AccountUnlockSmt tx failed
func (m *ModelDB) AccountUnlockSmt(addr common.Address, amount *big.Int) error {
	a := m.AccountGetAccount(addr)
	if a.LockedSmt.Cmp(amount) < 0 {
		return fmt.Errorf("error unlock smt ,unlock amount=%s,locked=%s", amount, a.LockedSmt)
	}
	a.LockedSmt.Sub(a.LockedSmt, amount)
	m.accountSanity(a)
	err := m.db.Save(a)
	return err
}

//AccountUseSmt tx success
func (m *ModelDB) AccountUseSmt(addr common.Address, amount *big.Int) error {
	a := m.AccountGetAccount(addr)
	if a.LockedSmt.Cmp(amount) < 0 {
		return fmt.Errorf("error unlock smt ,unlock amount=%s,locked=%s", amount, a.LockedSmt)
	}
	a.LockedSmt.Sub(a.LockedSmt, amount)
	a.UsedSmt.Add(a.UsedSmt, amount)
	m.accountSanity(a)
	err := m.db.Save(a)
	return err
}

//AccountGetAccount returns account info
func (m *ModelDB) AccountGetAccount(addr common.Address) *Account {
	var a = &Account{}
	err := m.db.One("Address", addr.String(), a)
	if err == storm.ErrNotFound {
		a = &Account{
			Address:          addr.String(),
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

func (m *ModelDB) accountSanity(a *Account) {
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
