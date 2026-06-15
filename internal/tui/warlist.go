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

// Warlist is the simple war/peace/team store keyed by player name. The same
// store is mutated by chat commands and read by the scoreboard renderer, so
// access is locked (§V4, §V14). Persisted to ~/.config/teetui/warlist.txt.
type Warlist struct {
	mu  sync.Mutex
	rel map[string]Relation
}

// NewWarlist returns an empty warlist.
func NewWarlist() *Warlist { return &Warlist{rel: map[string]Relation{}} }

// Set assigns a relation to name (RelNeutral removes it).
func (w *Warlist) Set(name string, r Relation) {
	if name == "" {
		return
	}
	w.mu.Lock()
	if r == RelNeutral {
		delete(w.rel, name)
	} else {
		w.rel[name] = r
	}
	w.mu.Unlock()
}

// Del clears any relation for name.
func (w *Warlist) Del(name string) { w.Set(name, RelNeutral) }

// Get returns the relation for name (RelNeutral if unset).
func (w *Warlist) Get(name string) Relation {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.rel[name]
}

// Style returns the color for a name, if it has a relation.
func (w *Warlist) Style(name string) (tcell.Style, bool) {
	return RelationStyle(w.Get(name))
}

// Load reads "<relation>\t<name>" lines; a missing file is not an error.
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
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		tok, name, ok := strings.Cut(line, "\t")
		if !ok || name == "" {
			continue
		}
		if r, ok := relationFromName(tok); ok && r != RelNeutral {
			rel[name] = r
		}
	}
	if err := sc.Err(); err != nil {
		return err
	}
	w.mu.Lock()
	w.rel = rel
	w.mu.Unlock()
	return nil
}

// Save writes the warlist to path, creating parent dirs.
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
		b.WriteByte('\n')
	}
	w.mu.Unlock()
	return os.WriteFile(path, []byte(b.String()), 0o644)
}
