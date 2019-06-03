package models

import (
	"math/big"

	"github.com/jinzhu/gorm"

	"fmt"

	"github.com/SmartMeshFoundation/Photon/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/labstack/gommon/log"
)

//Account save available smt
type Account struct {
	Address          []byte
	TotalReceivedSmt *big.Int //单增
	UsedSmt          *big.Int //单增
	LockedSmt        *big.Int //执行之前先锁定,成功的话,则减去响应的 smt,否则应该退还.
	// TODO NeedSmt该值会在一个锁对应的DelegateUnlock及DelegateAnnounceDispose同时存在时产生误差,暂时没处理
	NeedSmt *big.Int //还需要多少 smt, 才能执行所有提交的 tx,供查询,不是计费的依据,
}

func (a *Account) String() string {
	return fmt.Sprintf("addr=%s,total=%s,used=%s,locked=%s,need=%s", common.BytesToAddress(a.Address).String(),
		a.TotalReceivedSmt, a.UsedSmt, a.LockedSmt, a.NeedSmt)
}
func (a *Account) toSerialization() *accountSerialization {
	return &accountSerialization{
		Address:               a.Address,
		TotalReceivedSmtBytes: a.TotalReceivedSmt.Bytes(),
		UsedSmtBytes:          a.UsedSmt.Bytes(),
		LockedSmtBytes:        a.LockedSmt.Bytes(),
		NeedSmtBytes:          a.NeedSmt.Bytes(),
	}
}

type accountSerialization struct {
	Address               []byte `gorm:"primary_key"`
	TotalReceivedSmtBytes []byte
	UsedSmtBytes          []byte
	LockedSmtBytes        []byte
	NeedSmtBytes          []byte
}

func (al *accountSerialization) toAccount() *Account {
	return &Account{
		Address:          al.Address,
		TotalReceivedSmt: new(big.Int).SetBytes(al.TotalReceivedSmtBytes),
		UsedSmt:          new(big.Int).SetBytes(al.UsedSmtBytes),
		LockedSmt:        new(big.Int).SetBytes(al.LockedSmtBytes),
		NeedSmt:          new(big.Int).SetBytes(al.NeedSmtBytes),
	}
}

/*
AccountAddSmt account receive new deposit,amount must be positive
*/
func (model *ModelDB) AccountAddSmt(addr common.Address, amount *big.Int) {
	var al = &accountSerialization{}
	al.Address = addr[:]
	err := model.db.Where(al).Find(al).Error
	if err == gorm.ErrRecordNotFound {
		model.lock.Lock()
		err = model.db.Save(al).Error
		if err != nil {
			log.Error(fmt.Sprintf("AccountAddSmt new account err %s", err))
		}
		model.lock.Unlock()
	}
	a := al.toAccount()
	a.TotalReceivedSmt.Add(a.TotalReceivedSmt, amount)
	model.accountSanity(a)
	al = a.toSerialization()
	err = model.db.Model(al).UpdateColumn("TotalReceivedSmtBytes", al.TotalReceivedSmtBytes).Error
	if err != nil {
		panic(fmt.Sprintf("save account err %s", err.Error()))
	}
	log.Info(fmt.Sprintf("receive smt %s,now total=%s", amount, a.TotalReceivedSmt))
}

func (model *ModelDB) accountUpdateNeedSmt(addr common.Address, amount *big.Int) {
	a := model.AccountGetAccount(addr)
	a.NeedSmt = new(big.Int).Set(amount)
	model.accountSanity(a)
	err := model.db.Save(a.toSerialization()).Error
	if err != nil {
		panic(fmt.Sprintf("save account err %s", err.Error()))
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
	err := model.db.Save(a.toSerialization()).Error
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
	err := model.db.Save(a.toSerialization()).Error
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
	err := model.db.Save(a.toSerialization()).Error
	return err
}

//AccountGetAccount returns account info
func (model *ModelDB) AccountGetAccount(addr common.Address) *Account {
	var a = &accountSerialization{}
	a.Address = addr[:]
	err := model.db.Where(a).Find(a).Error
	if err == gorm.ErrRecordNotFound {
		err = nil
	}
	if err != nil {
		log.Error(err.Error())
	}
	return a.toAccount()
}

//GetAccountInTx returns account info with t
func GetAccountInTx(tx *gorm.DB, addr common.Address) *Account {
	var a = &accountSerialization{}
	a.Address = addr[:]
	err := tx.Where(a).Find(a).Error
	if err == gorm.ErrRecordNotFound {
		err = nil
	}
	if err != nil {
		log.Error(err.Error())
	}
	return a.toAccount()
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
