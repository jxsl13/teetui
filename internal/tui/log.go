package tui

import (
	"fmt"
	"strings"
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
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

// View returns the height-tall window of VISUAL rows honoring the scroll offset
// (oldest first). Each logical line is wrapped to width, so a line wider than the
// log band continues on the next row(s) rather than being truncated (§T98/§V59).
// offset 0 shows the newest rows; scrolling up reveals older history (§T30). The
// offset counts visual (wrapped) rows and is re-clamped here against the current
// width, so a resize never leaves the view past the end.
func (l *Log) View(width, height int) []LogLine {
	l.mu.Lock()
	defer l.mu.Unlock()
	if height < 1 || width < 1 {
		return nil
	}
	// Flatten every logical line into wrapped visual rows, carrying each line's
	// style onto its continuation rows.
	var vis []LogLine
	for _, ln := range l.lines {
		for _, seg := range wrapLine(ln.Text, width) {
			vis = append(vis, LogLine{Text: seg, Style: ln.Style})
		}
	}
	total := len(vis)
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
	copy(out, vis[start:end])
	return out
}

// wrapLine breaks s into segments each at most width display columns wide
// (runewidth-aware, §V6). It wraps on spaces (word-wrap); a single token wider
// than width is hard-split across rows so no content is ever dropped (§V59).
func wrapLine(s string, width int) []string {
	if width < 1 {
		width = 1
	}
	if runewidth.StringWidth(s) <= width {
		return []string{s}
	}
	var lines []string
	var cur []rune
	curW := 0
	flush := func() {
		lines = append(lines, string(cur))
		cur = cur[:0]
		curW = 0
	}
	for _, word := range strings.Fields(s) {
		ww := runewidth.StringWidth(word)
		// A word longer than the width is hard-split rune by rune.
		if ww > width {
			if curW > 0 {
				flush()
			}
			for _, r := range word {
				rw := runewidth.RuneWidth(r)
				if rw == 0 {
					rw = 1
				}
				if curW+rw > width {
					flush()
				}
				cur = append(cur, r)
				curW += rw
			}
			continue
		}
		sep := 0
		if curW > 0 {
			sep = 1
		}
		if curW+sep+ww > width {
			flush()
			sep = 0
		}
		if sep == 1 {
			cur = append(cur, ' ')
			curW++
		}
		cur = append(cur, []rune(word)...)
		curW += ww
	}
	if len(cur) > 0 || len(lines) == 0 {
		flush()
	}
	return lines
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
