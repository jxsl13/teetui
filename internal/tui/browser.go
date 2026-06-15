package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/jxsl13/twclient/master"
	"github.com/jxsl13/twclient/packet"
)

// serverRow is one browser entry distilled from a master.ServerEntry.
type serverRow struct {
	Addr       string
	Name       string
	GameType   string
	MapName    string
	Players    int
	MaxPlayers int
	Passworded bool
	Version    packet.Version
}

// Browser tab indices (← transcript Internet/LAN/Favorites/DDNet/KoG).
const (
	tabInternet = iota
	tabLAN
	tabFavorites
	tabDDNet
	tabKoG
	tabVanilla
)

var browserTabs = []string{"Internet", "LAN", "Favorites", "DDNet", "KoG", "Vanilla"}

// tabMatch reports whether a row belongs to tab i. Internet/LAN pass everything
// (LAN draws from a separate scanned source); Favorites checks the fav set; the
// rest filter by gametype.
func tabMatch(tab int, r serverRow, fav map[string]bool) bool {
	gt := strings.ToLower(r.GameType)
	switch tab {
	case tabFavorites:
		return fav[r.Addr]
	case tabDDNet:
		return strings.Contains(gt, "ddnet") || strings.Contains(gt, "ddrace") || strings.Contains(gt, "gores")
	case tabKoG:
		return strings.Contains(gt, "kog") || strings.Contains(gt, "block")
	case tabVanilla:
		return strings.Contains(gt, "dm") || strings.Contains(gt, "ctf") || strings.Contains(gt, "tdm") || strings.Contains(gt, "vanilla")
	default: // Internet, LAN
		return true
	}
}

// Browser is the server-browser model: full list, active tab, search filter and
// selection. The list is filled from the async master fetch goroutine, so all
// access is locked (§V4). Filter/nav are pure and unit-tested.
type Browser struct {
	mu      sync.Mutex
	all     []serverRow
	lanRows []serverRow // LAN-scanned servers (separate source)
	view    []serverRow
	tab     int
	search  []rune
	sel     int
	loading bool
	err     string
	bsearch bool            // search input focused
	fav     map[string]bool // favorite addresses
}

// NewBrowser returns an empty browser.
func NewBrowser() *Browser { return &Browser{fav: map[string]bool{}} }

// SetLoading marks an in-flight fetch.
func (b *Browser) SetLoading(v bool) { b.mu.Lock(); b.loading = v; b.err = ""; b.mu.Unlock() }

// SetError records a fetch failure.
func (b *Browser) SetError(e string) { b.mu.Lock(); b.loading = false; b.err = e; b.mu.Unlock() }

// SetEntries converts and stores the master list, then refilters.
func (b *Browser) SetEntries(entries []master.ServerEntry) {
	rows := make([]serverRow, 0, len(entries))
	for _, e := range entries {
		if len(e.Addresses) == 0 {
			continue
		}
		a := e.Addresses[0]
		rows = append(rows, serverRow{
			Addr:       a.String(),
			Name:       e.Info.Name,
			GameType:   e.Info.GameType,
			MapName:    e.Info.MapName,
			Players:    e.Info.NumClients,
			MaxPlayers: e.Info.MaxClients,
			Passworded: e.Info.Passworded,
			Version:    a.Version,
		})
	}
	b.mu.Lock()
	b.all = rows
	b.loading = false
	b.mu.Unlock()
	b.refilter()
}

// refilter rebuilds the visible slice from tab + search and clamps selection.
func (b *Browser) refilter() {
	b.mu.Lock()
	defer b.mu.Unlock()
	term := strings.ToLower(strings.TrimSpace(string(b.search)))
	source := b.all
	if b.tab == tabLAN {
		source = b.lanRows
	}
	b.view = b.view[:0]
	for _, r := range source {
		if !tabMatch(b.tab, r, b.fav) {
			continue
		}
		if term != "" && !strings.Contains(strings.ToLower(r.Name), term) &&
			!strings.Contains(strings.ToLower(r.MapName), term) {
			continue
		}
		b.view = append(b.view, r)
	}
	b.clampSel()
}

func (b *Browser) clampSel() {
	if b.sel < 0 {
		b.sel = 0
	}
	if b.sel >= len(b.view) {
		b.sel = len(b.view) - 1
	}
	if b.sel < 0 {
		b.sel = 0
	}
}

// Move shifts the selection by d, clamped.
func (b *Browser) Move(d int) { b.mu.Lock(); b.sel += d; b.clampSel(); b.mu.Unlock() }

// SetTab switches category (wrapping) and refilters.
func (b *Browser) SetTab(d int) {
	b.mu.Lock()
	b.tab = (b.tab + d + len(browserTabs)) % len(browserTabs)
	b.sel = 0
	b.mu.Unlock()
	b.refilter()
}

// SearchFocused reports whether the search box has focus.
func (b *Browser) SearchFocused() bool { b.mu.Lock(); defer b.mu.Unlock(); return b.bsearch }

