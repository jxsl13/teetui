package tui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/jxsl13/teetui/feature"
	"github.com/jxsl13/twclient/client"
	"github.com/jxsl13/twclient/master"
	"github.com/jxsl13/twclient/packet"
)

// input modes (← chillerbot terminalui enum). modeNormal drives the tee;
// the rest feed the single input line.
const (
	modeNormal = iota
	modeChat
	modeChatTeam
	modeLocalCon // F1
	modeRconAuth // F2, awaiting password
	modeRcon     // F2, authenticated
	modeBrowser  // B, server browser overlay
)

// App owns the tcell screen and drives the render/event loop. The poll loop runs
// on its own goroutine and feeds events through a channel; UI state is mutated
// on the main loop. Fields written from twclient callback goroutines (popup,
// connected) are guarded (atomic / mu) so there is no data race (§V4, §V8).
type App struct {
	scr            tcell.Screen
	log            *Log
	connectTimeout time.Duration // handshake timeout (0 = DefaultConnectTimeout, §T54)

	// sessions are the open connections (primary + dummies, §C36); the ACTIVE one
	// is rendered + controlled (§V77). Guarded by sessMu (slice + active).
	sessions []*session
	active   int
	sessMu   sync.Mutex

	reconnecting  atomic.Bool  // an auto-reconnect of the primary is in flight (§T25)
	reconnAttempt atomic.Int32 // auto-reconnect attempt counter (§T25)

	mode     int
	line     TextInput
	histAll  *History
	histTeam *History
	histCon  *History
	histRcon *History

	search     bool
	searchTerm []rune
	searchHit  string

	compActive  bool
	compMatches []string
	compIdx     int
	compStart   int

	visual     bool
	subcell    bool           // half-block sub-cell map render (§T46)
	freeLook   bool           // free-look map-pan sub-mode (§T94/§V54)
	panX, panY int            // free-look camera pan offset, in tiles (§T94)
	camera     cameraSmoother // eases the rendered camera center (§T43)
	help       bool
	bcast      string    // current server broadcast text (§T121), "" = none
	bcastUntil time.Time // broadcast hides after this (DDNet ~10s, §V82)
	escMenu    escMenu   // DDNet-style overlay action bar (§T111/§V74)
	scoreboard bool
	hookOn     bool
	drawFrame  int  // advances each redraw; drives the connecting spinner (§T33)
	forceSync  bool // next present() does a full scr.Sync (viewport floor, §T130/§V90)

	browser *Browser
	// dialer builds a client bound to a given session's Observer/Controller, with
	// callbacks captured for that session (§T113). main + e2e inject it.
	dialer func(s *session, addr string, ver packet.Version) *client.Client

	pendingAddr string         // connect requested by config before Start (§T89)
	pendingVer  packet.Version // protocol for the pending connect

	keymap     *Keymap
	playerName string
	playerClan string     // our clan tag, for chat-query answers (§T62)
	cfg        *Config    // console-settable client config (§T39/§T40)
	cfgMu      sync.Mutex // guards a.cfg vs off-main readers (callbacks/loops, §V4)

	limiter frameLimiter // render repaint throttle (§T73), Run goroutine only

	// feature-module registries (§T76): populated by Host calls during Provision.
	dynVars      map[string]*dynVar                            // feature-defined cvars
	featCmds     map[string]*featCmd                           // feature console commands (§T92)
	featActRune  map[rune]func()                               // feature actions bound to a rune
	featActKey   map[tcell.Key]func()                          // feature actions bound to a named key
	featActions  []featAction                                  // feature action metadata for legend/help (§T95/§T96)
	statusFields []func() string                               // status-bar contributions
	nameStylers  []func(name, clan string) (tcell.Style, bool) // per-name styling
	services     map[string]any                                // cross-feature service registry
	sendChatHook []func(msg string, team bool) (string, bool)  // outgoing-chat chain

	mu      sync.Mutex // guards popup + sent (callback goroutines)
	popup   Popup
	sendBuf *sendBuffer // rate-limited outgoing chat queue (§T65)
	sent    []sentChat  // recently sent chat, for own-echo dedupe (§V29)

	quit chan struct{}
}

// NewApp wires the app to its shared state, input controller and log on a real
// terminal screen, and loads persisted input history from disk (§V16).
func NewApp(server string, state *State, input *InputController, log *Log) (*App, error) {
	scr, err := tcell.NewScreen()
	if err != nil {
		return nil, err
	}
	if err := scr.Init(); err != nil { // alt-buffer, raw mode, color caps (§V5)
		return nil, err
	}
	scr.Clear() // keyboard-only: mouse never enabled (§C31/§V69)
	return NewAppWithScreen(scr, server, state, input, log), nil
}

// NewAppWithScreen wires an app around an ALREADY-initialized tcell screen — a
// real one (NewApp) or a tcell.SimulationScreen. The simulation path lets the
// e2e harness drive the whole UI headlessly and assert the rendered cell buffer
// against a live server (§C14/§V23), exercising the same handlers main uses.
func NewAppWithScreen(scr tcell.Screen, server string, state *State, input *InputController, log *Log) *App {
	a := &App{
		scr:      scr,
		log:      log,
		histAll:  NewHistory(64),
		histTeam: NewHistory(64),
		histCon:  NewHistory(64),
		histRcon: NewHistory(64),
		browser:  NewBrowser(),
		keymap:   DefaultKeymap(),
		cfg:      NewConfig(),
		sendBuf:  newSendBuffer(sendMinInterval, sendQueueMax), // spam-safe send (§T65)
		visual:   true,
		popup:    greetingPopup(), // startup greeting (§T31)
		quit:     make(chan struct{}),

		dynVars:     map[string]*dynVar{}, // feature registries (§T76)
		featCmds:    map[string]*featCmd{},
		featActRune: map[rune]func(){},
		featActKey:  map[tcell.Key]func(){},
		services:    map[string]any{},
	}
	// The primary session owns the passed-in render state + input controller; its
	// State drives feature OnTick + per-tick redraw (§T70/§T109, set in newSession).
	primary := a.newSession("main", state, input)
	primary.server = server
	a.sessions = []*session{primary}
	a.active = 0
	a.loadHistory()
	if p, err := configDir(); err == nil {
		_ = a.browser.LoadFavorites(filepath.Join(p, "favorites.txt"))
		_ = a.keymap.Load(filepath.Join(p, "keymap.txt")) // rebindable keys (§V19/§T42)
	}
	a.provisionFeatures() // provision registered feature modules (§T76)
	return a
}

