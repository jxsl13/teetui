//go:build e2e

// Package e2e holds teetui's Docker-backed end-to-end test harness (SPEC §C14 /
// §V23), mirroring the twclient repo's e2e/ harness. The Go tests drive the
// high-level twclient client.Client against the bot-populated game servers in
// docker-compose.yml and assert that each protocol/server pair connects and
// produces a decoded snapshot.
//
// # Gating
//
// Everything here is behind the `e2e` build tag AND gated at runtime by the
// TW_E2E environment variable, exactly like twclient:
//
//   - The `e2e` build tag keeps this package OUT of the ordinary
//     `go test ./...` compile — without `-tags e2e` no test files here are
//     built (the package presents zero tests).
//   - At runtime, TW_E2E must be set (=1); otherwise every test SKIPS cleanly,
//     so even `go test -tags e2e ./...` on a machine without the harness is a
//     no-op rather than a failure.
//
// # Servers (docker-compose.yml)
//
//	ddnet       sixup DDNet — serves BOTH 0.6 and 0.7 on ddnet:8303 (DDRace,
//	            dbg_dummies → multi-character snapshots).
//	teeworlds7  vanilla teeworlds 0.7 on teeworlds7:8303 (ctf1, dbg_dummies).
//
// Servers are addressed by compose SERVICE NAME (env-overridable, defaults
// ddnet:8303 / teeworlds7:8303). macOS Docker host UDP port-forward is broken
// (§C15 / §B3), so the suite is meant to run IN the compose network — see
// README.md.
//
// # Snapshot dispatch (§V22)
//
// dialClient starts `go c.RunFrontends(ctx)` after a successful Connect: that
// loop is what drives teetui's Observer (render) and Controller (input)
// frontends. Connect alone does not dispatch frontends (§V22 / §B2), so the
// harness reproduces the exact lifecycle teetui uses in production.
package e2e
