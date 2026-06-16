package warlist

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/gdamore/tcell/v2"
)

// Relation is the warlist standing for a name (← chillerbot warlist).
type Relation int

const (
	RelNeutral Relation = iota
	RelWar
	RelPeace
	RelTeam
)

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

// relationStyle returns the scoreboard color for a relation.
func relationStyle(r Relation) (tcell.Style, bool) {
	c := func(rr, g, b int32) tcell.Style {
		return tcell.StyleDefault.Foreground(tcell.NewRGBColor(rr, g, b))
	}
	switch r {
	case RelWar:
		return c(255, 80, 80), true // red
	case RelPeace:
		return c(80, 255, 80), true // green
	case RelTeam:
		return c(0, 200, 255), true // cyan
	default:
		return tcell.StyleDefault, false
	}
}

// Store is the war/peace/team store (§T21/§T24/§V14): per-name relations (with
// optional war reason) and per-clan-tag relations. Mutated by chat commands and
// read by the scoreboard styler + the chat-query service, so access is locked.
type Store struct {
	mu      sync.Mutex
	rel     map[string]Relation
	reasons map[string]string
	clanRel map[string]Relation
}

func newStore() *Store {
	return &Store{rel: map[string]Relation{}, reasons: map[string]string{}, clanRel: map[string]Relation{}}
}

func (w *Store) Set(name string, r Relation) {
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

func (w *Store) Del(name string) { w.Set(name, RelNeutral) }

func (w *Store) Get(name string) Relation {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.rel[name]
}

func (w *Store) namesWith(r Relation) []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	var out []string
	for n, rel := range w.rel {
		if rel == r {
			out = append(out, n)
		}
	}
	sort.Strings(out)
	return out
}

func (w *Store) clansWith(r Relation) []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	var out []string
	for c, rel := range w.clanRel {
		if rel == r {
			out = append(out, c)
		}
	}
	sort.Strings(out)
	return out
}

// Search returns "name (relation[: reason])" lines matching sub (§T67).
func (w *Store) Search(sub string) []string {
	low := strings.ToLower(sub)
	w.mu.Lock()
	defer w.mu.Unlock()
	var names []string
	for n := range w.rel {
		if sub == "" || strings.Contains(strings.ToLower(n), low) {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	out := make([]string, 0, len(names))
	for _, n := range names {
		line := n + " (" + relationName(w.rel[n])
		if r := w.reasons[n]; r != "" {
			line += ": " + r
		}
		out = append(out, line+")")
	}
	return out
}

func (w *Store) SetReason(name, reason string) {
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

func (w *Store) Reason(name string) string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.reasons[name]
}

func (w *Store) SetClan(clan string, r Relation) {
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

// effective resolves the relation for a player: per-name wins, else clan tag.
func (w *Store) effective(name, clan string) Relation {
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

// EffectiveStyle returns the scoreboard color for a player (name then clan).
func (w *Store) EffectiveStyle(name, clan string) (tcell.Style, bool) {
	return relationStyle(w.effective(name, clan))
}

// Load reads the warlist file (missing = no error). Accepts the legacy simple,
// reason and clan line shapes.
func (w *Store) Load(path string) error {
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
		rel[fields[1]] = r
		if len(fields) >= 3 && fields[2] != "" {
			reasons[fields[1]] = fields[2]
		}
	}
	if err := sc.Err(); err != nil {
		return err
	}
	w.mu.Lock()
	w.rel, w.reasons, w.clanRel = rel, reasons, clanRel
	w.mu.Unlock()
	return nil
}

// Save writes the warlist to path, creating parent dirs.
func (w *Store) Save(path string) error {
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

// --- "warlist" service adapter (string relations, §V53) ---
// Provided under the "warlist" name; consumers declare their own minimal view.

func (w *Store) Relation(name string) string {
	if r := w.Get(name); r != RelNeutral {
		return relationName(r)
	}
	return ""
}

func (w *Store) NamesWith(relation string) []string {
	if r, ok := relationFromName(relation); ok {
		return w.namesWith(r)
	}
	return nil
}

func (w *Store) ClansWith(relation string) []string {
	if r, ok := relationFromName(relation); ok {
		return w.clansWith(r)
	}
	return nil
}
