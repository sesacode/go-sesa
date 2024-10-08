package integration

import (
	"io"
	"io/ioutil"
	"os"
	"path"
	"strings"

	"github.com/sesanetwork/go-vassalo/hash"
	"github.com/sesanetwork/go-vassalo/native/dag"
	"github.com/sesanetwork/go-vassalo/sesadb"
	"github.com/sesanetwork/go-vassalo/sesadb/flaggedproducer"
	"github.com/sesanetwork/go-vassalo/sesadb/flushable"
	"github.com/sesanetwork/go-vassalo/sesadb/leveldb"
	"github.com/sesanetwork/go-vassalo/sesadb/multidb"
	"github.com/sesanetwork/go-vassalo/sesadb/pebble"
	"github.com/sesanetwork/go-vassalo/utils/fmtfilter"
	"github.com/sesanetwork/go-sesa/cmd/utils"
	"github.com/sesanetwork/go-sesa/log"
	"github.com/sesanetwork/go-sesa/metrics"

	"github.com/sesanetwork/go-sesa/gossip"
	"github.com/sesanetwork/go-sesa/utils/dbutil/asyncflushproducer"
	"github.com/sesanetwork/go-sesa/utils/dbutil/dbcounter"
)

type DBsConfig struct {
	Routing       RoutingConfig
	RuntimeCache  DBsCacheConfig
	GenesisCache  DBsCacheConfig
	MigrationMode string
}

type DBCacheConfig struct {
	Cache   uint64
	Fdlimit uint64
}

type DBsCacheConfig struct {
	Table map[string]DBCacheConfig
}

func SupportedDBs(chaindataDir string, cfg DBsCacheConfig) (map[multidb.TypeName]sesadb.IterableDBProducer, map[multidb.TypeName]sesadb.FullDBProducer) {
	if chaindataDir == "inmemory" || chaindataDir == "" {
		chaindataDir, _ = ioutil.TempDir("", "sesa-tmp")
	}
	cacher, err := DbCacheFdlimit(cfg)
	if err != nil {
		utils.Fatalf("Failed to create DB cacher: %v", err)
	}

	leveldbFsh := dbcounter.Wrap(leveldb.NewProducer(path.Join(chaindataDir, "leveldb-fsh"), cacher), true)
	leveldbFlg := dbcounter.Wrap(leveldb.NewProducer(path.Join(chaindataDir, "leveldb-flg"), cacher), true)
	leveldbDrc := dbcounter.Wrap(leveldb.NewProducer(path.Join(chaindataDir, "leveldb-drc"), cacher), true)
	pebbleFsh := dbcounter.Wrap(pebble.NewProducer(path.Join(chaindataDir, "pebble-fsh"), cacher), true)
	pebbleFlg := dbcounter.Wrap(pebble.NewProducer(path.Join(chaindataDir, "pebble-flg"), cacher), true)
	pebbleDrc := dbcounter.Wrap(pebble.NewProducer(path.Join(chaindataDir, "pebble-drc"), cacher), true)

	if metrics.Enabled {
		leveldbFsh = WrapDatabaseWithMetrics(leveldbFsh)
		leveldbFlg = WrapDatabaseWithMetrics(leveldbFlg)
		leveldbDrc = WrapDatabaseWithMetrics(leveldbDrc)
		pebbleFsh = WrapDatabaseWithMetrics(pebbleFsh)
		pebbleFlg = WrapDatabaseWithMetrics(pebbleFlg)
		pebbleDrc = WrapDatabaseWithMetrics(pebbleDrc)
	}

	return map[multidb.TypeName]sesadb.IterableDBProducer{
			"leveldb-fsh": leveldbFsh,
			"leveldb-flg": leveldbFlg,
			"leveldb-drc": leveldbDrc,
			"pebble-fsh":  pebbleFsh,
			"pebble-flg":  pebbleFlg,
			"pebble-drc":  pebbleDrc,
		}, map[multidb.TypeName]sesadb.FullDBProducer{
			"leveldb-fsh": flushable.NewSyncedPool(leveldbFsh, FlushIDKey),
			"leveldb-flg": flaggedproducer.Wrap(leveldbFlg, FlushIDKey),
			"leveldb-drc": &DummyScopedProducer{leveldbDrc},
			"pebble-fsh":  asyncflushproducer.Wrap(flushable.NewSyncedPool(pebbleFsh, FlushIDKey), 200000),
			"pebble-flg":  flaggedproducer.Wrap(pebbleFlg, FlushIDKey),
			"pebble-drc":  &DummyScopedProducer{pebbleDrc},
		}
}

func DbCacheFdlimit(cfg DBsCacheConfig) (func(string) (int, int), error) {
	fmts := make([]func(req string) (string, error), 0, len(cfg.Table))
	fmtsCaches := make([]DBCacheConfig, 0, len(cfg.Table))
	exactTable := make(map[string]DBCacheConfig, len(cfg.Table))
	// build scanf filters
	for name, cache := range cfg.Table {
		if !strings.ContainsRune(name, '%') {
			exactTable[name] = cache
		} else {
			fn, err := fmtfilter.CompileFilter(name, name)
			if err != nil {
				return nil, err
			}
			fmts = append(fmts, fn)
			fmtsCaches = append(fmtsCaches, cache)
		}
	}
	return func(name string) (int, int) {
		// try exact match
		if cache, ok := cfg.Table[name]; ok {
			return int(cache.Cache), int(cache.Fdlimit)
		}
		// try regexp
		for i, fn := range fmts {
			if _, err := fn(name); err == nil {
				return int(fmtsCaches[i].Cache), int(fmtsCaches[i].Fdlimit)
			}
		}
		// default
		return int(cfg.Table[""].Cache), int(cfg.Table[""].Fdlimit)
	}, nil
}

func isEmpty(dir string) bool {
	f, err := os.Open(dir)
	if err != nil {
		return true
	}
	defer f.Close()
	_, err = f.Readdirnames(1)
	return err == io.EOF
}

func dropAllDBs(chaindataDir string) {
	_ = os.RemoveAll(chaindataDir)
}

func dropAllDBsIfInterrupted(chaindataDir string) {
	if isInterrupted(chaindataDir) {
		log.Info("Restarting genesis processing")
		dropAllDBs(chaindataDir)
	}
}

type GossipStoreAdapter struct {
	*gossip.Store
}

func (g *GossipStoreAdapter) GetEvent(id hash.Event) dag.Event {
	e := g.Store.GetEvent(id)
	if e == nil {
		return nil
	}
	return e
}

func MakeDBDirs(chaindataDir string) {
	dbs, _ := SupportedDBs(chaindataDir, DBsCacheConfig{})
	for typ := range dbs {
		if err := os.MkdirAll(path.Join(chaindataDir, string(typ)), 0700); err != nil {
			utils.Fatalf("Failed to create chaindata/leveldb directory: %v", err)
		}
	}
}

type DummyScopedProducer struct {
	sesadb.IterableDBProducer
}

func (d DummyScopedProducer) NotFlushedSizeEst() int {
	return 0
}

func (d DummyScopedProducer) Flush(_ []byte) error {
	return nil
}

func (d DummyScopedProducer) Initialize(_ []string, flushID []byte) ([]byte, error) {
	return flushID, nil
}

func (d DummyScopedProducer) Close() error {
	return nil
}
