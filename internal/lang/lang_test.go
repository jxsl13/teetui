package lang

import "testing"

// §T77: word-boundary matching is case-insensitive and unicode-aware, never
// matching inside a larger word.
func TestFindWord(t *testing.T) {
	cases := []struct {
		text, word string
		want       bool
	}{
		{"hello there", "hello", true},
		{"oh, HELLO!", "hello", true},
		{"helloween party", "hello", false},
		{"say hi", "hi", true},
		{"this is high", "hi", false},
		{"привет всем", "привет", true},
		{"приветствие", "привет", false},
		{"ПРИВЕТ всем", "привет", true}, // §V64 case-fold (Cyrillic)
		{"", "hi", false},
		{"hi", "", false},
		// §V64 accent-fold: query without accents matches accented text.
		{"nice café here", "cafe", true},
		{"über alles", "uber", true},
		{"helloween", "hello", false}, // boundary kept after folding
	}
	for _, c := range cases {
		if got := FindWord(c.text, c.word); got != c.want {
			t.Errorf("FindWord(%q,%q) = %v want %v", c.text, c.word, got, c.want)
		}
	}
}

// §T77: the intent classifiers cover en/de/fr/ru and don't cross-fire.
func TestClassifiers(t *testing.T) {
	if !HasGreeting("hey bob") || !HasGreeting("Hallo") || !HasGreeting("привет") {
		t.Error("greeting misses a known form")
	}
	if HasGreeting("history channel") {
		t.Error("greeting false-positive on 'history'")
	}
	if !HasFarewell("cya later") || !HasFarewell("tschau") || !HasFarewell("good night") {
		t.Error("bye misses a known form")
	}
	if !HasAskToAsk("can i ask you something") || !HasAskToAsk("kann ich was fragen") {
		t.Error("ask-to-ask misses")
	}
	if !HasWhy("why") || !HasWhy("warum") || !HasWhy("почему") {
		t.Error("why-question misses")
	}
	if !HasHow("how are you") || !HasHow("wie gehts") {
		t.Error("how-question misses")
	}
	if !HasWhoWhatWhich("who are you") || !HasWhatWhich("what is this") {
		t.Error("who/what-question misses")
	}
	if HasInsult("nice shot") || !HasInsult("you noob") {
		t.Error("insult classify wrong")
	}
	if !ContainsAny("HOW r u", "how r u") || !HasQuestionMark("ok?") {
		t.Error("containsAny/questionMark wrong")
	}
}

// §C28/§V64: matching is accent- and case-insensitive and NFC/NFD-agnostic.
func TestFoldNormalizedMatching(t *testing.T) {
	// composed "é" (U+00E9) vs decomposed "e"+U+0301 must both match "cafe".
	composed := "café"
	decomposed := "cafe\u0301" // e + combining acute (NFD)
	if !FindWord(composed+" time", "cafe") || !FindWord(decomposed+" time", "cafe") {
		t.Error("composed/decomposed accent not folded")
	}
	// The umlaut in "tschüss" folds away → real spelling matches the bye list.
	if !HasFarewell("tschüss") || !HasFarewell("tschuss") {
		t.Error("tschüss/tschuss not folded in HasFarewell")
	}
	// ContainsName is accent- and case-insensitive.
	if !ContainsName("hey JöRG!", "jorg") {
		t.Error("ContainsName not fold-normalized")
	}
	// Accent folding must not break word boundaries.
	if FindWord("cafeteria", "cafe") {
		t.Error("boundary lost: 'cafe' matched inside 'cafeteria'")
	}
}
