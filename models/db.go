package models

import (
	"fmt"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres" //for gorm
	_ "github.com/jinzhu/gorm/dialects/sqlite"   //for gorm

	"sync"

	"os"

	"encoding/gob"

	"github.com/SmartMeshFoundation/Photon/log"
)

//ModelDB is thread safe
type ModelDB struct {
	db    *gorm.DB
	lock  sync.Mutex
	mlock sync.Mutex
	Name  string
}

func newModelDB() (db *ModelDB) {
	return &ModelDB{}

}

// FileExists reports whether the named file or directory exists.
func FileExists(name string) bool {
	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

//OpenDb open or create a bolt db at dbPath
func OpenDb(dbPath string) (model *ModelDB, err error) {
	model = newModelDB()
	model.db, err = gorm.Open("sqlite3", dbPath)
	//model.db, err = storm.Open(dbPath, storm.BoltOptions(os.ModePerm, &bolt.Options{Timeout: 1 * time.Second}), storm.Codec(gobcodec.Codec))
	if err != nil {
		err = fmt.Errorf("cannot create or open db:%s,makesure you have write permission err:%v", dbPath, err)
		log.Crit(err.Error())
		return
	}
	model.Name = dbPath

	model.db.AutoMigrate(&Delegate{})
	model.db.AutoMigrate(&DelegatePunish{})
	model.db.AutoMigrate(&DelegateAnnounceDispose{})
	model.db.AutoMigrate(&accountSerialization{})
	model.db.AutoMigrate(&DelegateMonitor{})
	model.db.AutoMigrate(&ReceivedTransfer{})
	model.db.AutoMigrate(&lastBlockNumber{})

	return
}

//CloseDB close db
func (model *ModelDB) CloseDB() {
	model.lock.Lock()
	err := model.db.Close()
	if err != nil {
		log.Error(fmt.Sprintf(" close err %s", err))
	}
	model.lock.Unlock()
}

// UpdateObject panic if err
func (model *ModelDB) UpdateObject(o interface{}) {
	err := model.db.Save(o).Error
	if err != nil {
		panic(err)
	}
}

func init() {
	gob.Register(&Account{})
	//gob.Register(ReceivedTransfer{})
	//gob.Register(&ModelDB{}) //cannot save and restore by gob,only avoid noise by gob
}
