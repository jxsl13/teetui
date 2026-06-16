package feature

import "testing"

// §T102/§V63: NopAPI satisfies the full API, and each capability sub-interface is
// independently satisfiable — a consumer may depend on the minimal slice it needs
// (interface segregation), not the whole API.
var (
	_ API             = NopAPI{}
	_ ChatSender      = NopAPI{}
	_ ActionDoer      = NopAPI{}
	_ Logger          = NopAPI{}
	_ StateReader     = NopAPI{}
	_ ConfigStore     = NopAPI{}
	_ ActionRegistry  = NopAPI{}
	_ UIRegistry      = NopAPI{}
	_ ServiceRegistry = NopAPI{}
	_ Paths           = NopAPI{}
)

// sendOne depends ONLY on ChatSender, proving a handler/consumer need not take
// the whole API (§V63).
func sendOne(s ChatSender) { s.SendChat("hi", false) }

func TestSubInterfaceConsumer(t *testing.T) {
	sendOne(NopAPI{}) // compiles + runs → minimal-surface dependency works
	var a API = NopAPI{}
	a.Log("ok")
}
