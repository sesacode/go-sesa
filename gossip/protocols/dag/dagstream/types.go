package dagstream

import (
	"bytes"
	"math/big"

	"github.com/sesanetwork/go-sesa/common"
	"github.com/sesanetwork/go-sesa/rlp"

	"github.com/sesanetwork/go-vassalo/gossip/basestream"
	"github.com/sesanetwork/go-vassalo/hash"
	"github.com/sesanetwork/go-vassalo/native/dag"
)

type Request struct {
	Session   Session
	Limit     dag.Metric
	Type      basestream.RequestType
	MaxChunks uint32
}

type Response struct {
	SessionID uint32
	Done      bool
	IDs       hash.Events
	Events    []rlp.RawValue
}

type Session struct {
	ID    uint32
	Start Locator
	Stop  Locator
}

type Locator []byte

func (l Locator) Compare(b basestream.Locator) int {
	return bytes.Compare(l, b.(Locator))
}

func (l Locator) Inc() basestream.Locator {
	nextBn := new(big.Int).SetBytes(l)
	nextBn.Add(nextBn, common.Big1)
	return Locator(common.LeftPadBytes(nextBn.Bytes(), len(l)))
}

type Payload struct {
	IDs    hash.Events
	Events []rlp.RawValue
	Size   uint64
}

func (p *Payload) AddEvent(id hash.Event, eventB rlp.RawValue) {
	p.IDs = append(p.IDs, id)
	p.Events = append(p.Events, eventB)
	p.Size += uint64(len(eventB))
}

func (p *Payload) AddID(id hash.Event, size int) {
	p.IDs = append(p.IDs, id)
	p.Size += uint64(size)
}

func (p Payload) Len() int {
	return len(p.IDs)
}

func (p Payload) TotalSize() uint64 {
	return p.Size
}

func (p Payload) TotalMemSize() int {
	if len(p.Events) != 0 {
		return int(p.Size) + len(p.IDs)*128
	}
	return len(p.IDs) * 128
}

const (
	RequestIDs    basestream.RequestType = 0
	RequestEvents basestream.RequestType = 2
)
