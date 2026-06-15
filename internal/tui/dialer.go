package tui

import (
	"fmt"

	"github.com/jxsl13/twclient/client"
	"github.com/jxsl13/twclient/packet"
)

// DefaultDialer returns the twclient client factory used for the initial connect
// and for browser re-joins. It wires every teetui callback onto the app and its
// log so all comms stay inside twclient's public API (§V1/§V2/§V12):
//
//   - OnChat: own-echo de-dupe (§V29) + id-fallback name (§V26) + ping tracking
//     for the H auto-reply (§T23) and the tapped-out auto-reply (§T40),
//   - OnServerMsg / OnBroadcast / OnRconLine: routed to the log,
//   - OnDisconnect: DISCONNECTED popup + auto-reconnect (§T25).
//
// Rendering and input are bound via WithObserver(state)/WithController(input).
// main and the e2e harness share this so both drive identical callback paths.
func (a *App) DefaultDialer(name, clan, skin string) func(addr string, ver packet.Version) *client.Client {
	return func(addr string, v packet.Version) *client.Client {
		c := client.New(addr,
			client.WithPlayerInfo(name, clan, skin, -1),
			client.WithVersion(v),
			client.WithPrediction(true),
			client.WithObserver(a.state),
			client.WithController(a.input),
		)
		c.OnChat(func(cc *client.Client, e packet.EventChat) {
			if a.IsOwnEcho(e.ClientID, e.Msg) {
				return // already echoed locally on send (§V29)
			}
			from := ""
			if p, ok := cc.Player(e.ClientID); ok {
				from = p.Name
			}
			who := from
			if who == "" {
				who = fmt.Sprintf("%d", e.ClientID) // id fallback when name empty (§V26)
			}
			a.log.Addf(StyleChat, "[%s] %s", who, e.Msg)
			a.NoteChat(from, e.Msg)
		})
		c.OnServerMsg(func(_ *client.Client, e packet.EventServerMsg) {
			a.log.Addf(StyleSystem, "*** %s", e.Msg)
		})
		c.OnBroadcast(func(_ *client.Client, e packet.EventBroadcast) {
			a.log.Addf(StyleSystem, ">> %s", e.Text)
		})
		c.OnRconLine(func(_ *client.Client, e packet.EventRconLine) {
			a.log.Addf(StyleSystem, "rcon> %s", e.Line)
		})
		c.OnDisconnect(func(_ *client.Client, r client.DisconnectReason) {
			a.ShowDisconnect(r.Text)
		})
		return c
	}
}
