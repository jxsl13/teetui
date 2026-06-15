// Command teetui is a cross-platform terminal Teeworlds/DDNet client. It
// re-implements the chillerbot-ux terminal UI on top of the pure-Go twclient
// library, rendering the live game, chat and scoreboard with tcell.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/jxsl13/teetui/internal/tui"
	"github.com/jxsl13/twclient/client"
	"github.com/jxsl13/twclient/packet"
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

	// Client factory: all comms go through twclient (render via Observer, input
	// via Controller, chat/server/rcon/disconnect via callbacks — §V1, §V2,
	// §V12). The app reuses it to rejoin a server from the browser (§T18).
	newClient := func(addr string, v packet.Version) *client.Client {
		c := client.New(addr,
			client.WithPlayerInfo(*name, *clan, *skin, -1),
			client.WithVersion(v),
			client.WithPrediction(true),
			client.WithObserver(state),
			client.WithController(input),
		)
		c.OnChat(func(cc *client.Client, e packet.EventChat) {
			from := ""
			if p, ok := cc.Player(e.ClientID); ok {
				from = p.Name
			}
			who := from
			if who == "" {
				who = fmt.Sprintf("%d", e.ClientID)
			}
			log.Addf(tui.StyleChat, "[%s] %s", who, e.Msg)
			app.NoteChat(from, e.Msg) // ping tracking for H auto-reply
		})
		c.OnServerMsg(func(_ *client.Client, e packet.EventServerMsg) {
			log.Addf(tui.StyleSystem, "*** %s", e.Msg)
		})
		c.OnBroadcast(func(_ *client.Client, e packet.EventBroadcast) {
			log.Addf(tui.StyleSystem, ">> %s", e.Text)
		})
		c.OnRconLine(func(_ *client.Client, e packet.EventRconLine) {
			log.Addf(tui.StyleSystem, "rcon> %s", e.Line)
		})
		c.OnDisconnect(func(_ *client.Client, r client.DisconnectReason) {
			app.ShowDisconnect(r.Text)
		})
		return c
	}
	app.SetDialer(newClient)
	app.SetName(*name) // for ping detection (§T23)

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
