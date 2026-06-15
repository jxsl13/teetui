// Command teetui is a cross-platform terminal Teeworlds/DDNet client. It
// re-implements the chillerbot-ux terminal UI on top of the pure-Go twclient
// library, rendering the live game, chat and scoreboard with tcell.
//
// This file does only two things (§C21/§T86): import the feature modules (each
// self-registers in init(); add a feature = add one import line) and start the
// base client. All behavior lives in internal/tui (core) and features/*.
package main

import (
	"github.com/jxsl13/teetui/internal/tui"

	_ "github.com/jxsl13/teetui/features/chatfilter"
	_ "github.com/jxsl13/teetui/features/chillpw"
	_ "github.com/jxsl13/teetui/features/cmdhook"
	_ "github.com/jxsl13/teetui/features/lastping"
	_ "github.com/jxsl13/teetui/features/replytoping"
	_ "github.com/jxsl13/teetui/features/responders"
	_ "github.com/jxsl13/teetui/features/team"
	_ "github.com/jxsl13/teetui/features/warlist"
)

func main() { tui.Main() }
