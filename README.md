# teetui

A cross-platform **terminal** Teeworlds / DDNet client. It renders the live
game, chat and scoreboard directly in your terminal on **Linux, Windows and
macOS**, built on the pure-Go [`twclient`](https://github.com/jxsl13/twclient)
library and the [`tcell`](https://github.com/gdamore/tcell) terminal toolkit —
**no cgo, no OpenGL**.

teetui is an independent Go re-implementation of the terminal UI shipped by
**chillerbot-ux**. It is *not* a fork of that C++ codebase.

## Features

- **Cross-platform terminal UI** — Linux, Windows and macOS. 24-bit truecolor
  where the terminal supports it, automatically downsampled to 256/16 colors
  elsewhere.
- **Both protocols** — connect to Teeworlds/DDNet **0.6** and **0.7** servers
  (`-version`).
- **Live game view** (toggle with `v`) — the map and players rendered as colored
  text, camera following your tee:
  - map tiles colored by type: solid, freeze, unfreeze, death, unhookable,
    hook-through, teleporter, speed-boost, switch, and race **start/finish**;
  - your tee (red), other tees (blue), the ninja sword, hook lines, lasers and
    projectiles;
  - a live coordinate read-out of your tee.
- **Scoreboard** (`Tab`) — `score | name | clan`, sorted by score, your own row
  highlighted, from the live in-session player list.
- **Chat** — all chat (`t`) and team chat (`y`) with:
  - readline editing: cursor movement, `Ctrl-U` / `Ctrl-K` / `Ctrl-W` kill;
  - input history with `Up` / `Down`, **persisted across restarts**;
  - reverse history search with `Ctrl-R`.
- **Local console** (`F1`) — `help`, `echo`, `say`, `version`, `quit`, with its
  own history.
- **Remote console / rcon** (`F2`) — log in with a masked password, send admin
  commands, and see rcon output in the log.
- **Server browser** (`B`) — fetches the live master-server list, with:
  - category tabs **Internet / DDNet / KoG / Vanilla** (`←` / `→`);
  - incremental search (`/`);
  - select with `↑` / `↓` and join with `Enter` (reconnects to the chosen
    server);
  - a marker for password-protected servers.
- **Warlist** — mark players as war / peace / team from chat with `!war <name>`,
  `!peace <name>`, `!team <name>`, `!del <name>` (`!help` lists them). Marked
  players are colored in the scoreboard; the list is saved to
  `~/.config/teetui/warlist.txt`. Commands are applied locally and not sent to
  the server.
- **Auto-reply** — press `H` to reply to the last chat message that pinged you,
  using a small known-phrase table (otherwise a greeting addressed to the
  sender).
- **Tee control** — move (`a` / `d` / `s`), jump (`space`), toggle hook (`h`),
  self-kill (`k`), emote (`e`), vote yes/no (`F5` / `F6`).
- **Message log** — scroll with `PageUp` / `PageDown` and the mouse wheel.
- **Help overlay** (`?`), a startup key-hint popup, and a disconnect notice.
  The layout survives terminal resizes.

## Build

```sh
go build ./...
```

## Run

```sh
teetui -server 127.0.0.1:8303 -name "nameless tee" -version 0.6
```

Flags: `-server`, `-name`, `-clan`, `-skin`, `-version` (`0.6` | `0.7`).

Input history is stored under `~/.config/teetui/`.

## Keys

| key | action |
|---|---|
| `?` | toggle help |
| `B` | server browser |
| `F1` | local console |
| `F2` | remote console (rcon) |
| `t` / `y` | chat / team chat |
| `v` | toggle game view |
| `Tab` | scoreboard |
| `a` / `d` / `s` | move left / right / stop |
| `space` | jump |
| `h` | toggle hook |
| `H` | auto-reply to last ping |
| `k` | self-kill |
| `e` | emote |
| `F5` / `F6` | vote yes / no |
| `PgUp` / `PgDn` / wheel | scroll log |
| `Ctrl-U` / `Ctrl-K` / `Ctrl-W` | edit input line |
| `Ctrl-R` | search input history |
| `q` / `Esc` | quit |

## Credits & attributions

- **chillerbot-ux** — the reference terminal UI that teetui re-implements.
  <https://github.com/chillerbot/chillerbot-ux> (author: ChillerDragon).
  Based on DDNet → DDRace → Teeworlds.
- **DDNet** — <https://ddnet.org> / <https://github.com/ddnet/ddnet>.
- **Teeworlds** — <https://www.teeworlds.com> / <https://github.com/teeworlds/teeworlds>.
- **twclient** — the pure-Go headless Teeworlds/DDNet client library that powers
  all networking, prediction and the server browser.
  <https://github.com/jxsl13/twclient>.
- **tcell** — cross-platform terminal cell library.
  <https://github.com/gdamore/tcell> (© Garrett D'Amore, Apache-2.0).
- **go-runewidth** — wide-rune width for correct column layout.
  <https://github.com/mattn/go-runewidth>.

## Licenses

teetui is distributed under its `LICENSE`. The projects above retain their own
licenses; see each upstream repository (tcell is Apache-2.0; Teeworlds/DDNet
carry their respective licenses). teetui ships **no** game assets.
