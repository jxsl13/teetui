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

## §I — interfaces

### cli
- cmd: `teetui [flags]` → opens TUI, connects.
- flags (?final): `-server addr:port`, `-name`, `-clan`, `-skin`, `-version 0.6|0.7`, `-password`, `-no-color`, `-config path`.
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
camera: render_dist 16 tiles, frame ≤64w×32h, local tee centered (col 32,row 16). map `MapView` tile index → glyph+`tcell.Style`:
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
T25|~|disconnect/kick handling: OnDisconnect→DISCONNECTED popup + wake DONE; auto-reconnect status UI TODO|V11
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
T39|~|local console F1: command interpreter (help/echo/say/quit/version) + history DONE; twclient config cmds + tab-complete + help-text line TODO (← transcript F1)|I.modes,V9
T40|~|chillerbot AFK: `H` reply-to-ping DONE (T23); auto tapped-out message + `cl_tapped_out_message` toggle TODO (off by default — teetui is interactive, not AFK)|I.config
T41|x|reconcile keymap to §I key-binding table (?/B/F1/F2/T/Z/H/V/K/Tab//) — supersedes foundation `t`/`y`/`h`/`q`|I.modes,V17
T42|x|rebindable keymap: config file load/save, default = §I table, runtime bind (exceed reference)|V19,C12
T43|x|render-quality: Start/Finish/Checkpoint colored via MapView booleans DONE (Tele/Boost via class); sub-cell → T46; smooth camera (eased cameraSmoother, §T43) DONE|C11,V20,I.render
T44|x|parity-checklist verify: each §T30-41 feature ≥ chillerbot; doc gaps|C10,V20
T45|x|browser LAN + Favorites: favorites persist ~/.config/teetui/favorites.txt + `f` toggle + Favorites tab; LAN = connless probe of localhost ports (subnet broadcast would need twclient support)|I.windows,V13
T46|x|render sub-cell detail: half-block ▀▄ (2 tiles/cell vertical) | braille mode for finer map; toggle/auto (completes T43 sub-cell)|C11,V20,I.render
T47|x|render checkpoint tile color (orange, glyph 'C') via twclient v0.2.2 `MapView.Checkpoint`; precedence finish>start>checkpoint (← chillerbot colorCheckpoint)|C11,I.render
T48|x|e2e harness `e2e/` mirroring twclient: docker-compose (ddnet 0.6+0.7-sixup, teeworlds7 vanilla 0.7, Dockerfiles from source), gated `-tags e2e`+`TW_E2E`, service-name addrs; test connects each + RunFrontends + asserts snapshot ticks + roster|C14,V22,V23
T49|x|CI/CD e2e job: build server images (matrix), run `go test -tags e2e ./e2e/...` IN-NETWORK + race + coverage profile + per-pkg %; mirror twclient `.github/workflows/ci.yml`|C14,V23
T50|x|connect UX: actionable timeout msg (addr/version/network) in log + reconnect/retry key; ?auto-detect protocol via connless `QueryServerInfo` probe before Connect|V24,I.windows
T51|x|browser LAN tab → REAL subnet scan via twclient v0.2.3 `master.ScanLAN` (broadcast 0.6+0.7, dedupe), replacing localhost-port probe (upgrades T45). map `[]LANServer`→serverRow into LAN source|I.windows,V13
T52|x|FIX B4: `App.Join` → `Connect(fctx)` (long-lived session ctx); drop `defer cancel` of session ctx; bound handshake via watchdog goroutine that cancels fctx ONLY if still `!connected` after ~12s. + EXTEND e2e (T48): assert SUSTAINED liveness — snapshots keep advancing >15s (past sv_timeout), ⊥ just initial tick|V25,V22,V24
T53|x|FIX B6 spectator render: DrawGame/DrawGameHalf center on spectated target | free-view | any visible tee when no `Players[LocalID]`; render map+tees as spectator (⊥ "connecting…")|V27,I.render
T54|x|FIX B7 connect msg: raise connectTimeout (real-server map download) + make configurable; surface connect-fail in log ONLY on terminal failure (⊥ if a reconnect then succeeds)|V28,V25
T55|x|FIX B8 own-chat: locally echo sent chat (all+team) into log immediately on send; dedupe the server echo (by msg+recent time)|V29,I.windows
T56|x|B5 mitigation: scoreboard/chat id fallback when roster name empty (verify) + file twclient feature for 0.6 ClientInfo→registry decode (SPEC-player-registry T6)|V26

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
