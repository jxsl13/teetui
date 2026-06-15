package tui

import (
	"strings"
	"unicode"
)

// langparser is a small port of chillerbot-ux's CLangParser (engine/shared/
// chillerbot/langparser.cpp): cheap, dependency-free heuristics that classify an
// incoming chat line so the reply-to-ping engine (§T61) can pick a sensible
// canned response. It is intentionally fuzzy and multi-lingual (en/de/fr/ru),
// matching the reference's substring/word approach — not real NLP (§V33).

// findWord reports whether word occurs in text on word boundaries
// (case-insensitive), so "hello" matches "hello!" / "oh hello" but not
// "helloween". Boundaries are any non-letter, non-digit rune (unicode-aware, so
// it works for Cyrillic/accented text too).
func findWord(text, word string) bool {
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

// findAnyWord reports whether any of words occurs in text (word-boundary).
func findAnyWord(text string, words ...string) bool {
	for _, w := range words {
		if findWord(text, w) {
			return true
		}
	}
	return false
}

// containsAny reports whether any substring occurs in text (case-insensitive),
// for phrases where word boundaries are too strict ("how r u").
func containsAny(text string, subs ...string) bool {
	low := strings.ToLower(text)
	for _, s := range subs {
		if strings.Contains(low, strings.ToLower(s)) {
			return true
		}
	}
	return false
}

// isGreeting reports whether msg is a greeting (en/qq/rus, ← IsGreeting*).
func isGreeting(msg string) bool {
	return findAnyWord(msg, "hi", "hello", "hey", "heya", "hai", "yo", "henlo",
		"hallo", "moin", "servus", "salut", "bonjour", "ola", "hola") ||
		containsAny(msg, "qq", "o/", "\\o") ||
		findAnyWord(msg, "привет", "прив", "ку", "хай", "здарова", "здаров")
}

// isBye reports whether msg is a farewell (← IsBye).
func isBye(msg string) bool {
	return findAnyWord(msg, "bye", "cya", "cu", "gn", "cee", "ciao", "tschau",
		"tschuss", "ade", "adieu", "aurevoir") ||
		containsAny(msg, "good night", "good bye", "see you", "bb ", "bb!", "пока", "до встречи")
}

// isInsult reports whether msg is a (mild) insult (← IsInsult). Kept short and
// non-vulgar; the reply engine answers neutrally rather than escalating.
func isInsult(msg string) bool {
	return findAnyWord(msg, "noob", "nub", "trash", "bad", "loser", "ez", "bot",
		"idiot", "dumb", "scrub")
}

// isAskToAsk reports the "can I ask you something?" pattern (en+de, ←
// IsAskToAsk[German]) so the engine can answer "just ask".
func isAskToAsk(msg string) bool {
	return containsAny(msg,
		"can i ask", "may i ask", "can i ask you", "ask you something",
		"darf ich", "kann ich was fragen", "kann ich dich was fragen", "eine frage",
		"can i question", "ne frage")
}

// the question classifiers mirror IsQuestion* — a leading/contained interrogative
// plus a question mark (or a short message) reads as a question.

func hasQuestionMark(msg string) bool { return strings.Contains(msg, "?") }

// isQuestionWhy (← IsQuestionWhy): why / warum / pourquoi / почему.
func isQuestionWhy(msg string) bool {
	return findAnyWord(msg, "why", "warum", "wieso", "weshalb", "pourquoi", "почему", "зачем")
}

// isQuestionHow (← IsQuestionHow): how / wie / comment / как.
func isQuestionHow(msg string) bool {
	return findAnyWord(msg, "how", "wie", "comment", "как")
}

// isQuestionWhichWhat (← IsQuestionWhichWhat): what / which / was / welche / quoi.
func isQuestionWhichWhat(msg string) bool {
	return findAnyWord(msg, "what", "which", "was", "welche", "welcher", "quoi", "quel", "что", "какой")
}

// isQuestionWhoWhichWhat (← IsQuestionWhoWhichWhat): adds who.
func isQuestionWhoWhichWhat(msg string) bool {
	return isQuestionWhichWhat(msg) || findAnyWord(msg, "who", "wer", "qui", "кто")
}
