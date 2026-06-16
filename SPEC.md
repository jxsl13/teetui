# SPEC — teetui: cross-platform terminal Teeworlds/DDNet client (chillerbot-ux UX on twclient)

## §G — goal

Re-impl chillerbot-ux ncurses `terminalui` as Go terminal client on `github.com/jxsl13/twclient`. render live game (map+tees ASCII) + scoreboard + server browser + chat/console/rcon + warlist/auto-reply, drive tee from terminal. run on Linux + Windows + macOS terminals w/ color. TARGET: ≥ feature-parity w/ chillerbot-ux AND strictly BETTER impl + terminal UX + rendering (truecolor, smooth, rebindable, resize/scroll robust — ⊥ reference "duct-tape/cursed/wonky" jank).

## §C — constraints

- C1: Go latest (currently `go1.26.4`, ≥ twclient `1.26.1`). module `github.com/jxsl13/teetui`. no cgo.
- C2: comms ONLY via `twclient` pkgs — `client` (play/predict/input/events), `master` (browser), `packet` (types/events/actions). teetui ⊥ touch raw UDP/net6/net7. mirror twclient style: `With…` opts + `Default…` exports, ⊥ env vars (twclient README).
- C3: TUI lib = `github.com/gdamore/tcell/v2`. why: fastest pure-Go path — native cell-diff (redraw only changed cells), own Windows-console driver (⊥ cgo, ⊥ ANSI-emu needed), Linux/macOS via terminfo. user req: best perf. ⊥ Bubble Tea (full `View()` re-render/frame → GC churn @ 50Hz). ⊥ OpenTUI (Zig+cgo → Win build pain).
- C4: color cross-OS = tcell job. `tcell.NewRGBColor` truecolor on `COLORTERM=truecolor`; tcell auto-downsamples → 256 | 16 per `$TERM`/Windows. teetui ! map twclient/map RGB → `tcell.Style`, ⊥ hand-roll palette per OS. ref chillerbot crude `rgb_to_text_color_pair` (6 pairs) — we do better, full RGB + graceful fallback.
- C5: render driven by twclient consumer path: register `client.Observer` (view-only, `Mode()=TickModeFrame` → smoothed `IntraTick∈[0,1)`), receive `client.TickState`/tick. input via single `client.Controller` OR `client.Do(Action)`. ⊥ poll snapshots directly.
- C6: protocol 0.6 (`packet.Version06`) & 0.7 (`packet.Version07`) both — twclient hides diff; teetui picks via flag, ⊥ branch on version in render.
- C7: render hot path 50Hz — reuse buffers, ⊥ per-frame alloc, tcell `SetContent` per changed cell only. profile before optimize.
- C8: input thread ≠ render/tick thread. tcell `PollEvent` goroutine → channel → state under mutex (mirror chillerbot `m_LockKeyUntilRelease` intent). twclient callbacks fire from its eventLoop goroutine (twclient C3) → teetui handlers ! be goroutine-safe.
- C9: utf8 glyph width correct (tee `o`/`ø`, tiles `█▒░🮽🔳`) — use `mattn/go-runewidth` (tcell dep already) for column math. ref chillerbot `pad_utf8`.
- C10: PARITY FLOOR — every chillerbot feature in §I keybinds + §T30-41 (browser/console/rcon/scoreboard/visual/auto-reply/history/self-kill/spectate) ! match OR exceed reference. ⊥ regress vs reference behavior.
- C11: RENDER > reference — full RGB truecolor (⊥ 6-pair curses), accurate+legible map (color Start/Finish/Checkpoint/Tele/Boost via `MapView` booleans, ⊥ blank), smooth camera, optional sub-cell glyphs (half-block ▀▄ / braille) for finer detail. reference render = self-described "cursed/WIP" — teetui ! readable + correct.
- C12: UX > reference — keys REBINDABLE via config (reference can't, §V19), resize ⊥ glitch (V18), scroll ⊥ glitchy, popups/visual-mode clean open+close (reference "wonky/breaks on close"), help always escapable (V17).
- C13: CODE quality — tested (table+sim-screen), clean pkg boundaries, ⊥ "duct tape". each §V invariant has ≥1 test.
- C14: LIVE-TEST mandate (user). every feature/fix ! verified against a LIVE server before §T `x` — ⊥ claim done on build alone. teetui ! own e2e harness MIRRORING twclient repo `e2e/`: docker-compose w/ images built from source — **ddnet** (0.6 + 0.7-sixup) & **teeworlds7** (vanilla 0.7); gated by env `TW_E2E` + `-tags e2e`; addressed by compose service names (`ddnet:8303`, `teeworlds7:8303`), run IN-NETWORK. e2e asserts connect+snapshot ticks (via `RunFrontends`, V22). CI/CD ! run e2e + code coverage (race + `-coverprofile`, per-pkg %). ref twclient `e2e/{docker-compose.yml,ddnet.Dockerfile,teeworlds7.Dockerfile,harness_test.go}` + `.github/workflows/ci.yml`.
- C15: macOS Docker host UDP port-forward BROKEN → host `localhost:8303/8307` connless/connect TIMES OUT. ⊥ test teetui connect from macOS host against docker; run inside compose net (service names) or real server. (← B3)
- C16: PROCESS (user). any twclient BUG or MISSING functionality found → ALWAYS `gh issue create --repo jxsl13/twclient` (detailed English + repro + observed/expected + env). distinguish teetui-side (fix here) vs twclient-side (file issue). filed: #3 (0.6 registry empty), #4 (Connect ctx=lifetime footgun), #5 (v0.2.3 windows build).
- C17: RESPONSIVE. UI ! adapt to terminal size + scale live w/ window resize (smaller→lower res, larger→higher res). ALL windows (status/game/log/input) + overlays (scoreboard/help/popup/browser) derived from current `scr.Size()` EACH render — ⊥ fixed-size assumption, ⊥ cached dims. game view scales w/ terminal (⊥ hard `maxGameW`/64×32 cap that wastes big terminals). below a min usable size → single legible "resize" notice, ⊥ garble/panic. resize event → immediate relayout+redraw (tcell cell-diff, C3/C7). (extends V11/V18; supersedes §I.render ≤64×32)
- C18: CHILLERBOT SCOPE (from chillerbot-ux source diff vs DDNet, analyzed 2026-06-15 @ `~/Desktop/Development/chillerbot-ux` rev 14331d5). teetui = TERMINAL client + chillerbot chat-helper UX. IN-SCOPE parity gaps → §T60-68. EXPLICIT NON-GOALS (⊥ port — out of teetui's terminal/ethics scope):
  - graphical-only: `cl_render_pic`/playerpics, `cl_no_particles`, `cl_render_laser_head`, `cl_weapon_hud`, `cl_show_speed`, nameplate client-icons, `cl_skin_stealer`+saved colors. (no GUI in terminal)
  - cheat/automation: `cl_camp_hack` (auto-walk), `cl_spike_tracer`, skin steal. (⊥ cheat)
  - ABUSIVE — REFUSE: `stresser`/`cl_pentest` (server stress/DoS). ⊥ implement.
  - telemetry/privacy: `cl_send_online_time` (→zillyhuhn.com), `cl_chillerbot_id`, `cl_send_client_type`/`cl_show_client_type`. ⊥ phone home.
  - mod-specific: `city`/`cl_show_wallet`, `mmotee`, `vibebot`, in-game `edit_map`/minetee editor/`chiller_editor`. (not core TW/DDNet client)
  - security risk: `cl_remote_control` (execute whisper-delivered cmds on own client via token). ⊥ remote code exec via chat.
  - misc low-value: `cl_finish_rename`, `cl_change_tile_notification`, `cl_show_last_killer`, `cl_always_reconnect`/`cl_reconnect_when_empty` (T25 already covers drop→reconnect).
  NB: NONE of the above ships in teetui, but ALL are user-buildable via the hook API (C19) — teetui gives primitives, user supplies the behavior.
- C19: EXTENSIBLE. teetui ! expose a stable hook/callback API (§I.extension) so users implement out-of-scope (§C18) behavior themselves WITHOUT patching core: in-process Go `Hook` (events + safe action ctx) + opt-in external command hooks (`~/.config/teetui/hooks/`). hook surface = teetui's existing twclient public API ONLY (V1/V2/V12) — ⊥ raw net/packet, ⊥ DoS/flood primitive. teetui ⊥ SHIP any §C18 feature nor any abusive hook; user-supplied hooks = user responsibility. a hook panic ⊥ crash teetui (recover+disable+log). hooks opt-in, none active by default.
- C20: FPS-LIMIT. render repaint rate ! be cappable to a configurable max fps (`cl_max_fps` cvar + `-max-fps` flag; 0=unlimited) to throttle terminal CPU. PURE render-side throttle — ⊥ couple to tick rate (twclient stays 50Hz, C5); coalesce event/wake bursts into ≤ cap repaints, ALWAYS render the latest state (trailing-edge draw, ⊥ drop final frame); ⊥ stall input handling; ⊥ add per-frame heap alloc (V7). reuse tcell cell-diff (C3) so a no-change frame is cheap.
- C21: MODULAR FEATURES (Caddy-v2 / image-stdlib style). EVERY chillerbot-ux-specific feature lives in its OWN package `features/<name>`, SELF-REGISTERS in `init()` via `feature.Register(...)` (← `caddy.RegisterModule` / `image.RegisterFormat`), implemented EXCLUSIVELY against the public Host API (§I.feature) — ⊥ import `internal/tui`, ⊥ reach core internals. CORE (`internal/tui`) = base client + Host impl + module registry + render/input loop ONLY; ⊥ contain chillerbot feature logic. `main.go` = ONE file: blank-imports every feature package + builds/starts the base client; adding a feature = new package + one import line; removing = delete the import. if the Host API can't express a feature → EXTEND the public API (⊥ leak core, ⊥ globals): API ! be SUFFICIENT for all current chillerbot features. shared non-feature logic: used by ≥2 features → `internal/<name>` package (e.g. `internal/lang`); used by ONLY ONE feature → lives in THAT feature's pkg (⊥ standalone pkg). ⊥ public root-level lib. SAFETY infra stays core (send-pacing/spam-safe V37, own-echo dedupe V29, reconnect V25) — ⊥ optional. supersedes the §I "extension / hooks" surface (T69-71) → folded into `feature`.
- C22: LOG-AT-BOTTOM LAYOUT. windows stack VERTICALLY (⊥ left/right split): status(top) → game/visual → log band → input-legend(bottom). log band sits DIRECTLY above the input-legend bar; the visual render sits ABOVE the logs and, when ON, shrinks the log band to a small configurable strip (pushing older lines below the viewport). visual ON → log band height = clamp(`cl_log_lines` [+`-log-lines`], 1, ⌊h/2⌋), DEFAULT 10; the game fills the body above the band. visual OFF → logs fill the entire body. logs ⊥ EVER exceed ⌊h/2⌋ of terminal height when visual on (cap). recompute from live `h` on resize (C17). supersedes the §C17/§T57 left/right split + §I.windows game-left/log-right.
- C23: CONFIG-FILE-ONLY CLI. teetui takes NO per-setting flags — ONLY one optional config-file arg (`teetui [-config <file>]`). file = teeworlds-style `.cfg`: one `command [args]` per line, `#` comments, executed in order through the SAME console layer as F1 (cvars + `connect`/identity). no file → predefined defaults (⊥ auto-connect → open browser). REMOVE `-server`/`-name`/`-clan`/`-skin`/`-version`/`-connect-timeout`/`-max-fps`/`-log-lines` (+ never-built `-password`/`-no-color`). identity via cvars (`player_name`,`player_clan`); ⊥ `-skin` (dead — terminal tee = `o`, no skins). protocol version ⊥ a global flag: taken from the master/scan entry on a browser/LAN join, or from `connect <addr> [0.6|0.7]` in the config — version OMITTED ⇒ `packet.VersionAuto` → twclient PROBES the server and picks the protocol (prefers 0.6), resolved version read back via `Client.Version()` (twclient v0.2.5+ auto-detect; was hard `Version06`). explicit `0.6`/`0.7` still pins. (corrects §I.cli flag drift)
- C24: FREE-LOOK MAP NAV. visual mode ! support a free-look/pan sub-mode — detach camera from local tee, PAN map via arrows OR WASD (tile steps), recenter + exit keys. VIEW-ONLY: while panning ⊥ send aim/move/any tee input (mode-gated, V9/V12). HUD shows panned center tile coords + a "[free-look]" indicator. requires visual ON (entering ensures it); exit → camera re-locks to tee (smoother, T43). pan clamps so view ⊥ run off into garbage; resize/min-size safe (V30/V32). (extends V27 free-view; distinct from `subcell` half-block "detail" render, T46 — RENAME the legend's mislabeled `[V]detail` to its real meaning)
- C25: CONTEXT LEGEND. input-legend bar (bottom) + `?` help overlay ! be GENERATED from the LIVE keymap (V19 rebinds reflected) + feature actions (DefineAction key/help) — ⊥ hardcoded strings drifting from real bindings. legend = the MOST IMPORTANT commands AVAILABLE in the current context (normal | free-look | browser | input mode) as `[key]label`, priority-ordered; overflow → drop lowest-priority entries to fit width (⊥ overflow row, V30). help overlay = FULL binding list (core actions + every feature action), grouped, always escapable (V17). rebinding a key updates BOTH.
- C27: SDK DESIGN (research 2026-06-16). NAMES derived from Go stdlib idioms — Effective Go/Pike interface naming (single-method iface = method+`er`: `io.Reader`/`io.Writer`/`fmt.Stringer`), `io.Closer` for resource release, `net/http.Handler` for event receivers — NOT from Caddy (Caddy studied for the self-register MODULE pattern only, C21; ⊥ its `Provisioner`/`CleanerUpper` names). also SOLID interface-segregation + "accept interfaces / return structs". public `feature` SDK !:
  - SMALL, OPTIONAL capability interfaces discovered by type-assertion (comma-ok), the Go optional-interface idiom (e.g. code checks whether an `io.Reader` is also an `io.Closer`) — ⊥ ONE monolithic mandatory interface. adding a NEW capability = NEW optional interface ⇒ ⊥ break existing features (FORWARD-compatible extension).
  - `Feature` minimal = IDENTITY only (`Name() string`); init, lifecycle, events all OPTIONAL (asserted).
  - idiomatic naming: single-method iface = method+`er` (`Reader`/`Closer`/`Stringer`) | `…Handler` (← `net/http.Handler`); ⊥ `…Interface`/`…Events` vague bag; ⊥ stutter/pkg-name redundancy; ONE obvious name per concept.
  - NAME the capability surface `feature.API` (the teetui-client API given to a feature) — ⊥ `Host` (webserver host/guest framing; teetui = terminal client, ⊥ server), ⊥ `Context` (collides `context.Context`), ⊥ `Client` (collides twclient `client.Client`).
  - LARGE-interface anti-pattern: `feature.API` COMPOSED from small named sub-interfaces (`ChatSender`/`Logger`/`StateReader`/…); a consumer|handler depends on the MINIMAL sub-surface it needs (V53/ISP), ⊥ a flat opaque bag.
  - RESOURCE lifecycle: a feature owning goroutines/files/handles ! get a teardown hook — name it `Close()` per `io.Closer` — run on shutdown AND on disable-after-panic, working even after PARTIAL init. today there is NONE (cmdhook spawns procs, warlist/lastping persist) — gap.
  - SDK stays feature-AGNOSTIC (V53) + action surface stays the safe twclient API (V47); refactor ⊥ change observed behavior (V44).
- C26: SCALE + REFLOW (reinforces C17 — ALL UI scales w/ terminal size, no exceptions). two concrete gaps: (a) LOG WRAP — a log line wider than the log-band width ! CONTINUE on the next visual row (word-wrap, hard-split a token longer than width), ⊥ silently truncate log text; scroll counts VISUAL (wrapped) rows. (b) FLEX TABLES — overlay table columns (scoreboard name/clan, browser name/gametype/map/…) ! DERIVE from current rect width each render, ⊥ fixed col widths (waste on wide / over-truncate on narrow); narrow → shrink|drop low-priority cols, col sum ≤ width (⊥ overflow, V30). recompute from live size on resize (C17/V50).
- C28: LANG MATCH NORMALIZATION (research 2026-06-16, Go-native). `internal/lang` matching ! be ACCENT- & CASE-insensitive + Unicode-correct via the Go stdlib + `golang.org/x/text` (already indirect in graph → promote to direct) — ⊥ third-party NLP. fold key = `transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)` then `cases.Fold()`; so `café≈cafe`, `tschüss≈tschuss`, `über≈uber`, composed≈decomposed `é`, and `ß`/Greek-sigma/Turkish-i fold right (where `strings.ToLower` does NOT). fold the message ONCE per line; patterns pre-folded. word-boundary semantics preserved (`helloween` ⊥ match `hello`); empty word/name ⊥ match. `cases.Caser`/`transform.Transformer` ⊥ concurrency-safe → per-call or pooled (`lang` runs on the dispatch goroutine, V4). chat-rate only — ⊥ the 50Hz render path (V7 n/a). SLIM the word lists: drop the ASCII-umlaut dodges (`tschuss`/`tschau`) — fold makes the real spellings match. (supersedes the hand-rolled `strings.ToLower` matching in T60)
- C29: INGAME MOVEMENT KEYS, SELECTABLE (left-/right-handed). cvar `cl_move_keys` ∈ {`wasd`,`arrows`} (default `wasd`): the SELECTED set drives MOVEMENT, the OTHER set drives cardinal AIM — so a player moves with one hand's keys and aims with the other, either way round. MOVE set: jump / left / stop / right = `W A S D` or `Up Left Down Right`. AIM set (the complement): cardinal aim (the 4 directions set the ActInput target). movement sticky (terminal ⊥ key-release — direction holds till changed, C8/existing controller); `Space` always jumps. drives the core `InputController`→`client.Do(ActInput)` ONLY (V12). switchable live (cvar). CORE keymap/input change (tee control = core), ⊥ a `features/*` pkg. ⊥ conflict with free-look (`G`): free-look already intercepts arrows/WASD in its sub-mode before the router (V54).
- C30: SERVER GAME MESSAGES (← normal DDNet client, ddnet `src/game/client`). teetui ! show in the log the game events a vanilla DDNet client shows: player JOIN (enters game), LEAVE (+reason), join SPEC, join TEAM red|blue|game, SWITCH team, DEATH — who killed whom (+weapon) and plain death (self/world) — ALL generated CLIENT-side from twclient's UNIFIED events (`EventPlayerJoin`/`EventPlayerLeave`/`EventTeamSet`/`EventKill`, 0.6+0.7, via generic `client.On[E]`), ⊥ raw packet (V1/V8). own feature pkg `features/serverlog` on the public SDK. needs SDK EXTENSION: new OPTIONAL handler ifaces (`PlayerJoinHandler`/`PlayerLeaveHandler`/`TeamChangeHandler`) wired from the dialer (`KillHandler` already exists, but core logs NO kill today). ⊥ DUPLICATE a line when a 0.6 server ALSO sends the same as system chat — dedupe|prefer events. names via roster + id-fallback (V26). verify LIVE on all 3 e2e servers (V23).
- C31: NO MOUSE — teetui is KEYBOARD-ONLY. ⊥ `scr.EnableMouse()`, ⊥ handle `*tcell.EventMouse` (mouse forwarding is unreliable over ssh/tmux/many terminals & adds nondeterminism). everything reachable by keys; log scroll = `PgUp`/`PgDn` ONLY (drop mouse-wheel). (supersedes the wheel-scroll in §I.windows/§T30)
- C32: HELP EXPLAINS MODES. `?` help overlay ! do more than list keys — ! TEACH a newcomer: per interactive MODE (chat, team chat, local console, remote console/rcon, server browser, scoreboard, visual, free-look) give (a) the key to ENTER it, (b) ONE line of WHAT it is (e.g. "console = a command line to set options (cvars) and run commands like connect/say/help"; "rcon = remote admin console, needs the server password"), (c) how to EXIT (`Esc`). beginner-legible; still escapable (V17) + screen-clamped (V30).
- C33: LIVE RENDER ON TICK. render is event-driven (keys/resize/wake) — but a game TICK ⊥ trigger a redraw today, so the live view (tees/projectiles/camera ease/connecting spinner) FREEZES between input events. fix: each observed tick → `wake()` → a THROTTLED redraw, so the view animates at up to `cl_max_fps` (V42 coalesces + trailing draw, V7 no per-frame alloc, tcell cell-diff makes a no-change frame cheap). ⊥ redraw when idle/disconnected (no ticks arrive → no wake, V71 stays idle). (recommended fix over a free-running frame ticker: redraw only on real new state)
- C34: TEE INPUT MODEL — mimic DDNet under terminal limits. DDNet (ddnet `CControls`) builds `CNetObj_PlayerInput` EVERY tick from CURRENTLY-HELD keys: `m_Direction` (-1/0/1 from held left/right), `m_Jump` (1 while held), `m_Hook` (1 while held), `m_Fire` (counter, +1 per press EDGE), `m_TargetX/Y` (mouse); on key RELEASE the field returns to neutral. a terminal has NO key-release → emulate "held" via terminal KEY-REPEAT + DECAY: a movement/jump press sets the state + a short HOLD WINDOW that auto-repeat refreshes; once the window lapses (key released → repeats stop) the field returns to neutral (dir 0, jump 0). ⊥ a single press latching movement/jump FOREVER (B10). fire = edge counter (⊥ held); hook = explicit TOGGLE (terminal-sane); aim = sticky cardinal target (persists like a cursor rest). hold window > the terminal key-repeat interval (⊥ stutter) but short enough that release stops promptly; configurable `cl_input_hold_ms` (?default ~350). all input via `client.Do(ActInput)` ONLY (V12).
- C35: ESC OVERLAY MENU (← DDNet game menu, keyboard-only C31). when CONNECTED (spec OR ingame), `Esc` TOGGLES a top-of-viewport overlay action bar (the DDNet "Game" tab). KEYBOARD-navigable (⊥ mouse, C31): `←`/`→` or `Tab`/`Shift-Tab` move focus, `Enter` activates, `Esc` closes. buttons CONTEXT-set by game mode + state: TEAM mode (`GameFlags & GAMEFLAG_TEAMS`) → "Join red"/"Join blue"; SOLO/non-team → "Join game"/"Spectate"; when in-game add "Kill" + "Pause"; ALWAYS "Connect dummy" (only if `Capabilities().AllowDummy`) + "Disconnect". actions route to the existing safe surface — `ActSetTeam` (red 0\|blue 1\|spectators -1), `ActSetSpectator`, `ActKill`, `/pause` via chat, Disconnect, dummy-connect — ⊥ raw packet (V12). `Esc` ⊥ trapped (always closes); when closed the bar ⊥ steal movement/aim keys (C34). NON-GOALS (⊥ port): record demo, in-client settings/editor GUI — terminal uses config file + console.
- C36: DUMMIES / MULTI-SESSION (← DDNet dummy). teetui ! connect EXTRA clients ("dummies") to the same server — any number the server allows (per-IP limit ENFORCED server-side; gate the button on `Capabilities().AllowDummy`). each dummy = its OWN `client.Client` + Observer(`State`) + `InputController`, fully Connected (V22/V25). exactly ONE session ACTIVE: render reads the active session's `State`, input drives ONLY the active session's `Controller`; other sessions stay connected idle. the Esc menu lists own clients (main + dummies) → selecting one makes it ACTIVE = FOLLOW it + render from ITS perspective (camera resets, T43). Disconnect/Stop closes ALL sessions (⊥ leak goroutines/sockets). all via twclient public API (V1) — a dummy = just another connection, ⊥ special net.
- C37: BROADCAST OVERLAY (← DDNet `CBroadcast`). server broadcasts (`EventBroadcast`) render as a TRANSIENT centered overlay near the TOP of the viewport — like the graphical client — NOT (only) a log line. shown for the DDNet duration (~10s) since arrival, then FADES (dims) over the last ~1s and vanishes. driven by the per-tick redraw (V72); a new broadcast replaces the old + resets the timer; empty broadcast clears it. clamps to width (V30), ⊥ block other UI. supersedes logging broadcasts as `>>` lines (overlay replaces it, match GUI).
- C38: RELEASE PIPELINE. cross-OS STATIC binaries (linux/windows/darwin × amd64/arm64) via GoReleaser, `CGO_ENABLED=0` (pure Go, C1 — fully static on linux/windows; darwin cgo-free, not fully static = Apple libSystem, best achievable). TWO outputs: (a) EVERY push/PR CI run → `goreleaser build --snapshot` → binaries uploaded as GH Actions WORKFLOW ARTIFACTS (transient, per repo retention); (b) a `v*` tag → `goreleaser release` → archives (tar.gz/zip) + `checksums.txt` attached to the GitHub RELEASE page (permanent). already present: `.goreleaser.yaml`, `.github/workflows/release.yml` (tag→release ✓). GAP: CI ⊥ upload the built binaries (only coverage) → add a snapshot-build+upload job.
- C42: DISCONNECT SCOPE (dummy vs whole connection). TWO Esc-menu disconnect actions, scoped: (1) "Disconnect dummy" — shown ONLY when the active session is a DUMMY — closes ONLY that dummy, drops it, refollows the PRIMARY (render from main's perspective), main + other sessions STAY connected, ⊥ open browser, ⊥ reconnect. (2) "Disconnect" — closes the WHOLE connection = the primary AND every dummy (a dummy ⊥ outlive its primary) → back to the server browser, ⊥ reconnect. disconnecting the PRIMARY (button (2) or a primary drop) ALWAYS tears down all dummies too. fixes B20 (Disconnect on an active dummy closed only the dummy yet opened the browser while main stayed connected; no way to end all from a dummy; primary disconnect leaked dummies). (← user: disconnect dummy only, or disconnect main → both; extends C36/V77)
- C41: VIEWPORT MIN REDRAW RATE (liveness FLOOR). guarantee ≥ N COMPLETE redraws/sec of the ingame VISUAL viewport (game scene rect, shown when visual ON, toggle `V`/`v`) while CONNECTED — independent of tick/event arrival. DISTINCT from `cl_max_fps` (the CEILING, C20/V42): cap bounds bursts, this sets a FLOOR. everything OUTSIDE the scene rect (HUD/coords, log band, chat, status, input-legend, overlays "above the viewport") ⊥ REQUIRED to redraw on the floor heartbeat — may stay event-driven. configurable RATE/sec `cl_viewport_min_fps` (default 1; 0 = disabled → pure event/tick-driven, today C33). MECHANISM: a heartbeat at 1/rate forces a COMPLETE viewport repaint (scene cells flushed even if cell-diff would skip — `scr.Sync`) IFF no draw happened within the interval (⊥ double-draw when ticks/events already met the rate). floor ≤ ceiling: clamp `cl_viewport_min_fps` ≤ `cl_max_fps` when capped (>0); cap (V42) ⊥ exceeded; ⊥ per-frame steady alloc (V7); ⊥ block input/tick goroutines. INACTIVE when disconnected | visual OFF | rate==0 (no scene → V71 idle preserved).
- C40: DEFINITION OF DONE / RELEASE HYGIENE. a change ⊥ "done" + a tag ⊥ cut until the repo passes the full quality gate: deps at LATEST, formatted, generated-current, tidy, vuln-free. ⊥ ship stale deps, ⊥ ship gofmt drift, ⊥ ship untidy `go.mod`/`go.sum`, ⊥ ship known CVEs. gates run in CI (each own step) + locally before tag (V83 release). gates: `go get -u ./... && go mod tidy` (deps latest), `gofmt -l .` (empty), `go generate ./...` (no diff), `go mod tidy` (no go.mod/go.sum diff), `go vet ./...`, `govulncheck ./...` (no findings). dep bump that breaks build/tests → pin + note, ⊥ silently skip.
- C39: MAP DOWNLOAD UX (← gh issue #1). twclient `Connect(ctx)` does handshake+login+MAP DOWNLOAD together and on download failure CONNECTS ANYWAY ("map download failed, continuing without map", `client/client.go:425`) → teetui ends `connected` with `st.Map==nil`: chat flows but the game window shows the indefinite `connecting…` placeholder FOREVER (can't tell "downloading" from "connected, no map"); a reconnect re-downloads OK. teetui ! (a) when `connected && st.Map==nil`, show a CLEAR actionable notice ("map not loaded — press R to re-download"), ⊥ an endless spinner. PROGRESS: the map downloads INSIDE `Connect` with NO progress callback (twclient gap) → a real % bar needs twclient support → FILE jxsl13/twclient issues (C16): (i) failed map download silently connects-without-map (should error/retry or expose state), (ii) expose map-download progress (bytes/total). render a progress bar once twclient emits it; until then the joining spinner + the connected-no-map notice.

## §I — interfaces

### cli
- cmd: `teetui [flags]` → opens TUI, connects.
- flags (FINAL, §C23): ONLY `-config <file>` (optional). NO other flags. no file → defaults + open browser.
- config file = teeworlds-style `.cfg` (← TW client/server cfg): one `command [args]` per line, `#` comments, run via the console layer at startup (exec semantics). cmds = cvars (`cl_max_fps 60`, `cl_viewport_min_fps 1`, `player_name "foo"`, `player_clan "x"`, …) + `connect <addr> [0.6|0.7]`. identity = cvars; ⊥ skin (dead). version per-connect (master entry | `connect` arg | default 0.6).
- OLD flags removed: `-server`/`-name`/`-clan`/`-skin`/`-version`/`-password`/`-no-color`/`-connect-timeout`/`-max-fps`/`-log-lines` → all expressible as config cvars/cmds.
- env: NONE (C2).
- config file: warlist dir + key history (mirror chillerbot `chillerbot/warlist/*`, `chillerbot/history/*`). path `~/.config/teetui`.
- file: `README.md` — usage + ALL attributions/credits/references. ! credit chillerbot-ux (https://github.com/chillerbot/chillerbot-ux, orig author ChillerDragon, reference UX), DDNet (https://ddnet.org), Teeworlds (https://www.teeworlds.com), twclient (github.com/jxsl13/twclient), tcell (github.com/gdamore/tcell), go-runewidth. ! state licenses + that teetui = independent Go re-impl, not fork.

### twclient surface consumed (verbatim)
```
client.New(addr string, ...Option) *Client
client.WithPlayerInfo(name,clan,skin string, country int) Option
client.WithVersion(packet.Version06|Version07) Option
client.WithPrediction(bool) / WithAntiping(bool) Option
client.WithObserver(Observer) / WithController(Controller) Option
(*Client).Connect(ctx) error ; .Close()
(*Client).AddObserver(Observer) (remove func())
(*Client).SetController(Controller)
(*Client).Do(Action) error                       // input/chat/vote/emote/kill/spec
(*Client).SendChat(msg) ; .RangePlayers(func(id int, ch CharacterState) bool)
(*Client).Character() CharacterState ; .LocalID() int
client.TickState{Tick,IntraTick,LocalID,Players map[int]CharacterState,
  Projectiles[]packet.ProjectileState,Lasers[]LaserState,Pickups[]PickupState,
  Flags[]FlagState,Map *MapView,GameInfo GameInfoState,RaceTime,Events[]packet.Event}
client.CharacterState{Tick,X,Y,VelX,VelY,Angle,Direction,Jumped,HookedPlayer,
  HookState,HookTick,HookX,HookY,Health,Armor,AmmoCount,Weapon,Emote,AttackTick}
client.Observer{Mode() TickMode; Observe(*Client, TickState)}
client.Controller{Mode() TickMode; OnTick(*Client, TickState) []Action}
client.TickModeFrame  // smoothed, for UI ; TickModeFixed = 50Hz exact
Actions: ActInput{packet.PlayerInput}, ActChat{Team,Msg}, ActWhisper{ToID,Msg},
  ActEmoticon{packet.Emoticon}, ActKill{}, ActVote{Approve}, ActCallVote{Type,Value,Reason},
  ActSetTeam{Team}, ActSetSpectator{TargetID}
callbacks: (*Client).OnChat/OnWhisper/OnBroadcast/OnServerMsg/OnVoteSet/OnVoteStatus/
  OnKill/OnEmoticon/OnHookedBy/OnWeaponChange(fn) → func() unregister
master.New(...Option) *Client
(*master.Client).FetchServerList(ctx) ([]ServerEntry, error)
(*master.Client).QueryServerInfo(ctx, packet.Version, addr) (ServerInfo, error)
(*master.Client).ScanLAN(ctx, ...ScanOption) ([]LANServer, error)   // twclient v0.2.3: real LAN broadcast scan (0.6+0.7)
master.LANServer{Addr string, Version packet.Version, Info ServerInfo}
master.WithScanPorts(lo,hi)/WithBroadcastAddrs([]string)/WithScanTimeout(d) ScanOption // default ports 8303-8310, bcast 255.255.255.255, 2s
master.ServerEntry{Addresses[]Address, Location string, Info packet.ServerInfo}
packet.ServerInfo{Name,GameType,MapName,Passworded,NumPlayers,NumClients,
  MaxPlayers,MaxClients,Clients[]PlayerInfo}
packet.PlayerInfo{Name,Clan,Country,Score,IsPlayer}
packet.PlayerInput{...}  // movement/aim/jump/hook/fire/weapon
```

### tui windows (← chillerbot terminalui.h CWindowInfo + g_GameWindow)
LAYOUT = VERTICAL STACK, top→bottom (C22, supersedes old left/right split): status(top) / game-visual / log band / input-legend(bottom). logs sit DIRECTLY above the input-legend bar; the visual render sits ABOVE the logs.
- info window: status bar — input mode, server, race time, ping, fps. TOP row.
- game window: ASCII map + tees, camera on local tee (§I.render). FULL-WIDTH, between status and the log band, shown when visual on; pushes the log band down to its configured size.
- log window: chat/console/server-msg scrollback. FULL-WIDTH band just above the input-legend. visual ON → `cl_log_lines` rows (default 10, capped ⌊h/2⌋); visual OFF → fills the whole body.
- input window: textbox + cursor + tab-completion preview (grey) + reverse-i-search prompt; doubles as the key-legend bar. BOTTOM row.
- scoreboard (toggle): cols `score|name(20)|clan(20)`, per DDTeam.
- server-browser list (toggle): from `master.FetchServerList`, searchable, select→connect.
- help page (toggle). popup: MESSAGE | NOT_IMPORTANT | DISCONNECTED | WARNING.
- broadcast overlay (§C37): transient top-centered server broadcast, shown ~10s then fades — like the DDNet GUI; ⊥ a log line.

### input modes (← terminalui.h enum) + search variants
`OFF | NORMAL | LOCAL_CONSOLE | REMOTE_CONSOLE | CHAT | CHAT_TEAM | BROWSER_SEARCH`
+ reverse-i-search overlay per mode. per-mode input history (16 deep), persisted to disk.

### render mapping (← maplayers.cpp / renderer.go tiles)
camera: local tee centered in Game rect. frame = FULL Game rect (scales w/ terminal, T58/V31 — no fixed 64×32 cap; orig chillerbot frame was ≤64w×32h). map `MapView` tile index → glyph+`tcell.Style`:
```
tile        glyph   color(chillerbot→teetui RGB)
SOLID       █       grey {180,180,180}
FREEZE      ▒       cyan {0,180,255}
UNFREEZE    ░       {0,255,180}
DEATH       x       red  {200,40,40}
UNHOOK      🮽       {100,100,200}
THROUGH*    🔳      —
START       (cell)  green{0,255,0}
FINISH              magenta{255,0,255}
CHECKPOINT          {255,180,0}
TELEPORT           {200,100,255}
BOOST              {255,255,0}
air         space   —
tee self    o (ø ninja)  red {255,50,50}
tee other   o            blue{60,120,255}
hook line              yellow{255,230,0}
laser                  violet{180,0,255}
projectile             orange{255,160,0}
```
glyphs ?configurable (chillerbot `m_aTileSolidTexture`/`m_aTileUnhookTexture`).

### key bindings (← feature-video transcript; target = chillerbot parity)
```
?        toggle help page (works anywhere — shows available keys)
B        open server browser
Enter    close popup | join selected server | submit input
F1       open LOCAL console (config/cmds + tab-complete + help-text line)
F2       open REMOTE console (rcon: type pw → auth → admin cmds + complete)
T        chat (all)
Z        team chat
H        auto-reply to last ping (chillerbot reply-to-ping known-msg)
V        toggle visual mode (game map+tee render)
K        self-kill (ActKill)
Tab      in-game: scoreboard | browser: select | input: name/cmd complete
/        browser search bar
←/→      browser tab switch (Internet|LAN|Favorites|DDNet|KoG)
↑/↓      browser select | input: history prev/next
PageUp/Dn: scroll log (KEYBOARD-ONLY, ⊥ mouse-wheel — §C31)
Ctrl-R   reverse-i-search input history
Ctrl-U/K/W: readline kill (line-before / line-after / word)
WASD / arrows  movement+aim, SELECTABLE via `cl_move_keys` (wasd|arrows, default wasd): chosen set = jump/left/stop/right, OTHER set = cardinal aim (§C29/§V66). sticky; `Space` always jumps. left-/right-handed friendly.
G        toggle FREE-LOOK map-pan (visual on); arrows|WASD pan, Esc|G recenter+exit (C24, rebindable; default `G`, ⊥ collide aim/move)
j        JOIN the game (team 0) — `features/team` DefineAction → Host.Do(ActSetTeam{0}) (§T97/§V57/§V52); key for the existing console `join`
```
CHILLERBOT IN-GAME COMMANDS (research, 2026-06-16 @ chillerbot-ux 14331d5): chillerbot's CLIENT-side in-game chat commands are `.`-prefixed (TAB-completable, `chathelper.cpp`/`chatcommands.h`) = the WARLIST set ONLY: `.addwar .addteam .peace .war .team .unfriend .addreason .search .create`. teetui already covers these as `!war/!peace/!team/!del` chat + warlist console (§T60/§T78, `features/warlist`/`features/chatfilter`). Everything else a player types in-game with a leading `/` (e.g. DDNet `/pause /spec /kill /emote /r /w /rank /top5 /save /load /team /lock /mc /cp /info /rules`) is a SERVER command, mod-specific, ⊥ client feature — teetui sends any chat line verbatim (incl. leading `/`) to the server (V37), so they already work. teetui ⊥ reimplement server `/`commands. teetui-side gap = a KEY to JOIN (console `join` existed, no keybind) → §T97.
NOTE: `[V]detail` in the live legend is MISLABELED — `V` = `subcell` half-block render toggle (T46), NOT map navigation. free-look pan = its own action (`actFreeLook`, C24).
LEGEND/HELP ARE GENERATED (C25): the bottom input-legend + `?` overlay are built from the live keymap + feature `DefineAction`s each render — ⊥ hardcoded; legend shows context-available important cmds, help shows ALL bindings. this table = the DEFAULTS those reflect.
ESC MENU (§C35/§V74): when connected, `Esc` toggles a top overlay action bar; `←`/`→`/`Tab` focus a button, `Enter` activates, `Esc` closes (keyboard-only, ⊥ mouse). buttons (context): team→`Join red`/`Join blue` | solo→`Join game`/`Spectate`; +`Kill`/`Pause` in-game; +`Connect dummy` (iff server `AllowDummy`); +`Disconnect`; + a session list (main + dummies) to FOLLOW (render from that client). actions = `ActSetTeam`/`ActSetSpectator`/`ActKill`/`/pause` chat/Disconnect/dummy-connect (V12). SESSIONS (§C36/§V76/§V77): teetui holds `sessions []*session{client,*State,*InputController}` + `active int`; render+input use `sessions[active]`; dummies are extra sessions up to the server per-IP limit.
NOTE: current foundation keymap diverges (`t`/`y` chat, `h` hook, `q` quit) — reconcile to this table under T11/T16.
browser tabs: Internet | LAN | Favorites | DDNet | KoG. selected server highlighted. map download → progress bar on join.
in-game HUD: live local-tee coords (tile x,y) shown (← transcript).
chillerbot AFK: headless → detected "tapped out" always; `cl_tapped_out_message` config toggles auto-msg.
keybinds NOT rebindable yet (chillerbot limitation; ?future config).

### extension / hooks (teetui-specific, exceeds reference — C19)
Out-of-scope features (§C18) are NOT shipped but ARE user-buildable via a stable
hook API. teetui provides PRIMITIVES (events + a safe action surface), not policy.
```
pkg github.com/jxsl13/teetui/extension     // stable public surface
type Hook interface {                       // implement any subset (embed NopHook)
  OnConnect(HookCtx)
  OnDisconnect(HookCtx, reason string)
  OnChat(HookCtx, ChatEvent) (suppress bool)   // true → hide line from log
  OnBroadcast(HookCtx, string) ; OnServerMsg(HookCtx, string)
  OnKill(HookCtx, KillEvent) ; OnTick(HookCtx, client.TickState)
  OnKey(HookCtx, Key) (handled bool)           // true → consume key
}
type HookCtx interface {                    // SAFE action surface only (V1/V12)
  SendChat(msg string, team bool) ; Do(client.Action) error
  Log(style, msg string) ; Roster() []client.PlayerState
  Config(name string) (string, bool) ; Server() string
}
extension.Register(name string, h Hook)     // in-process Go hook (compiled in)
```
+ EXTERNAL command hooks (opt-in, no recompile): executables in
`~/.config/teetui/hooks/<event>` fed event JSON on stdin; stdout action lines
(`say …`, `do …`) parsed back. timeout-bounded, off unless dir present.
Hook surface = teetui's existing twclient public API ONLY — ⊥ raw packet/net/flood
primitive (⊥ a DoS amplifier). User hooks run under USER responsibility.

### feature modules (v2 — supersedes "extension / hooks" above, C21)
Every chillerbot feature = a self-registering module (← Caddy v2 / image stdlib).
The §I "extension / hooks" surface (Hook/HookCtx, T69-71) is FOLDED into this
richer, sufficient API. external command hooks become `features/cmdhook`.
```
pkg github.com/jxsl13/teetui/feature        // public module SDK
type Feature interface {
  Name() string                              // unique id (← ModuleInfo.ID / format name)
  Provision(Host) error                      // declare config/actions/status, look up deps
  Events                                      // embed NopFeature for unused events:
}                                             //  OnConnect/OnDisconnect/OnChat(→suppress)/
                                              //  OnBroadcast/OnServerMsg/OnKill/OnTick/OnKey(→handled)
feature.Register(Feature)                     // called in each feature pkg init()
feature.Registered() []Feature
// NopHost: a no-op Host to embed in tests/harnesses (override only what you use)

type Host interface {                          // the SUFFICIENT capability surface
  // actions (safe twclient API only, V1/V12 — no raw net/DoS, V39)
  SendChat(msg string, team bool); Do(client.Action) error; Log(style, msg string)
  // state
  Roster() []client.PlayerState; Tick() (client.TickState, bool)
  PlayerName() string; PlayerClan() string; Server() string
  // config: each feature OWNS its cvars (declared at Provision, V46)
  DefineConfig(name, def, help string); Config(name string) (string, bool)
  // outgoing-chat filter chain (for !commands / silent-chat, returns edited+send)
  AddSendChatFilter(func(msg string, team bool) (out string, send bool))
  // named, REBINDABLE actions (respect keymap, V19) + default key (for H, etc.)
  DefineAction(name, defaultKey, help string, run func())
  // F1 console commands (for !filter mgmt, `team`/`join`, … — replaces inline core cmds)
  DefineCommand(name, help string, run func(args string) (out []string))
  // status-bar / HUD contributions (for cl_show_last_ping, coords, …)
  AddStatusField(func() string)
  // render contributions (warlist name coloring into scoreboard/nameplate)
  AddNameStyle(func(name, clan string) (Style, bool))
  // cross-feature services (← caddy ctx.App): a feature Provides, others Lookup
  Provide(name string, svc any); Lookup(name string) (any, bool)
}
```
SERVICES are passed as `any` (V53): the providing feature `Provide`s its concrete
value; the CONSUMER `Lookup`s by name and type-asserts to a MINIMAL interface it
declares ITSELF. the public `feature` SDK ⊥ declare feature-specific service
contracts (⊥ `feature.Warlist`, ⊥ `feature.PingStore`) — those belong to the
consumer (or the provider's own pkg), keeping the SDK generic & feature-agnostic.
A feature panic in Provision/hook ⊥ crash core (recover+disable+log, V40/V47).
`main.go` (sole feature wiring):
```
import (
  "github.com/jxsl13/teetui/internal/tui"
  _ "github.com/jxsl13/teetui/features/warlist"
  _ "github.com/jxsl13/teetui/features/replytoping"
  _ "github.com/jxsl13/teetui/features/chatquery"
  _ "github.com/jxsl13/teetui/features/chatfilter"
  _ "github.com/jxsl13/teetui/features/responders"
  _ "github.com/jxsl13/teetui/features/lastping"
  _ "github.com/jxsl13/teetui/features/chillpw"
  _ "github.com/jxsl13/teetui/features/cmdhook"
)
func main(){ tui.Main() }   // base client provisions all feature.Registered()
```

### feature modules (v3 — supersedes the v2 `Events` monolith, C27)
Research-driven SDK reshape. `Feature` shrinks to IDENTITY; init, lifecycle and
every event become SMALL OPTIONAL interfaces the core discovers by type assertion
(Go optional-interface idiom, names from the stdlib `-er` rule + `io.Closer` +
`net/http.Handler`). Adding a new optional interface later ⊥ break any existing
feature (forward-compatible). The capability surface is `feature.API` (⊥ `Host` —
teetui = terminal client, ⊥ server).
```
pkg github.com/jxsl13/teetui/feature

type Feature interface { Name() string }            // identity ONLY

// lifecycle — all OPTIONAL, asserted per feature:
type Initializer interface { Init(API) error }      // declare cvars/actions, look up deps
type Validator   interface { Validate() error }     // verify config after Init
type Closer      interface { Close() error }        // release goroutines/files (← io.Closer);
                                                    //   runs on shutdown + panic-disable; safe
                                                    //   even after PARTIAL init

// events — small OPTIONAL handlers (idiomatic `…Handler`), implement only what you need;
// NopFeature is REMOVED (no forced no-op stubs):
type ConnectHandler   interface { OnConnect(API) }
type DisconnectHandler interface { OnDisconnect(API, reason string) }
type ChatHandler      interface { OnChat(API, ChatEvent) (suppress bool) }
type BroadcastHandler interface { OnBroadcast(API, string) }
type ServerMsgHandler interface { OnServerMsg(API, string) }
type KillHandler      interface { OnKill(API, KillEvent) }
type TickHandler      interface { OnTick(API, client.TickState) }
type KeyHandler       interface { OnKey(API, Key) (handled bool) }
// §C30/§V68 — player/team events (forward-compat additions, V60):
type PlayerJoinHandler  interface { OnPlayerJoin(API, PlayerJoinEvent) }
type PlayerLeaveHandler interface { OnPlayerLeave(API, PlayerLeaveEvent) }
type TeamChangeHandler  interface { OnTeamChange(API, TeamChangeEvent) }
// + structs PlayerJoinEvent{ClientID,Name,Clan,Team}, PlayerLeaveEvent{ClientID,Reason},
//   TeamChangeEvent{ClientID,Team,Silent} (SDK-defined, ← packet events).

// API = the teetui-client capability surface, COMPOSED from small named
// sub-interfaces (ISP) — a handler/consumer may depend on just the slice it needs:
type ChatSender     interface { SendChat(msg string, team bool) }
type ActionDoer     interface { Do(client.Action) error; RconLogin(pw string) }
type Logger         interface { Log(msg string) }
type StateReader    interface { Roster() []client.PlayerState; Tick() (client.TickState, bool)
                                PlayerName() string; PlayerClan() string; Server() string }
type ConfigStore    interface { DefineConfig(name, def, help string); Config(name string) (string, bool) }
type ActionRegistry interface { DefineAction(name, defKey, help string, run func())
                                DefineCommand(name, help string, run func(args string) []string) }
type UIRegistry     interface { AddStatusField(func() string); AddNameStyle(func(name, clan string) (Style, bool))
                                AddSendChatFilter(func(msg string, team bool) (out string, send bool)) }
type ServiceRegistry interface { Provide(name string, svc any); Lookup(name string) (any, bool) }  // V53
type Paths          interface { DataPath(name string) string }
type API interface { ChatSender; ActionDoer; Logger; StateReader; ConfigStore
                     ActionRegistry; UIRegistry; ServiceRegistry; Paths }

// NopAPI (renamed from NopHost) — no-op API to embed in tests. core Fire*/lifecycle
// dispatch type-asserts each registered Feature to the handler/lifecycle interface
// and skips those it does not implement; dispatch panic-isolated (V40/V47).
```
Migration: every `features/*` drops `feature.NopFeature` + implements only its
handlers (+ `Initializer` if it needs setup). behavior identical (V44). main.go unchanged.

## §V — invariants

- V1: all server comms via `twclient` pub API only. ⊥ import net6/net7/network/packer from teetui. (C2)
- V2: render reads `client.TickState` from registered `Observer`/`Controller`. ⊥ direct snap poll. (C5)
- V3: Observer `Mode()==TickModeFrame` (smoothed) for visual render. ML/fixed ⊥ used by UI.
- V4: tcell `PollEvent` runs own goroutine; shared UI state guarded by mutex; ⊥ data race on TickState / input buffer. (C8)
- V5: every color → `tcell.Style`; on non-truecolor term tcell downsample ! ⊥ crash/garbage; `-no-color` → mono renders legibly. (C4)
- V6: utf8 glyph column advance via runewidth; wide glyph ⊥ corrupt cell grid / overrun window border. (C9)
- V7: render hot path = zero steady-state heap alloc per frame (reuse frame buffers); only changed cells `SetContent`. bench-proven. (C7)
- V8: both Version06 & Version07 connect+render+chat identical from user view; ⊥ version branch above twclient. (C6)
- V9: input mode state machine total — every mode has defined key handling + exit; locked key ignored until release (no key bleed across context switch). (chillerbot m_LockKeyUntilRelease)
- V10: per-mode input history bounded 16; oldest evicted; persisted load/save ⊥ lose/corrupt entries.
- V11: on disconnect/kick → DISCONNECTED popup + twclient auto-reconnect; UI ⊥ hang/panic. resize term → relayout, ⊥ crash.
- V12: tee control input sent via `client.Do(ActInput)` / Controller only; ⊥ teetui craft raw input packets.
- V13: server browser list from `master.FetchServerList`; password server flagged (`ServerInfo.Passworded`) before connect.
- V14: warlist war/peace/team tag colors applied to nameplates + scoreboard names consistently; chat `!war/!peace/!team/!delteam` mutate same store.
- V15: README ! present + credit chillerbot-ux/ChillerDragon, DDNet, Teeworlds, twclient, tcell + licenses; ⊥ ship w/o attribution. (I.cli README)
- V16: input history persisted to disk + reloaded next start; ⊥ lose entries across restart. (← transcript "history persisted across restarts")
- V17: `?` help overlay openable+closable from any mode/context, lists that mode's keys; ⊥ trap user (always escapable). (← transcript)
- V18: visual-mode toggle + popups + browser ⊥ break on terminal resize (relayout, no crash/garble). (← transcript "doesn't break if you resize")
- V19: keymap rebindable via config; default = §I table; ⊥ hardcoded-only (exceed reference). (C12)
- V20: ∀ chillerbot transcript feature → teetui equiv ≥ parity (C10 checklist). render legible+colored beyond 6-pair (C11). ⊥ ship feature strictly worse than reference.
- V21: popup ⊥ swallow keys it advertises. greeting popup ! act on `B`→browser & `?`→help while shown (⊥ require Enter-first). (← B1)
- V22: after EVERY successful `Connect`, ! `go Client.RunFrontends(fctx)` — that loop drives Observer(render)+Controller(input). `Connect` alone ⊥ dispatch. fctx long-lived (≠ connect timeout), cancelled on next Join/Stop. (← B2)
- V23: ⊥ mark §T `x` w/o LIVE-server pass. connect+snapshot ! verified against live ddnet(0.6), ddnet-sixup(0.7) & teeworlds7(0.7) via e2e harness (C14). (← B3)
- V24: connect failure ! surface actionable msg in log (addr + version + "check network"), ⊥ silent hang past timeout.
- V25: ctx passed to `client.Connect(ctx)` = SESSION LIFETIME (twclient binds reader+keepalive+I/O to it; docstring "context governs the entire client lifetime"). ⊥ pass short-timeout ctx, ⊥ `defer cancel()` firing after Connect returns. session ctx = long-lived (= fctx, cancel on next Join/Stop). handshake timeout via watchdog cancelling ONLY while still connecting. (← B4)
- V26: 0.6 roster names ?empty (twclient gap — 0.6 `Sv_ClientInfo`/`ObjClientInfo` ⊥ decoded to registry; e2e: 0.6 roster=0 vs 0.7-sixup=5). teetui ! degrade gracefully (id fallback when name empty), ⊥ blank/crash. REAL fix = twclient 0.6 ClientInfo decode (SPEC-player-registry T6). (← B5) [RESOLVED upstream: twclient v0.2.6 #6/#3 — 0.6 Roster()/Player() now populate; teetui id-fallback kept as defense]
- V27: game render ! work as SPECTATOR / when local tee absent — center camera on spectated target | free-view coords | any visible tee; ⊥ require `Players[LocalID]` (else blank "connecting…"). (← B6)
- V28: connect-fail msg shown ONLY on terminal failure; ⊥ when a (re)connect then succeeds. connectTimeout generous/configurable for real-server map-download; watchdog ⊥ abort a still-progressing handshake. (← B7)
- V29: sent chat ! echoed LOCALLY into log immediately (⊥ depend on server echo — some servers ⊥ echo own line; 0.6 echo has empty name). dedupe if server also echoes. (← B8)
- V30: layout FULLY responsive — every window rect + EVERY overlay (scoreboard/help/popup/browser) computed from current terminal size each render; resize → immediate relayout+redraw; ⊥ stale dims, ⊥ draw past screen bounds (overlays clamp/reflow to fit), ⊥ crash on any size ≥ min. (extends V11/V18; C17)
- V31: game render FILLS the available Game rect — camera frame = rect, scales UP and DOWN w/ terminal (larger terminal ⇒ more visible map = higher res); ⊥ hard-capped to fixed 64×32 (wastes big | garbles small); HUD/coords stay in-bounds. (C17, supersedes §I.render cap)
- V32: below a min usable size (Wmin×Hmin, defined) UI degrades to ONE legible "terminal too small — resize to ≥ WxH" notice; ⊥ negative/zero-width draws, ⊥ panic; growing back ≥ min restores full UI identical to never-shrunk. (C17)
- V33: auto/H reply triggered ONLY by a real ping (own name highlight, ⊥ self, ⊥ non-ping); reply intent chosen by lang classifier (greeting/ask-to-ask/bye/insult/smalltalk/question-why·how·which·who/no-context-ping) multi-lang (en/de/fr/ru per chillerbot); rate-limited; ⊥ reply-storm. (← chillerbot langparser/replytoping/smalltalk)
- V34: chat-query answers derive ONLY from teetui state — warlist relation+reason, roster, map/coords; ⊥ fabricate. war-status answer ("is X war?"/"why kill me") = warlist store for that name (consistent w/ scoreboard colors, V14). (← chathelper check_war/list_wars/where)
- V35: last-ping queue bounded 16, newest-first; H replies newest + can cycle older; eviction ⊥ corrupt/lose order. (← chathelper m_aLastPings)
- V36: incoming chat spam/insult/user filters hide ONLY matching lines per `cl_chat_spam_filter`(0/1/2)+filter list; ⊥ hide own line, ⊥ hide non-matching; off by default; mode 2 = hide+autoreply. (← chathelper FilterChat/IsSpam)
- V37: outgoing chat rate-limited via spam-safe send buffer (≤N queued, min interval) — ⊥ flood/trip server mute; FIFO order preserved; full→deterministic queue/drop. (← chathelper SayBuffer)
- V38: chillpw auto-login reads opt-in local secrets file, matches by server addr, sends pw ONLY to that server; secret ⊥ logged/echoed/saved elsewhere; inactive unless flag+file present. (← chillpw, security)
- V39: hook API stable+documented (§I.extension); hooks receive events + an action ctx limited to teetui's twclient public surface (V1/V2/V12) — ⊥ raw packet/net/flood, ⊥ DoS amplifier. registered hooks dispatched in deterministic order; OnChat suppress + OnKey handled composable (first true wins, recorded). (C19)
- V40: a hook (Go or external) that panics / errors / times out ⊥ crash or hang teetui — recovered, logged, that hook disabled for the session; core UI continues. (C19)
- V41: hooks opt-in — none active by default; §C18 out-of-scope features ⊥ shipped by teetui but ARE implementable via the hook API; teetui ships primitives, ⊥ policy, ⊥ any abusive hook. (C18/C19)
- V42: render repaint capped at `cl_max_fps` (0=unlimited) — actual repaints/sec ⊥ exceed cap under any event/wake burst; coalesced draws ALWAYS converge to the latest state (trailing draw, ⊥ stale final frame); throttle ⊥ block input/tick goroutines, ⊥ per-frame alloc (V7); cap=0 → behaves exactly as today (every event draws). (C20)
- V43: import isolation — `internal/tui` (core) ⊥ import any `features/*`; `features/*` ⊥ import `internal/tui` — features depend ONLY on the public `feature` API + shared libs. enforced by a test scanning imports. (C21)
- V44: behavior parity — extracting a feature into its package ⊥ change observed behavior; the migrated chillerbot features (reply/query/filter/responders/warlist/lastping/chillpw/cmdhook) reproduce their pre-refactor effect exactly (same tests pass, relocated). (C21)
- V45: features self-register in `init()`; the active feature set = EXACTLY the packages blank-imported by `main.go`; `main.go` holds NO feature logic beyond imports + base-client start. (C21)
- V46: each feature OWNS its cvars (DefineConfig), keybinds (DefineAction, rebindable per V19) and status fields at Provision; core declares NONE of them; duplicate cvar/action names detected at registration. (C21)
- V47: Host API is sufficient + safe — a needed capability is added to the PUBLIC Host (⊥ core leak/global); action surface stays the twclient public API (no raw net/DoS, V39); a feature panic in Provision or any hook ⊥ crash core (recover+disable+log, extends V40). (C21)
- V48: layout is a VERTICAL stack top→bottom: status / game-visual / log band / input-legend; logs ALWAYS render directly above the input-legend bar; ⊥ left/right split. (C22)
- V49: visual ON → log band = clamp(`cl_log_lines`, 1, ⌊h/2⌋) rows (default 10), game fills the body above it; visual OFF → logs fill the whole body. log band ⊥ exceed ⌊h/2⌋ when visual on, for ANY h ≥ min (V32). (C22)
- V50: layout recomputed from live terminal size each render (C17/V30); resize re-clamps the log band; min-size guard (V32) still wins below Wmin×Hmin. (C22)
- V51: CLI surface = ONLY `-config <file>`; ∀ other setting via the config file (cvars/cmds) or runtime console — ⊥ per-setting flags. missing/partial file → defaults, ⊥ crash. connect protocol version = master/scan entry on browser/LAN join | `connect` arg | omitted → auto-detect (`VersionAuto`, V87); ⊥ global version flag. (C23)
- V52: team join/switch via `client.ActSetTeam{Team}` ONLY (V12) — ⊥ raw team packet. team ids: spectators=-1, red/game-flock=0, blue=1. non-team game → `join`=team 0. console `team <spectators|red|blue|game>` + `join`; distinct from spectate (V27/§T37, `ActSetSpectator`). exceeds chillerbot terminal (no team-select there). (← GUI client team menu)
- V53: public `feature` SDK is feature-AGNOSTIC — ⊥ declare any feature-specific service contract (`feature.Warlist`, `feature.PingStore`, …). cross-feature services flow through `Provide(name, any)` + `Lookup(name) any`, the CONSUMER declaring the minimal interface it needs and type-asserting. warlist/lastping/etc are normal features that USE the SDK, ⊥ part of it. (C21; extends V43/V47)
- V54: free-look pan sub-mode (visual ON only) — arrows|WASD pan camera in tile steps, decoupled from tee-lock (drawScene uses panned center, ⊥ cameraCenter); Esc|toggle recenter + exit → re-lock to tee. while active ⊥ send aim/move/any tee input (view-only, mode-gated, V9/V12/V27); HUD shows panned center tile + "[free-look]". entering ensures visual on; pan clamps in-bounds; reset on disconnect (camera.reset); resize/min-size safe (V30/V32). (C24)
- V55: input-legend GENERATED from live keymap + feature actions EACH render — context-available important cmds as `[key]label`, current bindings (V19 rebinds reflected), priority-ordered; ⊥ hardcoded; width overflow → drop lowest-priority entries (⊥ draw past bounds, V30). context-aware: free-look→pan/recenter/exit, normal→core cmds, browser/input-mode→that mode's keys. (C25)
- V56: `?` help overlay lists ALL bindings (core actions + every feature DefineAction key+help) sourced from keymap/registry, ⊥ stale hardcoded; grouped; always escapable from any mode (V17); clamps to screen (V30). rebinding a key updates BOTH legend (V55) + help. (C25)
- V57: JOIN key — `features/team` DefineAction (default `j`, OWNED by the feature per V46) → `Host.Do(client.ActSetTeam{Team:0})` (V12/V52, ⊥ raw packet) + logs outcome. same effect as console `join`/`team game`, different trigger. works from spectator or in-game; ⊥ require leaving free-look. (← user "join with a key"; extends V52)
- V58: overlay TABLE columns flex with rect width each render — scoreboard name/clan + browser name/gametype/map/plrs/ver derived from current width, ⊥ fixed; narrow → shrink|drop low-priority cols (clan first, then gametype/ver), wide → grow name; col sum ≤ width, ⊥ overflow (V30/V6). recompute on resize (V50). (C26, extends V30/V31)
- V60: event handling = SMALL OPTIONAL interfaces (`ChatHandler`/`TickHandler`/`KeyHandler`/…), type-asserted per feature — ⊥ monolithic `Events`. a feature implements ONLY the handlers it needs (⊥ forced no-op stubs, ⊥ `NopFeature`). `Fire*` asserts each registered feature to the handler iface; absent → skipped. adding a NEW optional handler iface ⊥ break existing features (forward-compat). (C27)
- V61: idiomatic naming — public SDK identifiers follow Go STDLIB conventions (⊥ Caddy names): single-method iface = method+`er` (`io.Reader`/`io.Closer`/`fmt.Stringer`) | `…Handler` (`net/http.Handler`); lifecycle release = `Close()`/`Closer` (`io.Closer`), ⊥ `Cleanup`/`CleanerUpper`; setup = `Init()`/`Initializer`, ⊥ `Provision`/`Provisioner`. ⊥ `…Interface`/vague `Events` bag; ⊥ pkg-name stutter; ONE obvious name per concept. capability surface = `feature.API` (⊥ `Host`: terminal client, ⊥ webserver framing). (C27)
- V62: feature LIFECYCLE optional + safe — `Initializer`(Init)|`Validator`(Validate)|`Closer`(Close, ← `io.Closer`). Validate runs after Init; Close runs on shutdown AND when a feature is disabled after a panic (V47), MUST work after PARTIAL init, ⊥ panic/leak; all dispatch panic-isolated (V40/V47). (C27)
- V63: `feature.API` COMPOSED from small named capability sub-interfaces (`ChatSender`/`ActionDoer`/`Logger`/`StateReader`/`ConfigStore`/`ActionRegistry`/`UIRegistry`/`ServiceRegistry`/`Paths`), `API` embeds them — ⊥ flat opaque bag; a consumer/handler may depend on the MINIMAL sub-surface (ISP, V53); action surface stays the safe twclient API (V47). (C27)
- V64: `internal/lang` matching is FOLD-NORMALIZED — FindWord/FindAnyWord/ContainsAny/ContainsName + all classifiers compare on a fold key (accent-stripped NFD→Mn-removed→NFC, then `cases.Fold`), ⊥ raw `strings.ToLower`. `café`≈`cafe`, composed≈decomposed `é`, `ß`/Greek-sigma/Turkish-i correct, `tschüss`≈`tschuss`. word-boundary kept (`helloween` ⊥ match `hello`); empty word/name ⊥ match. message folded ONCE per call; patterns pre-folded; off the render path (V7 n/a). (C28; refines V33)
- V65: lang fold uses ONLY Go-native libs — stdlib + `golang.org/x/text` (`unicode/norm`, `runes`, `cases`, `transform`), ⊥ third-party NLP. `cases.Caser`/`transform.Transformer` created per-call OR pooled, ⊥ shared across goroutines (V4). (C28)
- V66: tee movement/aim keys SELECTABLE via `cl_move_keys` ∈ {`wasd`,`arrows`} (default `wasd`) — the selected set = movement (jump/left/stop/right), the COMPLEMENT set = cardinal aim; both drive `client.Do(ActInput)` via the core `InputController` ONLY (V12). movement sticky (terminal ⊥ key-release); `Space` always jumps; switchable at runtime. ⊥ conflict with free-look pan (mode-gated, V54). (C29)
- V67: server game messages — ∀ {join, leave(+reason), spec, team red|blue|game, team-switch, kill(killer→victim,+weapon), death(no killer→self/world)} → exactly ONE DDNet-style log line, generated from twclient UNIFIED events (V1/V8 0.6+0.7), name via roster/id-fallback (V26); ⊥ duplicate vs a 0.6 server's own system chat (dedupe); ⊥ crash on unknown/-1 id. (C30)
- V68: SDK extension — new OPTIONAL handler ifaces `PlayerJoinHandler`/`PlayerLeaveHandler`/`TeamChangeHandler` (+ event structs `PlayerJoinEvent`/`PlayerLeaveEvent`/`TeamChangeEvent`) added to `feature` with `Fire*` dispatch + dialer wiring (`client.On[packet.EventPlayerJoin/EventPlayerLeave/EventTeamSet]`); adding them ⊥ break existing features (forward-compat, V60); dispatch panic-isolated (V40/V47). (C30; extends V60)
- V69: NO mouse — `scr.EnableMouse` ⊥ called and `*tcell.EventMouse` ⊥ handled (a mouse event reaching the loop = inert no-op, ⊥ panic); ALL interaction keyboard-only; log scroll via `PgUp`/`PgDn`. (C31)
- V70: `?` help has a MODES section — ∀ interactive mode {chat, team chat, local console, rcon, browser, scoreboard, visual, free-look}: enter-key + one-line purpose + exit (`Esc`); rendered with the generated key list (V56), escapable (V17), clamped (V30). (C32)
- V74: `Esc` (only when CONNECTED) toggles a top overlay action bar, keyboard-navigable (`←`/`→`/`Tab` focus, `Enter` activate, `Esc` close — ⊥ mouse, C31); buttons context-set by game mode (team→Join red/blue; solo→Join game/Spectate) + Kill/Pause (in-game) + Connect dummy (iff `AllowDummy`) + Disconnect; actions via existing `Act*`/chat/Disconnect (V12). ⊥ shown when not connected (idle = browser, V71); closed bar ⊥ steal movement/aim keys (C34). (C35)
- V75: game mode derived from `TickState.GameFlags` — team = `GameFlags & GAMEFLAG_TEAMS` (1<<0); menu Join buttons adapt; ⊥ guess. (C35)
- V76: a dummy = an INDEPENDENT `client.Client`+Observer+Controller, fully Connected (V22/V25); up to the server's per-IP limit (server-enforced); "Connect dummy" gated on `Capabilities().AllowDummy`; a dummy connect failure ⊥ affect other sessions. (C36)
- V77: exactly ONE active session — render reads the active session's `State`, input drives ONLY its `Controller`; selecting a client in the Esc menu switches the active session (follow + render from its perspective, camera reset T43); Disconnect/Stop closes ALL sessions, ⊥ leak. (C36)
- V73: tee input mimics DDNet held-input under terminal limits (C34) — per-tick `ActInput` emits movement direction & jump from a DECAYING held-state: a press sets the state + a hold window refreshed by key-repeat; window lapse (release) → neutral (dir 0, jump 0). a single press ⊥ latch movement/jump beyond the window (B10). fire = edge counter; hook = explicit toggle; aim = sticky cardinal target. hold window > key-repeat interval, configurable. all via `client.Do(ActInput)` (V12). (C34) [DIRECTION part SUPERSEDED by V80]
- V78: aim target NEVER the zero vector — default/last aim is a STABLE nonzero direction (e.g. facing right); `SetTarget(0,0)` makes the engine's aim degenerate/jittery (B11). aim changes only on an aim key; ⊥ spring with no aim input. (C34)
- V79: on disconnect the session's render `State` is CLEARED (st zeroed, have=false) so the stale map/tees vanish at once (B12) — game window shows the connecting/disconnected placeholder, ⊥ keep rendering the dead session's map. cleared also at the start of a (re)Join. (C33)
- V80: [SUPERSEDED by V81 — sticky direction got STUCK: terminal has no key-release, B15] movement DIRECTION is STICKY (press left/right → move until `stop`/opposite), JUMP momentary. (C34)
- V86: when `connected && st.Map==nil` (twclient connected "without map", B18) the game window shows a CLEAR actionable notice ("map not loaded — press R to re-download"), ⊥ the indefinite `connecting…`/download spinner; the spinner shows ONLY while joining (handshake/download in flight). reconnect (R) re-attempts the download. (C39)
- V87: connect version OMITTED → `packet.VersionAuto` → twclient probes the server (prefers 0.6) and connects; explicit `0.6`/`0.7` pins (skips detection). the RESOLVED version (`Client.Version()`) is recorded on the session after a successful connect, so status/reconnect/dummy reuse it. (C23; ← twclient v0.2.5 auto-detect; supersedes the hard-0.6 default)
- V88: while the map downloads (in-`Connect`, joining) teetui renders a PROGRESS bar — "downloading map NN%" from twclient `WithMapDownloadProgress(received,total)` — instead of the indeterminate spinner; total==0 (no progress yet / cache) → spinner. progress reset per (re)connect; drawn via the join redraw (V72). connected-without-map (`HasMap()` false) → the retry notice (V86). (C39; ← twclient v0.2.7 #9)
- V85: the scoreboard renders from the LIVE client registry (`cur().cli.Roster()`) — the same source as chat/completion/serverlog that demonstrably has players — ⊥ the per-tick `TickState.Roster` (which can stay empty). local id from the tick. (fixes B17; refines V14)
- V84: a user-initiated disconnect (Esc-menu Disconnect) tears down the active session SYNCHRONOUSLY — cancel frontend, close client, `state.Clear()`, open browser — ⊥ relying on the async `OnDisconnect` callback (which may not fire once the frontend ctx is cancelled, B16). a deliberate close ⊥ auto-reconnect (per-session `userClosing` guard). (refines V79; fixes B16)
- V83: release artifacts — `v*` tag → `goreleaser release` attaches static archives + `checksums.txt` to the GitHub Release (permanent, V15 credits in README ship too); EVERY push/PR → `goreleaser build --snapshot` uploads the cross-OS binaries as workflow artifacts. all builds `CGO_ENABLED=0` (⊥ cgo, C1); linux/windows fully static, darwin cgo-free. (C38)
- V82: a server broadcast renders as a top-centered overlay for ~10s (DDNet timing) since arrival, dims over the final ~1s, then disappears; re-evaluated each redraw by wall-clock (V72 tick-wake); a new broadcast replaces+resets, empty clears; clamps to width (V30); ⊥ logged as a `>>` line anymore (overlay only). (C37)
- V81: movement DIRECTION follows the ACTUAL key via DECAY — a press sets dir + a hold window that terminal key-repeat refreshes while the key is physically down; when the key is RELEASED (repeats stop) dir → 0 within the window (⊥ stuck, B15). the window (`cl_input_hold_ms`, default sized ≥ a jump-tap gap) bridges a momentary jump pressed mid-hold, so hold-direction + tap-jump stays COMBINABLE (B13). jump momentary (B10); aim sticky-nonzero (V78). (C34; restores V73 direction-decay, supersedes V80)
- V72: every observed game tick requests a redraw (tick hook / `State.Observe` → `wake()`), so the live view animates; actual repaints throttled to `cl_max_fps` (V42, coalesced trailing draw, no per-frame alloc V7). idle/disconnected (no ticks) → ⊥ redraw (V71). (C33)
- V71: IDLE ≠ connecting — at startup w/ no connect attempted (⊥ client, ⊥ `joining`, ⊥ `reconnecting`) the UI reads "not connected" + a browser hint, NEVER "connecting"/spinner/"downloading map". "connecting" shows ONLY while a handshake or auto-reconnect is in flight; "connected" once joined. (C23 no auto-connect; ← B9)
- V92: Esc-menu disconnect is SCOPED (C42). active session DUMMY → menu shows "Disconnect dummy" (close active dummy only: cancel its frontend, close its client, dropSession, setActive→primary, main + others STAY connected, ⊥ browser, ⊥ reconnect) BEFORE the "Disconnect" item. "Disconnect" ALWAYS ends the whole connection: close PRIMARY + EVERY dummy synchronously (⊥ leak), clear state, open browser, ⊥ reconnect (per-session userClosing). a PRIMARY teardown (deliberate or drop) ⊥ leave any dummy connected (dummy ⊥ outlive primary). (B20; refines V77/V84; ← C42)
- V91: serverlog player-LEAVE line emitted ONLY when the leaver's REAL name is known (roster name | local own name); name resolving to the bare `#<id>` id-fallback (never-named/phantom slot, empty 0.6 name) → SUPPRESS the line (⊥ "'#<id>' has left the game" noise). join uses the event name (has it); leave carries no name → roster-only → suppress on unknown. ⊥ suppress a leave whose real name IS known. (B19; refines V67/V26)
- V90: ∀ second while CONNECTED & visual ON & `cl_viewport_min_fps`>0 → the visual viewport (game scene rect) gets ≥ `cl_viewport_min_fps` COMPLETE redraws (ALL scene cells flushed, ⊥ skipped by tcell cell-diff — `scr.Sync`), EVEN with NO new ticks/events; heartbeat SUPPRESSED when tick/event-driven draws already met the rate (⊥ double-draw, ⊥ exceed `cl_max_fps`/V42). disconnected | visual OFF | rate==0 → ⊥ forced redraw (V71 idle preserved). heartbeat ⊥ per-frame steady alloc (V7), ⊥ block input/tick goroutines (V4). rate clamped ≤ `cl_max_fps` when cap>0. live-settable cvar. (C41; extends V42/V72/V33-render; respects V71/V7/V4)
- V89: DEFINITION OF DONE — a change ⊥ marked `x` in §T & a `v*` tag ⊥ pushed (V83) until ALL gates pass clean: (a) deps LATEST — `go get -u ./... && go mod tidy`, build+tests green on the bumped versions (a breaking bump → pin + note, ⊥ skip); (b) `gofmt -l .` → empty (no format drift); (c) `go generate ./...` → no working-tree diff (generated code current); (d) `go mod tidy` → `go.mod`/`go.sum` UNCHANGED (no stray/missing deps); (e) `go vet ./...` clean; (f) `govulncheck ./...` → no findings. CI enforces each as its OWN step (non-zero → red); same gates run locally before a release tag. (C40)
- V59: log lines WRAP to the log-band width — a line wider than width continues on the next visual row(s): word-wrap on spaces, hard-split any single token longer than width; ⊥ silent truncation of log text (V6 keeps wide glyphs intact). `Log.View(width,height)` returns VISUAL (wrapped) rows; scroll offset counts visual rows; recompute on resize (C17/V50). (C26)

## §T — tasks

id|status|task|cites
T1|x|scaffold module `github.com/jxsl13/teetui`, go.mod require twclient, main.go cli flags|C1,I.cli
T2|x|tcell init: screen, alt-buffer, color caps detect, `-no-color` path, resize handler, clean teardown|C3,C4,V5,V11
T3|x|window/layout mgr: game/log/info/input + border draw + relayout on resize (← terminalui.h CWindowInfo, DrawAllBorders)|I.windows,V11
T4|x|twclient connect: build `client.New` from flags, ctx, Connect/Close, status→info window|C2,I.cli,V1
T5|x|register `Observer` (TickModeFrame) → store latest TickState thread-safe (← Window.UpdateState)|C5,V2,V3,V4
T6|x|color→tcell.Style map fn (RGB truecolor + fallback) for tiles+entities|C4,V5,I.render
T7|x|map render: MapView tiles → glyph frame, camera-centered, 16-tile dist, 64×32 (← maplayers RenderTilemap)|I.render,V6,V7
T8|x|entity render: tees (self/other, ninja ø), hook line, weapon/aim, projectiles, lasers (← window.go drawScene); pickups/flags TODO|I.render,V6
T9|x|log window: chat/server-msg/broadcast scrollback via OnChat/OnBroadcast/OnServerMsg callbacks|I.windows,C8,V4
T10|x|info/status bar: input mode, server, RaceTime (MM:SS.mmm), tick/fps, conn state|I.windows
T11|x|input mode machine: NORMAL/CHAT/CHAT_TEAM/LOCAL_CONSOLE(F1)/RCON(F2) + enterMode dispatch (BROWSER mode TODO w/ T32; key-lock TODO)|I.modes,V9
T12|x|input textbox: cursor, edit, submit; CHAT→Do(ActChat{Team}) + readline + history + per-mode submit (chat/console/rcon)|I.windows,V12
T13|x|per-mode input history + persist load/save to disk (~/.config/teetui/history)|I.config,V10,V16
T14|x|reverse-i-search overlay (Ctrl-R) per mode (← RenderInputSearch/_UpdateInputSearch)|I.modes
T15|x|tab completion: player names (chat, from Roster) + console commands, cycling on repeat Tab (← CompleteNames/CompleteCommands). grey preview TODO|I.windows
T16|x|tee control: NORMAL-mode keys → packet.PlayerInput → Do(ActInput) via Controller (have: move/jump/hook; TODO: aim/fire/weapon, key-release handling)|V12,I.twclient
T17|x|scoreboard: cols score\|name\|clan, sort score-desc, local highlight, toggle — via twclient v0.2.0 `TickState.Roster` (PlayerState). 0.6 self-name weak (twclient T6)|I.windows,V6
T18|x|server browser: async master.FetchServerList, list render, `/` search, ↑↓ select, Enter→rejoin (close+dialer+Connect), password 🔒 flag (← menus.cpp OpenServerList)|I.windows,V13
T19|x|popups: greeting/message/DISCONNECTED + drawPopup, Enter/Esc close, mutex-guarded (← terminalui.h m_Popup). WARNING kind TODO|I.windows,V11,V17
T20|x|rcon: RCON mode (F2) → RconLogin (masked pw, off-loop) + Rcon send + OnRconLine→log (← remotecontrol.cpp)|I.modes
T21|x|warlist store (simple): war/peace/team/del + scoreboard name coloring + persist ~/.config/teetui/warlist.txt (← warlist.cpp). in-game nameplate N/A (no names in tile view)|V14
T22|x|chat commands `!war/!peace/!team/!del/!help` parse + apply via parseChatCommand, `cl_silent_chat_commands` default on (← chatcommand.cpp)|V14
T23|x|auto-reply: `H` reply-to-last-ping + known-phrase table; ping detect via name in OnChat (← chathelper/replytoping.cpp)|I.twclient
T24|x|warlist advanced mode: folders, multi-name bundle, war reasons, clan war (← warlist_commands_advanced.cpp)|V14
T25|x|disconnect/kick handling: OnDisconnect→DISCONNECTED popup + wake + auto-reconnect (attempt counter, "reconnecting #N" status, suppressed on quit) DONE|V11
T26|x|bench render hot path; prove zero steady alloc; optimize proven hot cells|C7,V7
T27|x|cross-OS smoke: build+run Linux/Windows(pwsh)/macOS terminals, color + glyph check|C3,C4,V5,V6
T28|x|help page content + key cheatsheet, `?`/Esc toggle, always escapable (← RenderHelpPage)|I.windows,V17
T29|x|write README.md: usage + full attributions/credits/references (chillerbot-ux/ChillerDragon, DDNet, Teeworlds, twclient, tcell, runewidth) + licenses|V15,I.cli
T30|x|log scrollback: PageUp/PageDown + mouse-wheel scroll, follow-tail (← transcript scroll log)|I.windows
T31|x|startup greeting popup w/ keybind hints, Enter close (← transcript boot menu)|I.windows,V11,V17
T32|x|browser tabs Internet/LAN/Favorites/DDNet/KoG/Vanilla + ←/→ switch + `/` search + Enter join + `f` favorite|I.windows,V13
T33|x|map download progress bar on join (← transcript download bar)|I.windows
T34|x|in-game HUD: live local-tee coords (tile x,y) readout (← transcript coords change on move)|I.render
T35|x|visual-mode toggle key `v`: show/hide game render, resize-safe via Sync (← transcript visual mode)|I.modes,V11,V18
T36|x|action keys: self-kill `k`→ActKill, emote `e`→ActEmoticon, vote F5/F6→ActVote (← transcript K self-kill)|V12,I.twclient
T37|x|spectate/pause: console `spec/spectate/pause [name]` → name→id via Roster → ActSetSpectator (free-view when no name) (← transcript pause follow)|V12,I.twclient
T38|x|input readline edit: Ctrl-U/Ctrl-K/Ctrl-W kill + cursor move (Left/Right/Home/End) (← transcript Ctrl-U/K); tab name-complete TODO|I.windows
T39|x|local console F1: command interpreter (help/echo/say/spec/quit/version) + history + config cvars (get/set) + tab-complete + per-command help-text line DONE (← transcript F1)|I.modes,V9
T40|x|chillerbot AFK: `H` reply-to-ping DONE (T23); auto tapped-out message + `cl_tapped_out_message`/`_text` cvars + rate-limit DONE (off by default — teetui is interactive, not AFK)|I.config
T41|x|reconcile keymap to §I key-binding table (?/B/F1/F2/T/Z/H/V/K/Tab//) — supersedes foundation `t`/`y`/`h`/`q`|I.modes,V17
T42|x|rebindable keymap: config file load/save, default = §I table, runtime bind (exceed reference)|V19,C12
T43|x|render-quality: Start/Finish/Checkpoint colored via MapView booleans DONE (Tele/Boost via class); sub-cell → T46; smooth camera (eased cameraSmoother, §T43) DONE|C11,V20,I.render
T44|x|parity-checklist verify: each §T30-41 feature ≥ chillerbot; doc gaps|C10,V20
T45|x|browser LAN + Favorites: favorites persist ~/.config/teetui/favorites.txt + `f` toggle + Favorites tab; LAN = connless probe of localhost ports (subnet broadcast would need twclient support)|I.windows,V13
T46|x|render sub-cell detail: half-block ▀▄ (2 tiles/cell vertical) | braille mode for finer map; toggle/auto (completes T43 sub-cell)|C11,V20,I.render
T47|x|render checkpoint tile color (orange, glyph 'C') via twclient v0.2.2 `MapView.Checkpoint`; precedence finish>start>checkpoint (← chillerbot colorCheckpoint)|C11,I.render
T48|x|e2e harness `e2e/` mirroring twclient: docker-compose (ddnet 0.6+0.7-sixup, teeworlds7 vanilla 0.7, Dockerfiles from source), gated `-tags e2e`+`TW_E2E`, service-name addrs; test connects each + RunFrontends + asserts snapshot ticks + roster. + full-UI screen-validation matrix (TestE2EUI: real App on tcell SimulationScreen via App.Join+DefaultDialer, drives greeting/status/HUD/scoreboard/help/visual/chat-echo/console/cvar/browser per server, asserts rendered cells; live-verified 21 checks ×3 servers, race-clean)|C14,V22,V23,V29
T49|x|CI/CD e2e job: build server images (matrix), run `go test -tags e2e ./e2e/...` IN-NETWORK + race + coverage profile + per-pkg %; mirror twclient `.github/workflows/ci.yml`|C14,V23
T50|x|connect UX: actionable timeout msg (addr/version/network) in log + reconnect/retry key; ?auto-detect protocol via connless `QueryServerInfo` probe before Connect|V24,I.windows
T51|x|browser LAN tab → REAL subnet scan via twclient v0.2.3 `master.ScanLAN` (broadcast 0.6+0.7, dedupe), replacing localhost-port probe (upgrades T45). map `[]LANServer`→serverRow into LAN source|I.windows,V13
T52|x|FIX B4: `App.Join` → `Connect(fctx)` (long-lived session ctx); drop `defer cancel` of session ctx; bound handshake via watchdog goroutine that cancels fctx ONLY if still `!connected` after ~12s. + EXTEND e2e (T48): assert SUSTAINED liveness — snapshots keep advancing >15s (past sv_timeout), ⊥ just initial tick|V25,V22,V24
T53|x|FIX B6 spectator render: DrawGame/DrawGameHalf center on spectated target | free-view | any visible tee when no `Players[LocalID]`; render map+tees as spectator (⊥ "connecting…")|V27,I.render
T54|x|FIX B7 connect msg: raise connectTimeout (real-server map download) + make configurable; surface connect-fail in log ONLY on terminal failure (⊥ if a reconnect then succeeds)|V28,V25
T55|x|FIX B8 own-chat: locally echo sent chat (all+team) into log immediately on send; dedupe the server echo (by msg+recent time)|V29,I.windows
T56|x|B5 mitigation: scoreboard/chat id fallback when roster name empty (verify) + file twclient feature for 0.6 ClientInfo→registry decode (SPEC-player-registry T6)|V26
T57|x|responsive layout: `Compute` scales game view w/ terminal (relax `maxGameW` so large terminals use more width, keep proportional split + min log width + min game width); overlays (scoreboard/help/popup/browser) clamp+reflow to current size, ⊥ overflow|C17,V30,I.windows
T58|x|render fills Game rect at any size: camera frame = rect (drop 64×32 assumption), DrawGame/DrawGameHalf scale up/down, tee stays centered, HUD/coords in-bounds; test tiny+huge rects|C17,V31,I.render
T59|x|min-size guard + live resize: below Wmin×Hmin show single "resize to ≥WxH" notice (⊥ garble/panic), restore on grow; EventResize → recompute+immediate redraw (not just Sync); test sub-min + round-trip|C17,V32,V30,V11
T60|x|lang classifier (port chillerbot `langparser`): FindWord (word-boundary, fold-normalized §V64), Has* presence matchers — HasGreeting(en/qq/rus)/HasFarewell/HasInsult/HasAskToAsk(+de)/HasWhy·HasHow·HasWhatWhich·HasWhoWhatWhich (renamed from Is*: they test token PRESENCE, ⊥ whole-msg identity); pure pkg, table-tested multi-lang|C18,V33,I.twclient
T61|x|reply-to-ping engine: replace simple `autoReplies` table — use T60 classifier + multi-lang smalltalk (how-are-you/ca-va/wie-gehts/wbu) + no-context ping→"name ?"; H + auto(cl_auto_reply) reply; rate-limited|C18,V33
T62|x|chat-query answers from state: war-status ("is X war?"/"why do you kill me"→warlist relation+reason), list wars/clan wars, how-to-join-clan, where(map+tile coords), what-os; answer via chat reply|C18,V34,V14
T63|x|last-ping queue (16, newest-first, ← chathelper m_aLastPings): H replies newest + cycles older; optional last-ping line in status/HUD (cl_show_last_ping)|C18,V35
T64|x|incoming chat filters: `cl_chat_spam_filter` 0/1/2 + insult filter + user filter list (console addfilter/listfilter/delfilter); hide matching pings from log; mode2=hide+autoreply; off default|C18,V36
T65|x|spam-safe outgoing send buffer: rate-limited chat queue (≤8, min interval, ← chathelper SayBuffer) so teetui ⊥ flood/get muted; FIFO; replaces immediate multi-line sends|C18,V37
T66|x|warlist auto-reload (`cl_war_list_auto_reload` secs): reload warlist/ files on interval (mtime) so external edits apply live; 0=off|C18,V14,I.config
T67|x|extended warlist chat commands (← chatcommands.h): `!search <name>`, `!create <war\|team\|neutral\|traitor> [folder] <name>`, `!addreason`, `!unfriend`, folder arg parity; extends T22/T24 parseChatCommand|C18,V14
T68|x|chillpw auto-login (`cl_chillpw`/`cl_password_file`): opt-in local secrets file → on connect match server addr, auto-send rcon/login pw to THAT server only; secret never logged; README security note|C18,V38,I.config
T69|x|extension API pkg `extension`: `Hook` interface (OnConnect/OnDisconnect/OnChat→suppress/OnBroadcast/OnServerMsg/OnKill/OnTick/OnKey→handled) + `NopHook` embed + `HookCtx` safe action surface (SendChat/Do/Log/Roster/Config/Server) + `Register`; panic-recover wrapper (V40); table-tested|C19,V39,V40,I.extension
T70|x|wire hook dispatch into App event paths: chat/broadcast/servermsg/kill/tick/connect/disconnect/key call registered hooks in order; honor OnChat suppress (hide line) + OnKey handled (consume); ⊥ break core when no hooks|C19,V39,V41
T71|x|external command hooks (opt-in): run `~/.config/teetui/hooks/<event>` executables w/ event JSON on stdin, parse stdout action lines (say/do), timeout-bounded, errors isolated (V40); off unless dir present|C19,V40,V41,I.config
T72|x|docs: README "Extensibility / Hooks" — list §C18 out-of-scope features + HOW to build each via hooks (example Go hook + example external script), security note (user responsibility, no DoS primitive); credit chillerbot features as the inspiration|C19,V41,I.cli
T73|x|render throttle: coalescing FPS cap — `frameLimiter` (pure: lastDraw+interval → drawNow|wait) + integrate in Run/draw so event/wake bursts repaint ≤ cl_max_fps, trailing-edge draw guarantees latest state; cap 0 = unlimited (today's behavior); ⊥ per-frame alloc|C20,V42,V7
T74|x|`cl_max_fps` config surface: `-max-fps` CLI flag + `cl_max_fps` cvar (console get/set), default 60, 0=unlimited; wire into frameLimiter (runtime cvar change applies live)|C20,V42,I.cli,I.config
T75|x|public `feature` SDK pkg: Feature/NopFeature/Hooks interfaces + Host interface (actions/state/DefineConfig/OnSendChat/DefineAction/AddStatusField/AddNameStyle/Provide/Lookup) + Register/Registered; absorb extension event types (ChatEvent/KillEvent/Key/Style); panic-isolated dispatch (V47); table-tested|C21,V43,V47,I.feature
T76|x|core Host impl + module registry in `internal/tui`: at startup Provision all `feature.Registered()` (dup name/cvar/action detection V46), dispatch every event to features (suppress/handled compose), run OnSendChat chain on outgoing chat, expose DefineConfig→cvar store, DefineAction→keymap, AddStatusField→status bar, AddNameStyle→scoreboard, Provide/Lookup service registry; base client has ZERO feature logic|C21,V44,V46,V47
T77|x|shared `lang` library pkg: move langparser (findWord/isGreeting/…/question classifiers) out of core into an importable lib for features (⊥ a feature itself). shared by ≥2 features → `internal/lang` (⊥ public root pkg, §C21)|C21,V43
T78|x|feature `features/warlist`: warlist store + `!war/!peace/!team/!del/!reason/!search/!create/!addreason/!unfriend` (+clan) via OnSendChat + scoreboard/nameplate coloring via AddNameStyle + auto-reload + own cvars (cl_silent_chat_commands, cl_war_list_auto_reload); Provides "warlist" service|C21,V44,V14
T79|x|feature `features/replytoping`: H DefineAction → composeReply (lang lib smalltalk/greeting/no-context) over a last-ping queue; reads PlayerName via Host|C21,V44,V33
T80|x|feature `features/chatquery`: war-status/where/os/list answers; Lookup("warlist") for relations+reasons; uses Roster/Tick/PlayerClan from Host|C21,V44,V34
T81|x|feature `features/chatfilter`: incoming spam/insult/user filters via OnChat suppress; own cvars (cl_chat_spam_filter[_insults]) + console addfilter/listfilter/delfilter via Host.DefineCommand|C21,V44,V36
T82|x|feature `features/responders`: tapped-out (cl_tapped_out_message[_text]) + auto-reply (cl_auto_reply[_msg]) on ping; own cvars; rate-limited; reads PlayerName|C21,V44,V33
T83|x|feature `features/lastping`: 16-deep ping queue + AddStatusField (cl_show_last_ping); Provides "pings" for replytoping (or replytoping owns queue + Provides)|C21,V44,V35
T84|x|feature `features/chillpw`: opt-in rcon auto-login from secrets file on OnConnect; own cvars (cl_chillpw, cl_password_file); secret never logged|C21,V44,V38
T85|x|feature `features/cmdhook`: external command hooks (~/.config/teetui/hooks/<event>) re-expressed as a feature on the new Host API (replaces T71 core wiring)|C21,V44,V40
T86|x|`main.go` single-file: blank-import all feature packages + `tui.Main()`; STRIP feature logic from core/main; + import-isolation guard test (V43: ⊥ core↔features import) + parity check (V44: migrated feature tests pass in their pkgs)|C21,V43,V45,V44
T87|x|layout redesign → vertical stack: rewrite `Compute` (status top / game / log band / input bottom, full-width, ⊥ left/right); logBandHeight fn (visual on → clamp(cfg.LogLines,1,⌊h/2⌋); off → full body); rewire `draw()` (game above band, logs above legend); update layout tests (responsive + cap + resize)|C22,V48,V49,V50,I.windows
T88|x|`cl_log_lines` config (default 10) + `-log-lines` flag: log-band rows when visual on, clamped ⌊h/2⌋ at render; runtime cvar change applies live|C22,V49,I.cli,I.config
T89|x|config-file exec: teeworlds-style `.cfg` parser (one `command [args]` per line, `#` comments, quoted strings) → run each via the console/cvar layer at startup; add `player_name`/`player_clan` cvars + `connect <addr> [0.6|0.7]` console cmd; identity from cvars|C23,V51,I.cli,I.config
T90|x|reduce CLI to `-config <file>` only: delete all other flags from `main.go`; load+exec the cfg if given else defaults; ⊥ auto-connect when no `connect` cmd → open browser|C23,V51,I.cli
T91|x|connect uses per-entry protocol version: browser/LAN join passes master/scan `Version` (verify, already wired); `connect` cmd arg or default 0.6 otherwise; ⊥ global version flag/cvar|C23,V51,V8
T92|x|feature `features/team` (NEW, exceeds chillerbot — GUI team-select has no terminal equiv): Host.DefineCommand `team <spectators|red|blue|game>` + `join` (+ ?DefineAction key) → Host.Do(ActSetTeam{spectators=-1\|red/game=0\|blue=1}); non-team game → join=team 0; distinct from spectate (§T37). needs Host.DefineCommand (extends §I.feature/T76 host, V47)|C21,V52,V12,I.feature
T93|x|de-leak SDK: remove `feature.Warlist` + `feature.PingStore` from the public `feature` pkg. providers `Provide` their concrete store; consumers (`features/replytoping`, `features/chatquery` if separate) declare a MINIMAL local interface + type-assert the `Lookup(any)`. SDK stays feature-agnostic; update fakes/tests + README/§I.feature|C21,V53,V43,I.feature
T94|x|free-look map-pan sub-mode (§C24): add `actFreeLook` (default `G`, rebindable V19) + keymap/name/order entries; App.freeLook bool + pan offset(panX,panY tiles); toggle ensures visual on, Esc|toggle recenters+exits. while active: arrows|WASD adjust pan (⊥ SetAim/SetDirection/any tee input — mode-gate before the parametric arrow/weapon switch, V9/V12); drawScene renders around (cameraCenter+pan) NOT tee-lock; clamp pan; HUD shows panned tile + "[free-look]". reset freeLook+pan on disconnect (a.camera.reset path). test: pan step math, input-gating (no Action sent while free-look), recenter, visual-forced-on|C24,V54,V27,V9,V12,I.render,I.keybinds
T95|x|dynamic context legend (§C25): replace hardcoded drawInput legend (`internal/tui/app.go:1371`) w/ generated legend — core builds `[]legendItem{key,label,priority,available(ctx)}` from the live Keymap (tokensFor each action) + feature actions (host featAct* + help); render context-available items as `[key]label` priority-ordered, truncate to `r.W` dropping lowest priority (⊥ overflow). reflect rebinds (V19). fix mislabeled `[V]detail`. test: legend reflects a rebind, truncation ⊥ exceed width, context switch normal vs free-look vs browser shows different sets|C25,V55,V19,V30,I.windows
T96|x|generated help overlay (§C25): build `helpLines` from Keymap + feature DefineAction (key+help) instead of the hardcoded slice (`internal/tui/help.go`); FULL binding list grouped (core / movement / input / feature); keep escapable (V17) + screen-clamp (V30). add core enumeration of feature actions (key+help) for legend+help to share. test: help lists a rebound key + a registered feature action; still escapable from each mode|C25,V56,V17,V19,V30
T97|x|join-the-game key: `features/team` add DefineAction("join_game","j","join the game (team 0)") → Host.Do(client.ActSetTeam{Team:0}) + Host.Log outcome (reuse setTeam path); key-trigger for the existing console `join`. legend/help pick it up automatically (§T95/§T96). update README keybinds + team pkg doc. test: action fires ActSetTeam{0} via a fake Host (NopHost override)|V57,V52,V12,V46,I.keybinds
T98|x|log line wrap: new `wrapLine(s string, width int) []string` (word-wrap on spaces + hard-split a token wider than width, runewidth-aware V6); `Log.View(width,height int)` wraps every logical line to width, flattens to visual rows, windows the last `height` honoring offset (offset = VISUAL rows). update render loop `internal/tui/app.go:1304` to pass `lay.Log.W`; migrate `View(int)` callers/tests (`models_test.go`). ⊥ truncate log text. test: wrapLine (short/multi-word/overlong-token/wide-rune), View windows wrapped rows, scroll on visual rows|C26,V59,V6,V50,I.windows
T99|x|responsive overlay tables: scoreboard `scoreboardLine`/`DrawScoreboard` (`internal/tui/scoreboard.go`) name+clan cols flex from rect `r.W` (drop `clan` when too narrow, then shrink `name`; grow `name` when wide), ⊥ fixed `nameColW`/`clanColW`; browser header+rows (`internal/tui/browser.go` `%-30s %-10s %-14s …`) flex from `w` (shrink|drop gametype/ver/map first). col sum ≤ width (V30/V6). update `scoreboard_test.go`. test: scoreboard cols adapt (narrow drops clan, wide grows name); browser line width ≤ w at several sizes|C26,V58,V30,V6,I.windows
T100|x|split monolithic `feature.Events` → small OPTIONAL handler ifaces (`ConnectHandler` `DisconnectHandler` `ChatHandler` `BroadcastHandler` `ServerMsgHandler` `KillHandler` `TickHandler` `KeyHandler`); `Feature`=`Name()` + optional `Initializer{Init(API) error}` (rename method `Provision`→`Init`); `Fire*` + the init dispatch (rename `ProvisionAll`→`InitAll`) type-assert each registered feature (skip absent); REMOVE `NopFeature`; migrate all `features/*` to implement only their handlers. ⊥ behavior change. test: feature w/ only ChatHandler gets OnChat ⊥ OnTick; a new optional iface ⊥ break existing; suppress/handled composition preserved (V39)|C27,V60,V61,V44,V47,I.feature
T101|x|feature LIFECYCLE — add optional `Validator{Validate() error}` + `Closer{Close() error}` (← `io.Closer`, ⊥ `CleanerUpper`); core runs Validate after Init (disable+log on err), Close on shutdown (`App.Stop`) + on panic-disable, even after partial init; panic-isolated (V40). wire `features/cmdhook` Close (stop spawned procs/goroutines) + any file/handle owner. test: Close called on shutdown + after an Init panic; partial-init Close safe; Validate err disables feature|C27,V62,V40,V47,I.feature
T102|x|RENAME `Host`→`feature.API` + compose it from sub-interfaces (`ChatSender` `ActionDoer` `Logger` `StateReader` `ConfigStore` `ActionRegistry` `UIRegistry` `ServiceRegistry` `Paths`), `API` embeds them; rename `NopHost`→`NopAPI` + core `appHost`→`appAPI` (internal); apply V61 naming (drop `Events`, ⊥ webserver `Host`). update every `features/*` signature (`Init`/handlers take `feature.API`), `feature` pkg doc, README §I.feature, in-repo example. ⊥ behavior change (V44); import isolation intact (V43). test: `NopAPI` satisfies `API`; minimal sub-interface assertions compile (consume a `ChatSender` alone)|C27,V63,V61,V44,V43,I.feature
T103|x|fold-normalized lang matching (§C28): add `foldKey(s string) string` in `internal/lang` = `transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)` + `cases.Fold()` (per-call|pooled, ⊥ shared across goroutines V4). migrate FindWord/FindAnyWord/ContainsAny/ContainsName + classifiers to compare on fold keys (fold the message ONCE; pre-fold patterns), keep word-boundary check; use `strings.EqualFold` for whole-token eq. promote `golang.org/x/text` to a DIRECT require. SLIM word lists (drop `tschuss`/`tschau` ASCII dodges → real spellings). update pkg doc (no longer "dependency-free"). test: accent (`café`/`cafe`, `tschüss`), composed/decomposed `é`, case (`ß`, Cyrillic `ПРИВЕТ`), boundary (`helloween` ⊥), empty ⊥; existing classifier table stays green (V33)|C28,V64,V65,V33,V4
T104|x|ingame movement keys, SELECTABLE (CORE, §C29): add core cvar `cl_move_keys` (default `wasd`, accepts `wasd`|`arrows`); a movement/aim ROUTER in `handleNormal` reads it — the selected set (`W A S D` | `Up Left Down Right`) → jump/left/stop/right (InputController SetJump/SetDirection), the COMPLEMENT set → cardinal aim (SetAim); replaces the static a/d/s move bindings + the fixed arrow-aim block. `Space` still jumps. ⊥ break free-look gate (free-look intercepts WASD/arrows first, V54). update help + legend + README; document the cvar in the config table. test: `cl_move_keys=wasd` → WASD move + arrows aim; `=arrows` → arrows move + WASD aim; live switch; free-look still pans|C29,V66,V19,V12,V54,I.keybinds
T105|x|SDK extension for player/team events (§C30): add `feature.PlayerJoinEvent`/`PlayerLeaveEvent`/`TeamChangeEvent` structs + `PlayerJoinHandler`/`PlayerLeaveHandler`/`TeamChangeHandler` optional ifaces; `registry.go` FirePlayerJoin/FirePlayerLeave/FireTeamChange (type-assert, skip absent, panic-isolated V40); `dialer.go` register `client.On[packet.EventPlayerJoin]`/`[EventPlayerLeave]`/`[EventTeamSet]` → Fire*. ⊥ break existing features (V60). test: a feature implementing only PlayerJoinHandler gets it ⊥ others; dispatch panic-isolated|C30,V68,V60,V40,V1,I.feature
T106|x|feature `features/serverlog` (§C30): NEW pkg + one blank-import in `main.go`; implements KillHandler + PlayerJoinHandler + PlayerLeaveHandler + TeamChangeHandler → `API.Log` DDNet-style lines (join `"X entered the game"`?, leave `"X has left the game"`/`"… (reason)"`, spec `"X joined the spectators"`, team `"X joined team red|blue"`/`"X joined the game"`, kill `"X killed Y"`(+weapon?), death `"Y died"` when killer<0\|killer==victim) — exact strings ← ddnet `src/game/client` (`?` confirm); names via `API.Roster` + id-fallback (V26); cvar `cl_show_game_messages` (default 1); DEDUPE vs 0.6 server system chat (⊥ double lines). test: format table (join/leave/spec/team/kill/death) + name resolution + dedupe; LIVE e2e on ddnet-0.6/0.7-sixup/vanilla-0.7 — assert messages appear, no dupes (V23)|C30,V67,V44,V26,V23,I.feature
T107|x|remove mouse input (§C31): drop `scr.EnableMouse()` (`app.go` NewApp) + the `*tcell.EventMouse` case in `handle()` (wheel→log scroll); log scroll stays `PgUp`/`PgDn`. update §I.windows/§I.keybinds + README (drop "mouse-wheel"/"wheel"). test: feeding a `*tcell.EventMouse` to `handle` is a no-op (⊥ scroll, ⊥ panic); PgUp/PgDn still scroll|C31,V69,I.keybinds
T108|x|help explains MODES (§C32): add a "modes" section to `helpLines` (`help.go`) — per mode {chat,team chat,local console,rcon,browser,scoreboard,visual,free-look}: enter-key + one-line what-it-is + `Esc` to exit (e.g. "F1 local console — set options (cvars) & run commands: connect/say/help; Esc to leave"). keep the generated key list (V56); escapable (V17), clamped (V30). test: help text contains the console explanation, the rcon explanation, an `Esc`/exit hint, and each mode's enter key|C32,V70,V56,V17,V30
T109|x|render on tick (§C33): in the tick hook (`app.go` SetTickHook) call `a.wake()` after `feature.FireTick`, so each observed tick requests a throttled redraw (live view animates). throttle by `cl_max_fps` unchanged (V42); ⊥ extra alloc (V7); ticks only arrive while connected → idle ⊥ redraw (V71). test: `state.Observe(tick)` posts a redraw request (sim `HasPendingEvent` true after a tick, false when idle/no tick)|C33,V72,V42,V7,V71
T110|x|decay-based held input (§C34, fixes B10): rework `InputController` — hold `moveDir int`+`moveUntil time.Time`, `jumpUntil time.Time`, plus fire counter / hook bool / aim x,y / weapon. `PressLeft`/`PressRight` set moveDir+`moveUntil=now+hold`; `PressStop` clears; `PressJump` sets `jumpUntil=now+hold`. `OnTick` builds `packet.PlayerInput` fresh per tick applying decay (now>moveUntil→dir 0; now>jumpUntil→jump 0), keep Fire edge counter + Hook toggle + sticky aim. wire `handleMoveAim` + `doAction(actJump/actMoveLeft/Right/Stop)` to the Press* methods (drop SetDirection/SetJump latch). hold window const (~350ms, ?cvar `cl_input_hold_ms`). update controller doc. test: a press sets dir/jump; after window w/o refresh → neutral; refresh within window keeps it; jump pulses then clears (⊥ infinite jump); fire stays edge; aim sticky|C34,V73,V12,I.twclient
T111|x|ESC overlay action bar (CORE, §C35): `escMenu{open bool, focus int, items []menuItem{label,enabled,run}}`; `Esc` toggles ONLY when connected (else existing Esc behavior); `←`/`→`/`Tab`/`Shift-Tab` move focus, `Enter` runs focused item, `Esc` closes (keyboard-only, C31). draw a top-of-viewport bar (responsive, V30). items built per context: Join red/blue (team) | Join game/Spectate (solo) + Kill/Pause (in-game) + Disconnect (Connect-dummy/follow added in T114/T115). actions → `do(ActSetTeam{...})`/`do(ActSetSpectator{-1})`/`do(ActKill{})`/sendChat `/pause`/Disconnect. ⊥ trap Esc; closed bar ⊥ eat movement keys. test: opens only when connected; team vs solo item set; focus nav + Enter runs; Esc closes|C35,V74,V75,V12,V30,I.menu
T112|x|game-mode detection (§C35): `const GameflagTeams = 1<<0`; `teamMode(st client.TickState) bool = st.GameFlags&GameflagTeams != 0`; menu uses it for the Join button set. test: team/solo classification from GameFlags|C35,V75
T113|x|multi-session core for dummies (§C36): refactor the single (`cli`/`state`/`input`) into a `session{client *client.Client, state *State, input *InputController}`; `App.sessions []*session` + `active int` (main = sessions[0]); render + input + dialer wiring read `sessions[active]`; `Join` builds session[0]; `Stop`/Disconnect closes ALL sessions. ⊥ change single-session behavior (V44-style); each session Connect+RunFrontends (V22/V25). test: one session = today's behavior; adding a session leaves active untouched; Stop closes all|C36,V76,V77,V22,V25,V12
T114|x|follow/select session (§C36): Esc menu lists sessions (main + dummies) by name/#id; selecting sets `active` → render reads that session's State + input drives its Controller; `camera.reset` on switch (T43). test: switch active → render/input target the new session; camera reset|C36,V77,T43,I.menu
T115|x|connect-dummy action (§C36): Esc-menu "Connect dummy" enabled ONLY when `sessions[active].client.Capabilities().AllowDummy`; builds+Joins a new session to the same addr/version (dummy identity, e.g. name suffix); per-IP limit → server rejects extra (connect-fail logged, others unaffected, V76). test: button gated on AllowDummy; a failed dummy connect ⊥ disturb the main session|C36,V76,I.menu
T116|x|fix aim spring (§B11/§V78): `InputController` default aim = nonzero (e.g. `aimX=aimReach, aimY=0` facing right) set in `NewInputController`; `OnTick` ⊥ ever emit `SetTarget(0,0)` (guard: if aimX==0&&aimY==0 use the default). test: a fresh controller's emitted target ≠ (0,0); aim stays stable across ticks with no aim input|V78,V12,I.twclient
T117|x|reset render on disconnect (§B12/§V79): add `State.Clear()` (zero st, have=false); call it in `onDisconnect(s)` for the session and at the start of `joinSession` (fresh slate). drawScene then shows the placeholder, ⊥ the dead map. test: after Observe(tick-with-map) then Clear(), `Get()` reports have=false; an idle/disconnected frame ⊥ render map glyphs|V79,V71,I.windows
T118|x|sticky direction + momentary jump (§B13/§V80): rework `InputController` movement — `PressLeft/Right` set a STICKY `moveDir` (no decay), `PressStop` + opposite clears; JUMP keeps the momentary pulse (jumpUntil decay, ⊥ infinite jump B10). `OnTick` emits dir (sticky) + jump (pulse) independently → combinable. update T110 decay tests (direction now sticky; jump still pulses). test: dir + jump emitted together from separate presses; stop clears dir; jump auto-clears|V80,V73,V12,I.twclient
T119|x|expose WASD/arrow swap (§C29/§V66): the swap already exists as cvar `cl_move_keys` — make it discoverable: add an Esc-menu item "Swap move keys (wasd/arrows)" toggling it live; confirm handleMoveAim honors it post-refactor. update README/help. test: menu toggle flips cl_move_keys wasd↔arrows; movement/aim swap takes effect|V66,C29,I.menu
T120|x|un-stick movement: restore direction DECAY (§B15/§V81, supersedes T118 sticky): `InputController` direction = `moveDir`+`moveUntil`; `PressLeft/Right` set dir + `moveUntil=now+hold`; `OnTick` emits dir only while `now<moveUntil` (else 0); `PressStop` clears. keep momentary jump + nonzero sticky aim. bump default `cl_input_hold_ms` to ~500ms (bridge a jump tap → run+jump still combinable, B13). update tests: dir decays to 0 after window w/o refresh; refresh keeps it; dir+jump combine within window; stop clears|V81,V73,V12,I.twclient
T121|x|broadcast overlay (§C37/§V82): `App.broadcast{text string, until time.Time}`; `const broadcastDur = 10s`; dialer `OnBroadcast` → `a.setBroadcast(text)` (text + until=now+dur, wake) and REMOVE the `a.log.Addf(">> …")` line; `drawBroadcast(w,h)` draws the text centered horizontally near the top while `now<until`, using a DIM style in the final ~1s; hidden when empty/expired; called in `draw()` before Show. test: visible before `until`, hidden after; new broadcast replaces+resets timer; empty clears; dim in fade phase. update README/§I windows|C37,V82,V72,V30,I.windows
T122|x|CI snapshot-artifact build (§C38/§V83): add a job to `.github/workflows/ci.yml` (e.g. `artifacts`, on push/PR) — `goreleaser/goreleaser-action@v6` with `args: build --snapshot --clean` (CGO_ENABLED=0) then `actions/upload-artifact@v4` of `dist/**` (the linux/windows/darwin × amd64/arm64 binaries). leaves the existing tag→release flow (`release.yml`) untouched. verify: `goreleaser check` passes + cross-compile sanity (`GOOS=linux\|windows\|darwin GOARCH=amd64 go build ./...`); workflow YAML valid. (no source-code change)|C38,V83,I.cli
T123|x|fix stuck map on Esc-menu Disconnect (§B16/§V84): `disconnectUser` does the teardown ITSELF — set `s.userClosing` (new `session.userClosing atomic.Bool`), cancel+close, `s.connected/joining=false`, `s.state.Clear()`, `openBrowser()`, `wake()`; `onDisconnect` checks `s.userClosing.Swap(false)` → skip reconnect/popup (deliberate close). drop the global `userDisc` reliance. test: after `disconnectUser` the active session's state is cleared + mode==modeBrowser; a server-initiated drop (onDisconnect without userClosing) still reconnects|V84,V79,I.menu
T124|x|fix empty scoreboard (§B17/§V85): render from the LIVE registry — change `DrawScoreboard` to take `roster []client.PlayerState` + `localID int` (instead of `st`); caller passes `cur().cli.Roster()` + `st.LocalID`. `rosterRows` takes a slice. update scoreboard_test. test: scoreboard lists players from a populated registry; sort/highlight unchanged|V85,V14,I.windows
T125|x|connected-without-map notice (§B18/§V86/gh#1): in `scenePlaceholder`/the spinner gate, distinguish `connected && st.Map==nil` → show "map not loaded — press R to re-download" (⊥ the endless `connecting…` spinner, which stays ONLY while `joining`). reconnect (R) already re-downloads. test: connected+no-map → notice text (not the spinner); joining → spinner; connected+map → game|V86,V71,I.windows
T126|x|map-download progress (§C39/gh#1, C16): FILED jxsl13/twclient#8 (Connect silently succeeds without map on download failure → should error/retry/expose state) + #9 (expose map-download PROGRESS bytes/total via callback/event), linked in §B18. RENDERING a progress bar is BLOCKED on twclient#9 (download runs inside Connect, no progress signal) → deferred until upstream ships; teetui meanwhile shows the joining spinner + the connected-no-map retry notice (T125)|C39,V86,I.twclient
T127|x|adopt protocol auto-detect (§C23/§V87, twclient v0.2.6): `doConnect` default version → `packet.VersionAuto` (was `Version06`) when the arg is omitted; explicit `0.6`/`0.7` still pin. `joinSession` records `s.version = c.Version()` after a successful connect (resolved protocol for status/reconnect/dummy). `versionLabel(VersionAuto)` → "auto". console help: version optional (auto-detected). test: `doConnect` maps ""→Auto, "0.6"/"0.7"→pinned; versionLabel(auto)|C23,V87,V51,I.cli
T128|x|map-download progress bar (§C39/§V88, twclient v0.2.7 #9 — unblocks T126 render): session holds `mapRecv`/`mapTotal` (atomic.Int64), reset at `joinSession` start; dialer adds `client.WithMapDownloadProgress(func(recv,total){ s.mapRecv/Total.Store; a.wake() })`. render: during joining with `mapTotal>0` draw "↓ downloading map NN% (a/b)" instead of the spinner; total==0 → spinner. test: progress stored + formatted %; 0 → spinner path|C39,V88,V72,I.windows
T129|x|enforce DoD gates (§C40/§V89) in CI + locally: add CI steps (own steps/job) — `gofmt -l .` (fail if non-empty), `go mod tidy` + `git diff --exit-status go.mod go.sum`, `go generate ./...` + `git diff --exit-status`, `govulncheck ./...` (install `golang.org/x/vuln/cmd/govulncheck`), keep `go vet`. add a periodic/dispatch dep-bump check (`go get -u ./... && go mod tidy`, build+test) — scheduled workflow or documented pre-release step. run all gates NOW on HEAD, fix any drift (gofmt/tidy/vuln), bump deps to latest. test: CI red on injected fmt drift / untidy go.mod / known-vuln dep|C40,V89,V83,I.cli
T130|x|viewport min redraw rate (§C41/§V90): add `Config.ViewportMinFPS int` (default 1) + cvar `cl_viewport_min_fps` ("min complete viewport redraws/sec; 0=off", clamp [0,1000]); effective rate = min(ViewportMinFPS, MaxFPS) when MaxFPS>0. Run loop (`app.go`): add a heartbeat timer at `time.Second/rate`; on fire iff `connected && a.visual && now-lastDraw ≥ interval` → force COMPLETE viewport repaint (set a `forceSync` flag consumed by `draw()` → `scr.Sync()` instead of `scr.Show()`) then record draw + reset heartbeat; rate==0 | disconnected | visual off → heartbeat dormant (V71). reuse the existing `limiter.last`/`record` for last-draw time; ⊥ extra per-frame alloc (V7). test: heartbeat schedules a forced viewport redraw after 1/rate of silence when connected+visual; suppressed when a recent draw met the rate; dormant when rate==0 / disconnected / visual-off; rate clamped ≤ cl_max_fps|C41,V90,V42,V71,V7,I.cli
T131|.|suppress id-only leave line (§B19/§V91): in `features/serverlog/serverlog.go` split `nameOf` into a `resolveName(h,id) (string,bool)` returning ok=false when only the `#<id>` fallback applies (no roster name, not local-with-name); `nameOf` keeps current behavior via `resolveName` (fallback string when !ok) for join/team/kill. `OnPlayerLeave`: if `!ok` → return without logging; else log as today (with/without reason). test: leave for an unknown/never-named id → NO line; leave for a known roster name → "'bob' has left the game" (+reason) unchanged|V91,V67
T132|.|scoped disconnect (§B20/§C42/§V92): rename `disconnectUser`→`disconnectAll` — loop ALL sessions (primary+dummies): set `userClosing`, cancel frontend, close client, then `state.Clear()`, camera reset, exit free-look, `openBrowser()`, leave only the primary session in `a.sessions` (drop dummies). add `disconnectDummy()` — active session only (must be a dummy): set `userClosing`, cancel frontend, close client, `dropSession(s)`, `setActive(0)` (refollow primary), ⊥ browser, ⊥ reconnect. `buildEscMenuItems` (escmenu.go): if `!isPrimary(cur())` add `{"Disconnect dummy", a.disconnectDummy}` before `{"Disconnect", a.disconnectAll}`. update the B16 test refs (disconnectUser→disconnectAll). test: active=dummy → "Disconnect dummy" present, runs → dummy dropped + active==0 + primary still connected + no browser; "Disconnect" with a dummy → all sessions closed (len==1 primary, closed) + browser; active=primary → no "Disconnect dummy" item|C42,V92,V77,I.windows

id|date|cause|fix
B1|2026-06-15|`B` server-browser key dead: startup greeting popup intercepted ALL keys in handlePopup (only Enter/Esc/?/q closed), so `B` swallowed & openBrowser unreachable while popup shown — though popup advertises "B server browser"|V21
B2|2026-06-15|"connecting to servers does not work": teetui never called `Client.RunFrontends` → Observer(render)+Controller(input) NEVER dispatched. Connected & snaps ticked but UI stuck "connecting…" (observerTicks=0 vs snapTick advancing). fix: `go c.RunFrontends(fctx)` after each Connect, via unified `App.Join`|V22
B3|2026-06-15|connect "context deadline exceeded": NOT a teetui code bug — connect verified OK vs live teeworlds7:8303 0.7 (0.0s, in compose net). cause = (a) macOS Docker host UDP forward broken → host can't reach localhost:8303/8307 (C15); (b) `-version` mismatch → handshake never completes → 12s deadline. mitigate: run in compose net OR matching `-version`; automate via e2e (T48/T49) + UX (T50)|V23,V24
B4|2026-06-15|connect succeeds then session DIES (server sv_timeout disconnect): `App.Join` passed `context.WithTimeout(bg,12s)` to `Connect(ctx)` + `defer cancel()` in connect goroutine. twclient binds reader+keepalive+all I/O to the Connect ctx (= session lifetime) → cancel (fired right after Connect returns via defer, or @12s) tore down the LIVE session → no recv/keepalive → DDNet sv_timeout drops client. reproduced: snapshots stop exactly @ ctx deadline (delta 100/2s → 0 @ ~12s). fix: Connect(fctx) long-lived; handshake bounded by watchdog cancelling fctx only if !connected|V25,V22
B5|2026-06-15|players show id but NO name (scoreboard/chat/nameplate): on 0.6 the twclient player REGISTRY is empty — `Roster()`=0, `Player(id)` not found, even own player. probed live vs ddnet:8303 0.6: roster empty after 6s; own chat echo arrives w/ `name=""`. twclient ⊥ decode 0.6 Sv_ClientInfo/ObjClientInfo into registry (0.7-sixup roster=5 ✓). dep gap. teetui mitigate: id fallback; real fix twclient|V26
B6|2026-06-15|as SPECTATOR the visual/view mode renders nothing: `DrawGame`/`DrawGameHalf` do `self,ok:=st.Players[st.LocalID]; if !ok {return "connecting…"}`. spectator/free-view has no local character in Players → early-return → blank. fix: center on spectated target/free-view/any tee|V27
B7|2026-06-15|"context deadline exceeded" shown at connect though a later connect succeeds: `App.Join` handshake watchdog (connectTimeout 12s) aborts a still-progressing connect (real-server map download >12s) → Connect returns ctx err → connectFailMsg logged, yet retry connects. msg misleading + timeout too short. fix: raise/configurable timeout; show fail only on terminal failure|V28
B8|2026-06-15|own chat lines ⊥ visible: teetui ⊥ locally echo sent chat, relies on server echo. probe: docker server DOES echo own line but w/ empty name (0.6, B5) → "[0]"/looks missing; other servers ⊥ echo own chat → invisible. fix: local echo of sent chat immediately, dedupe server echo|V29
B11|2026-06-16|aim springs with no input: `InputController.OnTick` always `in.SetTarget(c.aimX, c.aimY)` with default 0,0 → a ZERO aim vector → engine aim direction degenerate, jitters/springs back-forth (esp. w/ prediction+intratick). fix: default aim nonzero (facing right); never emit (0,0)|V78
B12|2026-06-16|after disconnect the dead map is still rendered: the session's render `State` keeps the last `TickState` (map+tees); nothing clears it on disconnect, so `drawScene(cur().state.Get())` keeps drawing the stale map. fix: `State.Clear()` on disconnect (+ at Join start) → placeholder shows instead|V79
B13|2026-06-16|left/right ⊥ combinable with jump (no simultaneous keys): direction used key-repeat DECAY (V73); a terminal auto-repeats only ONE key, so holding a direction can't coexist with a jump press → run+jump impossible. fix: sticky direction (V80) + momentary jump → combinable via separate presses|V80
B14|2026-06-16|kill/death message shows "'#0' died" not the player name: `features/serverlog.nameOf` resolves via `Roster()` only; the LOCAL player's roster entry can be nameless (twclient gap, B5/V26) → id-fallback "#0" even though we know our own name. fix: nameOf falls back to `PlayerName()` for `id == Tick().LocalID` before the "#id" fallback|V26
B15|2026-06-16|the actually-pressed movement key gets STUCK: V80/T118 made direction STICKY, but a terminal sends NO key-release → a pressed left/right never clears, so the tee keeps moving (esp. visible after respawn). sticky was the wrong fix for B13. fix: restore DECAY (V81) — dir held only while key-repeat refreshes it, → 0 after release; window sized to bridge a jump tap so run+jump still combines|V81
B20|2026-06-16|controlling a dummy + "Disconnect" closes ONLY the dummy yet opens the server browser while the MAIN stays connected; no way to end the whole connection from a dummy; and a primary `disconnectUser` only closes `cur()` → dummies leak. root: `disconnectUser` acts on `a.cur()` (the active session) + unconditionally `openBrowser()`. fix: scope it (C42) — "Disconnect dummy" (active dummy only, refollow primary, ⊥ browser) vs "Disconnect" (close primary + all dummies → browser); primary teardown always closes all dummies|V92
B19|2026-06-16|"'#2' has left the game" noise: `serverlog.OnPlayerLeave` logs `nameOf(h,id)`, which falls back to the bare `#<id>` placeholder when the leaver's REAL name was never known (phantom/unknown slot, or empty 0.6 name) — so a never-named client leaving prints "'#2' has left the game". named players still resolve via the roster (intact at callback). fix: suppress the leave line when the name resolves only to the id-fallback (unknown player) — emit ONLY when a real name is known|V91
B16|2026-06-16|Esc-menu Disconnect leaves the map stuck on screen: `disconnectUser` cancels the frontend ctx FIRST, then `Close()`, and RELIES on the async `OnDisconnect` callback to `state.Clear()` + `openBrowser()` — but cancelling the frontend stops the dispatch loop, so the callback often never fires → state never cleared, browser never opened, map frozen. fix: `disconnectUser` tears down SYNCHRONOUSLY (clear state + open browser itself); a per-session `userClosing` flag suppresses reconnect if the callback does fire|V84
B18|2026-06-16|(gh #1) connected but NO map render → eternal "connecting…": twclient `Connect` includes the map download and SUCCEEDS even when it fails (`client/client.go:425` "continuing without map"), so teetui is `connected` with `st.Map==nil` → chat works but the game window shows the connecting spinner forever; reconnect re-downloads OK. fix (teetui): connected-no-map → actionable "press R to re-download" notice; filed jxsl13/twclient#8 (silent connect-without-map) + #9 (map-download progress API) per C16|V86
B17|2026-06-16|scoreboard EMPTY with players online: `DrawScoreboard` reads `st.Roster` (the per-tick `TickState.Roster`), which stays empty even though the live client registry has the players (chat names + serverlog joins work via `cli.Roster()`). fix: render the scoreboard from `cur().cli.Roster()` + tick `LocalID`|V85
B10|2026-06-16|input latches → tee jumps/moves with no interaction: `InputController` is STICKY — `SetDirection`/`SetJump` set the held `PlayerInput` and it NEVER clears (terminal has no key-release), so one `w`/`a`/`d` press makes the tee jump or move FOREVER (`OnTick` re-sends the same held input each tick). DDNet sends per-tick CURRENTLY-HELD keys (neutral on release). fix: decay model (C34) — press sets state + hold window refreshed by key-repeat; `OnTick` returns dir/jump to neutral once the window lapses|V73
B9|2026-06-16|idle app says "connecting": at startup w/ no config/no Join, the status bar `connLabel` defaulted to "connecting" and the game window drew the spinner + `drawScene` "connecting…" whenever `!connected` — so an idle client looked like it was connecting (to nothing). fix: `joining` flag; idle → "not connected" + browser hint; spinner/placeholder/"connecting" gated on joining\|reconnecting\|connected|V71