// cfgSnap returns a consistent shallow copy of the live config for readers on
// goroutines other than the UI thread (twclient callbacks, the reload loop), so
// console writes don't data-race with them (§V4). Config has no internal mutex,
// so it is safely value-copied under cfgMu.
func (a *App) cfgSnap() Config {
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	return *a.cfg
}

// favPath returns the favorites file path (best-effort).
func (a *App) favPath() string {
	if p, err := configDir(); err == nil {
		return filepath.Join(p, "favorites.txt")
	}
	return ""
}

func (a *App) loadHistory() {
	for slug, h := range a.histBySlug() {
		if p, err := historyPath(slug); err == nil {
			_ = h.Load(p)
		}
	}
}

func (a *App) saveHistory() {
	for slug, h := range a.histBySlug() {
		if p, err := historyPath(slug); err == nil {
			_ = h.Save(p)
		}
	}
}

func (a *App) histBySlug() map[string]*History {
	return map[string]*History{
		"chat": a.histAll, "team_chat": a.histTeam,
		"local_console": a.histCon, "remote_console": a.histRcon,
	}
}

// SetClient installs the connected client for chat/input/rcon sends.
func (a *App) SetClient(c *client.Client) { a.cur().cli.Store(c) }

// Client returns the currently active client (may differ after a browser join).
func (a *App) Client() *client.Client { return a.cur().cli.Load() }

// SetName records the local player name for ping detection (§T23).
func (a *App) SetName(name string) { a.playerName = name }

// SetDialer installs the factory used to (re)build a client when joining a
// server (§T18/§T113). main supplies it so callbacks stay wired.
func (a *App) SetDialer(fn func(s *session, addr string, ver packet.Version) *client.Client) {
	a.dialer = fn
}

// SetConnected updates the status indicator.
func (a *App) SetConnected(b bool) { a.cur().connected.Store(b) }

// ShowDisconnect handles a disconnect of the active session (external/test entry,
// §T25/§V11). Real callbacks use onDisconnect with the specific session.
func (a *App) ShowDisconnect(reason string) { a.onDisconnect(a.cur(), reason) }

// onDisconnect handles session s dropping. A dummy is simply removed (§V77); the
// primary raises the disconnect popup and auto-reconnects — unless the user asked
// to disconnect (→ browser) or the app is quitting. Safe from a twclient callback
// goroutine (§V4).
func (a *App) onDisconnect(s *session, reason string) {
	s.connected.Store(false)
	s.joining.Store(false)
	s.state.Clear()                         // drop the dead map/tees from the view (§T117/§V79)
	feature.FireDisconnect(a.api(), reason) // notify feature modules (§T76)

	// Deliberate close (Esc-menu Disconnect/Disconnect dummy): closeSession already
	// cleared state synchronously (§B16); just don't reconnect.
	if s.userClosing.Swap(false) {
		return
	}

	if !a.isPrimary(s) { // a dummy dropped — just remove it (§V77)
		a.log.Addf(StyleSystem, "dummy '%s' disconnected", s.name)
		a.dropSession(s)
		a.wake()
		return
	}

	a.camera.reset() // next session snaps the camera, no slide across the map
	a.exitFreeLook() // drop free-look on disconnect; next session locks to tee (§V54)
	if a.quitting() {
		return
	}
	a.mu.Lock()
	a.popup = disconnectPopup(reason)
	a.mu.Unlock()
	a.wake()
	go a.reconnect()
}

// closeSession tears one session down SYNCHRONOUSLY at the user's request:
// cancel its frontend, close its client, and clear its render state. userClosing
// marks the close deliberate so onDisconnect (if it does fire after the ctx is
// cancelled) skips the auto-reconnect (§B16/§V84).
func (a *App) closeSession(s *session) {
	s.userClosing.Store(true)
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	if c := s.cli.Load(); c != nil {
		_ = c.Close()
	}
	s.connected.Store(false)
	s.joining.Store(false)
	s.state.Clear() // drop the dead map immediately (§B16/§V79)
}

// disconnectAll ends the WHOLE connection at the user's request (Esc-menu
// "Disconnect", §T111/§C42): close the primary AND every dummy (a dummy ⊥
// outlive its primary, §V92), drop the dummies, and open the browser HERE rather
// than waiting on the async OnDisconnect callback (§B16/§V84).
func (a *App) disconnectAll() {
	sessions, _ := a.sessionList()
	for _, s := range sessions {
		a.closeSession(s)
	}
	a.sessMu.Lock()
	if len(a.sessions) > 0 {
		a.sessions = []*session{a.sessions[0]} // keep only the primary; drop dummies
	}
	a.active = 0
	a.sessMu.Unlock()
	a.camera.reset() // ⊥ slide on next session
	a.exitFreeLook()
	a.openBrowser() // back to the server browser
	a.wake()
}

// disconnectDummy ends ONLY the active dummy (Esc-menu "Disconnect dummy",
// §C42/§V92): close it, drop it, and refollow the primary — which stays
// connected. ⊥ browser, ⊥ reconnect. The menu shows this only when a dummy is
// active; the primary guard makes it a no-op otherwise.
func (a *App) disconnectDummy() {
	s := a.cur()
	if a.isPrimary(s) {
		return
	}
	a.closeSession(s)
	a.dropSession(s)
	a.setActive(0) // refollow primary (camera reset + wake); main stays connected
	a.log.Addf(StyleSystem, "dummy '%s' disconnected", s.name)
}

// quitting reports whether Stop has been called (the quit channel is closed), so
// background goroutines (auto-reconnect) do not fight a shutdown.
func (a *App) quitting() bool {
	select {
	case <-a.quit:
		return true
	default:
		return false
	}
}

// connStatus snapshots the connection state for the status bar (§T25).
// connecting reports whether a connect handshake or auto-reconnect is in flight
// (≠ idle). At idle teetui shows "not connected", never "connecting" (§B9).
func (a *App) connecting() bool { return a.cur().joining.Load() || a.reconnecting.Load() }

// scenePlaceholder is the game-window text when there is no map yet (§B9/§B18):
// "connecting…" while a join/download is in flight; once connected with still no
// map, twclient gave up on the download ("continuing without map") → an actionable
// retry notice rather than an endless spinner; otherwise the idle browser hint.
func (a *App) scenePlaceholder() string {
	switch {
	case a.connecting():
		return "connecting…"
	case a.cur().connected.Load():
		return "map not loaded — press R to re-download"
	default:
		return "not connected — press B for the server browser"
	}
}

func (a *App) connStatus() connStatus {
	return connStatus{
		connected:    a.cur().connected.Load(),
		reconnecting: a.reconnecting.Load(),
		joining:      a.cur().joining.Load(),
		attempt:      int(a.reconnAttempt.Load()),
	}
}

