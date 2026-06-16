package tui

import (
	"testing"

	"github.com/jxsl13/twclient/client"
)

// §T117/§V79/§B12: Clear drops the stored tick so the renderer stops drawing the
// dead session's map after a disconnect.
func TestStateClear(t *testing.T) {
	s := NewState()
	s.Observe(nil, client.TickState{Tick: 7})
	if _, have := s.Get(); !have {
		t.Fatal("Observe should set have=true")
	}
	s.Clear()
	if st, have := s.Get(); have || st.Tick != 0 {
		t.Errorf("Clear should reset: have=%v tick=%d", have, st.Tick)
	}
}
