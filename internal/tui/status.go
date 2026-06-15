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

// statusText builds the status-bar line: input mode, server, race, tick.
func statusText(mode, server string, connected bool, st client.TickState, have bool) string {
	conn := "connecting"
	if connected {
		conn = "connected"
	}
	race := "-"
	tick := 0
	if have {
		race = raceField(st.RaceTime)
		tick = st.Tick
	}
	return fmt.Sprintf(" %s | %s (%s) | race %s | tick %d ", mode, server, conn, race, tick)
}