// wake nudges the event loop so background state changes redraw promptly.
func (a *App) wake() { _ = a.scr.PostEvent(tcell.NewEventInterrupt(nil)) }

// Stop persists history + warlist, tears the screen down and unblocks Run. It
// closes ALL sessions (primary + dummies, §V77) so no goroutine/socket leaks.
func (a *App) Stop() {
	for _, s := range a.sessions {
		if s.cancel != nil {
			s.cancel()
		}
		if c := s.cli.Load(); c != nil {
			_ = c.Close()
		}
	}
	feature.CloseAll(a.api()) // release feature resources on shutdown (§T101/§V62)
	a.saveHistory()
	select {
	case <-a.quit:
	default:
		close(a.quit)
	}
}

// drainSends paces queued outgoing chat through the spam-safe buffer, emitting
// at most one line per tick interval until the app quits (§T65/§V37).
func (a *App) drainSends() {
	t := time.NewTicker(100 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-a.quit:
			return
		case now := <-t.C:
			a.sendBuf.drain(now, a.flushSend)
		}
	}
}

// flushSend performs the actual server send for one dequeued chat line (§T65).
func (a *App) flushSend(msg string, team bool) {
	c := a.cur().cli.Load()
	if c == nil {
		return
	}
	if team {
		_ = c.Do(client.ActChat{Team: true, Msg: msg})
	} else {
		_ = c.SendChat(msg)
	}
}

// Run renders until quit. It returns after restoring the terminal.
func (a *App) Run() {
	defer a.scr.Fini()
	events := make(chan tcell.Event, 16)
	go a.scr.ChannelEvents(events, a.quit)
	go a.drainSends() // pace outgoing chat (§T65)

	// Render throttle (§T73/§V42): repaints are capped at cl_max_fps. Events are
	// always handled immediately (input never stalls), but the repaint they
	// trigger is coalesced — if a draw happened too recently, one trailing draw
	// is scheduled so the latest state is shown without exceeding the cap. A timer
	// drives that trailing draw. cl_max_fps==0 → wait is always 0 → draw per event
	// (today's behavior).
	timer := time.NewTimer(time.Hour)
	if !timer.Stop() {
		<-timer.C
	}
	pending := false

	drawNow := func() {
		a.draw()
		a.limiter.record(time.Now())
		pending = false
	}
	requestDraw := func() {
		now := time.Now()
		if w := a.limiter.wait(now, fpsInterval(a.cfg.MaxFPS)); w <= 0 {
			drawNow()
		} else if !pending {
			pending = true
			timer.Reset(w)
		}
	}

	// Viewport floor (§T130/§V90/§C41): a heartbeat that forces a COMPLETE viewport
	// repaint at least cl_viewport_min_fps/sec while connected + visual, even with
	// no new ticks/events. Suppressed when a recent draw already met the rate
	// (heartbeatDue checks now-lastDraw), so event/tick-driven frames don't get
	// doubled; capped by cl_max_fps (viewportInterval). When disabled/idle it polls
	// at 1s (negligible) so a live cvar re-enable is picked up promptly.
	nextHB := func() time.Duration {
		if iv := viewportInterval(a.cfg.ViewportMinFPS, a.cfg.MaxFPS); iv > 0 {
			return iv
		}
		return time.Second
	}
	heartbeat := time.NewTimer(nextHB())

	drawNow() // initial frame
	for {
		select {
		case <-a.quit:
			return
		case ev, ok := <-events:
			if !ok {
				return
			}
			a.handle(ev)
			requestDraw()
		case <-timer.C:
			if pending {
				drawNow()
			}
		case <-heartbeat.C:
			iv := viewportInterval(a.cfg.ViewportMinFPS, a.cfg.MaxFPS)
			if heartbeatDue(time.Now(), a.limiter.last, iv, a.cur().connected.Load(), a.visual) {
				a.forceSync = true // complete viewport repaint, bypass cell-diff (§V90)
				drawNow()
			}
			heartbeat.Reset(nextHB())
		}
	}
}

// Dispatch feeds one event through the normal handler then redraws, exactly as
// the Run loop does per event. Exported so the e2e harness can drive the full UI
// synchronously (key → handler → draw) without owning a terminal and then read
// back the rendered cell buffer (§C14).
func (a *App) Dispatch(ev tcell.Event) {
	a.handle(ev)
	a.draw()
}

// Redraw forces a repaint, picking up background state changes (snapshots,
// chat/disconnect callbacks) without a key event. The Run loop redraws on its
// wake interrupt; the e2e harness calls this after waiting on a live update.
func (a *App) Redraw() { a.draw() }

// Connected reports whether the current session has completed its handshake
// (status-bar/e2e use).
func (a *App) Connected() bool { return a.cur().connected.Load() }

func (a *App) popupActive() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.popup.active()
}

func (a *App) closePopup() {
	a.mu.Lock()
	a.popup = Popup{}
	a.mu.Unlock()
}

func (a *App) handle(ev tcell.Event) {
	switch ev := ev.(type) {
	case *tcell.EventInterrupt:
		// background wake; redraw happens in Run.
	case *tcell.EventResize:
		a.scr.Sync() // relayout, no garble (§V18)
	// No mouse handling (§C31/§V69): teetui is keyboard-only; a mouse event (if a
	// terminal sends one despite EnableMouse never being called) is inert. Log
	// scroll is PgUp/PgDn.
	case *tcell.EventKey:
		// The Esc overlay menu captures all keys while open (§T111/§V74), before
		// features or mode handlers see them.
		if a.escMenu.open {
			a.handleEscMenu(ev)
			return
		}
		// Features get first refusal on the key (§T76/§V39): a handler returning
		// true consumes it before teetui's own handling. No-op when none registered.
		if feature.FireKey(a.api(), featureKey(ev)) {
			return
		}
		switch {
		case a.popupActive():
			a.handlePopup(ev)
		case a.mode == modeBrowser:
			a.handleBrowser(ev)
		case a.search:
			a.handleSearch(ev)
		case a.mode == modeNormal:
			a.handleNormal(ev)
		default:
			a.handleInput(ev)
		}
	}
}

// handlePopup closes any popup on Enter/Esc (always escapable, §V17). The
// greeting popup also honors its advertised hint keys (B → browser, ? → help)
// instead of swallowing them (§V21, §B1).
func (a *App) handlePopup(ev *tcell.EventKey) {
	a.mu.Lock()
	greeting := a.popup.Kind == popupGreeting
	a.mu.Unlock()

	if ev.Key() == tcell.KeyEnter || ev.Key() == tcell.KeyEscape {
		a.closePopup()
		return
	}
	switch ev.Rune() {
	case 'q':
		a.closePopup()
	case '?':
		a.closePopup()
		if greeting {
			a.help = true
		}
	case 'b', 'B':
		a.closePopup()
		if greeting {
			a.openBrowser()
		}
	}
}

