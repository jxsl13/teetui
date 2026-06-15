package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/jxsl13/teetui/extension"
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
	state          *State
	input          *InputController
	log            *Log
	server         string
	version        packet.Version // protocol of the current/last Join, for reconnect (§T50)
	connectTimeout time.Duration  // handshake timeout (0 = DefaultConnectTimeout, §T54)

	cli           atomic.Pointer[client.Client]
	connected     atomic.Bool
	reconnecting  atomic.Bool  // an auto-reconnect is in flight (§T25)
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
	camera     cameraSmoother // eases the rendered camera center (§T43)
	help       bool
	scoreboard bool
	hookOn     bool
	drawFrame  int // advances each redraw; drives the connecting spinner (§T33)

	browser        *Browser
	dialer         func(addr string, ver packet.Version) *client.Client
	frontendCancel context.CancelFunc // stops RunFrontends of the current session

	warlist    *Warlist
	keymap     *Keymap
	playerName string
	playerClan string     // our clan tag, for chat-query answers (§T62)
	cfg        *Config    // console-settable client config (§T39/§T40)
	cfgMu      sync.Mutex // guards a.cfg vs off-main readers (callbacks/loops, §V4)

	warlistPath  string       // path of the loaded warlist, for auto-reload (§T66)
	warlistMtime time.Time    // last-seen warlist mtime (reload goroutine only)
	limiter      frameLimiter // render repaint throttle (§T73), Run goroutine only

	tappedOutAt time.Time // last auto tapped-out reply, for rate limiting (§T40)
	autoReplyAt time.Time // last cl_auto_reply, for rate limiting (§T61)

	mu        sync.Mutex // guards popup + sent (callback goroutines)
	popup     Popup
	pings     *pingQueue  // last-16 pings, newest-first (§T63)
	pingCycle int         // H reply cursor into pings (0 = newest), guarded by mu
	sendBuf   *sendBuffer // rate-limited outgoing chat queue (§T65)
	sent      []sentChat  // recently sent chat, for own-echo dedupe (§V29)

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
	scr.EnableMouse()
	scr.Clear()
	return NewAppWithScreen(scr, server, state, input, log), nil
}

