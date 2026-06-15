package tui

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gdamore/tcell/v2"
)

// Relation is the warlist standing assigned to a player name (← chillerbot
// warlist: war / peace / team).
type Relation int

const (
	RelNeutral Relation = iota
	RelWar
	RelPeace
	RelTeam
)

// relationName maps a relation to its persisted/command token.
func relationName(r Relation) string {
	switch r {
	case RelWar:
		return "war"
	case RelPeace:
		return "peace"
	case RelTeam:
		return "team"
	default:
		return "neutral"
	}
}

func relationFromName(s string) (Relation, bool) {
	switch s {
	case "war":
		return RelWar, true
	case "peace":
		return RelPeace, true
	case "team":
		return RelTeam, true
	case "neutral":
		return RelNeutral, true
	}
	return RelNeutral, false
}

// RelationStyle returns the nameplate/scoreboard color for a relation.
func RelationStyle(r Relation) (tcell.Style, bool) {
	switch r {
	case RelWar:
		return fg(255, 80, 80), true // red
	case RelPeace:
		return fg(80, 255, 80), true // green
	case RelTeam:
		return fg(0, 200, 255), true // cyan
	default:
		return tcell.StyleDefault, false
	}
}

// Warlist is the war/peace/team store. In the advanced mode (§T24,
// warlist_commands_advanced.cpp) a per-name relation may carry a war reason, and
// a relation may also be assigned to a whole clan tag — a clan relation tints
// every player wearing that tag unless a per-name relation overrides it. The
// same store is mutated by chat commands and read by the scoreboard renderer, so
// access is locked (§V4, §V14). Persisted to ~/.config/teetui/warlist.txt.
type Warlist struct {
	mu      sync.Mutex
	rel     map[string]Relation // per-name relation (non-neutral only)
	reasons map[string]string   // per-name war reason (optional)
	clanRel map[string]Relation // per-clan-tag relation (non-neutral only)
}

// NewWarlist returns an empty warlist.
func NewWarlist() *Warlist {
	return &Warlist{
		rel:     map[string]Relation{},
		reasons: map[string]string{},
		clanRel: map[string]Relation{},
	}
}

// Set assigns a relation to name (RelNeutral removes it, along with any reason).
func (w *Warlist) Set(name string, r Relation) {
	if name == "" {
		return
	}
	w.mu.Lock()
	if r == RelNeutral {
		delete(w.rel, name)
		delete(w.reasons, name)
	} else {
		w.rel[name] = r
	}
	w.mu.Unlock()
}

// Del clears any relation (and reason) for name.
func (w *Warlist) Del(name string) { w.Set(name, RelNeutral) }

// Get returns the relation for name (RelNeutral if unset).
func (w *Warlist) Get(name string) Relation {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.rel[name]
}

// SetReason records a war reason for name (empty clears it).
func (w *Warlist) SetReason(name, reason string) {
	if name == "" {
		return
	}
	w.mu.Lock()
	if reason == "" {
		delete(w.reasons, name)
	} else {
		w.reasons[name] = reason
	}
	w.mu.Unlock()
}

// Reason returns the war reason recorded for name ("" if none).
func (w *Warlist) Reason(name string) string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.reasons[name]
}

// SetClan assigns a relation to a clan tag (RelNeutral removes it).
func (w *Warlist) SetClan(clan string, r Relation) {
	if clan == "" {
		return
	}
	w.mu.Lock()
	if r == RelNeutral {
		delete(w.clanRel, clan)
	} else {
		w.clanRel[clan] = r
	}
	w.mu.Unlock()
}

// ClanRel returns the relation assigned to a clan tag (RelNeutral if unset).
func (w *Warlist) ClanRel(clan string) Relation {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.clanRel[clan]
}

// Effective resolves the relation that applies to a player: a per-name relation
// wins, otherwise the relation of the player's clan tag is used (§T24/§V14).
func (w *Warlist) Effective(name, clan string) Relation {
	w.mu.Lock()
	defer w.mu.Unlock()
	if r := w.rel[name]; r != RelNeutral {
		return r
	}
	if clan != "" {
		return w.clanRel[clan]
	}
	return RelNeutral
}

// Style returns the color for a name, if it has a per-name relation.
func (w *Warlist) Style(name string) (tcell.Style, bool) {
	return RelationStyle(w.Get(name))
}

// EffectiveStyle returns the color for a player given their name and clan,
// applying the name-then-clan precedence so clan wars tint players too (§V14).
func (w *Warlist) EffectiveStyle(name, clan string) (tcell.Style, bool) {
	return RelationStyle(w.Effective(name, clan))
}

// Load reads the warlist file. Three back-compatible line shapes are accepted:
//
//	<rel>\t<name>            simple per-name relation (legacy §T21)
//	<rel>\t<name>\t<reason>  per-name relation with a war reason (§T24)
//	clan\t<rel>\t<clantag>   per-clan-tag relation (§T24)
//
// A missing file is not an error.
func (w *Warlist) Load(path string) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()
	rel := map[string]Relation{}
	reasons := map[string]string{}
	clanRel := map[string]Relation{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Split(sc.Text(), "\t")
		if len(fields) >= 3 && fields[0] == "clan" {
			// clan<TAB>rel<TAB>clantag
			if r, ok := relationFromName(fields[1]); ok && r != RelNeutral && fields[2] != "" {
				clanRel[fields[2]] = r
			}
			continue
		}
		if len(fields) < 2 || fields[1] == "" {
			continue
		}
		r, ok := relationFromName(fields[0])
		if !ok || r == RelNeutral {
			continue
		}
		name := fields[1]
		rel[name] = r
		if len(fields) >= 3 && fields[2] != "" {
			reasons[name] = fields[2]
		}
	}
	if err := sc.Err(); err != nil {
		return err
	}
	w.mu.Lock()
	w.rel = rel
	w.reasons = reasons
	w.clanRel = clanRel
	w.mu.Unlock()
	return nil
}

// Save writes the warlist to path, creating parent dirs. Per-name relations are
// written with their reason when set; clan relations follow on "clan" lines.
func (w *Warlist) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	w.mu.Lock()
	var b strings.Builder
	for name, r := range w.rel {
		b.WriteString(relationName(r))
		b.WriteByte('\t')
		b.WriteString(name)
		if reason := w.reasons[name]; reason != "" {
			b.WriteByte('\t')
			b.WriteString(reason)
		}
		b.WriteByte('\n')
	}
	for clan, r := range w.clanRel {
		b.WriteString("clan\t")
		b.WriteString(relationName(r))
		b.WriteByte('\t')
		b.WriteString(clan)
		b.WriteByte('\n')
	}
	w.mu.Unlock()
	return os.WriteFile(path, []byte(b.String()), 0o644)
}