// aimReach is the fixed magnitude (world units) of the keyboard-driven aim
// vector. Terminals have no mouse-move, so arrow keys snap aim to a cardinal
// direction at this reach (§T16).
const aimReach = 256

// weaponForRune maps the number-row keys 1..6 to a weapon selection. The packet
// weapon consts are 1-indexed (WeaponHammer==1), so key '1' selects the hammer,
// '6' the ninja (§T16).
func weaponForRune(r rune) (packet.Weapon, bool) {
	switch r {
	case '1':
		return packet.WeaponHammer, true
	case '2':
		return packet.WeaponGun, true
	case '3':
		return packet.WeaponShotgun, true
	case '4':
		return packet.WeaponGrenade, true
	case '5':
		return packet.WeaponLaser, true
	case '6':
		return packet.WeaponNinja, true
	default:
		return packet.WeaponNone, false
	}
}

func (a *App) handleNormal(ev *tcell.EventKey) {
	if a.help {
		if ev.Key() == tcell.KeyEscape || ev.Rune() == '?' {
			a.help = false
		}
		return
	}
	// Free-look map-pan: arrows/WASD pan the camera and no tee input is sent
	// while active (§T94/§V54). Intercept BEFORE the keymap so a/s/d (move) and
	// the arrows (aim) are repurposed to panning; the toggle key falls through.
	if a.freeLook && a.handleFreeLook(ev) {
		return
	}
	// Esc opens the overlay action menu while connected (§T111/§V74) — instead of
	// quitting. Idle/disconnected Esc keeps its keymap meaning (quit).
	if ev.Key() == tcell.KeyEscape && a.cur().connected.Load() {
		a.openEscMenu()
		return
	}
	// Movement/aim router (§T104/§V66): WASD + arrows drive jump/left/stop/right
	// or cardinal aim depending on cl_move_keys. Runs before the keymap so the
	// selected set is movement and the other set is aim.
	if a.handleMoveAim(ev) {
		return
	}
	// Discrete named commands resolve through the rebindable keymap (§V19/§T42).
	if act, ok := a.keymap.Lookup(ev.Key(), ev.Rune()); ok {
		a.doAction(act)
		return
	}
	// Feature-defined actions (§T76/§V46) get the keys core does not bind.
	if len(a.featActRune) > 0 || len(a.featActKey) > 0 {
		if a.runFeatureAction(ev) {
			return
		}
	}
	// Continuous/parametric controls stay direct: log scroll + weapon select
	// (1..6). Movement/aim (WASD/arrows) is handled by handleMoveAim above.
	switch ev.Key() {
	case tcell.KeyPgUp:
		a.log.ScrollUp(10)
		return
	case tcell.KeyPgDn:
		a.log.ScrollDown(10)
		return
	}
	if a.freeLook { // view-only: no weapon select while panning (§V54)
		return
	}
	if w, ok := weaponForRune(ev.Rune()); ok {
		a.cur().input.SetWeapon(w)
	}
}

// doAction runs a keymap-resolved NORMAL-mode command. Centralizing the dispatch
// keeps behavior identical regardless of which key is bound to it (§T42).
func (a *App) doAction(act KeyAction) {
	// Free-look is view-only: drop any tee-affecting command while panning so a
	// stray k/Space/f/h/e/move cannot move or fire the tee (§T94/§V54/§V12).
	if a.freeLook && isTeeInput(act) {
		return
	}
	switch act {
	case actFreeLook:
		a.toggleFreeLook()
	case actQuit:
		a.Stop()
	case actHelp:
		a.help = !a.help
	case actVisual:
		a.visual = !a.visual
	case actSubcellToggle:
		a.subcell = !a.subcell
	case actBrowser:
		a.openBrowser()
	case actKill:
		a.do(client.ActKill{})
	case actEmote:
		a.do(client.ActEmoticon{Emoticon: packet.EmoticonHearts})
	case actChat:
		a.enterMode(modeChat)
	case actTeamChat:
		a.enterMode(modeChatTeam)
	case actLocalConsole:
		a.enterMode(modeLocalCon)
	case actRemoteConsole:
		if c := a.cur().cli.Load(); c != nil && c.RconAuthed() {
			a.enterMode(modeRcon)
		} else {
			a.enterMode(modeRconAuth)
		}
	case actScoreboard:
		a.scoreboard = !a.scoreboard
	case actVoteYes:
		a.do(client.ActVote{Approve: true})
	case actVoteNo:
		a.do(client.ActVote{Approve: false})
	case actMoveLeft:
		a.cur().input.PressLeft()
	case actMoveRight:
		a.cur().input.PressRight()
	case actMoveStop:
		a.cur().input.PressStop()
	case actJump:
		a.cur().input.PressJump()
	case actHook:
		a.hookOn = !a.hookOn
		a.cur().input.SetHook(a.hookOn)
	case actReconnect:
		a.reconnect()
	case actFire:
		a.cur().input.Fire()
	}
}

func (a *App) enterMode(mode int) {
	a.mode = mode
	a.line.Clear()
	if h := a.hist(); h != nil {
		h.ResetNav()
	}
}

// hist returns the history bound to the current input mode, or nil.
func (a *App) hist() *History {
	switch a.mode {
	case modeChat:
		return a.histAll
	case modeChatTeam:
		return a.histTeam
	case modeLocalCon:
		return a.histCon
	case modeRcon:
		return a.histRcon
	default:
		return nil
	}
}

func (a *App) handleInput(ev *tcell.EventKey) {
	if ev.Key() == tcell.KeyTab { // name/command completion (§T15)
		a.complete()
		return
	}
	a.compActive = false // any other key breaks a completion cycle
	switch ev.Key() {
	case tcell.KeyEscape:
		a.mode = modeNormal
		a.line.Clear()
	case tcell.KeyEnter:
		a.submit()
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		a.line.Backspace()
	case tcell.KeyDelete:
		a.line.Delete()
	case tcell.KeyLeft:
		a.line.Left()
	case tcell.KeyRight:
		a.line.Right()
	case tcell.KeyHome:
		a.line.Home()
	case tcell.KeyEnd:
		a.line.End()
	case tcell.KeyUp:
		if h := a.hist(); h != nil {
			if s, ok := h.Prev(); ok {
				a.line.SetString(s)
			}
		}
	case tcell.KeyDown:
		if h := a.hist(); h != nil {
			if s, ok := h.Next(); ok {
				a.line.SetString(s)
			}
		}
	case tcell.KeyCtrlU:
		a.line.KillToStart()
	case tcell.KeyCtrlK:
		a.line.KillToEnd()
	case tcell.KeyCtrlW:
		a.line.KillWord()
	case tcell.KeyCtrlR:
		if a.hist() != nil {
			a.search = true
			a.searchTerm = a.searchTerm[:0]
			a.searchHit = ""
		}
	default:
		if r := ev.Rune(); r != 0 {
			a.line.Insert(r)
		}
	}
}

