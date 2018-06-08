package models

import (
	"fmt"

	"sync"

	"time"

	"os"

	"encoding/gob"

	"github.com/SmartMeshFoundation/SmartRaiden/log"
	"github.com/asdine/storm"
	gobcodec "github.com/asdine/storm/codec/gob"
	bolt "github.com/coreos/bbolt"
)

//ModelDB is thread safe
type ModelDB struct {
	db    *storm.DB
	lock  sync.Mutex
	mlock sync.Mutex
	Name  string
}

var bucketMeta = "meta"

const dbVersion = 1

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
	needCreateDb := !FileExists(dbPath)
	var ver int
	model.db, err = storm.Open(dbPath, storm.BoltOptions(os.ModePerm, &bolt.Options{Timeout: 1 * time.Second}), storm.Codec(gobcodec.Codec))
	if err != nil {
		err = fmt.Errorf("cannot create or open db:%s,makesure you have write permission err:%v", dbPath, err)
		log.Crit(err.Error())
		return
	}
	model.Name = dbPath
	if needCreateDb {
		err = model.db.Set(bucketMeta, "version", dbVersion)
		if err != nil {
			log.Crit(fmt.Sprintf("unable to create db "))
			return
		}
		model.initDb()
		model.MarkDbOpenedStatus()
	} else {
		err = model.db.Get(bucketMeta, "version", &ver)
		if err != nil {
			log.Crit(fmt.Sprintf("wrong db file format "))
			return
		}
		if ver != dbVersion {
			log.Crit("db version not match")
		}
		var closeFlag bool
		err = model.db.Get(bucketMeta, "close", &closeFlag)
		if err != nil {
			log.Crit(fmt.Sprintf("db meta data error"))
		}
		if closeFlag != true {
			log.Error("database not closed  last..., try to restore?")
		}
	}

	return
}

/*
MarkDbOpenedStatus First step   open the database
Second step detection for normal closure IsDbCrashedLastTime
Third step  recovers the data according to the second step
Fourth step mark the database for processing the data normally. MarkDbOpenedStatus
*/
func (m *ModelDB) MarkDbOpenedStatus() {
	err := m.db.Set(bucketMeta, "close", false)
	if err != nil {
		log.Error(fmt.Sprintf("MarkDbOpenedStatus err %s", err))
	}
}

//IsDbCrashedLastTime return true when quit but  db not closed
func (m *ModelDB) IsDbCrashedLastTime() bool {
	var closeFlag bool
	err := m.db.Get(bucketMeta, "close", &closeFlag)
	if err != nil {
		log.Crit(fmt.Sprintf("db meta data error"))
	}
	return closeFlag != true
}

//CloseDB close db
func (m *ModelDB) CloseDB() {
	m.lock.Lock()
	err := m.db.Set(bucketMeta, "close", true)
	if err != nil {
		log.Error(fmt.Sprintf("set close err %s", err))
	}
	err = m.db.Close()
	if err != nil {
		log.Error(fmt.Sprintf(" close err %s", err))
	}
	m.lock.Unlock()
}

func init() {
	gob.Register(&Account{})
	gob.Register(&Delegate{})
	gob.Register(ReceivedTransfer{})
	//gob.Register(&ModelDB{}) //cannot save and restore by gob,only avoid noise by gob
}

func (m *ModelDB) initDb() {
	m.db.Init(&Account{})
	m.db.Init(&Delegate{})
	m.db.Init(&ReceivedTransfer{})
}
