# e2e — Docker game-server harness (SPEC §C14 / §V22 / §V23)

A reproducible end-to-end harness for **teetui**, mirroring the
`github.com/jxsl13/twclient` repo's `e2e/`. `docker compose` brings up two
**bot-populated** game servers; a build-tagged Go test then drives teetui's full
twclient `client.Client` lifecycle (Connect → `RunFrontends`, §V22) against them
and asserts `connect + a decoded, ticking snapshot` for every server/protocol
pair teetui supports.

This is the §C14 live-test mandate in code form: a §T task is only `x` once
connect + snapshot are verified against a live server (§V23), and this harness is
what verifies it.

## What comes up

| service      | image / build                          | in-net addr      | protocols      | gametype | bots            |
|--------------|----------------------------------------|------------------|----------------|----------|-----------------|
| `ddnet`      | DDNet `19.8.2` sixup, built from source | `ddnet:8303`      | 0.6 **and** 0.7 | DDRace   | `dbg_dummies 4` |
| `teeworlds7` | teeworlds `0.7.5`, built from source    | `teeworlds7:8303` | 0.7            | `ctf`    | `dbg_dummies 4` |

* **ddnet** is a *sixup* server: it serves both teeworlds 0.6 and 0.7 clients on
  the one UDP port, so it backs BOTH the `ddnet-0.6` and `ddnet-0.7-sixup` table
  rows. DDRace has no classic CTF, but `dbg_dummies` makes every snapshot
  multi-character — enough to validate character/player/roster decode on both
  protocols.
* **teeworlds7** is vanilla 0.7.5 on the stock `ctf1` map (the `vanilla-0.7`
  row), so its snapshots carry the two flags + pickups on top of the bots.

Both images are **built from source with a Debug (`CONF_DEBUG`) build** — that is
the only way `dbg_dummies` is registered; the release binaries reject it and
would yield single-character snapshots.

## Run it (IN the compose network)

> **macOS caveat (§C15 / §B3):** Docker's host UDP port-forward is broken on
> macOS, so connecting from the host to `localhost:8303/8307` times out. The
> test therefore runs **inside the compose network**, addressing the servers by
> compose **service name** (`ddnet:8303`, `teeworlds7:8303`). The host port maps
> in `docker-compose.yml` are decorative parity only.

```sh
# from the repo root — bring the harness up
docker compose -f e2e/docker-compose.yml up -d --build
# wait a few seconds for the servers to load their map + spawn dummies

# run the suite IN the compose network (network name = <project>_default = tw-e2e_default)
docker run --rm --network tw-e2e_default \
  -v "$PWD":/teetui -w /teetui \
  -e GOFLAGS=-mod=mod -e TW_E2E=1 \
  golang:1.26 \
  go test -tags e2e ./e2e/... -v -count=1

docker compose -f e2e/docker-compose.yml down
```

### Test gating / skip behaviour

* The whole package is behind the **`e2e` build tag**, so it never compiles into
  the normal `go test ./...` run (without `-tags e2e` the package presents zero
  tests).
* At runtime it also requires **`TW_E2E`** to be set; otherwise every test
  **skips** cleanly — even `go test -tags e2e ./e2e/...` without `TW_E2E` is a
  no-op, never a failure.
* Each server address is individually overridable:
  `TW_E2E_DDNET_06`, `TW_E2E_DDNET_07`, `TW_E2E_VANILLA_07` (defaults are the
  compose service names). A server that does not answer **skips just its row**
  with a reason; the suite stays green for the servers that are up.

## What it asserts

For each of `ddnet-0.6`, `ddnet-0.7-sixup`, `vanilla-0.7`:

1. `client.Connect` succeeds (with `WithVersion` + `WithPlayerInfo`),
2. `go client.RunFrontends(ctx)` is started — **§V22**: without it teetui's
   Observer/Controller never dispatch (the §B2 "stuck on connecting…" bug),
3. within ~10 s the client reaches `LastSnapTick() > 0` **and**
   `MapView() != nil` (the snapshot genuinely decoded + the map parsed), and
4. the roster size (twclient registry → teetui scoreboard) is logged.
