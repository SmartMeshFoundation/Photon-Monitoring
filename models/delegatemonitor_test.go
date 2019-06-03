package models

import (
	"testing"

	"github.com/SmartMeshFoundation/Photon-Monitoring/params"

	"github.com/SmartMeshFoundation/Photon/utils"
)

func TestModelDB_DelegateMonitorAdd(t *testing.T) {
	m := SetupTestDb(t)
	startBlock := int64(10000 - params.RevealTimeout)
	_, err := m.GetDelegateMonitorList(startBlock)
	if err != nil {
		t.Error(err)
		return
	}
	m.AddDelegateMonitor(&Delegate{
		SettleBlockNumber: 10000,
		Key:               utils.NewRandomHash().Bytes(),
	})
	ds, err := m.GetDelegateMonitorList(startBlock)
	if err != nil {
		t.Error(err)
		return
	}
	if len(ds) != 1 {
		t.Logf("ds=%#v", ds)
		t.Error("length err")
		return
	}
}
