package integration

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"path"

	"github.com/status-im/keycard-go/hexutils"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/sesanetwork/go-helios/consensus"
	"github.com/sesanetwork/go-helios/hash"
	"github.com/sesanetwork/go-helios/native/idx"
	"github.com/sesanetwork/go-helios/sesadb"
	"github.com/sesanetwork/go-helios/sesadb/multidb"
	"github.com/sesanetwork/go-sesa/accounts"
	"github.com/sesanetwork/go-sesa/accounts/keystore"
	"github.com/sesanetwork/go-sesa/cmd/utils"
	"github.com/sesanetwork/go-sesa/common"
	"github.com/sesanetwork/go-sesa/crypto"
	"github.com/sesanetwork/go-sesa/log"

	"github.com/sesanetwork/go-sesa/gossip"
	"github.com/sesanetwork/go-sesa/sesa/genesis"
	"github.com/sesanetwork/go-sesa/utils/adapters/vecmt2dagidx"
	"github.com/sesanetwork/go-sesa/utils/dbutil/compactdb"
	"github.com/sesanetwork/go-sesa/vecmt"
)

var (
	MetadataPrefix = hexutils.HexToBytes("0068c2927bf842c3e9e2f1364494a33a752db334b9a819534bc9f17d2c3b4e5970008ff519d35a86f29fcaa5aae706b75dee871f65f174fcea1747f2915fc92158f6bfbf5eb79f65d16225738594bffb")
	FlushIDKey     = append(common.CopyBytes(MetadataPrefix), 0x0c)
	TablesKey      = append(common.CopyBytes(MetadataPrefix), 0x0d)
)

// GenesisMismatchError is raised when trying to overwrite an existing
// genesis block with an incompatible one.
type GenesisMismatchError struct {
	Stored, New hash.Hash
}

// Error implements error interface.
func (e *GenesisMismatchError) Error() string {
	return fmt.Sprintf("database contains incompatible genesis (have %s, new %s)", e.Stored.String(), e.New.String())
}

type Configs struct {
	sesa            gossip.Config
	sesaStore       gossip.StoreConfig
	Hashgraph      consensus.Config
	HashgraphStore consensus.StoreConfig
	VectorClock    vecmt.IndexConfig
	DBs            DBsConfig
}

func panics(name string) func(error) {
	return func(err error) {
		log.Crit(fmt.Sprintf("%s error", name), "err", err)
	}
}

func mustOpenDB(producer sesadb.DBProducer, name string) sesadb.Store {
	db, err := producer.OpenDB(name)
	if err != nil {
		utils.Fatalf("Failed to open '%s' database: %v", name, err)
	}
	return db
}

func getStores(producer sesadb.FlushableDBProducer, cfg Configs) (*gossip.Store, *consensus.Store) {
	gdb := gossip.NewStore(producer, cfg.sesaStore)

	cMainDb := mustOpenDB(producer, "hashgraph")
	cGetEpochDB := func(epoch idx.Epoch) sesadb.Store {
		return mustOpenDB(producer, fmt.Sprintf("hashgraph-%d", epoch))
	}
	cdb := consensus.NewStore(cMainDb, cGetEpochDB, panics("Hashgraph store"), cfg.HashgraphStore)
	return gdb, cdb
}

func getEpoch(producer sesadb.FlushableDBProducer, cfg Configs) idx.Epoch {
	gdb := gossip.NewStore(producer, cfg.sesaStore)
	defer gdb.Close()
	return gdb.GetEpoch()
}

func rawApplyGenesis(gdb *gossip.Store, cdb *consensus.Store, g genesis.Genesis, cfg Configs) error {
	_, _, _, err := rawMakeEngine(gdb, cdb, &g, cfg)
	return err
}

func rawMakeEngine(gdb *gossip.Store, cdb *consensus.Store, g *genesis.Genesis, cfg Configs) (*consensus.Consensus, *vecmt.Index, gossip.BlockProc, error) {
	blockProc := gossip.DefaultBlockProc()

	if g != nil {
		_, err := gdb.ApplyGenesis(*g)
		if err != nil {
			return nil, nil, blockProc, fmt.Errorf("failed to write Gossip genesis state: %v", err)
		}

		err = cdb.ApplyGenesis(&consensus.Genesis{
			Epoch:      gdb.GetEpoch(),
			Validators: gdb.GetValidators(),
		})
		if err != nil {
			return nil, nil, blockProc, fmt.Errorf("failed to write Hashgraph genesis state: %v", err)
		}
	}

	// create consensus
	vecClock := vecmt.NewIndex(panics("Vector clock"), cfg.VectorClock)
	engine := consensus.NewConsensus(cdb, &GossipStoreAdapter{gdb}, vecmt2dagidx.Wrap(vecClock), panics("Hashgraph"), cfg.Hashgraph)
	return engine, vecClock, blockProc, nil
}

