// Package lang is a small natural-language heuristics library ported from
// chillerbot-ux's CLangParser (engine/shared/chillerbot/langparser.cpp). It
// classifies short chat lines (greetings, byes, insults, ask-to-ask, question
// types) across en/de/fr/ru so reply features can pick a canned response. It is
// a plain library (NOT a feature, §C21/§T77) imported by the chat features —
// fuzzy substring/word matching, not real NLP.
//
// Matching is FOLD-NORMALIZED (§C28/§V64): every comparison runs on a fold key
// that strips accents and case-folds via the Go-native golang.org/x/text, so
// "café"≈"cafe", "tschüss"≈"tschuss", composed≈decomposed "é", and ß / Greek
// sigma / Turkish-i fold correctly (where strings.ToLower would not).
package lang

import (
	"strings"
	"unicode"

	"golang.org/x/text/cases"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// foldKey returns the accent-stripped, case-folded form of s used for all
// matching (§C28/§V64): NFD-decompose, drop combining marks (Mn), recompose NFC,
// then Unicode case-fold. The transformer and Caser are created fresh per call —
// they are NOT safe for concurrent reuse and lang runs on the dispatch goroutine
// (§V4/§V65). Chat-rate only, off the render hot path (§V7 n/a).
func foldKey(s string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	out, _, err := transform.String(t, s)
	if err != nil {
		out = s
	}
	return cases.Fold().String(out)
}

// FindWord reports whether word occurs in text on word boundaries, accent- and
// case-insensitive and unicode-aware (§V64) — "hello" matches "hello!" but not
// "helloween"; "café" matches "cafe"; "приветствие" does not match "привет".
func FindWord(text, word string) bool {
	if word == "" {
		return false
	}
	lt := []rune(foldKey(text))
	lw := []rune(foldKey(word))
	if len(lw) == 0 { // word folded to nothing (e.g. only combining marks)
		return false
	}
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

// ContainsAny reports whether any substring occurs in text (accent- and
// case-insensitive, §V64), for phrases where word boundaries are too strict
// ("how r u"). The text is folded once; each sub is folded before the search.
func ContainsAny(text string, subs ...string) bool {
	low := foldKey(text)
	for _, s := range subs {
		if strings.Contains(low, foldKey(s)) {
			return true
		}
	}
	return false
}

// ContainsName reports whether msg mentions name (accent- and case-insensitive
// ping detection, §V64). Empty name never matches.
func ContainsName(msg, name string) bool {
	if name == "" {
		return false
	}
	return strings.Contains(foldKey(msg), foldKey(name))
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
		"tschüss", "ade", "adieu", "aurevoir") ||
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
