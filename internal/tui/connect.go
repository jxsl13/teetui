package tui

import (
	"fmt"
	"time"

	"github.com/jxsl13/twclient/packet"
)

// mapDownloadLine returns the map-download progress line for the active session
// while a download is in flight (§T128/§V88), and false when there is no progress
// to show yet (total==0 → fall back to the indeterminate spinner).
func (a *App) mapDownloadLine() (string, bool) {
	s := a.cur()
	total := s.mapTotal.Load()
	if total <= 0 {
		return "", false
	}
	recv := s.mapRecv.Load()
	pct := recv * 100 / total
	if pct > 100 {
		pct = 100
	}
	return fmt.Sprintf(" ↓ downloading map %d%% (%s / %s) ", pct, humanBytes(recv), humanBytes(total)), true
}

// humanBytes formats a byte count compactly (B/KB/MB).
func humanBytes(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.0f KB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
}

// versionLabel renders a protocol version as its dotted user-facing string
// (matching the -version flag): "0.6", "0.7", or the raw number otherwise.
func versionLabel(ver packet.Version) string {
	switch ver {
	case packet.VersionAuto:
		return "auto"
	case packet.Version06:
		return "0.6"
	case packet.Version07:
		return "0.7"
	default:
		return fmt.Sprintf("%d", int(ver))
	}
}

// connectFailMsg builds the actionable connect-failure log line (§V24/§T50). It
// surfaces the address, the protocol version that was tried, the underlying
// error, and a hint pointing at the three usual causes (wrong address, wrong
// -version, network) so the user is never left with a silent hang past the
// timeout.
func connectFailMsg(addr string, ver packet.Version, err error) string {
	return fmt.Sprintf("connect failed: %s (%s): %v — check address, -version, and network",
		addr, versionLabel(ver), err)
}

// connectTimeoutMsg is the log line when the handshake watchdog gives up after
// the configured timeout — distinct from a hard protocol error so a slow or
// unreachable server reads as retryable, not broken (§V28/§B7).
func connectTimeoutMsg(addr string, ver packet.Version, d time.Duration) string {
	return fmt.Sprintf("connect timed out after %s: %s (%s) — server slow or unreachable (check -version, raise -connect-timeout)",
		d, addr, versionLabel(ver))
}

// spinnerFrames cycles a small ASCII spinner for indeterminate progress.
var spinnerFrames = []rune{'|', '/', '-', '\\'}

// connectingLine returns the indeterminate "connecting / downloading map"
// indicator shown while a join is in flight (connected==false) (§T33). twclient
// v0.2.2 exposes no map-download byte/percent accessor on its public Client
// API, so progress cannot be a filled bar — it is an animated spinner instead.
// The caller advances frame on each redraw.
func connectingLine(frame int) string {
	if frame < 0 {
		frame = -frame
	}
	sp := spinnerFrames[frame%len(spinnerFrames)]
	return fmt.Sprintf("%c connecting / downloading map …", sp)
}
