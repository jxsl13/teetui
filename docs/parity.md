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
| Popups (message / disconnected; WARNING kind TODO) | yes | yes | T19 | done |
| Server browser: master list + search + select + join + password flag | yes | yes | T18 | done |
| Browser tabs (Internet / DDNet / KoG / Vanilla) + `←`/`→` switch + `f` favorite | yes | yes | T32 | done |
| Browser LAN + Favorites (favorites persisted; LAN = localhost probe) | yes | yes (LAN narrowed) | T45 | done |
| Map download progress bar on join | yes | no | T33 | todo |
| Local console (`F1`) command interpreter | yes | partial | T39 | partial |
| Remote console / rcon (`F2`): masked login + send + output | yes | yes | T20 | done |
| Chat all (`t`/`T`) + team chat (`y`/`Z`) | yes | yes | T12 | done |
| Readline editing (cursor move, `Ctrl-U`/`Ctrl-K`/`Ctrl-W` kill) | yes | yes | T38 | done |
| Per-mode input history (bounded 16) | yes | yes | T13 | done |
| Input history persisted across restarts | yes | yes | T13 | done |
| Reverse-i-search (`Ctrl-R`) | yes | yes | T14 | done |
| Name + command tab-completion (cycling) | yes | partial (no grey preview) | T15 | partial |
| Scoreboard (`Tab`): score / name / clan, sorted, local highlight | yes | yes | T17 | done |
| Visual map render (tiles → colored glyphs), camera-centered | yes | yes | T7 | done |
| Entity render (self/other tees, ninja, hook, lasers, projectiles) | yes | yes | T8 | done |
| In-game HUD: live local-tee coordinates | yes | yes | T34 | done |
| Visual-mode toggle (`v`), resize-safe | yes | yes | T35 | done |
| Tile coloring Start/Finish/Checkpoint/Tele/Boost | yes (6-pair) | yes (truecolor) | T43 / T47 | done (T43 partial: smooth cam TODO) |
| Sub-cell render detail (half-block / braille) | no | no | T46 | todo |
| Tee control: move / jump / hook | yes | partial (aim/fire/weapon TODO) | T16 | partial |
| Self-kill (`k`), emote (`e`), vote (`F5`/`F6`) | yes | yes | T36 | done |
| Spectate / pause-follow | yes | yes (console `spec`/`pause [name]`) | T37 | done |
| Auto-reply to last ping (`H`) + known-phrase table | yes | yes | T23 | done |
| Warlist store: `!war`/`!peace`/`!team`/`!del` + scoreboard coloring + persist | yes | yes | T21 / T22 | done |
| Warlist advanced (folders, bundles, reasons, clan war) | yes | no | T24 | todo |
| AFK auto "tapped-out" message + `cl_tapped_out_message` toggle | yes | partial (intentionally off) | T40 | partial |
| Log scrollback: `PageUp`/`PageDown` + mouse wheel | yes | yes | T30 | done |
| Disconnect / kick handling + popup | yes | partial (auto-reconnect UI TODO) | T25 | partial |
| Resize-safe relayout (windows / popups / browser / visual) | partial (reference "wonky") | yes | T2 / T3 | done |
| Rebindable keymap | no | no (planned) | T42 | todo |
| Keymap reconciled to §I binding table | n/a | no (foundation keys diverge) | T41 | todo |

## Gaps & differences

These are the points where teetui is partial relative to the reference or
intentionally diverges:

- **Scoreboard self-name (0.6):** under protocol 0.6 the local tee's name is
  weak / unreliable, a twclient limitation surfaced in the roster (T17).
- **LAN tab:** teetui's LAN browser is a connless probe of localhost ports, not
  a true subnet broadcast — subnet discovery would need twclient support (T45).
- **Tab-completion:** name/command completion cycles on repeated `Tab`, but the
  grey inline preview shown by the reference is not yet implemented (T15).
- **Local console (`F1`):** the built-in interpreter (`help`/`echo`/`say`/
  `version`/`quit`) plus history work, but twclient config commands, tab-complete
  and the per-command help-text line are still TODO (T39).
- **Tee control:** movement, jump and hook are wired, but aim / fire / weapon
  switching and full key-release handling are not yet done (T16).
- **AFK auto-message:** the `H` reply-to-ping path is done; the automatic
  "tapped out" message (`cl_tapped_out_message`) is intentionally **off** —
  chillerbot is a headless bot that is always detected as tapped out, whereas
  teetui is an interactive client, so auto-AFK is left disabled by default (T40).
- **Map download progress bar** (T33) and **warlist advanced mode** (T24) are
  not yet implemented.
- **Rebindable keys** (T42) and reconciliation of the foundation keymap to the
  §I binding table (T41) are still TODO; current keys (`t`/`y` chat, `h` hook,
  `q` quit) diverge from the target `?`/`B`/`F1`/`F2`/`T`/`Z`/`H`/`V`/`K` table.
- **Disconnect handling** raises the DISCONNECTED popup, but the auto-reconnect
  status UI is still pending (T25).

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
- **Rebindable keymap planned:** the §I binding table is the default, with
  config-driven rebinding scoped under T42 — something the reference cannot do
  (C12, V19).
