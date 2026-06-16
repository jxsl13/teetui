package tui

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gdamore/tcell/v2"
)

// KeyAction is a NORMAL-mode command the keymap can bind a key to. Routing keys
// through the keymap (rather than hardcoded runes) is what makes the bindings
// rebindable via config — exceeding the reference, which cannot rebind at all
// (§V19, §C12, §T42; reconciles the foundation keys to the §I table, §T41).
type KeyAction int

const (
	actNone KeyAction = iota
	actHelp
	actQuit
	actBrowser
	actChat
	actTeamChat
	actLocalConsole
	actRemoteConsole
	actVisual
	actKill
	actEmote
	actScoreboard
	actReconnect
	actVoteYes
	actVoteNo
	actHook
	actMoveLeft
	actMoveRight
	actMoveStop
	actJump
	actFire
	actSubcellToggle
	actFreeLook
)

// actionOrder is the canonical iteration order for deterministic Save output.
var actionOrder = []KeyAction{
	actHelp, actQuit, actBrowser, actChat, actTeamChat, actLocalConsole,
	actRemoteConsole, actVisual, actKill, actEmote, actScoreboard,
	actReconnect, actVoteYes, actVoteNo, actHook, actMoveLeft, actMoveRight,
	actMoveStop, actJump, actFire, actSubcellToggle, actFreeLook,
}

// actionNames is the persisted token for each action.
var actionNames = map[KeyAction]string{
	actHelp: "help", actQuit: "quit", actBrowser: "browser", actChat: "chat",
	actTeamChat: "team_chat", actLocalConsole: "local_console",
	actRemoteConsole: "remote_console", actVisual: "visual", actKill: "kill",
	actEmote: "emote", actScoreboard: "scoreboard",
	actReconnect: "reconnect", actVoteYes: "vote_yes", actVoteNo: "vote_no",
	actHook: "hook", actMoveLeft: "move_left", actMoveRight: "move_right",
	actMoveStop: "move_stop", actJump: "jump", actFire: "fire",
	actSubcellToggle: "subcell", actFreeLook: "free_look",
}

// actionByName is the reverse of actionNames.
var actionByName = func() map[string]KeyAction {
	m := make(map[string]KeyAction, len(actionNames))
	for a, n := range actionNames {
		m[n] = a
	}
	return m
}()

// namedKeys maps the non-rune tcell keys we bind to a config token, with the
// reverse for parsing. Runes are tokenized by their character (or "Space").
var namedKeys = map[tcell.Key]string{
	tcell.KeyF1: "F1", tcell.KeyF2: "F2", tcell.KeyF5: "F5", tcell.KeyF6: "F6",
	tcell.KeyTab: "Tab", tcell.KeyEscape: "Esc", tcell.KeyCtrlC: "Ctrl-C",
	tcell.KeyUp: "Up", tcell.KeyDown: "Down", tcell.KeyLeft: "Left", tcell.KeyRight: "Right",
}

var keyByName = func() map[string]tcell.Key {
	m := make(map[string]tcell.Key, len(namedKeys))
	for k, n := range namedKeys {
		m[n] = k
	}
	return m
}()

// Keymap maps keys to NORMAL-mode actions. Both rune keys and special tcell keys
// are supported; several keys may map to one action (e.g. the foundation 't' and
// the §I 'T' both open chat). Lookups are O(1) (§V19).
type Keymap struct {
	runes map[rune]KeyAction
	keys  map[tcell.Key]KeyAction
}

