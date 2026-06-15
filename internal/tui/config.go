package tui

import (
	"os"
	"path/filepath"
	"strings"
)

// Config holds runtime client-side settings, settable from the local console as
// "cvars" (← chillerbot config variables, §T39). The tapped-out fields drive the
// optional auto-reply when pinged while AFK (§T40); it is off by default because
// teetui is an interactive client, not a headless bot.
type Config struct {
	SilentChatCmds bool   // apply !commands locally without sending them (§V14)
	TappedOut      bool   // auto-reply with TappedOutText when pinged (§T40)
	TappedOutText  string // the auto tapped-out reply
}

// NewConfig returns the default configuration (§T39/§T40 defaults).
func NewConfig() *Config {
	return &Config{
		SilentChatCmds: true, // cl_silent_chat_commands default on (§V14)
		TappedOut:      false,
		TappedOutText:  "I'm currently tapped out (afk)",
	}
}

// cvar is one console-settable config variable: a name, one line of help text,
// and string get/set accessors so the console can treat all vars uniformly.
type cvar struct {
	name string
	help string
	get  func(*Config) string
	set  func(*Config, string)
}

// cvars is the registry of console-settable config variables (§T39/§T40).
var cvars = []cvar{
	{"cl_silent_chat_commands", "apply !war/!peace/… locally without sending to server (0/1)",
		func(c *Config) string { return b2s(c.SilentChatCmds) },
		func(c *Config, v string) { c.SilentChatCmds = s2b(v) }},
	{"cl_tapped_out_message", "auto-reply with the tapped-out message when pinged (0/1)",
		func(c *Config) string { return b2s(c.TappedOut) },
		func(c *Config, v string) { c.TappedOut = s2b(v) }},
	{"cl_tapped_out_message_text", "the text sent by the tapped-out auto-reply",
		func(c *Config) string { return c.TappedOutText },
		func(c *Config, v string) { c.TappedOutText = v }},
}

// findCvar returns the cvar named name, or nil.
func findCvar(name string) *cvar {
	for i := range cvars {
		if cvars[i].name == name {
			return &cvars[i]
		}
	}
	return nil
}

// b2s / s2b convert a bool config value to/from its console string form. s2b
// treats "1", "true", "on", "yes" (case-insensitive) as true; everything else
// is false.
func b2s(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

func s2b(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "on", "yes":
		return true
	default:
		return false
	}
}

// configDir returns the teetui config root (~/.config/teetui per §I), honoring
// XDG_CONFIG_HOME, and ensures it exists.
func configDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "teetui")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// historyPath returns the on-disk history file for an input mode slug.
func historyPath(modeSlug string) (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "history", modeSlug+".txt"), nil
}