func applyGenesis(dbs sesadb.FlushableDBProducer, g genesis.Genesis, cfg Configs) error {
	gdb, cdb := getStores(dbs, cfg)
	defer gdb.Close()
	defer cdb.Close()
	log.Info("Applying genesis state")
	err := rawApplyGenesis(gdb, cdb, g, cfg)
	if err != nil {
		return err
	}
	err = gdb.Commit()
	if err != nil {
		return err
	}
	return nil
}

func migrate(dbs sesadb.FlushableDBProducer, cfg Configs) error {
	gdb, cdb := getStores(dbs, cfg)
	defer gdb.Close()
	defer cdb.Close()
	err := gdb.Commit()
	if err != nil {
		return err
	}
	return nil
}

func CheckStateInitialized(chaindataDir string, cfg DBsConfig) error {
	if isInterrupted(chaindataDir) {
		return errors.New("genesis processing isn't finished")
	}
	runtimeProducers, runtimeScopedProducers := SupportedDBs(chaindataDir, cfg.RuntimeCache)
	dbs, err := MakeMultiProducer(runtimeProducers, runtimeScopedProducers, cfg.Routing)
	if err != nil {
		return err
	}
	return dbs.Close()
}

func compactDB(typ multidb.TypeName, name string, producer sesadb.DBProducer) error {
	humanName := path.Join(string(typ), name)
	db, err := producer.OpenDB(name)
	defer db.Close()
	if err != nil {
		return err
	}
	return compactdb.Compact(db, humanName, 16*opt.GiB)
}

func makeEngine(chaindataDir string, g *genesis.Genesis, genesisProc bool, cfg Configs) (*consensus.Consensus, *vecmt.Index, *gossip.Store, *consensus.Store, gossip.BlockProc, func() error, error) {
	// Genesis processing
	if genesisProc {
		setGenesisProcessing(chaindataDir)
		// use increased DB cache for genesis processing
		genesisProducers, _ := SupportedDBs(chaindataDir, cfg.DBs.GenesisCache)
		if g == nil {
			return nil, nil, nil, nil, gossip.BlockProc{}, nil, fmt.Errorf("missing --genesis flag for an empty datadir")
		}
		dbs, err := MakeDirectMultiProducer(genesisProducers, cfg.DBs.Routing)
		if err != nil {
			return nil, nil, nil, nil, gossip.BlockProc{}, nil, fmt.Errorf("failed to make DB multi-producer: %v", err)
		}
		err = applyGenesis(dbs, *g, cfg)
		if err != nil {
			_ = dbs.Close()
			return nil, nil, nil, nil, gossip.BlockProc{}, nil, fmt.Errorf("failed to apply genesis state: %v", err)
		}
		_ = dbs.Close()
		setGenesisComplete(chaindataDir)
	}
	// Compact DBs after first launch
	if genesisProc {
		genesisProducers, _ := SupportedDBs(chaindataDir, cfg.DBs.GenesisCache)
		for typ, p := range genesisProducers {
			for _, name := range p.Names() {
				if err := compactDB(typ, name, p); err != nil {
					return nil, nil, nil, nil, gossip.BlockProc{}, nil, err
				}
			}
		}
	}
	// Check DBs are synced
	{
		err := CheckStateInitialized(chaindataDir, cfg.DBs)
		if err != nil {
			return nil, nil, nil, nil, gossip.BlockProc{}, nil, err
		}
	}
	// Migration
	{
		runtimeProducers, _ := SupportedDBs(chaindataDir, cfg.DBs.RuntimeCache)
		dbs, err := MakeDirectMultiProducer(runtimeProducers, cfg.DBs.Routing)
		if err != nil {
			return nil, nil, nil, nil, gossip.BlockProc{}, nil, err
		}

		// drop previous epoch DBs, which do not survive restart
		epoch := getEpoch(dbs, cfg)
		leDB, err := dbs.OpenDB(fmt.Sprintf("hashgraph-%d", epoch))
		if err != nil {
			_ = dbs.Close()
			return nil, nil, nil, nil, gossip.BlockProc{}, nil, err
		}
		_ = leDB.Close()
		leDB.Drop()
		goDB, err := dbs.OpenDB(fmt.Sprintf("gossip-%d", epoch))
		if err != nil {
			_ = dbs.Close()
			return nil, nil, nil, nil, gossip.BlockProc{}, nil, err
		}
		_ = goDB.Close()
		goDB.Drop()

		err = migrate(dbs, cfg)
		_ = dbs.Close()
		if err != nil {
			return nil, nil, nil, nil, gossip.BlockProc{}, nil, fmt.Errorf("failed to migrate state: %v", err)
		}
	}
	// Live setup

	runtimeProducers, runtimeScopedProducers := SupportedDBs(chaindataDir, cfg.DBs.RuntimeCache)
	// open flushable DBs
	dbs, err := MakeMultiProducer(runtimeProducers, runtimeScopedProducers, cfg.DBs.Routing)
	if err != nil {
		return nil, nil, nil, nil, gossip.BlockProc{}, nil, err
	}

	gdb, cdb := getStores(dbs, cfg)
	defer func() {
		if err != nil {
			gdb.Close()
			cdb.Close()
			dbs.Close()
		}
	}()

	// compare genesis with the input
	genesisID := gdb.GetGenesisID()
	if genesisID == nil {
		err = errors.New("malformed chainstore: genesis ID is not written")
		return nil, nil, nil, nil, gossip.BlockProc{}, dbs.Close, err
	}
	if g != nil {
		if *genesisID != g.GenesisID {
			err = &GenesisMismatchError{*genesisID, g.GenesisID}
			return nil, nil, nil, nil, gossip.BlockProc{}, dbs.Close, err
		}
	}

	engine, vecClock, blockProc, err := rawMakeEngine(gdb, cdb, nil, cfg)
	if err != nil {
		err = fmt.Errorf("failed to make engine: %v", err)
		return nil, nil, nil, nil, gossip.BlockProc{}, dbs.Close, err
	}

	if genesisProc {
		err = gdb.Commit()
		if err != nil {
			err = fmt.Errorf("failed to commit DBs: %v", err)
			return nil, nil, nil, nil, gossip.BlockProc{}, dbs.Close, err
		}
	}

	return engine, vecClock, gdb, cdb, blockProc, dbs.Close, nil
}

