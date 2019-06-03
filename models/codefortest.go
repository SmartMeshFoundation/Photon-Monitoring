package models

import (
	"os"
	"path"
	"testing"

	"github.com/SmartMeshFoundation/Photon/log"
	"github.com/SmartMeshFoundation/Photon/utils"
)

func init() {
	log.Root().SetHandler(log.LvlFilterHandler(log.LvlTrace, utils.MyStreamHandler(os.Stderr)))
}

//SetupTestDb create a test db
func SetupTestDb(t *testing.T) (model *ModelDB) {
	dbPath := path.Join(os.TempDir(), "test.db")
	err := os.Remove(dbPath)
	err = os.Remove(dbPath + ".lock")
	model, err = OpenDb(dbPath)
	if err != nil {
		t.Error(err)
		return
	}
	//t.Log(model.db)
	return
}