// submit applies the current input line per mode (§T11 state machine).
func (a *App) submit() {
	text := a.line.String()
	mode := a.mode
	switch mode {
	case modeChat:
		a.chatLine(text, false)
		a.histAll.Add(text)
	case modeChatTeam:
		a.chatLine(text, true)
		a.histTeam.Add(text)
	case modeLocalCon:
		a.runLocal(text)
		a.histCon.Add(text)
	case modeRconAuth:
		a.line.Clear()
		a.rconAuth(text) // sets mode itself
		return
	case modeRcon:
		a.rconSend(text)
		a.histRcon.Add(text)
		a.line.Clear()
		return // stay in rcon mode for more commands
	}
	a.mode = modeNormal
	a.line.Clear()
}

// complete performs Tab completion of the word before the cursor against player
// names (chat) or console commands (local console). Repeated Tab cycles through
// the matches (§T15).
func (a *App) complete() {
	runes := []rune(a.line.String())
	start, prefix := currentWord(runes, a.line.Cursor())
	if a.compActive && start == a.compStart && len(a.compMatches) > 0 {
		a.compIdx = (a.compIdx + 1) % len(a.compMatches)
	} else {
		a.compMatches = completeMatches(prefix, a.completionCandidates())
		a.compIdx = 0
		a.compStart = start
		a.compActive = len(a.compMatches) > 0
	}
	if !a.compActive {
		return
	}
	a.line.SetString(string(runes[:start]) + a.compMatches[a.compIdx])
}

// completionCandidates returns the candidate set for the current input mode.
func (a *App) completionCandidates() []string {
	if a.mode == modeLocalCon {
		return consoleCommands
	}
	c := a.cur().cli.Load()
	if c == nil {
		return nil
	}
	var names []string
	for _, p := range c.Roster() {
		if p.Name != "" {
			names = append(names, p.Name)
		}
	}
	return names
}

// handleSearch drives the reverse-i-search overlay (§T14).
func (a *App) handleSearch(ev *tcell.EventKey) {
	switch ev.Key() {
	case tcell.KeyEscape, tcell.KeyCtrlC:
		a.search = false
	case tcell.KeyEnter:
		if a.searchHit != "" {
			a.line.SetString(a.searchHit)
		}
		a.search = false
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if n := len(a.searchTerm); n > 0 {
			a.searchTerm = a.searchTerm[:n-1]
		}
		if h := a.hist(); h != nil {
			a.searchHit, _ = h.Search(string(a.searchTerm))
		}
	default:
		if r := ev.Rune(); r != 0 {
			a.searchTerm = append(a.searchTerm, r)
			if h := a.hist(); h != nil {
				a.searchHit, _ = h.Search(string(a.searchTerm))
			}
		}
	}
}

func (a *App) do(act client.Action) {
	if c := a.cur().cli.Load(); c != nil {
		_ = c.Do(act)
	}
}

// chatLine submits a chat line. Warlist "!" commands are intercepted by the
// warlist feature's AddSendChatFilter hook inside sendChat (§T78/§V14).
func (a *App) chatLine(text string, team bool) {
	a.sendChat(text, team)
}

func (a *App) sendChat(msg string, team bool) {
	if msg == "" {
		return
	}
	c := a.cur().cli.Load()
	if c == nil {
		return
	}
	// Outgoing-chat hook chain (§T76): a feature may rewrite the line or cancel
	// the send (e.g. silent !commands). No hooks → passthrough.
	if len(a.sendChatHook) > 0 {
		var send bool
		if msg, send = a.runSendChatHooks(msg, team); !send {
			return
		}
	}
	// Queue the actual server send through the rate-limited spam-safe buffer so a
	// burst cannot flood the server / trip its mute (§T65/§V37). The local echo
	// below stays immediate for responsiveness.
	a.sendBuf.enqueue(msg, team)
	// Echo our own line locally and immediately — some servers do not echo the
	// sender's own chat, and on 0.6 the echo carries an empty name (§V29/§B8).
	a.noteSent(msg)
	me := a.playerName
	if me == "" {
		me = "me"
	}
	prefix := ""
	if team {
		prefix = "[team] "
	}
	a.log.Addf(StyleChat, "%s[%s] %s", prefix, me, msg)
}

// noteSent records a just-sent chat line so the server's echo of it can be
// de-duplicated (§V29).
func (a *App) noteSent(msg string) {
	a.mu.Lock()
	a.sent = append(a.sent, sentChat{msg: msg, at: time.Now()})
	if len(a.sent) > 32 {
		a.sent = a.sent[len(a.sent)-32:]
	}
	a.mu.Unlock()
}

// IsOwnEcho reports whether an incoming chat line is the server echoing our own
// recently-sent message (so the caller can skip logging it twice, §V29). It
// matches only the local client id and consumes the record on a hit.
func (a *App) IsOwnEcho(clientID int, msg string) bool {
	c := a.cur().cli.Load()
	if c == nil || clientID != c.LocalID() {
		return false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if i := findRecentSent(a.sent, msg, time.Now()); i >= 0 {
		a.sent = append(a.sent[:i], a.sent[i+1:]...)
		return true
	}
	return false
}

// runLocal executes a local-console line (§T39). The console reads/writes cfg,
// so it holds cfgMu to stay consistent with off-main readers (§V4).
func (a *App) runLocal(line string) {
	// Feature-defined cvars (§T76) are get/set here before the static console,
	// so `cl_feature_var` and `cl_feature_var 1` work like any core cvar.
	if out, ok := a.tryDynVar(line); ok {
		for _, o := range out {
			a.log.Addf(StyleSystem, "] %s", o)
		}
		return
	}
	// Feature-defined console commands (§T92) run before the static console.
	if out, ok := a.tryFeatCmd(line); ok {
		for _, o := range out {
			a.log.Addf(StyleSystem, "] %s", o)
		}
		return
	}
	a.cfgMu.Lock()
	r := runConsole(line, a.cfg)
	a.cfgMu.Unlock()
	for _, o := range r.Out {
		a.log.Addf(StyleSystem, "] %s", o)
	}
	if r.Chat != "" {
		a.sendChat(r.Chat, false)
	}
	if r.Spectate {
		a.spectate(r.SpecName)
	}
	if r.Connect {
		a.doConnect(r.ConnectAddr, r.ConnectVer)
	}
	if r.Quit {
		a.Stop()
	}
}

// doConnect handles a `connect <addr> [ver]` console/config command (§T89/§T91).
// Before the dialer exists (config exec at startup) it queues the connect for
// Start; afterwards it joins immediately. Version comes from the command arg;
// OMITTED → packet.VersionAuto, so twclient probes the server and picks the
// protocol (prefers 0.6, §C23/§V87). Explicit "0.6"/"0.7" pins. No global flag.
func (a *App) doConnect(addr, ver string) {
	if addr == "" {
		return
	}
	v := packet.VersionAuto // omitted → twclient auto-detects (§V87)
	switch ver {
	case "0.6":
		v = packet.Version06
	case "0.7":
		v = packet.Version07
	}
	if a.dialer == nil {
		a.pendingAddr, a.pendingVer = addr, v
		return
	}
	a.Join(addr, v)
}

// ExecConfig runs a teeworlds-style config file through the console layer at
// startup (§T89): one `command [args]` per line, `#` comments. Identity/render
// cvars + a `connect` command are all just console lines.
func (a *App) ExecConfig(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		a.runLocal(line)
	}
	return nil
}

