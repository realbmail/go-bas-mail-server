package bmaildb

import (
	"encoding/json"
	"errors"
	"github.com/BASChain/go-bas-mail-server/config"
	"github.com/BASChain/go-bmail-account"
	"github.com/google/uuid"
	"github.com/kprc/nbsnetwork/db"
	"github.com/kprc/nbsnetwork/tools"
	"sync"
)

type BMMailContentDB struct {
	db.NbsDbInter
	dbLock sync.Mutex
	cursor *db.DBCusor
}

var (
	mailContentStore     *BMMailContentDB
	mailContentStoreLock sync.Mutex
)

type MailContentMeta struct {
	Eid        uuid.UUID     `json:"-"`
	From       string        `json:"from"`
	FromAddr   bmail.Address `json:"from_addr"`
	To         string        `json:"to"`
	ToAddr     bmail.Address `json:"to_addr"`
	CreateTime int64         `json:"create_time"`
	RefCnt     int           `json:"ref_cnt"`
}

func newBMMailContentDb() *BMMailContentDB {
	cfg := config.GetBMSCfg()

	db := db.NewFileDb(cfg.GetMailContentDBPath()).Load()

	return &BMMailContentDB{NbsDbInter: db}
}

func GetBMMailContentDb() *BMMailContentDB {
	if mailContentStore == nil {
		mailContentStoreLock.Lock()
		defer mailContentStoreLock.Unlock()

		if mailContentStore == nil {
			mailContentStore = newBMMailContentDb()
		}
	}

	return mailContentStore
}

func (mcdb *BMMailContentDB) Insert(eid uuid.UUID, from string, fromAddr bmail.Address, to string, toAddr bmail.Address) error {
	mcdb.dbLock.Lock()
	defer mcdb.dbLock.Unlock()

	if _, err := mcdb.NbsDbInter.Find(eid.String()); err == nil {
		return errors.New("mail exists")
	}

	sm := &MailContentMeta{Eid: eid, From: from, FromAddr: fromAddr, To: to, ToAddr: toAddr}
	sm.CreateTime = tools.GetNowMsTime()

	if b, err := json.Marshal(*sm); err != nil {
		return err
	} else {
		return mcdb.NbsDbInter.Insert(eid.String(), string(b))
	}
}

func (mcdb *BMMailContentDB) IncRef(eid uuid.UUID) error {
	mcdb.dbLock.Lock()
	defer mcdb.dbLock.Unlock()

	if sm, err := mcdb.NbsDbInter.Find(eid.String()); err != nil {
		return err
	} else {
		mcm := &MailContentMeta{}
		if err = json.Unmarshal([]byte(sm), mcm); err != nil {
			return err
		}
		mcm.RefCnt++

		var bsm []byte
		bsm, err = json.Marshal(*mcm)
		if err != nil {
			return err
		}

		mcdb.NbsDbInter.Update(eid.String(), string(bsm))
	}

	return nil
}

func (mcdb *BMMailContentDB) Delete(eid uuid.UUID) error {
	mcdb.dbLock.Lock()
	defer mcdb.dbLock.Unlock()

	if sm, err := mcdb.NbsDbInter.Find(eid.String()); err != nil {
		return nil
	} else {
		mcm := &MailContentMeta{}
		if err = json.Unmarshal([]byte(sm), mcm); err != nil {
			return err
		}
		mcm.RefCnt--
		if mcm.RefCnt <= 0 {
			mcdb.NbsDbInter.Delete(eid.String())
			return nil
		}

		var bsm []byte
		bsm, err = json.Marshal(*mcm)
		if err != nil {
			return err
		}

		mcdb.NbsDbInter.Update(eid.String(), string(bsm))
	}

	return nil

}

func (mcdb *BMMailContentDB) Save() {
	mcdb.dbLock.Lock()
	defer mcdb.dbLock.Unlock()

	mcdb.NbsDbInter.Save()

}

func (mcdb *BMMailContentDB) Iterator() {
	mcdb.dbLock.Lock()
	defer mcdb.dbLock.Unlock()

	mcdb.cursor = mcdb.NbsDbInter.DBIterator()

}

func (mcdb *BMMailContentDB) Next() (key string, meta *MailContentMeta, r1 error) {
	if mcdb.cursor == nil {
		return
	}
	mcdb.dbLock.Lock()
	//s.dbLock.Unlock()
	k, v := mcdb.cursor.Next()
	if k == "" {
		mcdb.dbLock.Unlock()
		return "", nil, nil
	}
	mcdb.dbLock.Unlock()
	meta = &MailContentMeta{}

	if err := json.Unmarshal([]byte(v), meta); err != nil {
		return "", nil, err
	}

	meta.Eid = uuid.MustParse(k)

	key = k

	return
}