package tui

import "testing"

// §T64/§V36: filterDecision hides only matching incoming lines, never own lines,
// and is off by default; mode 2 requests an auto-reply.
func TestFilterDecision(t *testing.T) {
	cfg := NewConfig() // ChatSpamFilter default 0
	cfg.Filters = []string{"buy gold", "discord.gg"}

	// Off by default → nothing hidden even if it matches.
	if hide, _ := filterDecision("buy gold now", false, cfg); hide {
		t.Error("filter must be off by default")
	}

	cfg.ChatSpamFilter = 1
	if hide, ar := filterDecision("come buy GOLD cheap", false, cfg); !hide || ar {
		t.Errorf("mode1 match: hide=%v autoReply=%v want true,false", hide, ar)
	}
	if hide, _ := filterDecision("nice game everyone", false, cfg); hide {
		t.Error("non-matching line must not be hidden")
	}
	// Own line never filtered.
	if hide, _ := filterDecision("buy gold", true, cfg); hide {
		t.Error("own line must never be filtered")
	}

	// Mode 2 → hide + autoreply.
	cfg.ChatSpamFilter = 2
	if hide, ar := filterDecision("join discord.gg/spam", false, cfg); !hide || !ar {
		t.Errorf("mode2: hide=%v autoReply=%v want true,true", hide, ar)
	}

	// Insult filter only when enabled.
	cfg.Filters = nil
	if hide, _ := filterDecision("you noob", false, cfg); hide {
		t.Error("insult hidden without FilterInsults")
	}
	cfg.FilterInsults = true
	if hide, _ := filterDecision("you noob", false, cfg); !hide {
		t.Error("insult not hidden with FilterInsults on")
	}
}

// §T64: filter list add/del/dedup.
func TestFilterListOps(t *testing.T) {
	var f []string
	f, ok := addFilter(f, "spam")
	if !ok || len(f) != 1 {
		t.Fatalf("add failed: %v %v", f, ok)
	}
	if _, ok := addFilter(f, "SPAM"); ok {
		t.Error("duplicate (case-insensitive) should not add")
	}
	f, _ = addFilter(f, "ads")
	f, ok = delFilter(f, "spam")
	if !ok || len(f) != 1 || f[0] != "ads" {
		t.Errorf("del failed: %v %v", f, ok)
	}
	if _, ok := delFilter(f, "missing"); ok {
		t.Error("del missing should report not found")
	}
}
