package tui

import (
	"flag"
	"fmt"
	"os"
)

// Main is the teetui entry point (§T86/§C23). The CLI takes ONE optional flag —
// a teeworlds-style config file; every other setting is a cvar/command inside it
// or set live in the console. With no config it starts at the greeting/browser
// (no auto-connect). main.go is just `func main(){ tui.Main() }` plus the
// blank-imports that register the feature modules.
func Main() {
	configPath := flag.String("config", "", "teeworlds-style config file (cvars + connect)")
	flag.Parse()

	state := NewState()
	input := NewInputController()
	log := NewLog(500)

	app, err := NewApp("", state, input, log)
	if err != nil {
		fmt.Fprintln(os.Stderr, "teetui:", err)
		os.Exit(1)
	}

	if *configPath != "" {
		if err := app.ExecConfig(*configPath); err != nil {
			fmt.Fprintln(os.Stderr, "teetui: config:", err)
			os.Exit(1)
		}
	}

	app.Start() // apply identity/timeout, build dialer, connect if the config asked
	app.Run()   // blocks until quit; restores the terminal
	if cur := app.Client(); cur != nil {
		cur.Close()
	}
}
