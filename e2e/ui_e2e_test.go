//go:build e2e

package e2e

import (
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/jxsl13/teetui/internal/tui"
	"github.com/jxsl13/twclient/packet"

	// Load the same feature modules main does, so the e2e UI exercises the real
	// feature set (warlist cvars, etc.) — mirrors §C21 main wiring.
	_ "github.com/jxsl13/teetui/features/chatfilter"
	_ "github.com/jxsl13/teetui/features/chillpw"
	_ "github.com/jxsl13/teetui/features/cmdhook"
	_ "github.com/jxsl13/teetui/features/lastping"
	_ "github.com/jxsl13/teetui/features/replytoping"
	_ "github.com/jxsl13/teetui/features/responders"
	_ "github.com/jxsl13/teetui/features/serverlog"
	_ "github.com/jxsl13/teetui/features/team"
	_ "github.com/jxsl13/teetui/features/warlist"
)

// This file drives the FULL teetui terminal UI against the live dockerized
// servers and validates the rendered screen — not just that twclient connects
// (e2e_test.go does that), but that teetui's handlers + rendering behave and
// display as expected end to end (§C14/§V23, user request "validate behaviour
// and display").
//
// It runs the real App on a tcell.SimulationScreen: App.Join uses the same
// dialer + RunFrontends path as main (§V22/§B2), background twclient callbacks
// feed the log/observer, and App.Dispatch feeds key events through the genuine
// handlers. After each interaction we read the simulation cell buffer back and
// assert the expected text is on screen.

const uiPlayerName = "teetui-e2e"

// uiTimeout is how long a screen assertion polls for the expected text to
// appear (snapshots/echoes arrive asynchronously from twclient goroutines).
const uiTimeout = 8 * time.Second

// screenText flattens the simulation screen's front buffer into rows of text so
// assertions can substring-match what the user would see.
func screenText(sim tcell.SimulationScreen) string {
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

// rune_ / special dispatch one key event through the real handlers + a redraw.
func keyRune(app *tui.App, r rune) { app.Dispatch(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone)) }
func keySpecial(app *tui.App, k tcell.Key) {
	app.Dispatch(tcell.NewEventKey(k, 0, tcell.ModNone))
}

// typeString feeds each rune of s as a key event (text input).
func typeString(app *tui.App, s string) {
	for _, r := range s {
		keyRune(app, r)
	}
}

