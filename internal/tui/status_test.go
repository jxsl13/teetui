package tui

import (
	"strings"
	"testing"

	"github.com/jxsl13/twclient/client"
)

// §T25/§B9: the status bar shows the live connection state — connected, an
// auto-reconnect attempt with its number, an in-flight handshake, or idle.
func TestConnLabel(t *testing.T) {
	cases := []struct {
		cs   connStatus
		want string
	}{
		{connStatus{connected: true}, "connected"},
		{connStatus{reconnecting: true, attempt: 3}, "reconnecting #3"},
		{connStatus{joining: true}, "connecting"},
		{connStatus{}, "not connected"}, // idle — never claims "connecting" (§B9)
		// connected wins over a stale reconnecting flag.
		{connStatus{connected: true, reconnecting: true, attempt: 9}, "connected"},
	}
	for _, c := range cases {
		if got := connLabel(c.cs); got != c.want {
			t.Errorf("connLabel(%+v) = %q want %q", c.cs, got, c.want)
		}
	}

	// statusText embeds the connection label.
	line := statusText("NORMAL", "srv:8303", connStatus{reconnecting: true, attempt: 2},
		client.TickState{}, false)
	if !strings.Contains(line, "reconnecting #2") {
		t.Errorf("statusText missing reconnect state: %q", line)
	}
}
