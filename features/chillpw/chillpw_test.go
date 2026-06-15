package chillpw

import (
	"bufio"
	"strings"
	"testing"
)

// §T68/§V38: secrets-file parsing ignores blanks/comments, keeps passwords with
// spaces, and lookup matches exact addr then host-only.
func TestChillpwParseLookup(t *testing.T) {
	const file = `
# my servers
ddnet.example:8303 hunter2
1.2.3.4:8303	tab-pw with spaces
bad-line-no-pw
host-only s3cret
`
	m := parsePasswords(bufio.NewScanner(strings.NewReader(file)))

	if pw, ok := lookupPassword(m, "ddnet.example:8303"); !ok || pw != "hunter2" {
		t.Errorf("exact match = %q,%v", pw, ok)
	}
	if pw, ok := lookupPassword(m, "1.2.3.4:8303"); !ok || pw != "tab-pw with spaces" {
		t.Errorf("tab + spaces pw = %q,%v", pw, ok)
	}
	// host-only entry matches an addr with a port.
	if pw, ok := lookupPassword(m, "host-only:8303"); !ok || pw != "s3cret" {
		t.Errorf("host-only fallback = %q,%v", pw, ok)
	}
	// malformed line ignored.
	if _, ok := lookupPassword(m, "bad-line-no-pw"); ok {
		t.Error("malformed line should not yield a password")
	}
	// unknown server → no secret.
	if _, ok := lookupPassword(m, "other:8303"); ok {
		t.Error("unknown server must not match")
	}
}

// §T68: a missing secrets file is not an error (opt-in).
func TestChillpwMissingFile(t *testing.T) {
	m, err := loadPasswords("/nonexistent/teetui/chillpw.txt")
	if err != nil {
		t.Fatalf("missing file should be no error: %v", err)
	}
	if len(m) != 0 {
		t.Errorf("missing file should yield empty map, got %d", len(m))
	}
}