// waitScreen polls up to uiTimeout for substr to appear on screen, redrawing
// each iteration to pick up async background updates. Returns the final screen
// text and whether the substring was found.
func waitScreen(app *tui.App, sim tcell.SimulationScreen, substr string) (string, bool) {
	deadline := time.Now().Add(uiTimeout)
	for {
		app.Redraw()
		txt := screenText(sim)
		if strings.Contains(txt, substr) {
			return txt, true
		}
		if time.Now().After(deadline) {
			return txt, false
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// mustScreen fails the test if substr never appears, dumping the final screen.
func mustScreen(t *testing.T, app *tui.App, sim tcell.SimulationScreen, what, substr string) {
	t.Helper()
	if txt, ok := waitScreen(app, sim, substr); !ok {
		t.Fatalf("%s: %q not on screen within %s. screen was:\n%s", what, substr, uiTimeout, txt)
	}
}

// refuteScreen fails if substr IS present right now (after a redraw).
func refuteScreen(t *testing.T, app *tui.App, sim tcell.SimulationScreen, what, substr string) {
	t.Helper()
	app.Redraw()
	if txt := screenText(sim); strings.Contains(txt, substr) {
		t.Fatalf("%s: %q should NOT be on screen. screen was:\n%s", what, substr, txt)
	}
}

// connectUI builds a simulation-backed App, joins addr at version via the real
// main-equivalent path, and waits for the handshake. A server that does not
// answer SKIPS the subtest (harness state, not a code defect — §B3).
func connectUI(t *testing.T, version packet.Version, addr string) (*tui.App, tcell.SimulationScreen) {
	t.Helper()
	sim := tcell.NewSimulationScreen("UTF-8")
	if err := sim.Init(); err != nil {
		t.Fatalf("sim init: %v", err)
	}
	sim.SetSize(160, 48) // wide enough that the log column does not truncate assertions

	state := tui.NewState()
	input := tui.NewInputController()
	log := tui.NewLog(500)
	app := tui.NewAppWithScreen(sim, addr, state, input, log)
	app.SetName(uiPlayerName)
	app.SetDialer(app.DefaultDialer(uiPlayerName, "", "default"))
	app.SetConnectTimeout(connectTimeout)
	t.Cleanup(app.Stop)

	app.Join(addr, version)

	deadline := time.Now().Add(connectTimeout + snapTimeout)
	for time.Now().Before(deadline) {
		if app.Connected() {
			return app, sim
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Skipf("connect %s (proto %v) did not complete — server not answering (harness state): screen:\n%s",
		addr, version, screenText(sim))
	return nil, nil
}

// TestE2EUI is the live UI matrix: for each supported server/protocol it connects
// the full teetui UI and drives every major feature, asserting the rendered
// screen each time.
func TestE2EUI(t *testing.T) {
	requireHarness(t)

	for _, s := range liveServers() {
		s := s
		t.Run(s.name, func(t *testing.T) {
			app, sim := connectUI(t, s.version, s.addr)

			// Startup greeting popup is up; it advertises Enter-to-close (§T31).
			mustScreen(t, app, sim, "greeting", "press B for browser")
			keySpecial(app, tcell.KeyEnter)
			refuteScreen(t, app, sim, "greeting closed", "press B for browser")

			// Status bar reflects the live connection (§T10/§T25).
			mustScreen(t, app, sim, "status connected", "connected")
			mustScreen(t, app, sim, "status server addr", s.addr)

			// Game renders from the decoded snapshot: the HUD coord readout shows
			// (§T34) and the "connecting…" placeholder is gone (§V27/§B6).
			mustScreen(t, app, sim, "game HUD coords", "x:")
			refuteScreen(t, app, sim, "no connecting placeholder", "connecting…")

			t.Run("scoreboard", func(t *testing.T) {
				keySpecial(app, tcell.KeyTab) // toggle scoreboard (§T17)
				mustScreen(t, app, sim, "scoreboard header score", "score")
				mustScreen(t, app, sim, "scoreboard header name", "name")
				mustScreen(t, app, sim, "scoreboard header clan", "clan")
				keySpecial(app, tcell.KeyTab) // hide again
			})

			t.Run("help_overlay", func(t *testing.T) {
				keyRune(app, '?') // open help (§T28/§V17)
				mustScreen(t, app, sim, "help title", "teetui — keys")
				mustScreen(t, app, sim, "help lists browser key", "server browser")
				keySpecial(app, tcell.KeyEscape) // always escapable (§V17)
				refuteScreen(t, app, sim, "help closed", "teetui — keys")
			})

			t.Run("esc_menu", func(t *testing.T) {
				// Connected → Esc opens the overlay action bar (§T111/§V74).
				keySpecial(app, tcell.KeyEscape)
				mustScreen(t, app, sim, "esc menu open", "Disconnect")
				mustScreen(t, app, sim, "esc menu hint", "Esc close")
				keySpecial(app, tcell.KeyEscape) // close so later subtests run in normal mode
				refuteScreen(t, app, sim, "esc menu closed", "Esc close")
			})

			t.Run("visual_toggle", func(t *testing.T) {
				keyRune(app, 'v') // visual off → logs fill the body, game hidden (§C22)
				refuteScreen(t, app, sim, "game hidden when visual off", "x:")
				keyRune(app, 'v') // back on — HUD returns
				mustScreen(t, app, sim, "visual back on", "x:")
			})

			t.Run("free_look", func(t *testing.T) {
				keyRune(app, 'G') // enter free-look map-pan (§T94/§V54)
				mustScreen(t, app, sim, "free-look indicator", "[free-look]")
				keySpecial(app, tcell.KeyRight) // pan the camera off the tee
				keySpecial(app, tcell.KeyDown)
				mustScreen(t, app, sim, "still free-look after pan", "[free-look]")
				keyRune(app, 'G') // exit + recenter
				refuteScreen(t, app, sim, "free-look exited", "[free-look]")
			})

			t.Run("legend_generated", func(t *testing.T) {
				// the bottom legend is generated from the live keymap (§T95/§V55).
				mustScreen(t, app, sim, "legend browser key", "[B]browser")
				mustScreen(t, app, sim, "legend free-look entry", "free-look")
			})

			t.Run("join_game_key", func(t *testing.T) {
				keyRune(app, 'j') // join the game via key (§T97/§V57, team feature)
				mustScreen(t, app, sim, "join logged", "team → game")
			})

			t.Run("chat_local_echo", func(t *testing.T) {
				keyRune(app, 't') // enter chat (§T11/§I keymap)
				mustScreen(t, app, sim, "chat prompt", "say:")
				// Short message so it fits the log column without truncation at any
				// terminal width (the log window is only a fraction of the screen).
				msg := "e2e-chat"
				typeString(app, msg)
				keySpecial(app, tcell.KeyEnter) // submit
				// Sent chat is echoed locally immediately, even when the server does
				// not echo our own line / echoes it with an empty name (§V29/§B8).
				mustScreen(t, app, sim, "own chat echo", "["+uiPlayerName+"] "+msg)
			})

			t.Run("local_console_version", func(t *testing.T) {
				keySpecial(app, tcell.KeyF1) // local console (§T39)
				mustScreen(t, app, sim, "console prompt", "]")
				typeString(app, "version")
				keySpecial(app, tcell.KeyEnter)
				mustScreen(t, app, sim, "console version output", "teetui (twclient")
			})

			t.Run("console_cvar_get_set", func(t *testing.T) {
				keySpecial(app, tcell.KeyF1)
				typeString(app, "cl_silent_chat_commands")
				keySpecial(app, tcell.KeyEnter)
				mustScreen(t, app, sim, "cvar value shown", `cl_silent_chat_commands = "1"`)
			})

			t.Run("serverlog_active", func(t *testing.T) {
				// The serverlog feature is provisioned in the real binary on every
				// server variant (§T106): its cvar is present (message formatting is
				// covered by the feature's unit tests).
				keySpecial(app, tcell.KeyF1)
				typeString(app, "cl_show_game_messages")
				keySpecial(app, tcell.KeyEnter)
				mustScreen(t, app, sim, "serverlog cvar shown", `cl_show_game_messages = "1"`)
			})

			t.Run("server_browser_open_close", func(t *testing.T) {
				keyRune(app, 'B') // open browser overlay (§T18/§T32)
				mustScreen(t, app, sim, "browser tabs", "Internet")
				keyRune(app, 'B') // close back to game
				refuteScreen(t, app, sim, "browser closed", "Internet")
				mustScreen(t, app, sim, "game restored", "x:")
			})

			t.Run("responsive_resize", func(t *testing.T) {
				// Larger terminal: the live game still renders (HUD coords) at the
				// new, bigger resolution (§V31/§C17).
				sim.SetSize(220, 64)
				mustScreen(t, app, sim, "renders at large size", "x:")
				// Below the minimum: single resize notice, no garbled layout (§V32).
				sim.SetSize(120, 4)
				mustScreen(t, app, sim, "too-small notice", "terminal too small")
				// Restore: full UI + live game come back (§V30 round-trip).
				sim.SetSize(160, 48)
				mustScreen(t, app, sim, "restored after resize", "x:")
			})
		})
	}
}
