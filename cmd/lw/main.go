package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mbency/lazyworktree/internal/config"
	projectinit "github.com/mbency/lazyworktree/internal/init"
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
	var url, name string

	if len(os.Args) >= 3 {
		// Non-interactive: lw init <url>
		url = os.Args[2]
	} else {
		// Interactive: prompt for URL and optional project name
		scanner := bufio.NewScanner(os.Stdin)

		fmt.Print("Git URL: ")
		if scanner.Scan() {
			url = strings.TrimSpace(scanner.Text())
		}
		if url == "" {
			fmt.Fprintln(os.Stderr, "Error: git URL is required")
			os.Exit(1)
		}

		defaultName := projectinit.ExtractProjectName(url)
		fmt.Printf("Project name (%s): ", defaultName)
		if scanner.Scan() {
			name = strings.TrimSpace(scanner.Text())
		}
		// Empty input means accept the default
	}

	if err := projectinit.Run(url, name); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
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
