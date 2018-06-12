package smt

import (
	"fmt"
	"net/http"

	"encoding/json"
	"io/ioutil"

	"time"

	"github.com/SmartMeshFoundation/SmartRaiden-Monitoring/models"
	"github.com/SmartMeshFoundation/SmartRaiden-Monitoring/params"
	"github.com/labstack/gommon/log"
)

//Query monitor receiving smt
type Query struct {
	url      string
	db       *models.ModelDB
	quitChan chan struct{}
	from     int64
	stopped  bool
	interval time.Duration
}

//NewSmtQuery create qmt query
func NewSmtQuery(url string, db *models.ModelDB, fromBlockNumber int64) *Query {
	return &Query{
		url:      url,
		db:       db,
		from:     fromBlockNumber,
		quitChan: make(chan struct{}),
		interval: time.Second * 10,
	}
}

//Start query
func (s *Query) Start() {
	go s.loop()
}
func (s *Query) loop() {
	for {
		select {
		case <-time.After(time.Second * 10):
			s.getNewTransfer()
		case <-s.quitChan:
			return
		}
	}
}
func (s *Query) getNewTransfer() {
	res, err := http.Get(fmt.Sprintf("%s?from_block=%d", s.url, s.from))
	if err != nil {
		log.Error(fmt.Sprintf("getNewTransfer err %s", err))
		return
	}
	if s.stopped {
		return
	}
	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Error(fmt.Sprintf("read data err %s", err))
		return
	}
	var trs []*models.ReceivedTransfer
	err = json.Unmarshal(data, &trs)
	if err != nil {
		log.Error(fmt.Sprintf("unmarshal err %s", err))
		return
	}
	var maxBlock int64
	for _, tr := range trs {
		if tr.TokenAddress == params.SmtAddress && s.db.NewReceiveTransferFromReceiveTransfer(tr) {
			s.db.AccountAddSmt(tr.FromAddress, tr.Amount)
		}
		if tr.BlockNumber > maxBlock {
			maxBlock = tr.BlockNumber
		}
	}
	if maxBlock > s.from {
		s.from = maxBlock
	}
}

//Stop query
func (s *Query) Stop() {
	s.stopped = true
	close(s.quitChan)
}
