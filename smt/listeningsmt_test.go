package smt

import (
	"testing"
	"time"

	"github.com/SmartMeshFoundation/SmartRaiden-Monitoring/models"
)

func TestNewSmtQuery(t *testing.T) {
	s := NewSmtQuery("http://127.0.0.1:5001/api/1/queryreceivedtransfer", models.SetupTestDb(t), 0)
	s.Start()
	time.Sleep(time.Second * 5)
	s.Stop()
}
