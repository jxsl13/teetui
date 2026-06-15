package tui

import "time"

// sentChat is one locally-sent chat line, kept briefly so the server's echo of
// it can be de-duplicated (§T55/§V29).
type sentChat struct {
	msg string
	at  time.Time
}

// sentEchoWindow is how long a sent line is remembered for echo matching.
const sentEchoWindow = 5 * time.Second

// findRecentSent returns the index of the most recent sent line equal to msg
// within sentEchoWindow of now, or -1. Pure, so it is unit-tested.
func findRecentSent(sent []sentChat, msg string, now time.Time) int {
	for i := len(sent) - 1; i >= 0; i-- {
		if sent[i].msg == msg && now.Sub(sent[i].at) < sentEchoWindow {
			return i
		}
	}
	return -1
}