// DefaultKeymap returns the built-in bindings: the §I key-binding table, kept
// backward-compatible with the foundation lowercase keys so existing behavior is
// unchanged (§T41/§T42). A user config overrides these (Load).
func DefaultKeymap() *Keymap {
	k := &Keymap{runes: map[rune]KeyAction{}, keys: map[tcell.Key]KeyAction{}}
	// §I table + foundation aliases.
	k.bindRune('?', actHelp)
	k.bindRune('q', actQuit)
	k.bindKey(tcell.KeyEscape, actQuit)
	k.bindKey(tcell.KeyCtrlC, actQuit)
	k.bindRune('b', actBrowser)
	k.bindRune('B', actBrowser)
	k.bindRune('t', actChat)
	k.bindRune('T', actChat)
	k.bindRune('y', actTeamChat) // foundation
	k.bindRune('Z', actTeamChat) // §I
	k.bindKey(tcell.KeyF1, actLocalConsole)
	k.bindKey(tcell.KeyF2, actRemoteConsole)
	k.bindRune('v', actVisual)
	k.bindRune('V', actSubcellToggle) // §T46 half-block sub-cell render toggle
	k.bindRune('k', actKill)
	k.bindRune('K', actKill)
	k.bindRune('e', actEmote)
	k.bindKey(tcell.KeyTab, actScoreboard)
	k.bindRune('R', actReconnect)
	k.bindKey(tcell.KeyF5, actVoteYes)
	k.bindKey(tcell.KeyF6, actVoteNo)
	k.bindRune('h', actHook)
	// Movement (WASD/arrows) is routed by the cl_move_keys cvar (§T104), not the
	// static keymap — so left-/right-handed players swap move↔aim. Space still
	// jumps via the keymap.
	k.bindRune(' ', actJump)
	k.bindRune('f', actFire)
	k.bindRune('G', actFreeLook) // §T94 free-look map-pan toggle (rebindable)
	return k
}

func (k *Keymap) bindRune(r rune, a KeyAction) { k.runes[r] = a }
func (k *Keymap) bindKey(key tcell.Key, a KeyAction) {
	if key == tcell.KeyRune {
		return
	}
	k.keys[key] = a
}

// clearAction removes every key currently bound to a.
func (k *Keymap) clearAction(a KeyAction) {
	for r, act := range k.runes {
		if act == a {
			delete(k.runes, r)
		}
	}
	for key, act := range k.keys {
		if act == a {
			delete(k.keys, key)
		}
	}
}

// Lookup resolves a key event to an action. Non-rune special keys match the key
// table; rune events match the rune table.
func (k *Keymap) Lookup(key tcell.Key, r rune) (KeyAction, bool) {
	if key == tcell.KeyRune {
		a, ok := k.runes[r]
		return a, ok
	}
	a, ok := k.keys[key]
	return a, ok
}

// keyToken renders a rune as its config token ("Space" for the literal space).
func runeToken(r rune) string {
	if r == ' ' {
		return "Space"
	}
	return string(r)
}

// tokenToBind parses a config key token into a rune or a named key.
func tokenToBind(tok string) (isRune bool, r rune, key tcell.Key, ok bool) {
	if tok == "Space" {
		return true, ' ', 0, true
	}
	if key, found := keyByName[tok]; found {
		return false, 0, key, true
	}
	rs := []rune(tok)
	if len(rs) == 1 {
		return true, rs[0], 0, true
	}
	return false, 0, 0, false
}

// tokensFor returns the sorted config tokens bound to an action (deterministic).
func (k *Keymap) tokensFor(a KeyAction) []string {
	var toks []string
	for r, act := range k.runes {
		if act == a {
			toks = append(toks, runeToken(r))
		}
	}
	for key, act := range k.keys {
		if act == a {
			toks = append(toks, namedKeys[key])
		}
	}
	sort.Strings(toks)
	return toks
}

// Load replaces the keymap with the defaults, then applies overrides from path.
// A missing file leaves the defaults intact (§V19). Each "action = key" line
// rebinds an action: the first line seen for an action clears its default keys,
// so the new binding fully replaces the old one; additional lines add aliases.
func (k *Keymap) Load(path string) error {
	*k = *DefaultKeymap()
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()

	cleared := map[KeyAction]bool{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		name, tok, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		act, ok := actionByName[strings.TrimSpace(name)]
		if !ok {
			continue
		}
		isRune, r, key, ok := tokenToBind(strings.TrimSpace(tok))
		if !ok {
			continue
		}
		if !cleared[act] {
			k.clearAction(act)
			cleared[act] = true
		}
		if isRune {
			k.bindRune(r, act)
		} else {
			k.bindKey(key, act)
		}
	}
	return sc.Err()
}

// Save writes the keymap as "action = key" lines (one per bound key), creating
// parent dirs. Output order is deterministic for stable round-trips (§V19).
func (k *Keymap) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var b strings.Builder
	for _, a := range actionOrder {
		for _, tok := range k.tokensFor(a) {
			b.WriteString(actionNames[a])
			b.WriteString(" = ")
			b.WriteString(tok)
			b.WriteByte('\n')
		}
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}