// MakeEngine makes consensus engine from config.
func MakeEngine(chaindataDir string, g *genesis.Genesis, cfg Configs) (*consensus.Consensus, *vecmt.Index, *gossip.Store, *consensus.Store, gossip.BlockProc, func() error) {
	// Legacy DBs migrate
	if cfg.DBs.MigrationMode != "reformat" && cfg.DBs.MigrationMode != "rebuild" && cfg.DBs.MigrationMode != "" {
		utils.Fatalf("MigrationMode must be 'reformat' or 'rebuild'")
	}
	if !isEmpty(path.Join(chaindataDir, "gossip")) {
		MakeDBDirs(chaindataDir)
		genesisProducers, _ := SupportedDBs(chaindataDir, cfg.DBs.GenesisCache)
		dbs, err := MakeDirectMultiProducer(genesisProducers, cfg.DBs.Routing)
		if err != nil {
			utils.Fatalf("Failed to make engine: %v", err)
		}
		err = migrateLegacyDBs(chaindataDir, dbs, cfg.DBs.MigrationMode, cfg.DBs.Routing)
		_ = dbs.Close()
		if err != nil {
			utils.Fatalf("Failed to migrate state: %v", err)
		}
	}

	dropAllDBsIfInterrupted(chaindataDir)
	firstLaunch := isEmpty(chaindataDir)
	MakeDBDirs(chaindataDir)

	engine, vecClock, gdb, cdb, blockProc, closeDBs, err := makeEngine(chaindataDir, g, firstLaunch, cfg)
	if err != nil {
		if firstLaunch {
			dropAllDBs(chaindataDir)
		}
		utils.Fatalf("Failed to make engine: %v", err)
	}

	rules := gdb.GetRules()
	genesisID := gdb.GetGenesisID()
	if firstLaunch {
		log.Info("Applied genesis state", "name", rules.Name, "id", rules.NetworkID, "genesis", genesisID.String())
	} else {
		log.Info("Genesis is already written", "name", rules.Name, "id", rules.NetworkID, "genesis", genesisID.String())
	}

	return engine, vecClock, gdb, cdb, blockProc, closeDBs
}

// SetAccountKey sets key into accounts manager and unlocks it with pswd.
func SetAccountKey(
	am *accounts.Manager, key *ecdsa.PrivateKey, pswd string,
) (
	acc accounts.Account,
) {
	kss := am.Backends(keystore.KeyStoreType)
	if len(kss) < 1 {
		log.Crit("Keystore is not found")
		return
	}
	ks := kss[0].(*keystore.KeyStore)

	acc = accounts.Account{
		Address: crypto.PubkeyToAddress(key.PublicKey),
	}

	imported, err := ks.ImportECDSA(key, pswd)
	if err == nil {
		acc = imported
	} else if err.Error() != "account already exists" {
		log.Crit("Failed to import key", "err", err)
	}

	err = ks.Unlock(acc, pswd)
	if err != nil {
		log.Crit("failed to unlock key", "err", err)
	}

	return
}