// FocusSearch toggles the search box.
func (b *Browser) FocusSearch(v bool) { b.mu.Lock(); b.bsearch = v; b.mu.Unlock() }

// SearchType edits the search term and refilters.
func (b *Browser) SearchType(r rune) {
	b.mu.Lock()
	b.search = append(b.search, r)
	b.mu.Unlock()
	b.refilter()
}

// SearchBackspace removes the last search rune and refilters.
func (b *Browser) SearchBackspace() {
	b.mu.Lock()
	if n := len(b.search); n > 0 {
		b.search = b.search[:n-1]
	}
	b.mu.Unlock()
	b.refilter()
}

// Selected returns the highlighted row.
func (b *Browser) Selected() (serverRow, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.sel < 0 || b.sel >= len(b.view) {
		return serverRow{}, false
	}
	return b.view[b.sel], true
}

// ToggleFavorite flips the favorite flag for the selected server and returns its
// address (§T45). Empty if nothing selected.
func (b *Browser) ToggleFavorite() string {
	r, ok := b.Selected()
	if !ok {
		return ""
	}
	b.mu.Lock()
	if b.fav[r.Addr] {
		delete(b.fav, r.Addr)
	} else {
		b.fav[r.Addr] = true
	}
	b.mu.Unlock()
	b.refilter()
	return r.Addr
}

// SetLAN installs the LAN-scanned rows and refilters.
func (b *Browser) SetLAN(rows []serverRow) {
	b.mu.Lock()
	b.lanRows = rows
	b.loading = false
	b.mu.Unlock()
	b.refilter()
}

// Tab returns the active tab index.
func (b *Browser) Tab() int { b.mu.Lock(); defer b.mu.Unlock(); return b.tab }

// LoadFavorites reads one address per line; a missing file is not an error.
func (b *Browser) LoadFavorites(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	fav := map[string]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		if a := strings.TrimSpace(line); a != "" {
			fav[a] = true
		}
	}
	b.mu.Lock()
	b.fav = fav
	b.mu.Unlock()
	return nil
}

// SaveFavorites writes the favorite addresses, one per line.
func (b *Browser) SaveFavorites(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b.mu.Lock()
	var sb strings.Builder
	for a := range b.fav {
		sb.WriteString(a)
		sb.WriteByte('\n')
	}
	b.mu.Unlock()
	return os.WriteFile(path, []byte(sb.String()), 0o644)
}

// DrawBrowser renders the full-screen browser overlay (§T18/§T32).
func DrawBrowser(s tcell.Screen, w, h int, b *Browser) {
	b.mu.Lock()
	tab, sel, loading, errStr, search, bsearch := b.tab, b.sel, b.loading, b.err, string(b.search), b.bsearch
	view := make([]serverRow, len(b.view))
	copy(view, b.view)
	fav := make(map[string]bool, len(b.fav))
	for a := range b.fav {
		fav[a] = true
	}
	b.mu.Unlock()

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			s.SetContent(x, y, ' ', nil, tcell.StyleDefault)
		}
	}

	// Tab bar.
	x := 1
	for i, name := range browserTabs {
		st := StyleSystem
		if i == tab {
			st = StyleStatus
		}
		x += drawStr(s, x, 0, w-x, st, " "+name+" ") + 1
	}

	// Search / status line.
	statusY := 1
	switch {
	case loading:
		drawStr(s, 1, statusY, w-1, StyleSystem, "fetching server list…")
	case errStr != "":
		drawStr(s, 1, statusY, w-1, StyleSelf, "fetch error: "+errStr)
	default:
		cur := fmt.Sprintf("/%s", search)
		if bsearch {
			cur += "_"
		}
		drawStr(s, 1, statusY, w-1, StyleChat, fmt.Sprintf("%-40s  %d servers   [Enter]join [/]search [f]fav [←/→]tab [B/Esc]close", cur, len(view)))
	}

	// Header + rows.
	header := fmt.Sprintf(" %-30s %-10s %-14s %5s  %s", "name", "gametype", "map", "plrs", "ver")
	drawStr(s, 1, 3, w-1, StyleStatus, header)
	top := 0
	if sel >= h-5 {
		top = sel - (h - 5)
	}
	row := 4
	for i := top; i < len(view) && row < h; i++ {
		r := view[i]
		ver := "0.6"
		if r.Version == packet.Version07 {
			ver = "0.7"
		}
		mark := " "
		if fav[r.Addr] {
			mark = "*"
		}
		if r.Passworded {
			mark = "#"
		}
		line := fmt.Sprintf("%s%-30s %-10s %-14s %2d/%-2d  %s",
			mark, padCol(r.Name, 30), padCol(r.GameType, 10), padCol(r.MapName, 14), r.Players, r.MaxPlayers, ver)
		st := StyleChat
		if i == sel {
			st = StyleStatus
		}
		drawStr(s, 1, row, w-1, st, line)
		row++
	}
}
