package models

import (
	"math/big"
	"testing"

	"github.com/SmartMeshFoundation/Photon/utils"
	"github.com/stretchr/testify/assert"
)

func TestModelDB_AccountAddSmt(t *testing.T) {
	m := SetupTestDb(t)
	defer m.CloseDB()
	addr := utils.NewRandomAddress()
	m.AccountAddSmt(addr, big.NewInt(20))
	m.accountUpdateNeedSmt(addr, big.NewInt(20))
	err := m.AccountLockSmt(addr, big.NewInt(30))
	if err == nil {
		t.Errorf("lock amount too large ,should fail")
		return
	}
	err = m.AccountLockSmt(addr, big.NewInt(10))
	if err != nil {
		t.Errorf("lock should success %s", err)
		return
	}
	err = m.AccountLockSmt(addr, big.NewInt(10))
	if err != nil {
		t.Errorf("lock should success %s", err)
		return
	}
	err = m.AccountUnlockSmt(addr, big.NewInt(10))
	if err != nil {
		t.Errorf("unlcok should success %s", err)
		return
	}
	err = m.AccountUseSmt(addr, big.NewInt(20))
	if err == nil {
		t.Error("use too much")
		return
	}
	err = m.AccountUseSmt(addr, big.NewInt(10))
	if err != nil {
		t.Errorf("use shoulde  success %s", err)
	}
	a := m.AccountGetAccount(addr)
	assert.EqualValues(t, a.TotalReceivedSmt, big.NewInt(20))
	assert.EqualValues(t, a.UsedSmt, big.NewInt(10))
	assert.EqualValues(t, a.LockedSmt, big.NewInt(0))
}
