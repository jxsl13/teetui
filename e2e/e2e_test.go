//go:build e2e

package e2e

import (
	"testing"

	"github.com/jxsl13/twclient/packet"
)

// TestE2EConnectSnapshot is the §V23 live-server matrix: for each of teetui's
// supported server/protocol pairs — ddnet-0.6, ddnet-0.7-sixup (the SAME sixup
// container, both protocols) and vanilla teeworlds 0.7 — it runs the FULL
// teetui client lifecycle (Connect → RunFrontends, §V22) and asserts the client
// reaches a decoded, ticking snapshot:
//
//   - LastSnapTick() > 0   — snapshots are arriving and decoding, and
//   - MapView() != nil     — the map parsed, so the snapshot really decoded
//     (this is exactly what teetui needs to render the world; §I.SnapState).
//
// It then logs the roster size (twclient registry, the teetui scoreboard source
// — dbg_dummies bots make this non-trivial). A server that does not answer
// SKIPS its row with a reason rather than failing, so a partial harness is a
// partial run, not a red suite.
func TestE2EConnectSnapshot(t *testing.T) {
	requireHarness(t)

	for _, s := range liveServers() {
		t.Run(s.name, func(t *testing.T) {
			c := dialClientOrSkip(t, s.version, s.addr)

			if !waitSnapshot(t, c) {
				t.Fatalf("%s (%s, proto %v): no decoded snapshot within %s (LastSnapTick=%d, MapView=%v)",
					s.name, s.addr, s.version, snapTimeout, c.LastSnapTick(), c.MapView() != nil)
			}

			mv := c.MapView()
			t.Logf("%s (%s, proto %v): snapshot OK tick=%d localID=%d map=%dx%d roster=%d",
				s.name, s.addr, s.version, c.LastSnapTick(), c.LocalID(),
				mv.Width(), mv.Height(), len(c.Roster()))
		})
	}
}

// TestE2ESmoke is the minimal liveness probe: a single 0.6 connect to the ddnet
// sixup server must reach a snapshot. It uses the strict dialClient (fails, not
// skips) so a totally-broken harness surfaces loudly here while the matrix above
// degrades to skips.
func TestE2ESmoke(t *testing.T) {
	requireHarness(t)

	addr := env("TW_E2E_DDNET_06", "ddnet:8303")
	c := dialClient(t, packet.Version06, addr)
	if !waitSnapshot(t, c) {
		t.Fatalf("smoke %s: no snapshot within %s (LastSnapTick=%d)", addr, snapTimeout, c.LastSnapTick())
	}
	t.Logf("smoke %s: connected, snapshot tick=%d roster=%d", addr, c.LastSnapTick(), len(c.Roster()))
}
