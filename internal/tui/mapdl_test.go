package tui

import (
	"strings"
	"testing"
)

// §T128/§V88: map-download progress renders a % line while joining; no progress
// (total 0) falls back to the spinner.
func TestMapDownloadLine(t *testing.T) {
	app, _ := newTestApp(t)
	s := app.cur()

	// No progress yet → no line (caller shows the spinner).
	if _, ok := app.mapDownloadLine(); ok {
		t.Error("no progress should yield no line")
	}

	// 50% of a 2 MiB map.
	s.mapTotal.Store(2 << 20)
	s.mapRecv.Store(1 << 20)
	line, ok := app.mapDownloadLine()
	if !ok || !strings.Contains(line, "50%") || !strings.Contains(line, "MB") {
		t.Errorf("progress line = %q ok=%v", line, ok)
	}
}

func TestHumanBytes(t *testing.T) {
	cases := map[int64]string{500: "500 B", 2048: "2 KB", 3 << 20: "3.0 MB"}
	for n, want := range cases {
		if got := humanBytes(n); got != want {
			t.Errorf("humanBytes(%d) = %q want %q", n, got, want)
		}
	}
}
