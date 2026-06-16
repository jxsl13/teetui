package tui

import (
	"strings"
	"testing"

	"github.com/mattn/go-runewidth"
)

// §T95/§V55: the legend is generated from the live keymap, reflects rebinds,
// is context-aware, and never overflows the bar width (§V30).

func TestLegendReflectsRebind(t *testing.T) {
	app, _ := newTestApp(t)
	line := app.legendLine(200)
	if !strings.Contains(line, "[T]chat") {
		t.Fatalf("default legend missing chat key: %q", line)
	}

	// Rebind chat to 'c' and confirm the legend tracks the live binding (§V19).
	app.keymap.clearAction(actChat)
	app.keymap.bindRune('c', actChat)
	line = app.legendLine(200)
	if strings.Contains(line, "[T]chat") || !strings.Contains(line, "[c]chat") {
		t.Errorf("legend did not reflect rebind: %q", line)
	}
}

func TestLegendTruncatesToWidth(t *testing.T) {
	app, _ := newTestApp(t)
	for _, w := range []int{8, 20, 40, 200} {
		got := runewidth.StringWidth(app.legendLine(w))
		if got > w {
			t.Errorf("legend width %d exceeds budget %d", got, w)
		}
	}
}

func TestLegendContextFreeLook(t *testing.T) {
	app, _ := newTestApp(t)
	normal := app.legendLine(200)
	if !strings.Contains(normal, "browser") || !strings.Contains(normal, "free-look") {
		t.Fatalf("normal legend missing core commands: %q", normal)
	}
	app.freeLook = true
	fl := app.legendLine(200)
	if !strings.Contains(fl, "pan") || !strings.Contains(fl, "exit") {
		t.Errorf("free-look legend missing pan/exit: %q", fl)
	}
	if strings.Contains(fl, "browser") {
		t.Errorf("free-look legend should not list browser: %q", fl)
	}
}

func TestLegendIncludesFeatureAction(t *testing.T) {
	app, _ := newTestApp(t)
	app.featActions = append(app.featActions, featAction{name: "reply_to_ping", key: "H", help: "reply"})
	line := app.legendLine(400)
	if !strings.Contains(line, "[H]reply_to_ping") {
		t.Errorf("legend missing feature action: %q", line)
	}
}
