package dbcounter

import (
	"fmt"
	"sync/atomic"

	"github.com/sesanetwork/go-sesa/log"

	"github.com/sesanetwork/go-vassalo/sesadb"
)

type DBProducer struct {
	sesadb.IterableDBProducer
	warn bool
}

type Iterator struct {
	sesadb.Iterator
	itCounter *int64
	start     []byte
	prefix    []byte
}

type Snapshot struct {
	sesadb.Snapshot
	snCounter *int64
}

type Store struct {
	sesadb.Store
	name      string
	snCounter int64
	itCounter int64
	warn      bool
}

func Wrap(db sesadb.IterableDBProducer, warn bool) sesadb.IterableDBProducer {
	return &DBProducer{db, warn}
}

func WrapStore(s sesadb.Store, name string, warn bool) *Store {
	return &Store{
		Store: s,
		name:  name,
		warn:  warn,
	}
}

func (ds *Store) Close() error {
	itCounter, snCounter := atomic.LoadInt64(&ds.itCounter), atomic.LoadInt64(&ds.snCounter)
	if itCounter != 0 || snCounter != 0 {
		err := fmt.Errorf("%s DB leak: %d iterators, %d snapshots", ds.name, itCounter, snCounter)
		if ds.warn {
			log.Warn("Possible " + err.Error())
		} else {
			return err
		}
	}
	return ds.Store.Close()
}

func (ds *Snapshot) Release() {
	atomic.AddInt64(ds.snCounter, -1)
	ds.Snapshot.Release()
}

func (ds *Store) NewIterator(prefix []byte, start []byte) sesadb.Iterator {
	atomic.AddInt64(&ds.itCounter, 1)
	return &Iterator{
		Iterator:  ds.Store.NewIterator(prefix, start),
		itCounter: &ds.itCounter,
		start:     start,
		prefix:    prefix,
	}
}

func (it *Iterator) Release() {
	atomic.AddInt64(it.itCounter, -1)
	it.Iterator.Release()
}

func (ds *Store) GetSnapshot() (sesadb.Snapshot, error) {
	atomic.AddInt64(&ds.snCounter, 1)
	snapshot, err := ds.Store.GetSnapshot()
	if err != nil {
		return nil, err
	}
	return &Snapshot{
		Snapshot:  snapshot,
		snCounter: &ds.snCounter,
	}, nil
}

func (db *DBProducer) OpenDB(name string) (sesadb.Store, error) {
	s, err := db.IterableDBProducer.OpenDB(name)
	if err != nil {
		return nil, err
	}
	return WrapStore(s, name, db.warn), nil
}
