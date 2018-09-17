package models

import (
	"testing"

	"github.com/SmartMeshFoundation/SmartRaiden/utils"
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
