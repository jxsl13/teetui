package tui

import "testing"

// §T15: single match → ghost suffix; multiple → list hint; none/empty → nothing.
func TestCompletionPreview(t *testing.T) {
	cands := []string{"alice", "albert", "bob"}

	if g, l := completionPreview("ali", cands); g != "ce" || l != "" {
		t.Errorf("single = ghost %q list %q want ce / ''", g, l)
	}
	if g, l := completionPreview("al", cands); g != "" || l == "" {
		t.Errorf("multiple = ghost %q list %q want '' / non-empty", g, l)
	}
	if g, l := completionPreview("zz", cands); g != "" || l != "" {
		t.Errorf("no match = %q %q want empty", g, l)
	}
	if g, l := completionPreview("", cands); g != "" || l != "" {
		t.Errorf("empty prefix = %q %q want empty", g, l)
	}
}

// §T15: currentWord isolates the token ending at the cursor.
func TestCurrentWord(t *testing.T) {
	runes := []rune("hi al")
	start, w := currentWord(runes, len(runes))
	if start != 3 || w != "al" {
		t.Errorf("currentWord = %d,%q want 3,al", start, w)
	}
}
