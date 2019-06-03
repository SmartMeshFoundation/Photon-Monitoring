package models

import (
	"bytes"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"math/big"
	"reflect"
	"testing"

	"github.com/SmartMeshFoundation/Photon/transfer/mtree"

	"github.com/SmartMeshFoundation/Photon-Monitoring/params"

	"github.com/stretchr/testify/assert"

	"github.com/SmartMeshFoundation/Photon/utils"
)

func TestGob(t *testing.T) {
	s1 := Delegate{}
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(&s1)
	if err != nil {
		t.Error(err)
		return
	}
	encodedData := buf.Bytes()
	fmt.Printf("first\n%s", hex.Dump(encodedData))
	dec := gob.NewDecoder(bytes.NewBuffer(encodedData))
	var sb Delegate
	err = dec.Decode(&sb)
	if err != nil {
		t.Error(err)
		return
	}
	if !reflect.DeepEqual(s1, sb) {
		t.Error("not equal")
	}
	var buf2 bytes.Buffer
	enc2 := gob.NewEncoder(&buf2)
	enc2.Encode(&sb)
	encodedData2 := buf2.Bytes()
	fmt.Printf("second\n%s", hex.Dump(encodedData2))
	if !reflect.DeepEqual(encodedData, encodedData2) {
		t.Error("not equal")
	}

}

func TestModelDB_DelegateNewDelegate(t *testing.T) {
	ast := assert.New(t)
	m := SetupTestDb(t)
	defer m.CloseDB()
	m.SaveLatestBlockNumber(100)
	c := &ChannelFor3rd{
		ChannelIdentifier: utils.NewRandomHash(),
		OpenBlockNumber:   3,
	}
	addr := utils.NewRandomAddress()
	err := m.ReceiveDelegate(c, addr)
	if err != nil {
		t.Error(err)
		return
	}
	d := m.getDelegateByOriginKey(c.ChannelIdentifier, addr)
	err = m.DeleteDelegate(d.Key)
	if err != nil {
		t.Error(err)
		return
	}
	//channel is recreated
	c.OpenBlockNumber = 7
	err = m.ReceiveDelegate(c, addr)
	if err != nil {
		t.Error(err)
		return
	}
	c.UpdateTransfer.Nonce = 2
	err = m.ReceiveDelegate(c, addr)
	if err != nil {
		t.Error(err)
		return
	}
	err = m.ReceiveDelegate(c, utils.NewRandomAddress())
	if err != nil {
		t.Error(err)
		return
	}
	err = m.markDelegateRunning(c.ChannelIdentifier, addr)
	if err != nil {
		t.Error(err)
		return
	}
	c.UpdateTransfer.Nonce = 3
	//即使已经开始执行委托了,仍然可以更新unlock和punish以及AnnouceDisposed,但是不能更新updatebalanceProof
	err = m.ReceiveDelegate(c, addr)
	if err != nil {
		t.Error(err)
		return
	}
	d = m.getDelegateByOriginKey(c.ChannelIdentifier, addr)
	ast.EqualValues(d.UpdateBalanceProof().Nonce, 2) //不应该更新

	d.Status = DelegateStatusSuccessFinished
	m.UpdateObject(d)

	c.UpdateTransfer.Nonce = 1
	err = m.ReceiveDelegate(c, addr)
	if err != nil {
		t.Error(err)
		return
	}
}

func TestModelDB_DelegateNewDelegateWithPunishes(t *testing.T) {
	ast := assert.New(t)
	m := SetupTestDb(t)
	defer m.CloseDB()
	m.SaveLatestBlockNumber(100)
	c := &ChannelFor3rd{
		ChannelIdentifier: utils.NewRandomHash(),
		OpenBlockNumber:   3,
	}
	addr := utils.NewRandomAddress()
	err := m.ReceiveDelegate(c, addr)
	ast.Nil(err)
	c.Unlocks = []*Unlock{
		{
			Lock: &mtree.Lock{
				LockSecretHash: utils.NewRandomHash(),
				Amount:         big.NewInt(30),
			},
		},
	}
	c.Punishes = []*Punish{
		{
			LockHash: utils.NewRandomHash(),
		}, {
			LockHash: utils.NewRandomHash(),
		},
	}
	c.UpdateTransfer.Nonce = 1
	err = m.ReceiveDelegate(c, addr)
	ast.Nil(err)
	d := m.getDelegateByOriginKey(c.ChannelIdentifier, addr)
	dus := d.Unlocks()
	dps, _ := m.GetDelegatePunishListByDelegateKey(d.Key)
	ast.EqualValues(len(c.Unlocks), len(dus))
	ast.EqualValues(len(c.Punishes), len(dps))
	c.Punishes = []*Punish{
		{
			LockHash: utils.NewRandomHash(),
		}, {
			LockHash: utils.NewRandomHash(),
		},
	}
	c.Unlocks = nil
	c.UpdateTransfer.Nonce = 2
	err = m.ReceiveDelegate(c, addr)
	ast.Nil(err)
	d = m.getDelegateByOriginKey(c.ChannelIdentifier, addr)
	dus = d.Unlocks()
	dps, _ = m.GetDelegatePunishListByDelegateKey(d.Key)
	ast.EqualValues(len(dus), 0)
	ast.EqualValues(len(dps), 4)

	//测试AnnouceDisposed
	c.AnnouceDisposed = []*AnnouceDisposed{
		{utils.NewRandomHash()},
		{utils.NewRandomHash()},
	}
	c.UpdateTransfer.Nonce = 3
	err = m.ReceiveDelegate(c, addr)
	ast.Nil(err)
	d = m.getDelegateByOriginKey(c.ChannelIdentifier, addr)
	das, _ := m.GetDelegateAnnounceDisposeListByDelegateKey(d.Key)
	ast.EqualValues(len(das), 2)

	c.AnnouceDisposed = []*AnnouceDisposed{
		{utils.NewRandomHash()},
		{utils.NewRandomHash()},
	}
	c.UpdateTransfer.Nonce = 4
	err = m.ReceiveDelegate(c, addr)
	ast.Nil(err)
	d = m.getDelegateByOriginKey(c.ChannelIdentifier, addr)
	das, _ = m.GetDelegateAnnounceDisposeListByDelegateKey(d.Key)
	ast.EqualValues(len(das), 4)

	//测试nonce 覆盖问题,nonce可以相同,但是不能变小
	c.UpdateTransfer.Nonce = 3
	err = m.ReceiveDelegate(c, addr)
	ast.NotNil(err)
	params.DebugMode = true
	defer func() {
		params.DebugMode = false
	}()
	err = m.ReceiveDelegate(c, addr)
	ast.Nil(err)
}
