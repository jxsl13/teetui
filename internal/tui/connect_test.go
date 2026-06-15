package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/jxsl13/twclient/packet"
)

// §V24/§T50: the connect-failure line is actionable — it names the address, the
// protocol version, the underlying error, and the remediation hint.
func TestConnectFailMsg(t *testing.T) {
	msg := connectFailMsg("1.2.3.4:8303", packet.Version07, errors.New("context deadline exceeded"))
	for _, want := range []string{"1.2.3.4:8303", "0.7", "context deadline exceeded", "check address", "-version", "network"} {
		if !strings.Contains(msg, want) {
			t.Errorf("connectFailMsg missing %q: %q", want, msg)
		}
	}
	if got := connectFailMsg("h:1", packet.Version06, errors.New("x")); !strings.Contains(got, "(0.6)") {
		t.Errorf("version label wrong: %q", got)
	}
}

func TestVersionLabel(t *testing.T) {
	if versionLabel(packet.Version06) != "0.6" || versionLabel(packet.Version07) != "0.7" {
		t.Errorf("version labels: %q %q", versionLabel(packet.Version06), versionLabel(packet.Version07))
	}
}

// §T33: the connecting/downloading indicator animates through the spinner frames
// and always advertises what it is doing.
func TestConnectingLine(t *testing.T) {
	seen := map[rune]bool{}
	for i := 0; i < len(spinnerFrames); i++ {
		line := connectingLine(i)
		if !strings.Contains(line, "connecting") || !strings.Contains(line, "map") {
			t.Errorf("frame %d not descriptive: %q", i, line)
		}
		seen[[]rune(line)[0]] = true
	}
	if len(seen) != len(spinnerFrames) {
		t.Errorf("spinner did not cycle: %d distinct frames want %d", len(seen), len(spinnerFrames))
	}
	if connectingLine(0) != connectingLine(len(spinnerFrames)) {
		t.Error("spinner frame index must wrap")
	}
}
