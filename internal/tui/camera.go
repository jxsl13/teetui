package tui

import "math"

// cameraAlpha is the per-frame easing factor of the smooth camera (§T43). At
// ~50Hz a value near 0.3 converges to a moving target within a few frames while
// filtering the high-frequency jitter that integer tile-snapping produces when a
// predicted tee oscillates across a tile boundary.
const cameraAlpha = 0.3

// cameraSmoother eases the rendered camera center toward the target tile coords
// instead of snapping, removing per-frame jitter (§T43, smoother than the
// reference's hard-snapped view, §C11/§V20). It holds a fractional position
// across frames; the rendered center is the rounded value.
type cameraSmoother struct {
	x, y float64
	init bool
}

// step advances the smoother toward target (tx,ty) by alpha and returns the
// rounded integer center to render. The first call snaps to the target so the
// view does not visibly slide in from the origin on join. alpha is clamped to
// (0,1]; alpha>=1 disables smoothing (always snaps).
func (c *cameraSmoother) step(tx, ty int, alpha float64) (int, int) {
	if alpha <= 0 {
		alpha = cameraAlpha
	}
	if alpha > 1 {
		alpha = 1
	}
	if !c.init {
		c.x, c.y = float64(tx), float64(ty)
		c.init = true
	} else {
		c.x += (float64(tx) - c.x) * alpha
		c.y += (float64(ty) - c.y) * alpha
	}
	return int(math.Round(c.x)), int(math.Round(c.y))
}

// reset clears the smoother so the next step snaps to its target (used when the
// camera anchor disappears, e.g. on disconnect, to avoid sliding across the map
// on reconnect).
func (c *cameraSmoother) reset() { c.init = false }
