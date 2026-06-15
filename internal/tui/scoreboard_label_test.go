package tui

import (
	"testing"

	"github.com/jxsl13/twclient/client"
)

// §V26/§T56: name falls back to "#<id>" when the registry has no name (0.6 gap).
func TestPlayerLabel(t *testing.T) {
	if got := playerLabel(client.PlayerState{ClientID: 3, Name: "alice"}); got != "alice" {
		t.Errorf("named = %q want alice", got)
	}
	if got := playerLabel(client.PlayerState{ClientID: 7, Name: ""}); got != "#7" {
		t.Errorf("nameless = %q want #7", got)
	}
}
