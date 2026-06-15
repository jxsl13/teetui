package tui

import "testing"

// §T60/§V33: word-boundary matching is case-insensitive and unicode-aware, and
// must not match inside a larger word.
func TestFindWord(t *testing.T) {
	cases := []struct {
		text, word string
		want       bool
	}{
		{"hello there", "hello", true},
		{"oh, HELLO!", "hello", true},
		{"helloween party", "hello", false}, // no boundary
		{"say hi", "hi", true},
		{"this is high", "hi", false},
		{"привет всем", "привет", true},
		{"приветствие", "привет", false}, // boundary inside cyrillic word
		{"", "hi", false},
		{"hi", "", false},
	}
	for _, c := range cases {
		if got := findWord(c.text, c.word); got != c.want {
			t.Errorf("findWord(%q,%q) = %v want %v", c.text, c.word, got, c.want)
		}
	}
}

// §T60/§V33: the intent classifiers cover en/de/fr/ru and don't cross-fire on
// unrelated text.
func TestLangClassifiers(t *testing.T) {
	if !isGreeting("hey bob") || !isGreeting("Hallo") || !isGreeting("привет") {
		t.Error("greeting misses a known form")
	}
	if isGreeting("history channel") {
		t.Error("greeting false-positive on 'history'")
	}
	if !isBye("cya later") || !isBye("tschau") || !isBye("good night") {
		t.Error("bye misses a known form")
	}
	if !isAskToAsk("can i ask you something") || !isAskToAsk("kann ich was fragen") {
		t.Error("ask-to-ask misses")
	}
	if !isQuestionWhy("why did you do that") || !isQuestionWhy("warum") || !isQuestionWhy("почему") {
		t.Error("why-question misses")
	}
	if !isQuestionHow("how are you") || !isQuestionHow("wie gehts") {
		t.Error("how-question misses")
	}
	if !isQuestionWhoWhichWhat("who are you") || !isQuestionWhichWhat("what is this") {
		t.Error("who/what-question misses")
	}
	if isInsult("nice shot") {
		t.Error("insult false-positive")
	}
	if !isInsult("you noob") {
		t.Error("insult misses 'noob'")
	}
}
