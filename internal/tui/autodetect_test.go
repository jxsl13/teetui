package tui

import (
	"testing"

	"github.com/jxsl13/twclient/packet"
)

// §T127/§V87: connect version is auto-detected when omitted; explicit pins.
func TestDoConnectVersionDefault(t *testing.T) {
	cases := []struct {
		ver  string
		want packet.Version
	}{
		{"", packet.VersionAuto},
		{"0.6", packet.Version06},
		{"0.7", packet.Version07},
		{"garbage", packet.VersionAuto}, // unknown → auto-detect, not a hard 0.6
	}
	for _, c := range cases {
		app, _ := newTestApp(t)
		app.dialer = nil // no dialer yet → doConnect queues into pendingVer
		app.doConnect("srv:8303", c.ver)
		if app.pendingVer != c.want {
			t.Errorf("doConnect ver=%q → %v want %v", c.ver, app.pendingVer, c.want)
		}
	}
}

func TestVersionLabelAuto(t *testing.T) {
	if versionLabel(packet.VersionAuto) != "auto" {
		t.Errorf("versionLabel(auto) = %q", versionLabel(packet.VersionAuto))
	}
}
