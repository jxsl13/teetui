# teetui

A cross-platform **terminal** Teeworlds / DDNet client. It renders the live
game, chat and scoreboard directly in your terminal on **Linux, Windows and
macOS**, built on the pure-Go [`twclient`](https://github.com/jxsl13/twclient)
library and the [`tcell`](https://github.com/gdamore/tcell) terminal toolkit —
**no cgo, no OpenGL**.

teetui is an independent Go re-implementation of the terminal UI shipped by
**chillerbot-ux**. It is *not* a fork of that C++ codebase.

teetui has a small **core** (the base client: connect, render, input, console,
config) plus self-contained **feature modules** — every chillerbot-style behavior
(warlist, reply-to-ping, chat filters, …) is its own package that registers
itself and talks only to a public Host API. See
[Architecture](#architecture--extensibility).

---

## Contents

- [Quick start](#quick-start)
- [Configuration file](#configuration-file)
- [Configuration options](#configuration-options-cvars)
- [Features](#features)
- [Keybindings](#keybindings)
- [Console commands](#console-commands-f1)
- [Chat commands](#chat-commands-warlist)
- [Architecture & extensibility](#architecture--extensibility)
- [Credits](#credits--attributions)
- [Licenses](#licenses)

---

## Quick start

```sh
go build -o teetui .

# no arguments → opens the greeting + server browser (press B)
./teetui

# or boot straight into a server via a config file
./teetui -config my.cfg
```

The **only** command-line flag is `-config <file>`. Everything else —
identity, rendering, connecting — is a config-file line or a live console
command (see below). With no config and no `connect`, teetui starts at the
greeting popup / server browser and never auto-connects.

Per-user data lives under `~/.config/teetui/` (input history, `warlist.txt`,
`favorites.txt`, `keymap.txt`, optional `hooks/`).

---

## Configuration file

A teeworlds-style `.cfg`: **one `command [args]` per line**, `#` comments, blank
lines ignored. Each line is run through the same console layer as the in-game F1
console, so anything you can type at runtime you can put in the file. Settings
are *cvars* (`name value`); actions are *commands* (e.g. `connect`).

```sh
# my.cfg
player_name "nameless tee"
player_clan "ACAB"

cl_max_fps 60
cl_log_lines 12
cl_connect_timeout 30

# optional chat-helper behavior (all off by default)
cl_tapped_out_message 1
cl_auto_reply 0

# connect last → version comes from the arg (default 0.6 if omitted)
connect ddnet.example.org:8303 0.7
```

A bare cvar name prints its current value; `name value` sets it. The same works
live in the F1 console.

---

## Configuration options (cvars)

All cvars, their defaults and meaning. Booleans accept `0/1` (also
`true/false/on/off`). Owned-by shows which module declares the cvar (core or a
feature module).

| cvar | default | owned by | description |
|---|---|---|---|
| `player_name` | `nameless tee` | core | Your in-game player name (used on connect and for ping detection). |
| `player_clan` | *(empty)* | core | Your clan tag. |
| `cl_connect_timeout` | `30` | core | Handshake timeout in **seconds** (covers login + map download). The watchdog only aborts a *still-connecting* session, never a live one. |
| `cl_max_fps` | `60` | core | Cap on render repaints per second; `0` = unlimited (draw on every event). Pure render throttle — the network tick stays 50 Hz. |
| `cl_log_lines` | `10` | core | Number of log rows shown beneath the visual when the game view is on. Hard-capped to half the terminal height; with the visual off, logs fill the whole body. |
| `cl_silent_chat_commands` | `1` | warlist | Apply `!war`/`!peace`/… locally **without** sending the command line to the server. Set `0` to also send it. |
| `cl_war_list_auto_reload` | `10` | warlist | Reload `warlist.txt` every N **seconds** if it changed on disk (live external edits). `0` = off. |
| `cl_show_last_ping` | `0` | lastping | Show the most recent chat line that pinged you in the status bar. |
| `cl_tapped_out_message` | `0` | responders | When pinged, auto-reply once (rate-limited) with `cl_tapped_out_message_text`. Off by default — teetui is interactive, not a bot. |
| `cl_tapped_out_message_text` | `I'm currently tapped out (afk)` | responders | The text sent by the tapped-out auto-reply. |
| `cl_auto_reply` | `0` | responders | When pinged, auto-reply (rate-limited) with `cl_auto_reply_msg`. |
| `cl_auto_reply_msg` | `%n (teetui auto reply)` | responders | Auto-reply template; `%n` expands to the pinger's name. |
| `cl_chat_spam_filter` | `0` | chatfilter | Hide spammy incoming chat: `0` off, `1` hide matching lines, `2` hide + send a rate-limited canned reply. |
| `cl_chat_spam_filter_insults` | `0` | chatfilter | When the filter is on, also hide lines classified as insults. |
| `cl_chillpw` | `0` | chillpw | On connect, auto rcon-login using a password from the secrets file matched to the server address. Opt-in. |
| `cl_password_file` | `chillpw.txt` | chillpw | Secrets file (relative to `~/.config/teetui/` unless absolute). One `addr password` per line; `#` comments. The password is **never** logged. |

---

## Features

### Rendering & layout
- **Live game view** (`v`) — map and players as colored text, camera eased onto
  your tee (smooth, jitter-filtered). Map tiles colored by type (solid, freeze,
  unfreeze, death, unhookable, through, teleporter, boost, and race
  **start / finish / checkpoint**); tees (self red, others blue), ninja sword,
  hook lines, lasers, projectiles; a live `x:y` tile read-out.
- **Sub-cell detail** (`V`) — half-block rendering packs two map rows per cell
  for finer vertical resolution.
- **Spectator-aware** — renders as a spectator / free-view / following any
  visible tee when you have no own character.
- **Responsive, log-at-bottom layout** — vertical stack: status (top) → game →
  log band → input/legend (bottom). The log sits just above the legend; the
  visual pushes it down to `cl_log_lines` rows (capped at half the height). 24-bit
  truecolor where supported, auto-downsampled to 256/16 otherwise. Survives
  resize; below a minimum size it shows a single "resize" notice.
- **FPS limiting** — repaints capped at `cl_max_fps`, bursts coalesced with a
  trailing draw so the latest state always shows.

### Connection & browser
- **Both protocols** — 0.6 and 0.7; the version is taken from the server-browser
  entry on join, or the `connect` argument (default 0.6).
- **Server browser** (`B`) — live master list with tabs
  **Internet / LAN / Favorites / DDNet / KoG / Vanilla** (`←`/`→`), incremental
  search (`/`), `↑`/`↓` select, `Enter` join, `f` favorite (persisted),
  passworded-server marker. LAN tab does a real subnet broadcast scan.
- **Robust connect** — actionable timeout message, `R` to reconnect, automatic
  reconnect on drop with a `reconnecting #N` status; map-download spinner.

### Chat & console
- **Chat** (`t`) and **team chat** (`y`) — readline editing (`Ctrl-U/K/W`,
  Home/End/arrows), per-mode input history (`↑`/`↓`) persisted across restarts,
  reverse-i-search (`Ctrl-R`), `Tab` completion of player names / console
  commands with a grey inline preview. Your own line is echoed locally
  immediately (deduped against the server echo).
- **Local console** (`F1`) — cvars + commands with per-command help text; see
  [Console commands](#console-commands-f1).
- **Remote console / rcon** (`F2`) — masked-password login, admin commands, rcon
  output in the log.
- **Message log** — scroll with `PageUp`/`PageDown` / mouse wheel.

### Chat-helper feature modules
- **warlist** — mark players war / peace / team (with reasons, clans, folders)
  from chat `!` commands; marked names are tinted in the scoreboard; persisted to
  `warlist.txt` and live-reloaded. See [Chat commands](#chat-commands-warlist).
- **replytoping** — `H` replies to the most recent ping: first a state-derived
  answer (war status, "where are you", "what OS", "list your wars", clan-join),
  else a multilingual canned reply (greeting / smalltalk / bye / no-context),
  walking older pings on repeated presses.
- **responders** — opt-in tapped-out / auto-reply when pinged
  (`cl_tapped_out_message`, `cl_auto_reply`).
- **chatfilter** — hide spam / insults / user-substrings from incoming chat
  (`cl_chat_spam_filter`); manage the list with `addfilter`/`listfilter`/`delfilter`.
- **lastping** — keeps the last 16 pings and can show the latest in the status
  bar (`cl_show_last_ping`).
- **team** — `team <spectators|red|blue|game>` / `join` to switch team or join
  the game.
- **chillpw** — opt-in rcon auto-login from a local secrets file (`cl_chillpw`).
- **cmdhook** — run your own external scripts on events (see
  [extensibility](#architecture--extensibility)).

### Tee control
Move (`a`/`d`/`s`), jump (`space`), toggle hook (`h`), self-kill (`k`), emote
(`e`), select weapon (`1`–`6`), fire (`f`), aim by arrow keys, vote (`F5`/`F6`),
spectate/pause from the console.

---

## Keybindings

Defaults (rebindable via `~/.config/teetui/keymap.txt`, one `action = key` per
line). The bottom legend bar and the `?` help overlay are generated from the
live keymap and any feature actions, so they always reflect your current
bindings.

| key | action |
|---|---|
| `?` | toggle help overlay |
| `B` / `b` | server browser |
| `t` / `T` | chat |
| `y` / `Z` | team chat |
| `F1` | local console |
| `F2` | remote console (rcon) |
| `v` | toggle game view |
| `V` | toggle sub-cell (half-block) detail |
| `G` | free-look map-pan (arrows/WASD pan the camera, `Esc`/`G` recenter & exit) |
| `Tab` | scoreboard |
| `H` | reply to last ping *(replytoping feature)* |
| `R` | reconnect |
| `a` / `d` / `s` | move left / right / stop |
| `space` | jump |
| `h` | toggle hook |
| `k` / `K` | self-kill |
| `e` | emote |
| `1`–`6` | select weapon |
| `f` | fire |
| arrow keys / `WASD` | aim (cardinal); pan the map while free-look (`G`) is on |
| `F5` / `F6` | vote yes / no |
| `PgUp` / `PgDn` / wheel | scroll log |
| `Ctrl-U` / `Ctrl-K` / `Ctrl-W` | kill line-before / -after / word (input) |
| `Ctrl-R` | reverse-i-search input history |
| `q` / `Esc` / `Ctrl-C` | quit |

---

## Console commands (F1)

| command | description |
|---|---|
| `help [cmd]` | list commands, or show help for one |
| `connect <host:port> [0.6\|0.7]` | connect to a server (version default 0.6) |
| `say <msg>` | send a chat message |
| `spec` / `spectate` / `pause` `[name]` | spectate a player, or free-view |
| `echo <text>` | print text to the log |
| `version` | show client / library version |
| `quit` / `exit` | exit teetui |
| `<cvar> [value]` | print or set any [cvar](#configuration-options-cvars) |
| `team <spectators\|red\|blue\|game>` / `join` | switch team / join the game *(team feature)* |
| `addfilter <text>` / `delfilter <text>` / `listfilter` | manage chat filters *(chatfilter feature)* |

---

## Chat commands (warlist)

Typed in chat (`t`). Applied locally; not sent to the server when
`cl_silent_chat_commands` is `1` (default).

| command | description |
|---|---|
| `!war <name…>` | mark one or more names as war |
| `!peace <name…>` / `!team <name…>` | mark peace / team |
| `!del` / `!unfriend <name…>` | clear relation(s) |
| `!reason` / `!addreason <name> <text>` | attach a war reason |
| `!search <name>` | list matching warlist entries |
| `!create <war\|team\|neutral\|traitor> <name>` | set a relation by keyword |
| `!warclan` / `!peaceclan` / `!teamclan` / `!delclan <tag>` | per-clan relation |
| `!help` | list warlist commands |

---

## Architecture & extensibility

teetui is a small **core** (`internal/tui`: base client, render/input loop, the
console, and the module **Host**) plus **feature modules** under `features/*`.
Each feature self-registers in `init()` and is enabled by being blank-imported
from `main.go` — like Caddy v2 modules or the `image` stdlib's format registry.
Features talk **only** to the public `feature.Host` API (send chat, do a
twclient action, read roster/config, register cvars/commands/actions/status/
name-styles, look up other features' services). There is **no raw packet /
network primitive** in that surface, so a feature cannot become a flood/DoS tool,
and a panicking feature is recovered + disabled, never crashing the client.

A number of chillerbot-ux features are **deliberately not shipped** — but you can
build them yourself as a feature or an external hook:

| not shipped | why |
|---|---|
| graphical effects (player pics, particles, laser/weapon HUD, skin stealer) | no GUI in a terminal |
| cheats/automation (camp-hack auto-walk, spike tracer) | not a cheat client |
| **server stress / pentest** | abusive (DoS) — will not be implemented |
| telemetry (online-time ping, client-id / type broadcast) | teetui does not phone home |
| mod-specific (city/wallet, MMOTee, vibebot, in-game map editor) | not core TW/DDNet |
| remote-control your client via whispered tokens | remote code execution risk |

### In-process feature module (Go)

```go
package myfeat

import "github.com/jxsl13/teetui/feature"

type feat struct{ feature.NopFeature }

func (feat) Name() string                  { return "myfeat" }
func (feat) Provision(h feature.Host) error { return nil }

func (feat) OnChat(h feature.Host, e feature.ChatEvent) bool {
	if e.Msg == "!ping" {
		h.SendChat("pong", false)
	}
	return false // return true to hide the line
}

func init() { feature.Register(feat{}) }
```

Enable it by adding one blank import to `main.go`. Events: `OnConnect`,
`OnDisconnect`, `OnChat` (→ suppress), `OnBroadcast`, `OnServerMsg`, `OnKill`,
`OnTick`, `OnKey` (→ handled). At `Provision` a feature may `DefineConfig`
(its own cvars), `DefineCommand` (F1 commands), `DefineAction` (rebindable keys),
`AddStatusField`, `AddNameStyle` (scoreboard tint), and `Provide`/`Lookup`
cross-feature services.

### External command hooks (no recompile)

Drop executables in `~/.config/teetui/hooks/<event>` (`chat`, `connect`,
`disconnect`, `broadcast`, `servermsg`, `kill`). teetui feeds the event as JSON
on **stdin** and applies the action lines printed on **stdout**:

```
say <message>        # send public chat
say-team <message>   # send team chat
log <message>        # write to the teetui log
suppress             # (chat) hide the triggering line
```

```sh
#!/bin/sh
# ~/.config/teetui/hooks/chat   (chmod +x)
msg=$(cat)                                   # event JSON on stdin
echo "$msg" | grep -q '!ping' && echo "say pong"
```

Off unless `hooks/` exists. Scripts are timeout-bounded and isolated;
high-frequency tick/key events are not bridged to processes.

---

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
