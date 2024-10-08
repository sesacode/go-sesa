package vecmt

import (
	"testing"

	"github.com/sesanetwork/go-vassalo/hash"
	"github.com/sesanetwork/go-vassalo/native/dag"
	"github.com/sesanetwork/go-vassalo/native/dag/tdag"
	"github.com/sesanetwork/go-vassalo/native/pos"
	"github.com/sesanetwork/go-vassalo/sesadb/memorydb"

	"github.com/sesanetwork/go-sesa/native"
)

var (
	testASCIIScheme = `
a1.0   b1.0   c1.0   d1.0   e1.0
║      ║      ║      ║      ║
║      ╠──────╫───── d2.0   ║
║      ║      ║      ║      ║
║      b2.1 ──╫──────╣      e2.1
║      ║      ║      ║      ║
║      ╠──────╫───── d3.1   ║
a2.1 ──╣      ║      ║      ║
║      ║      ║      ║      ║
║      b3.2 ──╣      ║      ║
║      ║      ║      ║      ║
║      ╠──────╫───── d4.2   ║
║      ║      ║      ║      ║
║      ╠───── c2.2   ║      e3.2
║      ║      ║      ║      ║
`
)

type eventWithCreationTime struct {
	dag.Event
	creationTime native.Timestamp
}

func (e *eventWithCreationTime) CreationTime() native.Timestamp {
	return e.creationTime
}

func BenchmarkIndex_Add(b *testing.B) {
	b.StopTimer()
	ordered := make(dag.Events, 0)
	nodes, _, _ := tdag.ASCIIschemeForEach(testASCIIScheme, tdag.ForEachEvent{
		Process: func(e dag.Event, name string) {
			ordered = append(ordered, e)
		},
	})
	validatorsBuilder := pos.NewBuilder()
	for _, peer := range nodes {
		validatorsBuilder.Set(peer, 1)
	}
	validators := validatorsBuilder.Build()
	events := make(map[hash.Event]dag.Event)
	getEvent := func(id hash.Event) dag.Event {
		return events[id]
	}
	for _, e := range ordered {
		events[e.ID()] = e
	}

	vecClock := NewIndex(func(err error) { panic(err) }, LiteConfig())
	vecClock.Reset(validators, memorydb.New(), getEvent)

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		vecClock.Reset(validators, memorydb.New(), getEvent)
		b.StartTimer()
		for _, e := range ordered {
			err := vecClock.Add(&eventWithCreationTime{e, native.Timestamp(e.Seq())})
			if err != nil {
				panic(err)
			}
			i++
			if i >= b.N {
				break
			}
		}
	}
}
