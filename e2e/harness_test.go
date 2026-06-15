//go:build e2e

package e2e

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jxsl13/twclient/client"
	"github.com/jxsl13/twclient/packet"
)

// Live-integration harness helpers for teetui (SPEC §C14 / §V22 / §V23). These
// drive the FULL twclient client.Connect against the dockerized servers — the
// same lifecycle teetui uses (App.Join) — so an e2e pass means "connect +
// snapshot verified against a live server", which is the precondition §V23 puts
// on marking a §T task done.

const (
	// The harness is on the docker bridge — no real network latency, so a short
	// connect timeout makes the failure cases (unreachable, version mismatch)
	// fail FAST instead of hanging. Login includes the (local) map download.
	connectTimeout = 12 * time.Second
	// Snapshots arrive every ~2 ticks (≈40ms) once in-game; allow generous
	// head-room for a freshly-started server warming up (§B3 deadline lesson).
	snapTimeout = 10 * time.Second
)

// requireHarness skips the whole suite unless TW_E2E is set — it must never run
// as part of an ordinary `go test ./...` (it also needs the `e2e` build tag).
func requireHarness(t *testing.T) {
	t.Helper()
	if os.Getenv("TW_E2E") == "" {
		t.Skip("e2e harness disabled; set TW_E2E=1 and run IN the compose network (see e2e/README.md)")
	}
}

// env returns the environment variable key or def when unset/empty. Defaults are
// the compose SERVICE-NAME addresses, so the suite works in-network with no env
// at all; override to point at a manually-run harness.
func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// connectClient runs teetui's full client lifecycle against addr WITHOUT failing
// or skipping the test on error. It connects with WithVersion + WithPlayerInfo
// and — CRUCIAL per §V22 — starts `go c.RunFrontends(ctx)` after Connect: that
// loop drives the Observer/Controller frontends, exactly as teetui's App.Join
// does (§B2). Without it teetui would sit on "connecting…" even while snapshots
// tick. Close + frontend-cancel are registered as t.Cleanup. On a connect error
// it returns (nil, err) so the caller decides fail vs skip.
func connectClient(t *testing.T, version packet.Version, addr string) (*client.Client, error) {
	t.Helper()
	c := client.New(addr,
		client.WithVersion(version),
		client.WithPlayerInfo("teetui-e2e", "", "default", -1),
	)

	// §V25/§B4: the ctx passed to Connect is the SESSION lifetime (twclient binds
	// the reader + keepalive to it). It must be long-lived, NOT a connectTimeout
	// ctx — a short ctx tears the session down and the server times us out. Bound
	// the handshake with a watchdog that cancels ONLY while still connecting.
	// teetui's App.Join uses exactly this shape (§T52).
	sessCtx, sessCancel := context.WithCancel(context.Background())
	t.Cleanup(sessCancel)
	connected := make(chan struct{})
	go func() {
		select {
		case <-time.After(connectTimeout):
			select {
			case <-connected:
			default:
				sessCancel() // abort a stuck handshake; never caps a live session
			}
		case <-connected:
		}
	}()
	if err := c.Connect(sessCtx); err != nil {
		close(connected)
		return nil, err
	}
	close(connected)
	t.Cleanup(func() { _ = c.Close() })

	// §V22: drive the Observer/Controller frontends on the same long-lived
	// session ctx. teetui starts this after every successful Connect.
	go c.RunFrontends(sessCtx)

	return c, nil
}

// dialClient is connectClient that FAILS the test on a connect error — the
// strict variant for the smoke test.
func dialClient(t *testing.T, version packet.Version, addr string) *client.Client {
	t.Helper()
	c, err := connectClient(t, version, addr)
	if err != nil {
		t.Fatalf("connect %s (proto %v): %v", addr, version, err)
	}
	return c
}

// dialClientOrSkip is connectClient that SKIPS (not fails) the test when the
// server does not answer — a server that is down/unreachable is a harness-state
// condition, not a teetui code defect, so the table case skips with a reason.
func dialClientOrSkip(t *testing.T, version packet.Version, addr string) *client.Client {
	t.Helper()
	c, err := connectClient(t, version, addr)
	if err != nil {
		t.Skipf("connect %s (proto %v) failed — server not answering (harness state, not a code defect): %v", addr, version, err)
	}
	return c
}

// waitSnapshot polls up to snapTimeout for the client to have a decoded,
// ticking snapshot (LastSnapTick > 0) AND a parsed map (MapView != nil) — i.e.
// the snapshot really decoded. Returns false (the caller decides skip vs fail)
// if neither arrives in time.
func waitSnapshot(t *testing.T, c *client.Client) bool {
	t.Helper()
	deadline := time.Now().Add(snapTimeout)
	for time.Now().Before(deadline) {
		if c.LastSnapTick() > 0 && c.MapView() != nil {
			return true
		}
		time.Sleep(200 * time.Millisecond)
	}
	return false
}

// liveServer is one row of the both-protocols / both-implementations test table
// (§V23 live-server parity). The DDNet sixup server backs two rows (0.6 + 0.7).
type liveServer struct {
	name    string
	version packet.Version
	addr    string
}

// liveServers is the §V23 table: ddnet over 0.6 and 0.7-sixup (same container),
// plus vanilla teeworlds 0.7. Addresses default to the compose service names and
// are individually env-overridable.
func liveServers() []liveServer {
	return []liveServer{
		{"ddnet-0.6", packet.Version06, env("TW_E2E_DDNET_06", "ddnet:8303")},
		{"ddnet-0.7-sixup", packet.Version07, env("TW_E2E_DDNET_07", "ddnet:8303")},
		{"vanilla-0.7", packet.Version07, env("TW_E2E_VANILLA_07", "teeworlds7:8303")},
	}
}
