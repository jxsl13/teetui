package tui

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

// dumpSim flattens the simulation screen's front buffer into rows of text.
func dumpSim(sim tcell.SimulationScreen) string {
	cells, w, h := sim.GetContents()
	var b strings.Builder
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := cells[y*w+x]
			if len(c.Runes) == 0 || c.Runes[0] == 0 {
				b.WriteByte(' ')
				continue
			}
			b.WriteRune(c.Runes[0])
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// §V32/§C17: the min-size guard kicks in below the threshold and lifts above it.
func TestTooSmallThresholds(t *testing.T) {
	cases := []struct {
		w, h int
		want bool
	}{
		{minTermW - 1, 40, true},
		{120, minTermH - 1, true},
		{minTermW, minTermH, false},
		{120, 40, false},
		{0, 0, true},
	}
	for _, c := range cases {
		if got := tooSmall(c.w, c.h); got != c.want {
			t.Errorf("tooSmall(%d,%d) = %v want %v", c.w, c.h, got, c.want)
		}
	}
}

// §V32: shrinking below the minimum shows the single resize notice; growing back
// restores the full UI (status bar) — a clean round-trip with no leftover notice.
func TestMinSizeRoundTrip(t *testing.T) {
	app, sim := newTestApp(t)

	// Shrink below the minimum (height too small; wide enough to show the full
	// notice) → only the resize notice.
	sim.SetSize(120, 4)
	app.draw()
	if got := dumpSim(sim); !strings.Contains(got, "terminal too small") {
		t.Fatalf("sub-min screen missing resize notice:\n%s", got)
	}

	// A genuinely tiny terminal (both dims sub-min) must still not panic and the
	// (clipped) notice prefix is present.
	sim.SetSize(10, 4)
	app.draw()
	if got := dumpSim(sim); !strings.Contains(got, "terminal") {
		t.Fatalf("tiny screen missing clipped notice:\n%s", got)
	}

	// Grow back → full UI returns (status bar shows the NORMAL mode label), and
	// the notice is gone.
	sim.SetSize(120, 40)
	app.draw()
	got := dumpSim(sim)
	if strings.Contains(got, "terminal too small") {
		t.Fatalf("resize notice persisted after growing back:\n%s", got)
	}
	if !strings.Contains(got, "NORMAL") {
		t.Fatalf("full UI not restored after growing back (no status bar):\n%s", got)
	}
}