// Start applies the final config (identity + connect timeout), builds the client
// factory, and joins a server if the config requested one — otherwise it leaves
// the greeting/browser up (no auto-connect, §V51). Call after ExecConfig, before
// Run.
func (a *App) Start() {
	name := a.cfg.PlayerName
	if name == "" {
		name = "nameless tee"
	}
	a.SetName(name)
	a.cur().input.SetHold(time.Duration(a.cfg.InputHoldMs) * time.Millisecond) // §T110
	a.SetDialer(a.DefaultDialer(name, a.cfg.PlayerClan, "default"))
	if a.cfg.ConnectTimeout > 0 {
		a.SetConnectTimeout(time.Duration(a.cfg.ConnectTimeout) * time.Second)
	}
	if a.pendingAddr != "" {
		a.Join(a.pendingAddr, a.pendingVer)
	}
}

// spectate sets the spectated player by name, or free-view when name is empty
// (§T37). The name→id lookup uses the in-session roster.
func (a *App) spectate(name string) {
	id := -1
	if name != "" {
		if pid := a.findPlayer(name); pid >= 0 {
			id = pid
		} else {
			a.log.Addf(StyleSelf, "no player named %q", name)
			return
		}
	}
	a.do(client.ActSetSpectator{TargetID: id})
	if id < 0 {
		a.log.Addf(StyleSystem, "spectating (free view)")
	} else {
		a.log.Addf(StyleSystem, "spectating %s", name)
	}
}

// findPlayer returns the client id of the first roster player whose name matches
// (case-insensitive), or -1.
func (a *App) findPlayer(name string) int {
	c := a.cur().cli.Load()
	if c == nil {
		return -1
	}
	low := strings.ToLower(name)
	for _, p := range c.Roster() {
		if strings.ToLower(p.Name) == low {
			return p.ClientID
		}
	}
	return -1
}

// rconAuth logs in to rcon with the typed password (§T20). RconLogin blocks on
// the server's auth reply, so it runs off the event loop.
func (a *App) rconAuth(pw string) {
	c := a.cur().cli.Load()
	if c == nil {
		a.mode = modeNormal
		return
	}
	a.mode = modeRcon // allow typing commands while auth completes
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		if err := c.RconLogin(ctx, pw); err != nil {
			a.log.Addf(StyleSelf, "rcon auth failed: %v", err)
		} else {
			a.log.Addf(StyleSystem, "rcon authenticated")
		}
		a.wake()
	}()
}

func (a *App) rconSend(cmd string) {
	if cmd == "" {
		return
	}
	c := a.cur().cli.Load()
	if c == nil {
		return
	}
	if err := c.Rcon(cmd); err != nil {
		a.log.Addf(StyleSelf, "rcon: %v", err)
	}
}

// openBrowser switches to the browser overlay and fetches the master list
// asynchronously (§T18, list filled under lock §V4, §V13).
func (a *App) openBrowser() {
	a.mode = modeBrowser
	a.browser.SetLoading(true)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		entries, err := master.New().FetchServerList(ctx)
		if err != nil {
			a.browser.SetError(err.Error())
		} else {
			a.browser.SetEntries(entries)
		}
		a.wake()
	}()
}

// handleBrowser drives navigation/search/join in the browser overlay (§T32).
func (a *App) handleBrowser(ev *tcell.EventKey) {
	if a.browser.SearchFocused() {
		switch ev.Key() {
		case tcell.KeyEnter, tcell.KeyEscape:
			a.browser.FocusSearch(false)
		case tcell.KeyBackspace, tcell.KeyBackspace2:
			a.browser.SearchBackspace()
		default:
			if r := ev.Rune(); r != 0 {
				a.browser.SearchType(r)
			}
		}
		return
	}
	switch ev.Key() {
	case tcell.KeyEscape:
		a.mode = modeNormal
	case tcell.KeyUp:
		a.browser.Move(-1)
	case tcell.KeyDown:
		a.browser.Move(1)
	case tcell.KeyPgUp:
		a.browser.Move(-10)
	case tcell.KeyPgDn:
		a.browser.Move(10)
	case tcell.KeyLeft:
		a.browser.SetTab(-1)
		a.maybeScanLAN()
	case tcell.KeyRight:
		a.browser.SetTab(1)
		a.maybeScanLAN()
	case tcell.KeyEnter:
		if r, ok := a.browser.Selected(); ok {
			a.mode = modeNormal
			a.Join(r.Addr, r.Version)
		}
	default:
		switch ev.Rune() {
		case 'b', 'B':
			a.mode = modeNormal
		case '/':
			a.browser.FocusSearch(true)
		case 'f':
			if a.browser.ToggleFavorite() != "" {
				if p := a.favPath(); p != "" {
					_ = a.browser.SaveFavorites(p)
				}
			}
		case 'r':
			if a.browser.Tab() == tabLAN {
				a.maybeScanLAN()
			} else {
				a.openBrowser()
			}
		}
	}
}

// maybeScanLAN runs a real LAN broadcast scan when the LAN tab is active,
// discovering 0.6 and 0.7 servers on the local subnet via twclient's
// master.ScanLAN and mapping the results into the LAN source (§T51). Empty
// results clear the LAN tab gracefully.
func (a *App) maybeScanLAN() {
	if a.browser.Tab() != tabLAN {
		return
	}
	a.browser.SetLoading(true)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		servers, err := master.New().ScanLAN(ctx, master.WithScanTimeout(2500*time.Millisecond))
		if err != nil {
			a.browser.SetError(err.Error())
			a.wake()
			return
		}
		rows := make([]serverRow, 0, len(servers))
		for _, s := range servers {
			rows = append(rows, lanServerRow(s))
		}
		a.browser.SetLAN(rows)
		a.wake()
	}()
}

