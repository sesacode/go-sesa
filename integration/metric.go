package integration

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/sesanetwork/go-helios/sesadb"
	"github.com/sesanetwork/go-sesa/log"
	"github.com/sesanetwork/go-sesa/metrics"
)

const (
	// metricsGatheringInterval specifies the interval to retrieve leveldb database
	// compaction, io and pause stats to report to the user.
	metricsGatheringInterval = 3 * time.Second
)

type DBProducerWithMetrics struct {
	sesadb.IterableDBProducer
}

type StoreWithMetrics struct {
	sesadb.Store

	diskSizeGauge  metrics.Gauge // Gauge for tracking the size of all the levels in the database
	diskReadMeter  metrics.Meter // Meter for measuring the effective amount of data read
	diskWriteMeter metrics.Meter // Meter for measuring the effective amount of data written

	quitLock sync.Mutex      // Mutex protecting the quit channel access
	quitChan chan chan error // Quit channel to stop the metrics collection before closing the database

	log log.Logger // Contextual logger tracking the database path
}

func WrapDatabaseWithMetrics(db sesadb.IterableDBProducer) sesadb.IterableDBProducer {
	wrapper := &DBProducerWithMetrics{db}
	return wrapper
}

func WrapStoreWithMetrics(ds sesadb.Store) *StoreWithMetrics {
	wrapper := &StoreWithMetrics{
		Store:    ds,
		quitChan: make(chan chan error),
	}
	return wrapper
}

func (ds *StoreWithMetrics) Close() error {
	ds.quitLock.Lock()
	defer ds.quitLock.Unlock()

	if ds.quitChan != nil {
		errc := make(chan error)
		ds.quitChan <- errc
		if err := <-errc; err != nil {
			ds.log.Error("Metrics collection failed", "err", err)
		}
		ds.quitChan = nil
	}
	return ds.Store.Close()
}

func (ds *StoreWithMetrics) meter(refresh time.Duration) {
	// Create storage for iostats.
	var iostats [2]float64

	var (
		errc chan error
		merr error
	)

	timer := time.NewTimer(refresh)
	defer timer.Stop()
	// Iterate ad infinitum and collect the stats
	for i := 1; errc == nil && merr == nil; i++ {
		// Retrieve the database size
		diskSize, err := ds.Stat("disk.size")
		if err != nil {
			ds.log.Error("Failed to read database stats", "err", err)
			merr = err
			continue
		}
		var nDiskSize int64
		if n, err := fmt.Sscanf(diskSize, "%d", &nDiskSize); n != 1 || err != nil {
			ds.log.Error("Bad syntax of disk size entry", "size", diskSize)
			merr = err
			continue
		}
		// Update all the disk size meters
		if ds.diskSizeGauge != nil {
			ds.diskSizeGauge.Update(nDiskSize)
		}

		// Retrieve the database iostats.
		ioStats, err := ds.Stat("iostats")
		if err != nil {
			ds.log.Error("Failed to read database iostats", "err", err)
			merr = err
			continue
		}
		var nRead, nWrite float64
		parts := strings.Split(ioStats, " ")
		if len(parts) < 2 {
			ds.log.Error("Bad syntax of ioStats", "ioStats", ioStats)
			merr = fmt.Errorf("bad syntax of ioStats %s", ioStats)
			continue
		}
		if n, err := fmt.Sscanf(parts[0], "Read(MB):%f", &nRead); n != 1 || err != nil {
			ds.log.Error("Bad syntax of read entry", "entry", parts[0])
			merr = err
			continue
		}
		if n, err := fmt.Sscanf(parts[1], "Write(MB):%f", &nWrite); n != 1 || err != nil {
			log.Error("Bad syntax of write entry", "entry", parts[1])
			merr = err
			continue
		}
		if ds.diskReadMeter != nil {
			ds.diskReadMeter.Mark(int64((nRead - iostats[0]) * 1024 * 1024))
		}
		if ds.diskWriteMeter != nil {
			ds.diskWriteMeter.Mark(int64((nWrite - iostats[1]) * 1024 * 1024))
		}
		iostats[0], iostats[1] = nRead, nWrite

		// Sleep a bit, then repeat the stats collection
		select {
		case errc = <-ds.quitChan:
			// Quit requesting, stop hammering the database
		case <-timer.C:
			timer.Reset(refresh)
			// Timeout, gather a new set of stats
		}
	}
	if errc == nil {
		errc = <-ds.quitChan
	}
	errc <- merr
}

var tmpDbNameMask = regexp.MustCompile("^([A-z]+)(-[0-9]+)$")

func genericNameOfTmpDB(name string) string {
	match := tmpDbNameMask.FindStringSubmatch(name)
	if len(match) == 3 {
		return match[1] + "-tmp"
	} else {
		return name
	}
}

func (db *DBProducerWithMetrics) OpenDB(name string) (sesadb.Store, error) {
	ds, err := db.IterableDBProducer.OpenDB(name)
	if err != nil {
		return nil, err
	}
	dm := WrapStoreWithMetrics(ds)

	name = genericNameOfTmpDB(name)

	dm.log = log.New("database", name)

	metric := "sesa/chaindata/" + strings.ReplaceAll(name, "-", "_")
	dm.diskReadMeter = metrics.GetOrRegisterMeter(metric+"/disk/read", nil)
	dm.diskWriteMeter = metrics.GetOrRegisterMeter(metric+"/disk/write", nil)
	// reset size metric as far as previous db will be dropped soon
	metrics.Unregister(metric + "/disk/size")
	dm.diskSizeGauge = metrics.NewRegisteredGauge(metric+"/disk/size", nil)

	// Start up the metrics gathering and return
	go dm.meter(metricsGatheringInterval)
	return dm, nil
}
