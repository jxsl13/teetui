package tui

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// History is a bounded, navigable command history for one input mode. Entries
// are persisted to disk so they survive client restarts (§V16). Navigation
// (Prev/Next) and reverse-i-search (Search) mirror readline.
type History struct {
	mu      sync.Mutex
	entries []string // oldest first; newest at the end
	max     int
	pos     int // navigation cursor; == len(entries) means "fresh line"
}

// NewHistory returns a history keeping at most max entries.
func NewHistory(max int) *History {
	if max < 1 {
		max = 1
	}
	return &History{max: max}
}

// Add appends a non-empty entry (deduping an immediate repeat) and resets
// navigation to the fresh line.
func (h *History) Add(s string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if s == "" {
		h.pos = len(h.entries)
		return
	}
	if n := len(h.entries); n == 0 || h.entries[n-1] != s {
		h.entries = append(h.entries, s)
		if len(h.entries) > h.max {
			h.entries = h.entries[len(h.entries)-h.max:]
		}
	}
	h.pos = len(h.entries)
}

// ResetNav moves the navigation cursor back to the fresh line.
func (h *History) ResetNav() {
	h.mu.Lock()
	h.pos = len(h.entries)
	h.mu.Unlock()
}

// Prev returns the previous (older) entry, or "" if at the top.
func (h *History) Prev() (string, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.pos == 0 || len(h.entries) == 0 {
		return "", false
	}
	h.pos--
	return h.entries[h.pos], true
}

// Next returns the next (newer) entry; past the newest it returns the fresh
// empty line.
func (h *History) Next() (string, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.pos >= len(h.entries) {
		return "", false
	}
	h.pos++
	if h.pos == len(h.entries) {
		return "", true
	}
	return h.entries[h.pos], true
}

// Search returns the most recent entry containing term (reverse-i-search).
func (h *History) Search(term string) (string, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if term == "" {
		return "", false
	}
	for i := len(h.entries) - 1; i >= 0; i-- {
		if strings.Contains(h.entries[i], term) {
			return h.entries[i], true
		}
	}
	return "", false
}

// Entries returns a copy of the stored lines (oldest first).
func (h *History) Entries() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]string, len(h.entries))
	copy(out, h.entries)
	return out
}

// Load reads entries from path (one per line). A missing file is not an error.
func (h *History) Load(path string) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()
	var lines []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		if t := sc.Text(); t != "" {
			lines = append(lines, t)
		}
	}
	if err := sc.Err(); err != nil {
		return err
	}
	if len(lines) > h.max {
		lines = lines[len(lines)-h.max:]
	}
	h.mu.Lock()
	h.entries = lines
	h.pos = len(h.entries)
	h.mu.Unlock()
	return nil
}

// Save writes all entries to path (one per line), creating parent dirs.
func (h *History) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	h.mu.Lock()
	lines := make([]string, len(h.entries))
	copy(lines, h.entries)
	h.mu.Unlock()
	data := strings.Join(lines, "\n")
	if data != "" {
		data += "\n"
	}
	return os.WriteFile(path, []byte(data), 0o644)
}
