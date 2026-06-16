package tui

import "github.com/gdamore/tcell/v2"

// Free-look map-pan sub-mode (§T94/§V54). When active the camera detaches from
// the local tee and the user pans the map with the arrow keys or WASD; the tee
// receives NO input while panning (view-only). Free-look requires the visual
// render on, so entering it forces visual on; exiting (Esc or the toggle key)
// recenters on the tee.

// teeInputActions are the NORMAL-mode actions that move/fire/affect the local
// tee; they are suppressed while free-look is active (§V54/§V12).
var teeInputActions = map[KeyAction]bool{
	actMoveLeft: true, actMoveRight: true, actMoveStop: true,
	actJump: true, actFire: true, actHook: true, actKill: true, actEmote: true,
}

// isTeeInput reports whether act controls the local tee.
func isTeeInput(act KeyAction) bool { return teeInputActions[act] }

// toggleFreeLook enters free-look (forcing visual on) or, if already active,
// exits and recenters. Pan resets to zero on every transition.
func (a *App) toggleFreeLook() {
	if a.freeLook {
		a.exitFreeLook()
		return
	}
	a.visual = true
	a.freeLook = true
	a.panX, a.panY = 0, 0
}

// exitFreeLook leaves free-look and recenters the camera on the tee.
func (a *App) exitFreeLook() {
	a.freeLook = false
	a.panX, a.panY = 0, 0
}

// handleFreeLook consumes a key while free-look is active: arrow keys and WASD
// pan the camera, Esc exits. It returns false for any other key (e.g. the
// free-look toggle, chat, browser) so normal handling still applies. Tee-input
// keys that fall through are dropped by doAction's guard (§V54).
func (a *App) handleFreeLook(ev *tcell.EventKey) bool {
	switch ev.Key() {
	case tcell.KeyEscape:
		a.exitFreeLook()
		return true
	case tcell.KeyUp:
		a.panBy(0, -1)
		return true
	case tcell.KeyDown:
		a.panBy(0, 1)
		return true
	case tcell.KeyLeft:
		a.panBy(-1, 0)
		return true
	case tcell.KeyRight:
		a.panBy(1, 0)
		return true
	case tcell.KeyRune:
		switch ev.Rune() {
		case 'w', 'W':
			a.panBy(0, -1)
			return true
		case 's', 'S':
			a.panBy(0, 1)
			return true
		case 'a', 'A':
			a.panBy(-1, 0)
			return true
		case 'd', 'D':
			a.panBy(1, 0)
			return true
		}
	}
	return false
}

// panBy shifts the free-look pan offset and clamps the resulting camera center
// to the map bounds, so panning never runs the view off the map into garbage
// (§V54). With no map/anchor yet the offset accumulates unclamped (harmless).
func (a *App) panBy(dx, dy int) {
	a.panX += dx
	a.panY += dy
	st, _ := a.cur().state.Get()
	cx, cy, ok := cameraCenter(st)
	if !ok || st.Map == nil {
		return
	}
	a.panX = clampInt(cx+a.panX, 0, st.Map.Width()-1) - cx
	a.panY = clampInt(cy+a.panY, 0, st.Map.Height()-1) - cy
}

// handleMoveAim routes the WASD + arrow keys to tee movement or cardinal aim per
// the cl_move_keys cvar (§T104/§V66): the selected set moves (jump/left/stop/
// right), the other aims. Returns true if it consumed the key. Not reached in
// free-look (that gate runs first, V54). Movement is sticky (terminal has no
// key-release) and goes through the InputController only (V12).
func (a *App) handleMoveAim(ev *tcell.EventKey) bool {
	const (
		dirUp = iota
		dirLeft
		dirDown
		dirRight
	)
	dir := -1
	isWASD := false
	switch ev.Key() {
	case tcell.KeyUp:
		dir = dirUp
	case tcell.KeyLeft:
		dir = dirLeft
	case tcell.KeyDown:
		dir = dirDown
	case tcell.KeyRight:
		dir = dirRight
	case tcell.KeyRune:
		switch ev.Rune() {
		case 'w', 'W':
			dir, isWASD = dirUp, true
		case 'a', 'A':
			dir, isWASD = dirLeft, true
		case 's', 'S':
			dir, isWASD = dirDown, true
		case 'd', 'D':
			dir, isWASD = dirRight, true
		}
	}
	if dir < 0 {
		return false
	}
	wasdMoves := a.cfgSnap().MoveKeys != "arrows"
	isMove := isWASD == wasdMoves // WASD-key in wasd mode, or arrow-key in arrows mode
	if isMove {
		switch dir {
		case dirUp:
			a.cur().input.PressJump()
		case dirLeft:
			a.cur().input.PressLeft()
		case dirDown:
			a.cur().input.PressStop()
		case dirRight:
			a.cur().input.PressRight()
		}
	} else {
		switch dir {
		case dirUp:
			a.cur().input.SetAim(0, -aimReach)
		case dirLeft:
			a.cur().input.SetAim(-aimReach, 0)
		case dirDown:
			a.cur().input.SetAim(0, aimReach)
		case dirRight:
			a.cur().input.SetAim(aimReach, 0)
		}
	}
	return true
}

// clampInt clamps v to [lo,hi]; if hi<lo it returns lo.
func clampInt(v, lo, hi int) int {
	if hi < lo {
		return lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
