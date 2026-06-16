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
	mu       sync.RWMutex
	st       client.TickState
	have     bool
	tickHook func(client.TickState) // optional per-tick callback (§T70 OnTick)
}

// NewState returns an empty render state.
func NewState() *State { return &State{} }

// SetTickHook installs a callback invoked on every observed tick (used to drive
// user OnTick hooks, §T70). nil disables it.
func (s *State) SetTickHook(fn func(client.TickState)) {
	s.mu.Lock()
	s.tickHook = fn
	s.mu.Unlock()
}

// Mode requests smoothed per-frame ticks for visual rendering (§V3).
func (s *State) Mode() client.TickMode { return client.TickModeFrame }

// Observe stores the latest tick. Called from the twclient tick goroutine, so
// it takes the write lock (§V4).
func (s *State) Observe(_ *client.Client, st client.TickState) {
	s.mu.Lock()
	s.st = st
	s.have = true
	hook := s.tickHook
	s.mu.Unlock()
	if hook != nil {
		hook(st)
	}
}

// Clear drops the stored tick so the renderer shows no stale map/tees — used on
// disconnect and at the start of a (re)connect (§T117/§V79).
func (s *State) Clear() {
	s.mu.Lock()
	s.st = client.TickState{}
	s.have = false
	s.mu.Unlock()
}

// Get returns the latest tick and whether one has arrived yet.
func (s *State) Get() (client.TickState, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.st, s.have
}
