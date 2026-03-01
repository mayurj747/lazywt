package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mbency/lazyworktree/internal/config"
	"github.com/mbency/lazyworktree/internal/tui"
)
func main() {
	if len(os.Args) > 1 && os.Args[1] == "init" {
		runInit()
		return
	}
	runTUI()
}
func runInit() {
	// TODO(mbency): implement lw init — Phase 8
	fmt.Println("lw init: not yet implemented")
}
func runTUI() {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	cfg, err := config.Load("lazywt.toml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	app := tui.NewApp(cfg, cwd)
	p := tea.NewProgram(app, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}