package tui

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/jxsl13/twclient/master"
	"github.com/jxsl13/twclient/packet"
)

func entry(name, gt, mp, host string, port int, v packet.Version) master.ServerEntry {
	return master.ServerEntry{
		Addresses: []master.Address{{Version: v, Host: host, Port: port}},
		Info:      packet.ServerInfo{Name: name, GameType: gt, MapName: mp, NumClients: 1, MaxClients: 16},
	}
}

func sampleBrowser() *Browser {
	b := NewBrowser()
	b.SetEntries([]master.ServerEntry{
		entry("DDNet GER", "DDNet", "Tutorial", "1.1.1.1", 8303, packet.Version06),
		entry("Vanilla CTF", "CTF", "ctf5", "2.2.2.2", 8303, packet.Version07),
		entry("KoG Server", "KoG", "kobra", "3.3.3.3", 8303, packet.Version06),
	})
	return b
}

// §V13/§T32: tabs filter the master list by category.
func TestBrowserTabs(t *testing.T) {
	b := sampleBrowser()
	if v, _ := b.Selected(); v.Addr == "" {
		t.Fatal("internet tab should have a selection")
	}
	if got := len(b.view); got != 3 {
		t.Errorf("internet tab = %d want 3", got)
	}
	b.SetTab(1) // DDNet
	if len(b.view) != 1 || b.view[0].Name != "DDNet GER" {
		t.Errorf("DDNet tab = %v", b.view)
	}
	b.SetTab(1) // KoG
	if len(b.view) != 1 || b.view[0].Name != "KoG Server" {
		t.Errorf("KoG tab = %v", b.view)
	}
}

// §T32: search filters by name/map substring.
func TestBrowserSearch(t *testing.T) {
	b := sampleBrowser()
	for _, r := range "ctf" {
		b.SearchType(r)
	}
	if len(b.view) != 1 || b.view[0].Name != "Vanilla CTF" {
		t.Errorf("search ctf = %v", b.view)
	}
	b.SearchBackspace()
	b.SearchBackspace()
	b.SearchBackspace()
	if len(b.view) != 3 {
		t.Errorf("cleared search = %d want 3", len(b.view))
	}
}

// §T18: selection moves and clamps.
func TestBrowserMoveClamp(t *testing.T) {
	b := sampleBrowser()
	b.Move(100)
	if v, _ := b.Selected(); v.Name != "KoG Server" {
		t.Errorf("clamp bottom = %q", v.Name)
	}
	b.Move(-100)
	if v, _ := b.Selected(); v.Name != "DDNet GER" {
		t.Errorf("clamp top = %q", v.Name)
	}
}

// §V11: drawing the browser overlay must not panic (incl empty/loading).
func TestDrawBrowserNoPanic(t *testing.T) {
	scr := tcell.NewSimulationScreen("")
	if err := scr.Init(); err != nil {
		t.Fatal(err)
	}
	defer scr.Fini()
	scr.SetSize(100, 30)
	DrawBrowser(scr, 100, 30, sampleBrowser())
	DrawBrowser(scr, 100, 30, NewBrowser()) // empty
	lb := NewBrowser()
	lb.SetLoading(true)
	DrawBrowser(scr, 100, 30, lb)
}
