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
	MaxFPS         int    // cap render repaints/sec; 0=unlimited (§T74)
	LogLines       int    // log-band rows when the visual is on (§T88)
	PlayerName     string // identity (§T89, player_name)
	PlayerClan     string // identity (§T89, player_clan)
	ConnectTimeout int    // handshake timeout seconds (§T89, cl_connect_timeout)
}

// NewConfig returns the default configuration. Feature-owned cvars are declared
// by their features at Provision (§T76); core keeps render + identity + connect
// settings, all settable from a config file or the console (§C23).
func NewConfig() *Config {
	return &Config{
		MaxFPS:         DefaultMaxFPS,   // cap repaints (§T74); 0 = unlimited
		LogLines:       DefaultLogLines, // log-band rows when visual on (§T88)
		PlayerName:     "nameless tee",
		PlayerClan:     "",
		ConnectTimeout: 30, // seconds (= DefaultConnectTimeout)
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
	{"cl_max_fps", "cap render repaints per second (0=unlimited)",
		func(c *Config) string { return itoa(c.MaxFPS) },
		func(c *Config, v string) { c.MaxFPS = clampAtoi(v, 0, 1000) }},
	{"cl_log_lines", "log-band rows when the visual is on (capped at half the height)",
		func(c *Config) string { return itoa(c.LogLines) },
		func(c *Config, v string) { c.LogLines = clampAtoi(v, 1, 1000) }},
	{"player_name", "your player name",
		func(c *Config) string { return c.PlayerName },
		func(c *Config, v string) { c.PlayerName = v }},
	{"player_clan", "your clan tag",
		func(c *Config) string { return c.PlayerClan },
		func(c *Config, v string) { c.PlayerClan = v }},
	{"cl_connect_timeout", "handshake timeout in seconds (login + map download)",
		func(c *Config) string { return itoa(c.ConnectTimeout) },
		func(c *Config, v string) { c.ConnectTimeout = clampAtoi(v, 1, 600) }},
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
