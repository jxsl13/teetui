// Package lang is a small, dependency-free natural-language heuristics library
// ported from chillerbot-ux's CLangParser (engine/shared/chillerbot/
// langparser.cpp). It classifies short chat lines (greetings, byes, insults,
// ask-to-ask, question types) across en/de/fr/ru so reply features can pick a
// canned response. It is a plain library (NOT a feature, §C21/§T77) imported by
// the chat features — fuzzy substring/word matching, not real NLP.
package lang

import (
	"strings"
	"unicode"
)

// FindWord reports whether word occurs in text on word boundaries
// (case-insensitive), unicode-aware — "hello" matches "hello!" but not
// "helloween" / "приветствие".
func FindWord(text, word string) bool {
	if word == "" {
		return false
	}
	lt := []rune(strings.ToLower(text))
	lw := []rune(strings.ToLower(word))
	for i := 0; i+len(lw) <= len(lt); i++ {
		if !runeSliceEq(lt[i:i+len(lw)], lw) {
			continue
		}
		beforeOK := i == 0 || !isWordRune(lt[i-1])
		afterOK := i+len(lw) == len(lt) || !isWordRune(lt[i+len(lw)])
		if beforeOK && afterOK {
			return true
		}
	}
	return false
}

func isWordRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }

func runeSliceEq(a, b []rune) bool {
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// FindAnyWord reports whether any of words occurs in text (word-boundary).
func FindAnyWord(text string, words ...string) bool {
	for _, w := range words {
		if FindWord(text, w) {
			return true
		}
	}
	return false
}

// ContainsAny reports whether any substring occurs in text (case-insensitive),
// for phrases where word boundaries are too strict ("how r u").
func ContainsAny(text string, subs ...string) bool {
	low := strings.ToLower(text)
	for _, s := range subs {
		if strings.Contains(low, strings.ToLower(s)) {
			return true
		}
	}
	return false
}

// ContainsName reports whether msg mentions name (case-insensitive ping
// detection). Empty name never matches.
func ContainsName(msg, name string) bool {
	if name == "" {
		return false
	}
	return strings.Contains(strings.ToLower(msg), strings.ToLower(name))
}

// IsGreeting reports whether msg is a greeting (en/qq/rus).
func IsGreeting(msg string) bool {
	return FindAnyWord(msg, "hi", "hello", "hey", "heya", "hai", "yo", "henlo",
		"hallo", "moin", "servus", "salut", "bonjour", "ola", "hola") ||
		ContainsAny(msg, "qq", "o/", "\\o") ||
		FindAnyWord(msg, "привет", "прив", "ку", "хай", "здарова", "здаров")
}

// IsBye reports whether msg is a farewell.
func IsBye(msg string) bool {
	return FindAnyWord(msg, "bye", "cya", "cu", "gn", "cee", "ciao", "tschau",
		"tschuss", "ade", "adieu", "aurevoir") ||
		ContainsAny(msg, "good night", "good bye", "see you", "bb ", "bb!", "пока", "до встречи")
}

// IsInsult reports whether msg is a (mild) insult. Non-vulgar shortlist; the
// reply feature answers neutrally rather than escalating.
func IsInsult(msg string) bool {
	return FindAnyWord(msg, "noob", "nub", "trash", "bad", "loser", "ez", "bot",
		"idiot", "dumb", "scrub")
}

// IsAskToAsk reports the "can I ask you something?" pattern (en+de).
func IsAskToAsk(msg string) bool {
	return ContainsAny(msg,
		"can i ask", "may i ask", "can i ask you", "ask you something",
		"darf ich", "kann ich was fragen", "kann ich dich was fragen", "eine frage",
		"can i question", "ne frage")
}

// HasQuestionMark reports whether msg contains '?'.
func HasQuestionMark(msg string) bool { return strings.Contains(msg, "?") }

// IsQuestionWhy: why / warum / pourquoi / почему.
func IsQuestionWhy(msg string) bool {
	return FindAnyWord(msg, "why", "warum", "wieso", "weshalb", "pourquoi", "почему", "зачем")
}

// IsQuestionHow: how / wie / comment / как.
func IsQuestionHow(msg string) bool {
	return FindAnyWord(msg, "how", "wie", "comment", "как")
}

// IsQuestionWhichWhat: what / which / was / welche / quoi / что.
func IsQuestionWhichWhat(msg string) bool {
	return FindAnyWord(msg, "what", "which", "was", "welche", "welcher", "quoi", "quel", "что", "какой")
}

// IsQuestionWhoWhichWhat: adds who.
func IsQuestionWhoWhichWhat(msg string) bool {
	return IsQuestionWhichWhat(msg) || FindAnyWord(msg, "who", "wer", "qui", "кто")
}
