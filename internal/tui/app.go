package tui

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gdamore/tcell/v2"
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
	scr     tcell.Screen
	state   *State
	input   *InputController
	log     *Log
	server  string
	version packet.Version // protocol of the current/last Join, for reconnect (§T50)

	cli       atomic.Pointer[client.Client]
	connected atomic.Bool

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
	help       bool
	scoreboard bool
	hookOn     bool
	drawFrame  int // advances each redraw; drives the connecting spinner (§T33)

	browser        *Browser
	dialer         func(addr string, ver packet.Version) *client.Client
	frontendCancel context.CancelFunc // stops RunFrontends of the current session

	warlist        *Warlist
	keymap         *Keymap
	playerName     string
	silentChatCmds bool

	mu       sync.Mutex // guards popup + ping (written from callback goroutines)
	popup    Popup
	pingFrom string
	pingMsg  string
	pingAt   time.Time

	quit chan struct{}
}

// NewApp wires the app to its shared state, input controller and log, and loads
// persisted input history from disk (§V16).
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

	a := &App{
		scr:            scr,
		state:          state,
		input:          input,
		log:            log,
		server:         server,
		histAll:        NewHistory(64),
		histTeam:       NewHistory(64),
		histCon:        NewHistory(64),
		histRcon:       NewHistory(64),
		browser:        NewBrowser(),
		warlist:        NewWarlist(),
		keymap:         DefaultKeymap(),
		silentChatCmds: true, // cl_silent_chat_commands default on (§V14)
		visual:         true,
		popup:          greetingPopup(), // startup greeting (§T31)
		quit:           make(chan struct{}),
	}
	a.loadHistory()
	if p, err := configDir(); err == nil {
		_ = a.warlist.Load(filepath.Join(p, "warlist.txt"))
		_ = a.browser.LoadFavorites(filepath.Join(p, "favorites.txt"))
		_ = a.keymap.Load(filepath.Join(p, "keymap.txt")) // rebindable keys (§V19/§T42)
	}
	return a, nil
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
	a.mu.Lock()
	a.pingFrom, a.pingMsg, a.pingAt = from, msg, time.Now()
	a.mu.Unlock()
}

// SetDialer installs the factory used to (re)build a client when joining a
// server from the browser (§T18). main supplies it so callbacks stay wired.
func (a *App) SetDialer(fn func(addr string, ver packet.Version) *client.Client) { a.dialer = fn }

// SetConnected updates the status indicator.
func (a *App) SetConnected(b bool) { a.connected.Store(b) }

// ShowDisconnect raises the disconnect popup. Safe to call from a twclient
// callback goroutine (§V4); it wakes the render loop (§T19/§T25).
func (a *App) ShowDisconnect(reason string) {
	a.connected.Store(false)
	a.mu.Lock()
	a.popup = disconnectPopup(reason)
	a.mu.Unlock()
	a.wake()
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

// Run renders until quit. It returns after restoring the terminal.
func (a *App) Run() {
	defer a.scr.Fini()
	events := make(chan tcell.Event, 16)
	go a.scr.ChannelEvents(events, a.quit)

	a.draw()
	for {
		select {
		case <-a.quit:
			return
		case ev, ok := <-events:
			if !ok {
				return
			}
			a.handle(ev)
			a.draw()
		}
	}
}

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

// autoReplyPing replies to the most recent message that pinged us (§T23/§T40,
// bound to H). It uses the canned known-phrase table, falling back to a greeting
// addressed to the pinger.
func (a *App) autoReplyPing() {
	a.mu.Lock()
	from, msg, at := a.pingFrom, a.pingMsg, a.pingAt
	a.mu.Unlock()
	if from == "" || time.Since(at) > 2*time.Minute {
		a.log.Addf(StyleSystem, "no recent ping to reply to")
		return
	}
	reply, ok := autoReply(msg)
	if !ok {
		reply = "hi"
	}
	a.sendChat(reply+" "+from, false)
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
	cur := a.line.Cursor()
	if cur > len(runes) {
		cur = len(runes)
	}
	start := cur
	for start > 0 && runes[start-1] != ' ' {
		start--
	}
	if a.compActive && start == a.compStart && len(a.compMatches) > 0 {
		a.compIdx = (a.compIdx + 1) % len(a.compMatches)
	} else {
		prefix := string(runes[start:cur])
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
		if !a.silentChatCmds {
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
	if team {
		_ = c.Do(client.ActChat{Team: true, Msg: msg})
	} else {
		_ = c.SendChat(msg)
	}
}

// runLocal executes a local-console line (§T39).
func (a *App) runLocal(line string) {
	r := runConsole(line)
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
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()
		if err := c.Connect(ctx); err != nil {
			a.log.Addf(StyleSelf, "%s", connectFailMsg(addr, ver, err))
			a.log.Addf(StyleSystem, "press R to reconnect")
			fcancel()
			a.wake()
			return
		}
		a.SetConnected(true)
		a.log.Addf(StyleSystem, "connected.")
		go c.RunFrontends(fctx) // drive observer (render) + controller (input)
		a.wake()
	}()
}

// reconnect re-runs Join against the current server using the protocol version
// recorded by the last Join, so the user can retry after a connect failure
// without re-typing flags (§T50/§V24).
func (a *App) reconnect() {
	if a.server == "" {
		return
	}
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
		DrawGame(a.scr, lay.Game.X, lay.Game.Y, lay.Game.W, lay.Game.H, st)
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

	status := statusText(a.modeLabel(), a.server, a.connected.Load(), st, have)
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
	default:
		a.scr.HideCursor()
		drawStr(a.scr, r.X, r.Y, r.W, StyleSystem,
			" [t]chat [B]browser [F1/F2]console [v]visual [k]kill [1-6/f]weapon [R]reconnect [Tab]board [?]help [q]quit ")
	}
}
