package evmstore

import (
	"github.com/syndtr/goleveldb/leveldb/opt"

	"github.com/sesanetwork/go-vassalo/sesadb/batched"

	"github.com/sesanetwork/go-sesa/sesa/genesis"
	"github.com/sesanetwork/go-sesa/utils/adapters/ethdb2udb"
	"github.com/sesanetwork/go-sesa/utils/dbutil/autocompact"
)

// ApplyGenesis writes initial state.
func (s *Store) ApplyGenesis(g genesis.Genesis) (err error) {
	db := batched.Wrap(autocompact.Wrap2M(ethdb2udb.Wrap(s.EvmDb), opt.GiB, 16*opt.GiB, true, "evm"))
	g.RawEvmItems.ForEach(func(key, value []byte) bool {
		err = db.Put(key, value)
		if err != nil {
			return false
		}
		return true
	})
	if err != nil {
		return err
	}
	return db.Write()
}

func (s *Store) WrapTablesAsBatched() (unwrap func()) {
	origTables := s.table

	batchedTxs := batched.Wrap(s.table.Txs)
	s.table.Txs = batchedTxs

	batchedTxPositions := batched.Wrap(s.table.TxPositions)
	s.table.TxPositions = batchedTxPositions

	unwrapLogs := s.EvmLogs.WrapTablesAsBatched()

	batchedReceipts := batched.Wrap(autocompact.Wrap2M(s.table.Receipts, opt.GiB, 16*opt.GiB, false, "receipts"))
	s.table.Receipts = batchedReceipts
	return func() {
		_ = batchedTxs.Flush()
		_ = batchedTxPositions.Flush()
		_ = batchedReceipts.Flush()
		unwrapLogs()
		s.table = origTables
	}
}
