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
		{"", "hi", false},
		{"hi", "", false},
	}
	for _, c := range cases {
		if got := FindWord(c.text, c.word); got != c.want {
			t.Errorf("FindWord(%q,%q) = %v want %v", c.text, c.word, got, c.want)
		}
	}
}

// §T77: the intent classifiers cover en/de/fr/ru and don't cross-fire.
func TestClassifiers(t *testing.T) {
	if !IsGreeting("hey bob") || !IsGreeting("Hallo") || !IsGreeting("привет") {
		t.Error("greeting misses a known form")
	}
	if IsGreeting("history channel") {
		t.Error("greeting false-positive on 'history'")
	}
	if !IsBye("cya later") || !IsBye("tschau") || !IsBye("good night") {
		t.Error("bye misses a known form")
	}
	if !IsAskToAsk("can i ask you something") || !IsAskToAsk("kann ich was fragen") {
		t.Error("ask-to-ask misses")
	}
	if !IsQuestionWhy("why") || !IsQuestionWhy("warum") || !IsQuestionWhy("почему") {
		t.Error("why-question misses")
	}
	if !IsQuestionHow("how are you") || !IsQuestionHow("wie gehts") {
		t.Error("how-question misses")
	}
	if !IsQuestionWhoWhichWhat("who are you") || !IsQuestionWhichWhat("what is this") {
		t.Error("who/what-question misses")
	}
	if IsInsult("nice shot") || !IsInsult("you noob") {
		t.Error("insult classify wrong")
	}
	if !ContainsAny("HOW r u", "how r u") || !HasQuestionMark("ok?") {
		t.Error("containsAny/questionMark wrong")
	}
}
