package bvallcheck

import (
	"github.com/sesanetwork/go-sesa/native"
)

type Checker struct {
	HeavyCheck HeavyCheck
	LightCheck LightCheck
}

type LightCheck func(bvs native.LlrSignedBlockVotes) error

type HeavyCheck interface {
	Enqueue(bvs native.LlrSignedBlockVotes, checked func(error)) error
}

type Callback struct {
	HeavyCheck HeavyCheck
	LightCheck LightCheck
}

// Enqueue tries to fill gaps the fetcher's future import queue.
func (c *Checker) Enqueue(bvs native.LlrSignedBlockVotes, checked func(error)) {
	// Run light checks right away
	err := c.LightCheck(bvs)
	if err != nil {
		checked(err)
		return
	}

	// Run heavy check in parallel
	_ = c.HeavyCheck.Enqueue(bvs, checked)
}
