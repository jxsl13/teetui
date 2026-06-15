package tui

import (
	"bufio"
	"os"
	"strings"
	"unicode"
)

// parsePasswords parses a chillpw secrets file (← chillerbot cl_password_file)
// into an addr→password map (§T68). Format: one entry per line,
// "<addr> <password>", whitespace-separated; the password may itself contain
// spaces (everything after the first field). Blank lines and '#' comments are
// ignored. The password is never logged anywhere (§V38).
func parsePasswords(r *bufio.Scanner) map[string]string {
	out := map[string]string{}
	for r.Scan() {
		line := strings.TrimSpace(r.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Split on the first whitespace run (space or tab) so a password may
		// itself contain spaces.
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

// loadPasswords reads and parses the secrets file at path. A missing file yields
// an empty map and no error (opt-in, §V38).
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

// lookupPassword finds the password for addr, trying an exact match first then a
// host-only match (entry without a port) so "ddnet.example:8303" can be keyed by
// either form (§T68).
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
