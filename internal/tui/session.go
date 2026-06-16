package tui

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/jxsl13/teetui/feature"
	"github.com/jxsl13/twclient/client"
	"github.com/jxsl13/twclient/packet"
)

// session is one connection to a server (§C36/§V76): its own client, render
// State (Observer), InputController (Controller), and frontend cancel. teetui
// holds one or more — the primary plus any dummies — and renders/controls the
// ACTIVE one (§V77).
type session struct {
	cli         atomic.Pointer[client.Client]
	state       *State
	input       *InputController
	cancel      context.CancelFunc // frontend (RunFrontends) cancel
	server      string
	version     packet.Version
	name        string // label shown in the follow list
	connected   atomic.Bool
	joining     atomic.Bool
	userClosing atomic.Bool // user-initiated close: suppress auto-reconnect (§B16/§V84)
}

// newSession builds a session with its own State + InputController, wiring the
// per-tick render + feature dispatch (§T109/§T70) onto its State.
func (a *App) newSession(name string, state *State, input *InputController) *session {
	if state == nil {
		state = NewState()
	}
	if input == nil {
		input = NewInputController()
	}
	state.SetTickHook(func(st client.TickState) {
		feature.FireTick(a.api(), st) // no-op when no feature is registered
		a.wake()                      // animate the live view (§T109/§V72)
	})
	return &session{state: state, input: input, name: name}
}

// cur returns the active session (§V77). There is always at least the primary.
func (a *App) cur() *session {
	a.sessMu.Lock()
	defer a.sessMu.Unlock()
	return a.sessions[a.active]
}

// sessionList returns a snapshot of the sessions and the active index, for the
// Esc-menu follow list (§T114).
func (a *App) sessionList() (out []*session, active int) {
	a.sessMu.Lock()
	defer a.sessMu.Unlock()
	out = make([]*session, len(a.sessions))
	copy(out, a.sessions)
	return out, a.active
}

// setActive switches the rendered/controlled session to index i (§V77 follow).
func (a *App) setActive(i int) {
	a.sessMu.Lock()
	if i >= 0 && i < len(a.sessions) {
		a.active = i
	}
	a.sessMu.Unlock()
	a.camera.reset() // snap the camera to the new perspective (§T43)
	a.wake()
}

// addSession appends a session and returns its index (§T115 connect dummy).
func (a *App) addSession(s *session) int {
	a.sessMu.Lock()
	defer a.sessMu.Unlock()
	a.sessions = append(a.sessions, s)
	return len(a.sessions) - 1
}

// dropSession removes session s (a dropped dummy); if it was active, falls back
// to the primary. The primary (index 0) is never dropped here.
func (a *App) dropSession(s *session) {
	a.sessMu.Lock()
	defer a.sessMu.Unlock()
	for i, e := range a.sessions {
		if e == s && i != 0 {
			a.sessions = append(a.sessions[:i], a.sessions[i+1:]...)
			if a.active >= len(a.sessions) {
				a.active = 0
			}
			break
		}
	}
}

// connectDummy opens an additional connection ("dummy") to the current server
// and follows it (§T115/§V76). The server's per-IP dummy limit is enforced
// server-side; a rejected dummy is dropped without disturbing the primary.
func (a *App) connectDummy() {
	cur := a.cur()
	addr, ver := cur.server, cur.version
	if addr == "" {
		return
	}
	s := a.newSession("dummy", nil, nil)
	s.input.SetHold(time.Duration(a.cfg.InputHoldMs) * time.Millisecond)
	i := a.addSession(s)
	a.joinSession(s, addr, ver, true)
	a.setActive(i) // follow the new dummy
}

// isPrimary reports whether s is the main session (index 0).
func (a *App) isPrimary(s *session) bool {
	a.sessMu.Lock()
	defer a.sessMu.Unlock()
	return len(a.sessions) > 0 && a.sessions[0] == s
}
