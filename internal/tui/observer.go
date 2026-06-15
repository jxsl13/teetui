package tui

import (
	"sync"

	"github.com/jxsl13/twclient/client"
)

// State is the render-facing store of the latest game tick. It implements
// client.Observer: teetui never polls snapshots directly, it only consumes the
// TickState handed to Observe (§V2). Frame cadence is requested so positions
// arrive smoothed for display (§V3).
type State struct {
	mu   sync.RWMutex
	st   client.TickState
	have bool
}

// NewState returns an empty render state.
func NewState() *State { return &State{} }

// Mode requests smoothed per-frame ticks for visual rendering (§V3).
func (s *State) Mode() client.TickMode { return client.TickModeFrame }

// Observe stores the latest tick. Called from the twclient tick goroutine, so
// it takes the write lock (§V4).
func (s *State) Observe(_ *client.Client, st client.TickState) {
	s.mu.Lock()
	s.st = st
	s.have = true
	s.mu.Unlock()
}

// Get returns the latest tick and whether one has arrived yet.
func (s *State) Get() (client.TickState, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.st, s.have
}
