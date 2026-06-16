// Package lastping tracks the chat lines that pinged us (← chillerbot
// m_aLastPings, §T83/§T63/§V35). Self-registering feature: OnChat queues pings
// (newest-first, bounded), optionally shows the latest in the status bar
// (cl_show_last_ping), and Provides the "pings" service (a *store exposing
// Newest/NextReply, §V53) that the reply feature consumes for the H reply.
package lastping

import (
	"sync"

	"github.com/jxsl13/teetui/feature"
	"github.com/jxsl13/teetui/lang"
)

const maxPings = 16

type entry struct{ from, msg string }

type store struct {
	mu     sync.Mutex
	buf    []entry // index 0 = newest
	cursor int     // H reply position (0 = newest)
}

func (s *store) push(from, msg string) {
	s.mu.Lock()
	s.buf = append([]entry{{from, msg}}, s.buf...)
	if len(s.buf) > maxPings {
		s.buf = s.buf[:maxPings]
	}
	s.cursor = 0 // a new ping resets the reply cursor to newest
	s.mu.Unlock()
}

// Newest returns the most recent ping ("pings" service, §V53).
func (s *store) Newest() (string, string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.buf) == 0 {
		return "", "", false
	}
	return s.buf[0].from, s.buf[0].msg, true
}

// NextReply returns the ping at the cursor and advances it (newest → older), so
// repeated H presses walk back through pending pings ("pings" service, §V53).
func (s *store) NextReply() (string, string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cursor < 0 || s.cursor >= len(s.buf) {
		s.cursor = 0
		return "", "", false
	}
	e := s.buf[s.cursor]
	s.cursor++
	return e.from, e.msg, true
}

type lastPing struct {
	feature.NopFeature
	store *store
}

func (*lastPing) Name() string { return "lastping" }

func (f *lastPing) Provision(h feature.Host) error {
	h.DefineConfig("cl_show_last_ping", "0", "show the most recent chat ping in the status bar (0/1)")
	h.Provide("pings", f.store)
	h.AddStatusField(func() string {
		if on, _ := h.Config("cl_show_last_ping"); on != "1" && on != "true" && on != "on" {
			return ""
		}
		if from, msg, ok := f.store.Newest(); ok {
			return "ping " + from + ": " + msg
		}
		return ""
	})
	return nil
}

// OnChat queues a line that mentions us (and isn't our own).
func (f *lastPing) OnChat(h feature.Host, e feature.ChatEvent) bool {
	me := h.PlayerName()
	if e.Name != me && lang.ContainsName(e.Msg, me) {
		f.store.push(e.Name, e.Msg)
	}
	return false
}

func init() { feature.Register(&lastPing{store: &store{}}) }
