// Package chillpw is the opt-in rcon auto-login feature (← chillerbot chillpw,
// §T84/§T68/§V38). On connect it matches the server address against a local
// secrets file and rcon-logs-in with the matching password. The secret is never
// logged. Self-registering module: blank-import to enable; off unless
// cl_chillpw is set and the file holds an entry for the server.
package chillpw

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/jxsl13/teetui/feature"
)

// parsePasswords parses a secrets file into addr→password (one entry per line,
// "<addr> <password>", '#' comments; the password may contain spaces).
func parsePasswords(r *bufio.Scanner) map[string]string {
	out := map[string]string{}
	for r.Scan() {
		line := strings.TrimSpace(r.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		i := strings.IndexFunc(line, unicode.IsSpace)
		if i < 0 {
			continue
		}
		addr := strings.TrimSpace(line[:i])
		pw := strings.TrimSpace(line[i:])
		if addr == "" || pw == "" {
			continue
		}
		out[addr] = pw
	}
	return out
}

// loadPasswords reads+parses the secrets file; a missing file is no error.
func loadPasswords(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, err
	}
	defer f.Close()
	return parsePasswords(bufio.NewScanner(f)), nil
}

// lookupPassword finds the password for addr (exact, then host-only).
func lookupPassword(m map[string]string, addr string) (string, bool) {
	if pw, ok := m[addr]; ok {
		return pw, true
	}
	host := addr
	if i := strings.LastIndex(addr, ":"); i >= 0 {
		host = addr[:i]
	}
	if pw, ok := m[host]; ok {
		return pw, true
	}
	return "", false
}

type chillpwFeature struct{}

func (chillpwFeature) Name() string { return "chillpw" }

func (chillpwFeature) Init(h feature.Host) error {
	h.DefineConfig("cl_chillpw", "0", "auto rcon-login from the secrets file on connect (0/1)")
	h.DefineConfig("cl_password_file", "chillpw.txt", "path to the chillpw secrets file (addr password per line)")
	return nil
}

func (chillpwFeature) OnConnect(h feature.Host) {
	if on, _ := h.Config("cl_chillpw"); on != "1" && on != "true" && on != "on" {
		return
	}
	path, _ := h.Config("cl_password_file")
	if path == "" {
		return
	}
	if !filepath.IsAbs(path) {
		if dp := h.DataPath(path); dp != "" {
			path = dp
		}
	}
	m, err := loadPasswords(path)
	if err != nil {
		h.Log("chillpw: cannot read secrets file")
		return
	}
	pw, ok := lookupPassword(m, h.Server())
	if !ok {
		return // no secret for this server
	}
	h.Log("chillpw: rcon auto-login for " + h.Server())
	h.RconLogin(pw) // async; secret never logged
}

func init() { feature.Register(chillpwFeature{}) }
