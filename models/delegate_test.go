package models

import (
	"testing"

	"github.com/SmartMeshFoundation/SmartRaiden/utils"
)

func TestModelDB_DelegateNewDelegate(t *testing.T) {
	m := SetupTestDb(t)
	defer m.CloseDB()
	c := &ChannelFor3rd{
		ChannelAddress: utils.NewRandomAddress().String(),
	}
	addr := utils.NewRandomAddress()
	err := m.DelegateNewDelegate(c, addr)
	if err != nil {
		t.Error(err)
		return
	}
	err = m.DelegateNewDelegate(c, addr)
	if err == nil {
		t.Error(err)
		return
	}
	c.UpdateTransfer.Nonce = 2
	err = m.DelegateNewDelegate(c, addr)
	if err != nil {
		t.Error(err)
		return
	}
	err = m.DelegateNewDelegate(c, utils.NewRandomAddress())
	if err != nil {
		t.Error(err)
		return
	}
	err = m.MarkDelegateRunning(c.ChannelAddress, addr)
	if err != nil {
		t.Error(err)
		return
	}
	c.UpdateTransfer.Nonce = 3
	err = m.DelegateNewDelegate(c, addr)
	if err == nil {
		t.Error(err)
		return
	}
	d := m.DelegatetGet(c.ChannelAddress, addr)
	d.Status = DelegateStatusSuccessFinished
	m.DelegateSave(d)

	c.UpdateTransfer.Nonce = 1
	err = m.DelegateNewDelegate(c, addr)
	if err != nil {
		t.Error(err)
		return
	}
}