// NewAppWithScreen wires an app around an ALREADY-initialized tcell screen — a
// real one (NewApp) or a tcell.SimulationScreen. The simulation path lets the
// e2e harness drive the whole UI headlessly and assert the rendered cell buffer
// against a live server (§C14/§V23), exercising the same handlers main uses.
func NewAppWithScreen(scr tcell.Screen, server string, state *State, input *InputController, log *Log) *App {
	a := &App{
		scr:      scr,
		state:    state,
		input:    input,
		log:      log,
		server:   server,
		histAll:  NewHistory(64),
		histTeam: NewHistory(64),
		histCon:  NewHistory(64),
		histRcon: NewHistory(64),
		browser:  NewBrowser(),
		warlist:  NewWarlist(),
		keymap:   DefaultKeymap(),
		cfg:      NewConfig(),
		pings:    newPingQueue(16),                             // last-16 ping history (§T63)
		sendBuf:  newSendBuffer(sendMinInterval, sendQueueMax), // spam-safe send (§T65)
		visual:   true,
		popup:    greetingPopup(), // startup greeting (§T31)
		quit:     make(chan struct{}),
	}
	// Drive user OnTick hooks from the observer (§T70); guarded so there is zero
	// per-tick cost when no hooks are registered.
	a.state.SetTickHook(func(st client.TickState) {
		if extension.Count() > 0 {
			extension.FireTick(a.hookCtx(), st)
		}
	})
	a.loadHistory()
	if p, err := configDir(); err == nil {
		a.warlistPath = filepath.Join(p, "warlist.txt")
		_ = a.warlist.Load(a.warlistPath)
		if fi, err := os.Stat(a.warlistPath); err == nil {
			a.warlistMtime = fi.ModTime()
		}
		_ = a.browser.LoadFavorites(filepath.Join(p, "favorites.txt"))
		_ = a.keymap.Load(filepath.Join(p, "keymap.txt")) // rebindable keys (§V19/§T42)
		// Opt-in external command hooks: registered only if ~/.config/teetui/hooks
		// exists (§T71). Off by default.
		if h := newCmdHook(filepath.Join(p, "hooks")); h != nil {
			extension.Register("external-cmd-hooks", h)
		}
	}
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
func (a *App) SetClient(c *client.Client) { a.cli.Store(c) }

// Client returns the currently active client (may differ after a browser join).
func (a *App) Client() *client.Client { return a.cli.Load() }

// SetName records the local player name for ping detection (§T23).
func (a *App) SetName(name string) { a.playerName = name }

// NoteChat is called for every incoming chat line (from the twclient callback
// goroutine). If the line pings us it is remembered for the H auto-reply (§V4).
func (a *App) NoteChat(from, msg string) {
	if from == a.playerName || !containsName(msg, a.playerName) {
		return
	}
	a.pings.push(from, msg, time.Now())
	a.mu.Lock()
	a.pingCycle = 0 // a new ping resets the H reply cursor to newest
	a.mu.Unlock()
	a.maybeTappedOut()
	a.maybeAutoReply(from, msg)
}

// autoReplySpam sends a canned reply to a line hidden by cl_chat_spam_filter==2
// (hide+autoreply, §T64), rate-limited so a spam burst can't turn teetui into a
// flooder. It reuses the context-aware composer (§T61).
func (a *App) autoReplySpam(from, msg string) {
	if from == "" {
		return
	}
	a.mu.Lock()
	if time.Since(a.autoReplyAt) < tappedOutInterval {
		a.mu.Unlock()
		return
	}
	a.autoReplyAt = time.Now()
	a.mu.Unlock()
	reply, ok := composeReply(msg, from, a.playerName)
	if !ok {
		reply = from + " stop"
	}
	a.sendChat(reply, false)
}

// maybeAutoReply auto-answers a ping when cl_auto_reply is on (§T61), rate-
// limited like the tapped-out reply. It uses the cl_auto_reply_msg template
// (%n → author); tapped-out (if also on) already fired, so this is the general
// auto-responder. Off by default — teetui is interactive.
func (a *App) maybeAutoReply(from, _ string) {
	cs := a.cfgSnap()
	if !cs.AutoReply || from == "" {
		return
	}
	a.mu.Lock()
	if time.Since(a.autoReplyAt) < tappedOutInterval {
		a.mu.Unlock()
		return
	}
	a.autoReplyAt = time.Now()
	tmpl := cs.AutoReplyMsg
	a.mu.Unlock()
	if msg := expandAutoReply(tmpl, from); msg != "" {
		a.sendChat(msg, false)
	}
}

// tappedOutInterval rate-limits the auto tapped-out reply so a burst of pings
// does not spam the chat (§T40).
const tappedOutInterval = 30 * time.Second

// maybeTappedOut sends the configured tapped-out auto-reply when the feature is
// enabled and we were just pinged, at most once per tappedOutInterval (§T40).
// Off by default — teetui is interactive, not a headless AFK bot.
func (a *App) maybeTappedOut() {
	cs := a.cfgSnap()
	if !cs.TappedOut || cs.TappedOutText == "" {
		return
	}
	a.mu.Lock()
	if time.Since(a.tappedOutAt) < tappedOutInterval {
		a.mu.Unlock()
		return
	}
	a.tappedOutAt = time.Now()
	a.mu.Unlock()
	a.sendChat(cs.TappedOutText, false)
}

// SetDialer installs the factory used to (re)build a client when joining a
// server from the browser (§T18). main supplies it so callbacks stay wired.
func (a *App) SetDialer(fn func(addr string, ver packet.Version) *client.Client) { a.dialer = fn }

// SetConnected updates the status indicator.
func (a *App) SetConnected(b bool) { a.connected.Store(b) }

// ShowDisconnect raises the disconnect popup and kicks off an auto-reconnect to
// the same server, surfacing "reconnecting #N" in the status bar (§T25/§V11).
// Safe to call from a twclient callback goroutine (§V4); it wakes the render
// loop. A user-initiated quit suppresses the reconnect.
func (a *App) ShowDisconnect(reason string) {
	a.connected.Store(false)
	a.camera.reset() // next session snaps the camera, no slide across the map
	a.mu.Lock()
	a.popup = disconnectPopup(reason)
	a.mu.Unlock()
	a.wake()

	if extension.Count() > 0 {
		extension.FireDisconnect(a.hookCtx(), reason) // user hooks (§T70)
	}
	if a.quitting() {
		return
	}
	go a.reconnect()
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
func (a *App) connStatus() connStatus {
	return connStatus{
		connected:    a.connected.Load(),
		reconnecting: a.reconnecting.Load(),
		attempt:      int(a.reconnAttempt.Load()),
	}
}

// wake nudges the event loop so background state changes redraw promptly.
func (a *App) wake() { _ = a.scr.PostEvent(tcell.NewEventInterrupt(nil)) }

// Stop persists history + warlist, tears the screen down and unblocks Run.
func (a *App) Stop() {
	if a.frontendCancel != nil {
		a.frontendCancel()
	}
	a.saveHistory()
	if p, err := configDir(); err == nil {
		_ = a.warlist.Save(filepath.Join(p, "warlist.txt"))
	}
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
	c := a.cli.Load()
	if c == nil {
		return
	}
	if team {
		_ = c.Do(client.ActChat{Team: true, Msg: msg})
	} else {
		_ = c.SendChat(msg)
	}
}

// reloadWarlistLoop re-reads the warlist file when it changes on disk, every
// cl_war_list_auto_reload seconds, so external edits apply live (§T66). 0 = off.
func (a *App) reloadWarlistLoop() {
	t := time.NewTicker(time.Second)
	defer t.Stop()
	var last time.Time
	for {
		select {
		case <-a.quit:
			return
		case now := <-t.C:
			iv := a.cfgSnap().WarListReload
			if iv <= 0 || now.Sub(last) < time.Duration(iv)*time.Second {
				continue
			}
			last = now
			a.checkWarlistReload()
		}
	}
}

// checkWarlistReload reloads the warlist if its file mtime advanced (§T66).
func (a *App) checkWarlistReload() {
	if a.warlistPath == "" {
		return
	}
	fi, err := os.Stat(a.warlistPath)
	if err != nil {
		return
	}
	if fi.ModTime().After(a.warlistMtime) {
		if err := a.warlist.Load(a.warlistPath); err == nil {
			a.warlistMtime = fi.ModTime()
			a.log.Addf(StyleSystem, "warlist reloaded")
			a.wake()
		}
	}
}

// Run renders until quit. It returns after restoring the terminal.
func (a *App) Run() {
	defer a.scr.Fini()
	events := make(chan tcell.Event, 16)
	go a.scr.ChannelEvents(events, a.quit)
	go a.drainSends()        // pace outgoing chat (§T65)
	go a.reloadWarlistLoop() // live warlist reload (§T66)

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
func (a *App) Connected() bool { return a.connected.Load() }

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
	case *tcell.EventMouse:
		switch ev.Buttons() {
		case tcell.WheelUp:
			a.log.ScrollUp(1)
		case tcell.WheelDown:
			a.log.ScrollDown(1)
		}
	case *tcell.EventKey:
		// User key hooks get first refusal; a hook returning handled consumes the
		// key before teetui's own handling (§T70/§V39). No hooks → no-op.
		if extension.Count() > 0 && extension.FireKey(a.hookCtx(), keyToHook(ev)) {
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
	// Discrete named commands resolve through the rebindable keymap (§V19/§T42).
	if act, ok := a.keymap.Lookup(ev.Key(), ev.Rune()); ok {
		a.doAction(act)
		return
	}
	// Continuous/parametric controls stay direct: log scroll, weapon select
	// (1..6) and keyboard aim (arrows). These map a group of keys to one
	// parametric handler rather than a single named action.
	switch ev.Key() {
	case tcell.KeyPgUp:
		a.log.ScrollUp(10)
		return
	case tcell.KeyPgDn:
		a.log.ScrollDown(10)
		return
	case tcell.KeyUp:
		a.input.SetAim(0, -aimReach)
		return
	case tcell.KeyDown:
		a.input.SetAim(0, aimReach)
		return
	case tcell.KeyLeft:
		a.input.SetAim(-aimReach, 0)
		return
	case tcell.KeyRight:
		a.input.SetAim(aimReach, 0)
		return
	}
	if w, ok := weaponForRune(ev.Rune()); ok {
		a.input.SetWeapon(w)
	}
}

// doAction runs a keymap-resolved NORMAL-mode command. Centralizing the dispatch
// keeps behavior identical regardless of which key is bound to it (§T42).
func (a *App) doAction(act KeyAction) {
	switch act {
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
		if c := a.cli.Load(); c != nil && c.RconAuthed() {
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
		a.input.SetDirection(-1)
	case actMoveRight:
		a.input.SetDirection(1)
	case actMoveStop:
		a.input.SetDirection(0)
	case actJump:
		a.input.SetJump(true)
	case actHook:
		a.hookOn = !a.hookOn
		a.input.SetHook(a.hookOn)
	case actAutoReply:
		a.autoReplyPing()
	case actReconnect:
		a.reconnect()
	case actFire:
		a.input.Fire()
	}
}

// autoReplyPing replies to the most recent message that pinged us (§T23/§T61/
// §T62, bound to H). It first tries a state-derived query answer (war status,
// where, OS, …, §T62), then the canned context reply (§T61), then a friendly
// default — always addressed to the pinger.
func (a *App) autoReplyPing() {
	// Cycle through the recent-ping queue: each H press answers one older ping
	// (newest first), so repeated H walks back through pending pings (§T63).
	a.mu.Lock()
	i := a.pingCycle
	a.mu.Unlock()
	p, ok := a.pings.at(i)
	if !ok {
		a.log.Addf(StyleSystem, "no recent ping to reply to")
		a.mu.Lock()
		a.pingCycle = 0
		a.mu.Unlock()
		return
	}
	a.mu.Lock()
	a.pingCycle = i + 1 // next H replies the next-older ping
	a.mu.Unlock()
	from, msg := p.from, p.msg
	if reply, ok := composeQueryReply(msg, from, a.queryEnv()); ok {
		a.sendChat(reply, false)
		return
	}
	reply, ok := composeReply(msg, from, a.playerName)
	if !ok {
		reply = from + " hi" // nothing matched → a friendly default
	}
	a.sendChat(reply, false)
}

// queryEnv snapshots the read-only state a chat-query answer may use (§T62/§V34).
func (a *App) queryEnv() queryEnv {
	env := queryEnv{
		warlist:  a.warlist,
		selfClan: a.playerClan,
		goos:     runtime.GOOS,
	}
	if c := a.cli.Load(); c != nil {
		for _, p := range c.Roster() {
			if p.Name != "" {
				env.rosterNames = append(env.rosterNames, p.Name)
			}
		}
	}
	if st, ok := a.state.Get(); ok {
		if cx, cy, has := cameraCenter(st); has {
			env.haveCoords, env.coordX, env.coordY = true, cx, cy
		}
	}
	return env
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
	c := a.cli.Load()
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
	if c := a.cli.Load(); c != nil {
		_ = c.Do(act)
	}
}

// chatLine submits a chat line, intercepting warlist "!" commands first. With
// cl_silent_chat_commands a handled command is applied locally and not sent to
// the server (§T22, §V14).
func (a *App) chatLine(text string, team bool) {
	if res := parseChatCommand(text, a.warlist); res.Handled {
		for _, l := range res.Reply {
			a.log.Addf(StyleSystem, "! %s", l)
		}
		if !a.cfg.SilentChatCmds {
			a.sendChat(text, team)
		}
		return
	}
	a.sendChat(text, team)
}

func (a *App) sendChat(msg string, team bool) {
	if msg == "" {
		return
	}
	c := a.cli.Load()
	if c == nil {
		return
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
	c := a.cli.Load()
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
	if r.Quit {
		a.Stop()
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
	c := a.cli.Load()
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
	c := a.cli.Load()
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
	c := a.cli.Load()
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
func (a *App) Join(addr string, ver packet.Version) {
	if a.dialer == nil {
		return
	}
	if a.frontendCancel != nil {
		a.frontendCancel()
		a.frontendCancel = nil
	}
	if old := a.cli.Load(); old != nil {
		_ = old.Close()
	}
	a.connected.Store(false)
	a.server = addr
	a.version = ver
	c := a.dialer(addr, ver)
	a.SetClient(c)

	fctx, fcancel := context.WithCancel(context.Background())
	a.frontendCancel = fcancel

	a.log.Addf(StyleSystem, "connecting to %s …", addr)
	go func() {
		// fctx is the SESSION lifetime: twclient binds the reader + keepalive +
		// all I/O to the ctx passed to Connect, so it MUST live as long as the
		// session — cancelling it tears the connection down and the server times
		// us out (§V25, §B4). The handshake is bounded by a watchdog that cancels
		// fctx ONLY while still connecting; once connected it never fires.
		stop := make(chan struct{})
		var timedOut atomic.Bool
		go func() {
			select {
			case <-time.After(a.connTimeout()):
				if !a.connected.Load() {
					timedOut.Store(true)
					fcancel() // abort a stuck handshake (does NOT cap a live session)
				}
			case <-stop:
			}
		}()
		if err := c.Connect(fctx); err != nil {
			close(stop)
			// Distinguish "we gave up after the timeout" from a real protocol
			// error, so a slow-but-reachable server reads as a retryable timeout
			// rather than a scary raw "context canceled" (§V28/§B7).
			if timedOut.Load() {
				a.log.Addf(StyleSelf, "%s", connectTimeoutMsg(addr, ver, a.connTimeout()))
			} else {
				a.log.Addf(StyleSelf, "%s", connectFailMsg(addr, ver, err))
			}
			a.log.Addf(StyleSystem, "press R to reconnect")
			a.reconnecting.Store(false) // attempt finished (failed) — stop the spinner
			fcancel()
			a.wake()
			return
		}
		a.SetConnected(true)
		a.reconnecting.Store(false)
		a.reconnAttempt.Store(0) // a clean connection resets the attempt count
		close(stop)              // connected → watchdog must not cancel the live session
		a.log.Addf(StyleSystem, "connected.")
		go c.RunFrontends(fctx) // drive observer (render) + controller (input)
		go a.chillpwLogin(addr) // opt-in rcon auto-login (§T68)
		if extension.Count() > 0 {
			extension.FireConnect(a.hookCtx()) // user hooks (§T70)
		}
		a.wake()
	}()
}

// chillpwLogin performs an opt-in rcon auto-login using a password from the
// secrets file matched to the server addr (§T68/§V38). The secret is NEVER
// logged — only the fact of an attempt and its success/failure. Off unless
// cl_chillpw is set and the file holds an entry for this server.
func (a *App) chillpwLogin(addr string) {
	cs := a.cfgSnap()
	if !cs.Chillpw {
		return
	}
	path := cs.PasswordFile
	if path == "" {
		return
	}
	if !filepath.IsAbs(path) {
		if dir, err := configDir(); err == nil {
			path = filepath.Join(dir, path)
		}
	}
	m, err := loadPasswords(path)
	if err != nil {
		a.log.Addf(StyleSelf, "chillpw: cannot read secrets file")
		return
	}
	pw, ok := lookupPassword(m, addr)
	if !ok {
		return // no secret for this server — nothing to do
	}
	c := a.cli.Load()
	if c == nil {
		return
	}
	a.log.Addf(StyleSystem, "chillpw: rcon auto-login for %s", addr)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if err := c.RconLogin(ctx, pw); err != nil {
		a.log.Addf(StyleSelf, "chillpw: rcon login failed") // never log the secret
		return
	}
	a.log.Addf(StyleSystem, "chillpw: rcon authenticated")
	a.wake()
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

// reconnect re-runs Join against the current server using the protocol version
// recorded by the last Join, so the user (R key) or an auto-reconnect after a
// drop can retry without re-typing flags (§T50/§V24/§T25). It bumps the attempt
// counter and flags the reconnecting state for the status bar; Join clears both
// on a terminal outcome.
func (a *App) reconnect() {
	if a.server == "" {
		return
	}
	a.reconnAttempt.Add(1)
	a.reconnecting.Store(true)
	a.wake()
	a.Join(a.server, a.version)
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

func (a *App) draw() {
	a.scr.Clear()
	a.drawFrame++
	w, h := a.scr.Size()

	// Below the minimum usable size, degrade to a single resize notice rather
	// than garbling the layout; growing back restores the full UI (§V32/§C17).
	if tooSmall(w, h) {
		drawTooSmall(a.scr, w, h)
		a.scr.Show()
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
		a.scr.Show()
		return
	}

	lay := Compute(w, h)
	st, have := a.state.Get()

	if a.visual {
		a.drawScene(lay.Game, st)
		if a.scoreboard && have {
			DrawScoreboard(a.scr, lay.Game, st, a.warlist)
		}
	} else {
		drawStr(a.scr, lay.Game.X, lay.Game.Y, lay.Game.W, StyleSystem, "[visual off — press v]")
	}

	// While a join is in flight (no map/snapshot yet) show the indeterminate
	// connecting / map-download indicator over the top of the game window (§T33).
	if !a.connected.Load() {
		drawStr(a.scr, lay.Game.X, lay.Game.Y, lay.Game.W, StyleSystem, connectingLine(a.drawFrame))
	}

	for i, ln := range a.log.View(lay.Log.H) {
		drawStr(a.scr, lay.Log.X, lay.Log.Y+i, lay.Log.W, ln.Style, ln.Text)
	}

	status := statusText(a.modeLabel(), a.server, a.connStatus(), st, have)
	if a.cfg.ShowLastPing { // optional last-ping readout (§T63)
		if p, ok := a.pings.newest(); ok {
			status += fmt.Sprintf("| ping %s: %s ", p.from, p.msg)
		}
	}
	for x := 0; x < lay.Status.W; x++ {
		a.scr.SetContent(x, lay.Status.Y, ' ', nil, StyleStatus)
	}
	drawStr(a.scr, lay.Status.X, lay.Status.Y, lay.Status.W, StyleStatus, status)

	a.drawInput(lay.Input)
	if a.help {
		drawHelp(a.scr, w, h)
	}
	a.mu.Lock()
	popup := a.popup
	a.mu.Unlock()
	if popup.active() {
		drawPopup(a.scr, w, h, popup)
	}
	a.scr.Show()
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
		drawStr(a.scr, r.X, r.Y, r.W, StyleSystem, "connecting…")
		return
	}
	cx, cy := a.camera.step(tx, ty, cameraAlpha)
	if a.subcell {
		drawGameHalfCentered(a.scr, r.X, r.Y, r.W, r.H, cx, cy, st)
	} else {
		drawGameCentered(a.scr, r.X, r.Y, r.W, r.H, cx, cy, st)
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
		drawStr(a.scr, r.X, r.Y, r.W, StyleSystem,
			" [t]chat [B]browser [F1/F2]console [v]visual [V]detail [k]kill [1-6/f]weapon [R]reconnect [Tab]board [?]help [q]quit ")
	}
}
