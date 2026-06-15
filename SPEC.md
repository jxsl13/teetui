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

## §I — interfaces

### cli
- cmd: `teetui [flags]` → opens TUI, connects.
- flags (?final): `-server addr:port`, `-name`, `-clan`, `-skin`, `-version 0.6|0.7`, `-password`, `-no-color`, `-config path`, `-connect-timeout dur`, `-max-fps n` (0=unlimited, §T74).
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
- game window: ASCII map + tees, camera on local tee (§I.render).
- log window: chat/console/server-msg scrollback.
- info window: status bar — input mode, server, race time, ping, fps.
- input window: textbox + cursor + tab-completion preview (grey) + reverse-i-search prompt.
- scoreboard (toggle): cols `score|name(20)|clan(20)`, per DDTeam.
- server-browser list (toggle): from `master.FetchServerList`, searchable, select→connect.
- help page (toggle). popup: MESSAGE | NOT_IMPORTANT | DISCONNECTED | WARNING.

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
PageUp/Dn + mouse-wheel: scroll log
Ctrl-R   reverse-i-search input history
Ctrl-U/K/W: readline kill (line-before / line-after / word)
A/D      move left/right (visual mode) ; Space jump
```
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
- V26: 0.6 roster names ?empty (twclient gap — 0.6 `Sv_ClientInfo`/`ObjClientInfo` ⊥ decoded to registry; e2e: 0.6 roster=0 vs 0.7-sixup=5). teetui ! degrade gracefully (id fallback when name empty), ⊥ blank/crash. REAL fix = twclient 0.6 ClientInfo decode (SPEC-player-registry T6). (← B5)
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
T60|x|lang classifier (port chillerbot `langparser`): FindWord (word-boundary, case-insens), IsGreeting(en/qq/rus)/IsBye/IsInsult/IsAskToAsk(+de)/IsQuestionWhy·How·WhichWhat·WhoWhichWhat; pure pkg, table-tested multi-lang|C18,V33,I.twclient
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

## §B — bugs

id|date|cause|fix
B1|2026-06-15|`B` server-browser key dead: startup greeting popup intercepted ALL keys in handlePopup (only Enter/Esc/?/q closed), so `B` swallowed & openBrowser unreachable while popup shown — though popup advertises "B server browser"|V21
B2|2026-06-15|"connecting to servers does not work": teetui never called `Client.RunFrontends` → Observer(render)+Controller(input) NEVER dispatched. Connected & snaps ticked but UI stuck "connecting…" (observerTicks=0 vs snapTick advancing). fix: `go c.RunFrontends(fctx)` after each Connect, via unified `App.Join`|V22
B3|2026-06-15|connect "context deadline exceeded": NOT a teetui code bug — connect verified OK vs live teeworlds7:8303 0.7 (0.0s, in compose net). cause = (a) macOS Docker host UDP forward broken → host can't reach localhost:8303/8307 (C15); (b) `-version` mismatch → handshake never completes → 12s deadline. mitigate: run in compose net OR matching `-version`; automate via e2e (T48/T49) + UX (T50)|V23,V24
B4|2026-06-15|connect succeeds then session DIES (server sv_timeout disconnect): `App.Join` passed `context.WithTimeout(bg,12s)` to `Connect(ctx)` + `defer cancel()` in connect goroutine. twclient binds reader+keepalive+all I/O to the Connect ctx (= session lifetime) → cancel (fired right after Connect returns via defer, or @12s) tore down the LIVE session → no recv/keepalive → DDNet sv_timeout drops client. reproduced: snapshots stop exactly @ ctx deadline (delta 100/2s → 0 @ ~12s). fix: Connect(fctx) long-lived; handshake bounded by watchdog cancelling fctx only if !connected|V25,V22
B5|2026-06-15|players show id but NO name (scoreboard/chat/nameplate): on 0.6 the twclient player REGISTRY is empty — `Roster()`=0, `Player(id)` not found, even own player. probed live vs ddnet:8303 0.6: roster empty after 6s; own chat echo arrives w/ `name=""`. twclient ⊥ decode 0.6 Sv_ClientInfo/ObjClientInfo into registry (0.7-sixup roster=5 ✓). dep gap. teetui mitigate: id fallback; real fix twclient|V26
B6|2026-06-15|as SPECTATOR the visual/view mode renders nothing: `DrawGame`/`DrawGameHalf` do `self,ok:=st.Players[st.LocalID]; if !ok {return "connecting…"}`. spectator/free-view has no local character in Players → early-return → blank. fix: center on spectated target/free-view/any tee|V27
B7|2026-06-15|"context deadline exceeded" shown at connect though a later connect succeeds: `App.Join` handshake watchdog (connectTimeout 12s) aborts a still-progressing connect (real-server map download >12s) → Connect returns ctx err → connectFailMsg logged, yet retry connects. msg misleading + timeout too short. fix: raise/configurable timeout; show fail only on terminal failure|V28
B8|2026-06-15|own chat lines ⊥ visible: teetui ⊥ locally echo sent chat, relies on server echo. probe: docker server DOES echo own line but w/ empty name (0.6, B5) → "[0]"/looks missing; other servers ⊥ echo own chat → invisible. fix: local echo of sent chat immediately, dedupe server echo|V29
