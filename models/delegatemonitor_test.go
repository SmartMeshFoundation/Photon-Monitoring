package models

import "testing"

func TestModelDB_DelegateMonitorAdd(t *testing.T) {
	m := SetupTestDb(t)
	_, err := m.DelegateMonitorGet(3)
	if err != nil {
		t.Error(err)
		return
	}
	err = m.DelegateMonitorAdd(3, "123")
	if err != nil {
		t.Error(err)
		return
	}
	ds, err := m.DelegateMonitorGet(3)
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
