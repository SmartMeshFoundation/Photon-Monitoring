package models

import (
	"testing"

	"github.com/SmartMeshFoundation/Photon-Monitoring/params"

	"github.com/stretchr/testify/assert"

	"github.com/SmartMeshFoundation/Photon/utils"
)

func TestModelDB_DelegateNewDelegate(t *testing.T) {
	m := SetupTestDb(t)
	defer m.CloseDB()
	c := &ChannelFor3rd{
		ChannelIdentifier: utils.NewRandomHash(),
		OpenBlockNumber:   3,
	}
	addr := utils.NewRandomAddress()
	err := m.DelegateNewOrUpdateDelegate(c, addr)
	if err != nil {
		t.Error(err)
		return
	}
	d := m.DelegatetGet(c.ChannelIdentifier, addr)
	err = m.DelegateDeleteDelegate(d)
	if err != nil {
		t.Error(err)
		return
	}
	err = m.DelegateDeleteDelegate(d)
	if err == nil {
		t.Error("cannot delete two times")
		return
	}
	//channel is recreated
	c.OpenBlockNumber = 7
	err = m.DelegateNewOrUpdateDelegate(c, addr)
	if err != nil {
		t.Error(err)
		return
	}
	c.UpdateTransfer.Nonce = 2
	err = m.DelegateNewOrUpdateDelegate(c, addr)
	if err != nil {
		t.Error(err)
		return
	}
	err = m.DelegateNewOrUpdateDelegate(c, utils.NewRandomAddress())
	if err != nil {
		t.Error(err)
		return
	}
	err = m.MarkDelegateRunning(c.ChannelIdentifier, addr)
	if err != nil {
		t.Error(err)
		return
	}
	c.UpdateTransfer.Nonce = 3
	err = m.DelegateNewOrUpdateDelegate(c, addr)
	if err == nil {
		t.Error(err)
		return
	}
	d = m.DelegatetGet(c.ChannelIdentifier, addr)
	d.Status = DelegateStatusSuccessFinished
	m.DelegateSave(d)

	c.UpdateTransfer.Nonce = 1
	err = m.DelegateNewOrUpdateDelegate(c, addr)
	if err != nil {
		t.Error(err)
		return
	}
}

func TestModelDB_DelegateNewDelegateWithPunishes(t *testing.T) {
	ast := assert.New(t)
	m := SetupTestDb(t)
	defer m.CloseDB()
	c := &ChannelFor3rd{
		ChannelIdentifier: utils.NewRandomHash(),
		OpenBlockNumber:   3,
	}
	addr := utils.NewRandomAddress()
	err := m.DelegateNewOrUpdateDelegate(c, addr)
	ast.Nil(err)
	c.Unlocks = []*Unlock{
		{
			Secret: utils.NewRandomHash(),
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
	err = m.DelegateNewOrUpdateDelegate(c, addr)
	ast.Nil(err)
	d := m.DelegatetGet(c.ChannelIdentifier, addr)
	ast.EqualValues(c.Unlocks, d.Content.Unlocks)
	ast.EqualValues(c.Punishes, d.Content.Punishes)
	c.Punishes = []*Punish{
		{
			LockHash: utils.NewRandomHash(),
		}, {
			LockHash: utils.NewRandomHash(),
		},
	}
	c.Unlocks = nil
	c.UpdateTransfer.Nonce = 2
	err = m.DelegateNewOrUpdateDelegate(c, addr)
	ast.Nil(err)
	d = m.DelegatetGet(c.ChannelIdentifier, addr)
	ast.EqualValues(len(d.Content.Unlocks), 0)
	ast.EqualValues(len(d.Content.Punishes), 4)

	//测试nonce 覆盖问题
	err = m.DelegateNewOrUpdateDelegate(c, addr)
	ast.NotNil(err)
	params.DebugMode = true
	defer func() {
		params.DebugMode = false
	}()
	err = m.DelegateNewOrUpdateDelegate(c, addr)
	ast.Nil(err)
}
