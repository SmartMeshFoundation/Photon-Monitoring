package models

import "github.com/asdine/storm"

const bucketDelegateMonitor = "bucketDelegateMonitor"

//DelegateMonitorGet returna all delegates which should be executed at `blockNumber`
func (model *ModelDB) DelegateMonitorGet(blockNumber int64) (ds []string, err error) {
	err = model.db.Get(bucketDelegateMonitor, blockNumber, &ds)
	if err == storm.ErrNotFound {
		err = nil
	}
	return
}

//DelegateMonitorAdd add a new token to db,
func (model *ModelDB) DelegateMonitorAdd(blockNumber int64, delegetKey string) error {
	var ds []string
	err := model.db.Get(bucketDelegateMonitor, blockNumber, &ds)
	if err != nil && err != storm.ErrNotFound {
		return err
	}
	ds = append(ds, delegetKey)

	err = model.db.Set(bucketDelegateMonitor, blockNumber, ds)
	return err
}
