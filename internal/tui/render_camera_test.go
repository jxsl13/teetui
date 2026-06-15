package tui

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/jxsl13/twclient/client"
)

// §V27/§B6: the camera centers on the local tee, falls back to following the
// lowest-id visible player while spectating, and reports !ok only when there is
// nothing to anchor on.
func TestCameraCenter(t *testing.T) {
	// Local tee present → center on it (320/32=10, 640/32=20).
	st := client.TickState{LocalID: 2, Players: map[int]client.CharacterState{
		2: {X: 320, Y: 640}, 5: {X: 0, Y: 0},
	}}
	if cx, cy, ok := cameraCenter(st); !ok || cx != 10 || cy != 20 {
		t.Errorf("local center = %d,%d,%v want 10,20,true", cx, cy, ok)
	}

	// Spectating (LocalID not in Players) → follow lowest-id player (id 3).
	spec := client.TickState{LocalID: 99, Players: map[int]client.CharacterState{
		7: {X: 0, Y: 0}, 3: {X: 96, Y: 160},
	}}
	if cx, cy, ok := cameraCenter(spec); !ok || cx != 3 || cy != 5 {
		t.Errorf("spectator follow = %d,%d,%v want 3,5,true", cx, cy, ok)
	}

	// Nothing to anchor on.
	if _, _, ok := cameraCenter(client.TickState{LocalID: 0}); ok {
		t.Error("empty state must report !ok")
	}
}

// §V27: rendering as a spectator (no local tee, but a visible player + map) must
// draw the scene, not sit on "connecting…", and must not panic.
func TestDrawGameSpectatorNoPanic(t *testing.T) {
	scr := tcell.NewSimulationScreen("")
	if err := scr.Init(); err != nil {
		t.Fatal(err)
	}
	defer scr.Fini()
	scr.SetSize(80, 24)
	st := client.TickState{
		LocalID: 99, // not present → spectating
		Players: map[int]client.CharacterState{4: {X: 320, Y: 320}},
		Map:     client.NewMapView(nil),
	}
	DrawGame(scr, 0, 0, 40, 20, st)
	DrawGameHalf(scr, 0, 0, 40, 20, st)
}
