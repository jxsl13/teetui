package tui

import (
	"fmt"
	"time"

	"github.com/jxsl13/twclient/client"
)

// formatRace renders a race duration as MM:SS.mmm (← chillerbot formatDuration).
func formatRace(d time.Duration) string {
	if d < 0 {
		d = -d
	}
	ms := d.Milliseconds()
	return fmt.Sprintf("%02d:%02d.%03d", ms/60000, (ms%60000)/1000, ms%1000)
}

// raceField summarizes the current race state for the status bar.
func raceField(rt client.RaceTime) string {
	switch {
	case rt.Finished:
		return "FINISH " + formatRace(rt.FinishTime)
	case rt.Active:
		return formatRace(rt.TickBased)
	default:
		return "-"
	}
}

// connStatus is the connection state shown in the status bar: connected,
// (auto-)reconnecting with an attempt count, or the initial connecting state
// (§T25).
type connStatus struct {
	connected    bool
	reconnecting bool
	joining      bool // a connect handshake is in flight (§B9)
	attempt      int
}

// connLabel renders the connection field of the status bar (§T25). An active
// auto-reconnect reads "reconnecting #N" so the user sees progress rather than a
// frozen "connecting".
func connLabel(cs connStatus) string {
	switch {
	case cs.connected:
		return "connected"
	case cs.reconnecting:
		return fmt.Sprintf("reconnecting #%d", cs.attempt)
	case cs.joining:
		return "connecting"
	default:
		return "not connected" // idle: no connect attempted (§B9) — ⊥ "connecting"
	}
}

// statusText builds the status-bar line: input mode, server, race, tick.
func statusText(mode, server string, cs connStatus, st client.TickState, have bool) string {
	conn := connLabel(cs)
	race := "-"
	tick := 0
	if have {
		race = raceField(st.RaceTime)
		tick = st.Tick
	}
	return fmt.Sprintf(" %s | %s (%s) | race %s | tick %d ", mode, server, conn, race, tick)
}
