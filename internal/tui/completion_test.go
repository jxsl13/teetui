package tui

import (
	"reflect"
	"testing"
)

// §T15: completion is prefix-based, case-insensitive, sorted and de-duplicated.
func TestCompleteMatches(t *testing.T) {
	cands := []string{"alice", "Albert", "bob", "alice", ""}

	if got := completeMatches("al", cands); !reflect.DeepEqual(got, []string{"Albert", "alice"}) {
		t.Errorf("prefix al = %v", got)
	}
	if got := completeMatches("BO", cands); !reflect.DeepEqual(got, []string{"bob"}) {
		t.Errorf("prefix BO = %v", got)
	}
	if got := completeMatches("zz", cands); len(got) != 0 {
		t.Errorf("no match = %v", got)
	}
	// empty prefix matches all distinct non-empty candidates, sorted.
	if got := completeMatches("", cands); !reflect.DeepEqual(got, []string{"Albert", "alice", "bob"}) {
		t.Errorf("empty prefix = %v", got)
	}
	// console commands self-complete.
	if got := completeMatches("he", consoleCommands); !reflect.DeepEqual(got, []string{"help"}) {
		t.Errorf("console he = %v", got)
	}
}
