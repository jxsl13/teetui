package tui

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Config holds runtime client-side settings, settable from the local console as
// "cvars" (← chillerbot config variables, §T39). The tapped-out fields drive the
// optional auto-reply when pinged while AFK (§T40); it is off by default because
// teetui is an interactive client, not a headless bot.
type Config struct {
	SilentChatCmds bool     // apply !commands locally without sending them (§V14)
	TappedOut      bool     // auto-reply with TappedOutText when pinged (§T40)
	TappedOutText  string   // the auto tapped-out reply
	AutoReply      bool     // auto-reply with AutoReplyMsg on every ping (§T61)
	AutoReplyMsg   string   // cl_auto_reply_msg template (%n → author)
	ShowLastPing   bool     // show the most recent ping in the status bar (§T63)
	ChatSpamFilter int      // 0=off 1=hide 2=hide+autoreply (§T64)
	FilterInsults  bool     // also hide insults when ChatSpamFilter>0 (§T64)
	Filters        []string // user chat filter substrings (§T64)
	WarListReload  int      // reload warlist every N seconds; 0=off (§T66)
	Chillpw        bool     // auto rcon-login from the secrets file on connect (§T68)
	PasswordFile   string   // chillpw secrets file (addr→password); never logged (§T68)
	MaxFPS         int      // cap render repaints/sec; 0=unlimited (§T74)
	LogLines       int      // log-band rows when the visual is on (§T88)
}

// NewConfig returns the default configuration (§T39/§T40/§T61 defaults).
func NewConfig() *Config {
	return &Config{
		SilentChatCmds: true, // cl_silent_chat_commands default on (§V14)
		TappedOut:      false,
		TappedOutText:  "I'm currently tapped out (afk)",
		AutoReply:      false,
		AutoReplyMsg:   "%n (teetui auto reply)",
		Chillpw:        false,           // opt-in: off until the user enables it (§V38)
		PasswordFile:   "chillpw.txt",   // under the config dir unless absolute
		MaxFPS:         DefaultMaxFPS,   // cap repaints (§T74); 0 = unlimited
		LogLines:       DefaultLogLines, // log-band rows when visual on (§T88)
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
	{"cl_auto_reply", "auto-reply with cl_auto_reply_msg on every ping (0/1)",
		func(c *Config) string { return b2s(c.AutoReply) },
		func(c *Config, v string) { c.AutoReply = s2b(v) }},
	{"cl_auto_reply_msg", "auto-reply template; %n = the pinger's name",
		func(c *Config) string { return c.AutoReplyMsg },
		func(c *Config, v string) { c.AutoReplyMsg = v }},
	{"cl_show_last_ping", "show the most recent chat ping in the status bar (0/1)",
		func(c *Config) string { return b2s(c.ShowLastPing) },
		func(c *Config, v string) { c.ShowLastPing = s2b(v) }},
	{"cl_chat_spam_filter", "hide spam pings (0=off 1=hide 2=hide+autoreply)",
		func(c *Config) string { return itoa(c.ChatSpamFilter) },
		func(c *Config, v string) { c.ChatSpamFilter = clampAtoi(v, 0, 2) }},
	{"cl_chat_spam_filter_insults", "also hide insults when cl_chat_spam_filter>0 (0/1)",
		func(c *Config) string { return b2s(c.FilterInsults) },
		func(c *Config, v string) { c.FilterInsults = s2b(v) }},
	{"cl_war_list_auto_reload", "reload the warlist file every N seconds (0=off)",
		func(c *Config) string { return itoa(c.WarListReload) },
		func(c *Config, v string) { c.WarListReload = clampAtoi(v, 0, 3600) }},
	{"cl_max_fps", "cap render repaints per second (0=unlimited)",
		func(c *Config) string { return itoa(c.MaxFPS) },
		func(c *Config, v string) { c.MaxFPS = clampAtoi(v, 0, 1000) }},
	{"cl_log_lines", "log-band rows when the visual is on (capped at half the height)",
		func(c *Config) string { return itoa(c.LogLines) },
		func(c *Config, v string) { c.LogLines = clampAtoi(v, 1, 1000) }},
	{"cl_chillpw", "auto rcon-login from the secrets file on connect (0/1)",
		func(c *Config) string { return b2s(c.Chillpw) },
		func(c *Config, v string) { c.Chillpw = s2b(v) }},
	{"cl_password_file", "path to the chillpw secrets file (addr password per line)",
		func(c *Config) string { return c.PasswordFile },
		func(c *Config, v string) { c.PasswordFile = v }},
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

// itoa / clampAtoi convert an int cvar to/from its console string form, clamping
// a parsed value into [lo,hi].
func itoa(i int) string { return strconv.Itoa(i) }

func clampAtoi(s string, lo, hi int) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return lo
	}
	if n < lo {
		return lo
	}
	if n > hi {
		return hi
	}
	return n
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
