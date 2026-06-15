package tui

import "github.com/jxsl13/teetui/lang"

// Transitional aliases to the lang library (§T77). Core chat code still calls
// the short unexported names; these forward to lang. They disappear as the chat
// logic is extracted into feature packages (T79-T82), which import lang directly.
var (
	containsAny     = lang.ContainsAny
	containsName    = lang.ContainsName
	findWord        = lang.FindWord
	findAnyWord     = lang.FindAnyWord
	hasQuestionMark = lang.HasQuestionMark
	isGreeting      = lang.IsGreeting
	isBye           = lang.IsBye
	isInsult        = lang.IsInsult
	isAskToAsk      = lang.IsAskToAsk
	isQuestionWhy   = lang.IsQuestionWhy
)
