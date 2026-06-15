# teetui parity checklist (SPEC T44)

teetui is an independent Go re-implementation of the chillerbot-ux ncurses
`terminalui`, built on the pure-Go [`twclient`](https://github.com/jxsl13/twclient)
library and the [`tcell`](https://github.com/gdamore/tcell) terminal toolkit. It
is not a fork of the C++ codebase. This document tracks feature parity against
the reference (chillerbot-ux's terminal UI), as required by SPEC parity floor
C10 / invariant V20. Each chillerbot terminal-UI feature is listed below with
its teetui standing and the SPEC §T task that owns it. Statuses are taken
verbatim from the §T task list (`x` = done, `~` = partial, `.` = todo) — a
feature is only marked **done** here when its owning task is `x` in SPEC.md.

## Feature parity table

| Feature | chillerbot-ux | teetui | SPEC task | Status |
|---|---|---|---|---|
| Help page (`?`, escapable from any mode) | yes | yes | T28 | done |
| Startup greeting / boot key-hint popup | yes | yes | T31 | done |
| Popups (greeting / message / DISCONNECTED; WARNING kind TODO) | yes | yes | T19 | done |
| Server browser: master list + search + select + join + password flag | yes | yes | T18 | done |
| Browser tabs (Internet / LAN / Favorites / DDNet / KoG / Vanilla) + `←`/`→` switch + `f` favorite | yes | yes | T32 | done |
| Browser Favorites (persisted) + LAN tab | yes | yes | T45 | done |
| LAN browser = real subnet broadcast scan (0.6 + 0.7, deduped) | yes | yes (`master.ScanLAN`) | T51 | done |
| Map download progress bar on join | yes | yes | T33 | done |
| Local console (`F1`) command interpreter | yes | partial | T39 | partial |
| Remote console / rcon (`F2`): masked login + send + output | yes | yes | T20 | done |
| Chat all (`T`) + team chat (`Z`) | yes | yes | T12 | done |
| Readline editing (cursor move, `Ctrl-U`/`Ctrl-K`/`Ctrl-W` kill) | yes | yes | T38 | done |
| Per-mode input history (bounded 16) | yes | yes | T13 | done |
| Input history persisted across restarts | yes | yes | T13 | done |
| Reverse-i-search (`Ctrl-R`) | yes | yes | T14 | done |
| Name + command tab-completion (cycling) | yes | partial (no grey preview) | T15 | partial |
| Scoreboard (`Tab`): score / name / clan, sorted, local highlight | yes | yes | T17 | done |
| Visual map render (tiles → colored glyphs), camera-centered | yes | yes | T7 | done |
| Entity render (self/other tees, ninja, hook, lasers, projectiles) | yes | yes | T8 | done |
| In-game HUD: live local-tee coordinates | yes | yes | T34 | done |
| Visual-mode toggle (`V`), resize-safe | yes | yes | T35 | done |
| Tile coloring Start/Finish/Checkpoint/Tele/Boost | yes (6-pair) | yes (truecolor) | T43 / T47 | done (T43 partial: smooth cam TODO) |
| Sub-cell render detail (half-block / braille) | no | partial (toggle in progress) | T46 | partial |
| Tee control: move / jump / hook / aim / fire / weapon | yes | yes | T16 | done |
| Self-kill (`K`), emote (`e`), vote (`F5`/`F6`) | yes | yes | T36 | done |
| Spectate / pause-follow | yes | yes (console `spec`/`pause [name]`) | T37 | done |
| Auto-reply to last ping (`H`) + known-phrase table | yes | yes | T23 | done |
| Warlist store: `!war`/`!peace`/`!team`/`!del` + scoreboard coloring + persist | yes | yes | T21 / T22 | done |
| Warlist advanced (folders, bundles, reasons, clan war) | yes | partial | T24 | partial |
| AFK auto "tapped-out" message + `cl_tapped_out_message` toggle | yes | partial (intentionally off) | T40 | partial |
| Log scrollback: `PageUp`/`PageDown` + mouse wheel | yes | yes | T30 | done |
| Disconnect / kick handling + popup | yes | partial (popup + reconnect key; status UI TODO) | T25 / T50 | partial |
| Connect UX: actionable timeout msg + reconnect/retry key | n/a | yes | T50 | done |
| Resize-safe relayout (windows / popups / browser / visual) | partial (reference "wonky") | yes | T2 / T3 | done |
| Rebindable keymap (config load/save, runtime bind) | no | yes | T42 | done |
| Keymap reconciled to §I binding table (`?`/`B`/`F1`/`F2`/`T`/`Z`/`H`/`V`/`K`) | n/a | yes | T41 | done |
| Live e2e harness (docker-compose ddnet 0.6/0.7-sixup + teeworlds7) | n/a | yes | T48 | done |
| CI/CD: e2e + race + per-pkg coverage | n/a | yes | T49 | done |

## Stability fixes (§B)

Reliability regressions found during live testing and their fixes (see SPEC §B):

- **B1 (V21):** the startup greeting popup intercepted every key in
  `handlePopup`, so `B` (open browser) and `?` (help) were swallowed while the
  popup was shown — even though the popup advertises them. Fixed so the popup no
  longer eats keys it advertises.
- **B2 (V22):** "connecting did not work" — teetui never called
  `Client.RunFrontends`, so the Observer (render) and Controller (input) loops
  were never dispatched; snapshots ticked but the UI stayed stuck "connecting…".
  Fixed by running `go c.RunFrontends(fctx)` after every successful Connect via
  the unified `App.Join`.
- **B4 (V25):** connect succeeded then the session died at the server's
  `sv_timeout` — `App.Join` passed a `WithTimeout(12s)` ctx to `Connect` with a
  `defer cancel()`, but twclient binds the reader + keepalive + all I/O to the
  Connect ctx (= session lifetime), so the cancel tore down the live session.
  Fixed under T52: `Connect(fctx)` long-lived; handshake bounded by a watchdog
  that cancels only while still `!connected`. The e2e harness now asserts
  *sustained* liveness (snapshots advance past `sv_timeout`, not just an initial
  tick).

## Gaps & differences

These are the points where teetui is partial relative to the reference or
intentionally diverges:

- **Scoreboard self-name (0.6):** under protocol 0.6 the local tee's name is
  weak / unreliable, a twclient limitation surfaced in the roster (T17).
- **Tab-completion:** name/command completion cycles on repeated `Tab`, but the
  grey inline preview shown by the reference is not yet implemented (T15).
- **Local console (`F1`):** the built-in interpreter (`help`/`echo`/`say`/
  `version`/`quit`) plus history work, but twclient config commands, tab-complete
  and the per-command help-text line are still TODO (T39).
- **Sub-cell render:** half-block / braille sub-cell detail is in progress (a
  finer-than-one-tile-per-cell mode the reference lacks entirely); the toggle is
  not yet finished (T46). Smooth-camera interpolation is also still TODO under
  the render-quality task (T43).
- **Warlist advanced:** the simple war/peace/team store is complete (T21/T22),
  but the advanced mode (folders, multi-name bundles, war reasons, clan war) is
  only partially implemented (T24).
- **AFK auto-message:** the `H` reply-to-ping path is done (T23); the automatic
  "tapped out" message (`cl_tapped_out_message`) is intentionally **off** —
  chillerbot is a headless bot that is always detected as tapped out, whereas
  teetui is an interactive client, so auto-AFK is left disabled by default (T40).
- **Disconnect handling:** the DISCONNECTED popup is raised and a
  reconnect/retry key plus actionable timeout messages are wired (T50), but the
  full auto-reconnect *status* UI is still pending (T25).

## teetui advantages over the reference

These are areas where teetui already meets or exceeds chillerbot-ux (per SPEC
goal §G, constraints C4/C11/C12 and the README feature set):

- **Truecolor rendering:** full 24-bit RGB via tcell with automatic downsample to
  256 / 16 colors, versus chillerbot's crude 6-pair `rgb_to_text_color_pair`
  curses palette (C4, C11, README).
- **Cross-platform:** runs on Linux, Windows and macOS terminals with no cgo and
  no OpenGL, whereas the reference is Linux / ncurses-only (§G, C3, README).
- **Resize robustness:** layout, popups, browser and visual mode relayout cleanly
  on terminal resize (V18), versus the reference's self-described "wonky / breaks
  on close" behavior (C12).
- **Both protocols transparently:** 0.6 and 0.7 connect, render and chat
  identically from the user's view, with no version branching above twclient
  (C6, V8).
- **Rebindable keymap:** the §I binding table is the default and keys are
  rebindable via config with runtime binding (T41/T42) — something the reference
  cannot do (C12, V19).
- **Real LAN discovery:** the LAN tab is a true subnet broadcast scan across 0.6
  and 0.7 via `master.ScanLAN` (T51), rather than the localhost-port probe it
  started as (T45).
- **Sub-cell rendering:** half-block / braille sub-cell map detail (in progress,
  T46) goes beyond the reference's one-tile-per-cell ASCII view entirely (C11).
- **Live-tested + CI:** a docker-compose e2e harness (ddnet 0.6 + 0.7-sixup,
  teeworlds7 vanilla 0.7, images built from source) asserts connect, sustained
  snapshot liveness and roster in-network, wired into CI with race detection and
  per-package coverage (C14, V22/V23, T48/T49) — the reference has no such
  harness.
