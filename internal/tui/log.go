package tui

import (
	"fmt"
	"sync"

	"github.com/gdamore/tcell/v2"
)

// LogLine is one styled scrollback entry.
type LogLine struct {
	Text  string
	Style tcell.Style
}

// Log is a bounded, goroutine-safe scrollback buffer for chat/console/server
// messages. twclient callbacks fire from its event-loop goroutine, so every
// mutation takes the lock (§V4).
type Log struct {
	mu     sync.Mutex
	lines  []LogLine
	max    int
	offset int // lines scrolled up from the tail; 0 == following newest
}

// NewLog returns a log that keeps at most max lines.
func NewLog(max int) *Log {
	if max < 1 {
		max = 1
	}
	return &Log{max: max, lines: make([]LogLine, 0, max)}
}

// Addf appends a formatted styled line, evicting the oldest when full.
func (l *Log) Addf(style tcell.Style, format string, a ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lines = append(l.lines, LogLine{Text: fmt.Sprintf(format, a...), Style: style})
	if len(l.lines) > l.max {
		l.lines = l.lines[len(l.lines)-l.max:]
	}
}

// Tail returns a copy of the last n lines (oldest first).
func (l *Log) Tail(n int) []LogLine {
	l.mu.Lock()
	defer l.mu.Unlock()
	if n > len(l.lines) {
		n = len(l.lines)
	}
	out := make([]LogLine, n)
	copy(out, l.lines[len(l.lines)-n:])
	return out
}

// View returns the height-tall window of lines honoring the scroll offset
// (oldest first). offset 0 shows the newest lines; scrolling up reveals older
// history (§T30).
func (l *Log) View(height int) []LogLine {
	l.mu.Lock()
	defer l.mu.Unlock()
	if height < 1 {
		return nil
	}
	total := len(l.lines)
	maxOff := total - height
	if maxOff < 0 {
		maxOff = 0
	}
	if l.offset > maxOff {
		l.offset = maxOff
	}
	end := total - l.offset
	start := end - height
	if start < 0 {
		start = 0
	}
	out := make([]LogLine, end-start)
	copy(out, l.lines[start:end])
	return out
}

// ScrollUp moves the view n lines toward older history.
func (l *Log) ScrollUp(n int) {
	l.mu.Lock()
	total := len(l.lines)
	l.offset += n
	if l.offset > total {
		l.offset = total
	}
	l.mu.Unlock()
}

// ScrollDown moves the view n lines toward the newest, clamping at the tail.
func (l *Log) ScrollDown(n int) {
	l.mu.Lock()
	l.offset -= n
	if l.offset < 0 {
		l.offset = 0
	}
	l.mu.Unlock()
}

// FollowTail resets scrolling to track the newest line.
func (l *Log) FollowTail() {
	l.mu.Lock()
	l.offset = 0
	l.mu.Unlock()
}