// Join closes the current session and connects to addr at ver, reusing the
// client factory so callbacks stay wired (§T18, §V1). On success it starts the
// twclient frontend loop (RunFrontends) which is what actually drives the
// Observer (render) and Controller (input) — without it nothing is dispatched
// (§V22, §B2). RunFrontends uses a long-lived context, distinct from the
// connect timeout.
// Join connects the PRIMARY session to addr at ver, dropping any dummies first
// (a fresh primary join resets to a single session, §T113).
func (a *App) Join(addr string, ver packet.Version) {
	a.sessMu.Lock()
	a.sessions = a.sessions[:1]
	a.active = 0
	primary := a.sessions[0]
	a.sessMu.Unlock()
	a.joinSession(primary, addr, ver, false)
}

// joinSession (re)connects session s to addr at ver. It starts the twclient
// frontend loop (RunFrontends) on success — without it nothing is dispatched
// (§V22/§B2). isDummy gates per-session vs primary status handling. Safe to call
// for any session; only OnDisconnect (captured in the dialer) routes back per
// session.
func (a *App) joinSession(s *session, addr string, ver packet.Version, isDummy bool) {
	if a.dialer == nil {
		return
	}
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	if old := s.cli.Load(); old != nil {
		_ = old.Close()
	}
	s.state.Clear() // fresh slate: ⊥ render the previous session's map (§T117/§V79)
	s.mapRecv.Store(0)
	s.mapTotal.Store(0) // reset map-download progress (§T128)
	s.connected.Store(false)
	s.joining.Store(true) // handshake in flight → show "connecting" (§B9)
	s.server = addr
	s.version = ver
	c := a.dialer(s, addr, ver)
	s.cli.Store(c)

	fctx, fcancel := context.WithCancel(context.Background())
	s.cancel = fcancel

	a.log.Addf(StyleSystem, "connecting to %s …", addr)
	go func() {
		// fctx is the SESSION lifetime: twclient binds the reader + keepalive + all
		// I/O to the ctx passed to Connect, so it MUST live as long as the session
		// (§V25, §B4). A watchdog cancels it ONLY while still connecting.
		stop := make(chan struct{})
		var timedOut atomic.Bool
		go func() {
			select {
			case <-time.After(a.connTimeout()):
				if !s.connected.Load() {
					timedOut.Store(true)
					fcancel() // abort a stuck handshake (does NOT cap a live session)
				}
			case <-stop:
			}
		}()
		if err := c.Connect(fctx); err != nil {
			close(stop)
			if timedOut.Load() {
				a.log.Addf(StyleSelf, "%s", connectTimeoutMsg(addr, ver, a.connTimeout()))
			} else {
				a.log.Addf(StyleSelf, "%s", connectFailMsg(addr, ver, err))
			}
			s.joining.Store(false) // handshake over (§B9)
			fcancel()
			if isDummy { // a failed dummy must not disturb the primary (§V76)
				a.dropSession(s)
			} else {
				a.log.Addf(StyleSystem, "press R to reconnect")
				a.reconnecting.Store(false)
			}
			a.wake()
			return
		}
		s.connected.Store(true)
		s.joining.Store(false)                           // handshake done (§B9)
		if rv := c.Version(); rv != packet.VersionAuto { // record resolved protocol (§V87)
			s.version = rv
		}
		if !isDummy {
			a.reconnecting.Store(false)
			a.reconnAttempt.Store(0) // a clean connection resets the attempt count
		}
		close(stop) // connected → watchdog must not cancel the live session
		a.log.Addf(StyleSystem, "connected.")
		go c.RunFrontends(fctx)      // drive observer (render) + controller (input)
		feature.FireConnect(a.api()) // notify feature modules (§T76)
		a.wake()
	}()
}

// DefaultConnectTimeout bounds the handshake (login + map download); after it
// the watchdog aborts a still-pending connect (§T52). It does NOT cap the live
// session (§V25). Generous by default so a real server's map download over a
// real network is not killed mid-handshake (§V28/§B7); override with
// -connect-timeout.
const DefaultConnectTimeout = 30 * time.Second

// connTimeout returns the configured handshake timeout, or the default.
func (a *App) connTimeout() time.Duration {
	if a.connectTimeout > 0 {
		return a.connectTimeout
	}
	return DefaultConnectTimeout
}

// SetConnectTimeout overrides the handshake timeout (0 = default).
func (a *App) SetConnectTimeout(d time.Duration) { a.connectTimeout = d }

// SetMaxFPS sets the render repaint cap (0 = unlimited), e.g. from the -max-fps
// flag (§T74). It may also be changed live via the cl_max_fps cvar.
func (a *App) SetMaxFPS(fps int) { a.cfg.MaxFPS = fps }

// SetLogLines sets the log-band row count shown when the visual is on (§T88),
// e.g. from -log-lines; clamped to ⌊h/2⌋ at render. Also settable live via the
// cl_log_lines cvar. Values < 1 fall back to the default.
func (a *App) SetLogLines(n int) {
	if n < 1 {
		n = DefaultLogLines
	}
	a.cfg.LogLines = n
}

// reconnect re-runs Join against the current server using the protocol version
// recorded by the last Join, so the user (R key) or an auto-reconnect after a
// drop can retry without re-typing flags (§T50/§V24/§T25). It bumps the attempt
// counter and flags the reconnecting state for the status bar; Join clears both
// on a terminal outcome.
func (a *App) reconnect() {
	if a.cur().server == "" {
		return
	}
	a.reconnAttempt.Add(1)
	a.reconnecting.Store(true)
	a.wake()
	a.Join(a.cur().server, a.cur().version)
}

func (a *App) prompt() string {
	switch a.mode {
	case modeChat:
		return "say: "
	case modeChatTeam:
		return "say (team): "
	case modeLocalCon:
		return "] "
	case modeRconAuth:
		return "rcon password: "
	case modeRcon:
		return "rcon] "
	default:
		return ""
	}
}

func (a *App) modeLabel() string {
	switch a.mode {
	case modeChat:
		return "CHAT"
	case modeChatTeam:
		return "TEAM CHAT"
	case modeLocalCon:
		return "LOCAL CONSOLE"
	case modeRconAuth, modeRcon:
		return "REMOTE CONSOLE"
	default:
		return "NORMAL"
	}
}

// present flushes the frame. Normally a cell-diff flush (scr.Show); when the
// viewport floor heartbeat asked for a complete repaint it does a full scr.Sync
// once, then clears the flag (§T130/§V90).
func (a *App) present() {
	if a.forceSync {
		a.forceSync = false
		a.scr.Sync()
		return
	}
	a.scr.Show()
}

