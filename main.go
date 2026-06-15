// Command teetui is a cross-platform terminal Teeworlds/DDNet client. It
// re-implements the chillerbot-ux terminal UI on top of the pure-Go twclient
// library, rendering the live game, chat and scoreboard with tcell.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/jxsl13/teetui/internal/tui"
	"github.com/jxsl13/twclient/packet"

	// Feature modules (§C21): each self-registers in init(); blank-import to
	// enable. Add a feature = add a package + one import line here.
	_ "github.com/jxsl13/teetui/features/chatfilter"
	_ "github.com/jxsl13/teetui/features/chillpw"
	_ "github.com/jxsl13/teetui/features/lastping"
	_ "github.com/jxsl13/teetui/features/replytoping"
	_ "github.com/jxsl13/teetui/features/responders"
	_ "github.com/jxsl13/teetui/features/team"
	_ "github.com/jxsl13/teetui/features/warlist"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "teetui:", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		server   = flag.String("server", "127.0.0.1:8303", "server address host:port")
		name     = flag.String("name", "nameless tee", "player name")
		clan     = flag.String("clan", "", "player clan")
		skin     = flag.String("skin", "default", "player skin")
		version  = flag.String("version", "0.6", "protocol version: 0.6 or 0.7")
		connTime = flag.Duration("connect-timeout", tui.DefaultConnectTimeout, "handshake timeout (login + map download)")
		maxFPS   = flag.Int("max-fps", tui.DefaultMaxFPS, "cap render repaints per second (0 = unlimited)")
		logLines = flag.Int("log-lines", tui.DefaultLogLines, "log rows shown below the visual (capped at half the height)")
	)
	flag.Parse()

	ver := packet.Version06
	if *version == "0.7" {
		ver = packet.Version07
	}

	state := tui.NewState()
	input := tui.NewInputController()
	log := tui.NewLog(500)

	app, err := tui.NewApp(*server, state, input, log)
	if err != nil {
		return err
	}
	app.SetConnectTimeout(*connTime)
	app.SetMaxFPS(*maxFPS)
	app.SetLogLines(*logLines)

	// Client factory: all comms go through twclient (render via Observer, input
	// via Controller, chat/server/rcon/disconnect via callbacks — §V1, §V2,
	// §V12). The app reuses it to rejoin a server from the browser (§T18). The
	// wiring lives in tui.DefaultDialer so the e2e harness drives the same paths.
	app.SetName(*name) // for ping detection (§T23)
	app.SetDialer(app.DefaultDialer(*name, *clan, *skin))

	// Initial connect goes through the same path as a browser join, so the
	// twclient frontend loop (RunFrontends) is started — that is what drives
	// the renderer and input (§V22, §B2).
	app.Join(*server, ver)

	app.Run() // blocks until quit; restores the terminal
	if cur := app.Client(); cur != nil {
		cur.Close()
	}
	return nil
}
