package evallcheck

import (
	"github.com/sesanetwork/go-sesa/native"
)

type Checker struct {
	HeavyCheck HeavyCheck
	LightCheck LightCheck
}

type LightCheck func(evs native.LlrSignedEpochVote) error

type HeavyCheck interface {
	Enqueue(evs native.LlrSignedEpochVote, checked func(error)) error
}

type Callback struct {
	HeavyCheck HeavyCheck
	LightCheck LightCheck
}

// Enqueue tries to fill gaps the fetcher's future import queue.
func (c *Checker) Enqueue(evs native.LlrSignedEpochVote, checked func(error)) {
	// Run light checks right away
	err := c.LightCheck(evs)
	if err != nil {
		checked(err)
		return
	}

	// Run heavy check in parallel
	_ = c.HeavyCheck.Enqueue(evs, checked)
}
