package tui

import (
	"strings"
	"testing"
)

// §T61/§V33: composeReply picks a context-aware, author-addressed reply across
// smalltalk / ask-to-ask / greeting / bye / insult / no-context, and declines
// when nothing fits.
func TestComposeReply(t *testing.T) {
	const self = "nameless"
	cases := []struct {
		msg     string
		wantSub string // substring the reply must contain
		wantOK  bool
	}{
		{"nameless how are you?", "good, and you", true},
		{"nameless wie gehts", "gut, und dir", true},
		{"nameless ca va", "je vais bien", true},
		{"nameless can i ask you something", "just ask", true},
		{"hey nameless", "hi", true},
		{"nameless cya", "cya", true},
		{"nameless you noob", ":)", true},
		{"nameless", "?", true},                                 // no-context bare ping
		{"nameless:", "?", true},                                // colon form
		{"nameless can you help me carry this part", "", false}, // real content → no canned reply
	}
	for _, c := range cases {
		got, ok := composeReply(c.msg, "bob", self)
		if ok != c.wantOK {
			t.Errorf("composeReply(%q) ok=%v want %v (got %q)", c.msg, ok, c.wantOK, got)
			continue
		}
		if ok {
			if !strings.HasPrefix(got, "bob") && !strings.Contains(got, "bob") {
				t.Errorf("composeReply(%q) = %q should address the author", c.msg, got)
			}
			if !strings.Contains(got, c.wantSub) {
				t.Errorf("composeReply(%q) = %q want substring %q", c.msg, got, c.wantSub)
			}
		}
	}

	// Empty author never replies.
	if _, ok := composeReply("hi nameless", "", self); ok {
		t.Error("empty author must not reply")
	}
}
