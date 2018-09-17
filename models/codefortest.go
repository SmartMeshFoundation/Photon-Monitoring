package models

import (
	"os"
	"path"
	"testing"

	"github.com/SmartMeshFoundation/SmartRaiden/log"
	"github.com/SmartMeshFoundation/SmartRaiden/utils"
)

func init() {
	log.Root().SetHandler(log.LvlFilterHandler(log.LvlTrace, utils.MyStreamHandler(os.Stderr)))
}

//SetupTestDb create a test db
func SetupTestDb(t *testing.T) (model *ModelDB) {
	dbPath := path.Join(os.TempDir(), "testxxxx.db")
	err := os.Remove(dbPath)
	err = os.Remove(dbPath + ".lock")
	model, err = OpenDb(dbPath)
	if err != nil {
		t.Error(err)
		return
	}
	t.Log(model.db)
	return
}