func (a *App) draw() {
	a.scr.Clear()
	a.drawFrame++
	w, h := a.scr.Size()

	// Below the minimum usable size, degrade to a single resize notice rather
	// than garbling the layout; growing back restores the full UI (§V32/§C17).
	if tooSmall(w, h) {
		drawTooSmall(a.scr, w, h)
		a.present()
		return
	}

	if a.mode == modeBrowser {
		DrawBrowser(a.scr, w, h, a.browser)
		a.mu.Lock()
		popup := a.popup
		a.mu.Unlock()
		if popup.active() {
			drawPopup(a.scr, w, h, popup)
		}
		a.present()
		return
	}

	lay := Compute(w, h, a.visual, a.cfg.LogLines)
	st, have := a.cur().state.Get()

	// Visual on → game fills the body above the log band; off → the log band
	// fills the whole body (no game), so there is nothing to draw here (§C22).
	if a.visual && lay.Game.H > 0 {
		a.drawScene(lay.Game, st)
		if a.scoreboard && have {
			// Live registry (same source as chat/serverlog) — st.Roster can be
			// empty even with players (§B17/§V85).
			var roster []client.PlayerState
			if c := a.cur().cli.Load(); c != nil {
				roster = c.Roster()
			}
			DrawScoreboard(a.scr, lay.Game, roster, st.LocalID, a.nameStyle)
		}
		// While a join is in flight (no map/snapshot yet) show the indeterminate
		// connecting / map-download indicator over the game window (§T33). Only
		// when actually connecting — idle (never joined) shows no spinner (§B9).
		if !a.cur().connected.Load() && a.connecting() {
			line := connectingLine(a.drawFrame) // indeterminate spinner by default
			if p, ok := a.mapDownloadLine(); ok {
				line = p // real % once twclient reports map-download progress (§T128/§V88)
			}
			drawStr(a.scr, lay.Game.X, lay.Game.Y, lay.Game.W, StyleSystem, line)
		}
	}

	for i, ln := range a.log.View(lay.Log.W, lay.Log.H) {
		drawStr(a.scr, lay.Log.X, lay.Log.Y+i, lay.Log.W, ln.Style, ln.Text)
	}

	status := statusText(a.modeLabel(), a.cur().server, a.connStatus(), st, have)
	// last-ping readout now contributed by features/lastping via AddStatusField.
	for _, fn := range a.statusFields { // feature status contributions (§T76)
		if s := fn(); s != "" {
			status += "| " + s + " "
		}
	}
	for x := 0; x < lay.Status.W; x++ {
		a.scr.SetContent(x, lay.Status.Y, ' ', nil, StyleStatus)
	}
	drawStr(a.scr, lay.Status.X, lay.Status.Y, lay.Status.W, StyleStatus, status)

	a.drawInput(lay.Input)
	if a.help {
		drawHelp(a.scr, w, h, a.helpLines())
	}
	a.mu.Lock()
	popup := a.popup
	a.mu.Unlock()
	if popup.active() {
		drawPopup(a.scr, w, h, popup)
	}
	a.drawBroadcast(w, h) // transient server-broadcast overlay (§T121/§V82)
	a.drawEscMenu(w)      // top overlay action bar (§T111/§V74)
	a.present()
}

// drawScene renders the game view with a smoothed camera (§T43). It computes the
// raw camera target from the tick, eases it through the smoother, then draws
// around the eased center. When there is nothing to anchor on it resets the
// smoother and shows the connecting placeholder (so a reconnect snaps cleanly
// rather than sliding across the map).
func (a *App) drawScene(r Rect, st client.TickState) {
	tx, ty, ok := cameraCenter(st)
	if st.Map == nil || !ok {
		a.camera.reset()
		drawStr(a.scr, r.X, r.Y, r.W, StyleSystem, a.scenePlaceholder())
		return
	}
	cx, cy := a.camera.step(tx, ty, cameraAlpha)
	if a.freeLook { // pan offset detaches the view from the tee (§T94/§V54)
		cx += a.panX
		cy += a.panY
	}
	if a.subcell {
		drawGameHalfCentered(a.scr, r.X, r.Y, r.W, r.H, cx, cy, st)
	} else {
		drawGameCentered(a.scr, r.X, r.Y, r.W, r.H, cx, cy, st)
	}
	if a.freeLook { // indicator at top-left of the game rect (panned coords in HUD)
		drawStr(a.scr, r.X, r.Y, r.W, StyleSystem, " [free-look] WASD/arrows pan · Esc exit ")
	}
}

func (a *App) drawInput(r Rect) {
	switch {
	case a.search:
		line := "(reverse-i-search)`" + string(a.searchTerm) + "': " + a.searchHit
		drawStr(a.scr, r.X, r.Y, r.W, StyleChat, line)
	case a.mode != modeNormal:
		prompt := a.prompt()
		shown := a.line.String()
		if a.mode == modeRconAuth { // mask password
			shown = strings.Repeat("*", len([]rune(shown)))
		}
		drawStr(a.scr, r.X, r.Y, r.W, StyleChat, prompt+shown)
		a.scr.ShowCursor(r.X+len(prompt)+a.line.Cursor(), r.Y)
		// Grey inline completion preview after the cursor (§T15) — not while
		// masking a password.
		if a.mode != modeRconAuth {
			_, prefix := currentWord([]rune(a.line.String()), a.line.Cursor())
			ghost, list := completionPreview(prefix, a.completionCandidates())
			cx := r.X + len(prompt) + a.line.Cursor()
			if ghost != "" {
				cx += drawStr(a.scr, cx, r.Y, r.X+r.W-cx, StyleGhost, ghost)
			}
			if list != "" {
				drawStr(a.scr, cx, r.Y, r.X+r.W-cx, StyleGhost, list)
			}
			// Local-console help-text line: once the command word is known and
			// nothing is being completed, show its one-line help (§T39, ←
			// chillerbot help-text line).
			if a.mode == modeLocalCon && ghost == "" && list == "" {
				cmd, _, _ := strings.Cut(strings.TrimSpace(a.line.String()), " ")
				if h := consoleHelp(cmd); h != "" {
					drawStr(a.scr, cx, r.Y, r.X+r.W-cx, StyleGhost, "  "+h)
				}
			}
		}
	default:
		a.scr.HideCursor()
		// Generated, context-aware key legend (§T95/§V55): reflects the live
		// keymap + feature actions, truncated to the bar width.
		drawStr(a.scr, r.X, r.Y, r.W, StyleSystem, a.legendLine(r.W))
	}
}
