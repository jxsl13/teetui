package tui

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/jxsl13/twclient/client"
)

// benchScreen returns an initialized 120x40 simulation screen for the render
// benchmarks (§C7/§V7). The caller is responsible for Fini via b.Cleanup.
func benchScreen(b *testing.B) tcell.Screen {
	b.Helper()
	scr := tcell.NewSimulationScreen("")
	if err := scr.Init(); err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { scr.Fini() })
	scr.SetSize(120, 40)
	return scr
}

// benchState builds a populated tick: a local tee, a few other players (entity
// path) and an all-solid map view so the tile loop runs on the steady path.
func benchState() client.TickState {
	return client.TickState{
		LocalID: 1,
		Map:     client.NewMapView(nil), // empty all-solid view → full tile loop
		Players: map[int]client.CharacterState{
			1: {X: 320, Y: 320, Weapon: 2},
			2: {X: 352, Y: 320, HookState: 1, HookX: 320, HookY: 320},
			3: {X: 288, Y: 352, Weapon: weaponNinja},
			4: {X: 416, Y: 288},
		},
	}
}

// BenchmarkDrawGame measures the single-resolution render hot path (§V7).
func BenchmarkDrawGame(b *testing.B) {
	scr := benchScreen(b)
	st := benchState()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		DrawGame(scr, 0, 0, 80, 32, st)
	}
}

// BenchmarkDrawGameHalf measures the half-block sub-cell render hot path (§T46).
func BenchmarkDrawGameHalf(b *testing.B) {
	scr := benchScreen(b)
	st := benchState()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		DrawGameHalf(scr, 0, 0, 80, 32, st)
	}
}
