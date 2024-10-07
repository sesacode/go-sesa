package udb2ethdb

import (
	"github.com/sesanetwork/go-vassalo/sesadb"
	"github.com/sesanetwork/go-sesa/ethdb"
)

type Adapter struct {
	sesadb.Store
}

func (db *Adapter) Stat(property string) (string, error) {
	//TODO implement me
	panic("implement me")
}

var _ ethdb.KeyValueStore = (*Adapter)(nil)

func Wrap(v sesadb.Store) *Adapter {
	return &Adapter{v}
}

// batch is a write-only memory batch that commits changes to its host
// database when Write is called. A batch cannot be used concurrently.
type batch struct {
	sesadb.Batch
}

// Replay replays the batch contents.
func (b *batch) Replay(w ethdb.KeyValueWriter) error {
	return b.Batch.Replay(w)
}

// NewBatch creates a write-only key-value store that buffers changes to its host
// database until a final write is called.
func (db *Adapter) NewBatch() ethdb.Batch {
	return &batch{db.Store.NewBatch()}
}

// NewIterator creates a binary-alphabetical iterator over a subset
// of database content with a particular key prefix, starting at a particular
// initial key (or after, if it does not exist).
func (db *Adapter) NewIterator(prefix []byte, start []byte) ethdb.Iterator {
	return db.Store.NewIterator(prefix, start)
}
