package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/jxsl13/twclient/packet"
)

// §V28/§T54: the timeout message names the duration, address and version and
// reads as retryable (not a hard error).
func TestConnectTimeoutMsg(t *testing.T) {
	m := connectTimeoutMsg("1.2.3.4:8303", packet.Version07, 30*time.Second)
	for _, want := range []string{"30s", "1.2.3.4:8303", "0.7", "timed out"} {
		if !strings.Contains(m, want) {
			t.Errorf("timeout msg missing %q: %s", want, m)
		}
	}
}

// connTimeout falls back to the default when unset and honors an override.
func TestConnTimeoutDefault(t *testing.T) {
	a := &App{}
	if got := a.connTimeout(); got != DefaultConnectTimeout {
		t.Errorf("default = %s want %s", got, DefaultConnectTimeout)
	}
	a.SetConnectTimeout(5 * time.Second)
	if got := a.connTimeout(); got != 5*time.Second {
		t.Errorf("override = %s want 5s", got)
	}
}
