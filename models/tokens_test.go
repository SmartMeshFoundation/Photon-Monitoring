package models

import (
	"os"
	"path"
	"testing"

	"github.com/SmartMeshFoundation/Photon/log"
	"github.com/SmartMeshFoundation/Photon/utils"
	"github.com/stretchr/testify/assert"
)

func init() {
	log.Root().SetHandler(log.LvlFilterHandler(log.LvlTrace, utils.MyStreamHandler(os.Stderr)))
}

var dbPath string

func setupDb(t *testing.T) (model *ModelDB) {
	dbPath = path.Join(os.TempDir(), "test.db")
	os.Remove(dbPath)
	os.Remove(dbPath + ".lock")
	model, err := OpenDb(dbPath)
	if err != nil {
		t.Error(err)
		return
	}
	t.Log(model.db)
	return
}
func TestToken(t *testing.T) {
	model := setupDb(t)
	defer func() {
		model.CloseDB()
	}()
	ts, err := model.GetAllTokens()
	if len(ts) > 0 {
		t.Error("should not found")
	}
	if len(ts) != 0 {
		t.Error("should be empty")
	}
	var am = make(AddressMap)
	t1 := utils.NewRandomAddress()
	am[t1] = utils.NewRandomAddress()
	err = model.AddToken(t1, am[t1])
	if err != nil {
		t.Error(err)
	}
	am2, _ := model.GetAllTokens()
	assert.EqualValues(t, am, am2)
	t2 := utils.NewRandomAddress()
	am[t2] = utils.NewRandomAddress()
	err = model.AddToken(t2, am[t2])
	if err != nil {
		t.Error(err)
	}
	am2, _ = model.GetAllTokens()
	assert.EqualValues(t, am, am2)

}
