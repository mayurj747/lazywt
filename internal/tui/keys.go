package tui

import (
	"github.com/charmbracelet/bubbles/key"
)

// --- Worktree panel keybindings ---

type worktreeKeyMap struct {
	Open       key.Binding
	New        key.Binding
	Delete     key.Binding
	Prune      key.Binding
	ShowCommit key.Binding
	Details    key.Binding
	Refresh    key.Binding
	Help       key.Binding
	Quit       key.Binding
}

func newWorktreeKeyMap() worktreeKeyMap {
	return worktreeKeyMap{
		Open: key.NewBinding(
			key.WithKeys("enter", "o"),
			key.WithHelp("enter/o", "open"),
		),
		New: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "new"),
		),
		Delete: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "delete"),
		),
		Prune: key.NewBinding(
			key.WithKeys("p"),
			key.WithHelp("p", "prune"),
		),
		ShowCommit: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "show commit"),
		),
		Details: key.NewBinding(
			key.WithKeys("v"),
			key.WithHelp("v", "details"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q"),
			key.WithHelp("q", "quit"),
		),
	}
}

func (k worktreeKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Open, k.New, k.Delete, k.Prune, k.ShowCommit, k.Details, k.Refresh, k.Help}
}

func (k worktreeKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Open, k.New, k.Delete, k.Prune},
		{k.ShowCommit, k.Details, k.Refresh, k.Help, k.Quit},
	}
}

// --- Branch panel keybindings ---

type branchKeyMap struct {
	Open       key.Binding
	ShowCommit key.Binding
	Refresh    key.Binding
	Help       key.Binding
	Quit       key.Binding
}

func newBranchKeyMap() branchKeyMap {
	return branchKeyMap{
		Open: key.NewBinding(
			key.WithKeys("enter", "o"),
			key.WithHelp("enter/o", "open/create wt"),
		),
		ShowCommit: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "show commit"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q"),
			key.WithHelp("q", "quit"),
		),
	}
}

func (k branchKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Open, k.ShowCommit, k.Refresh, k.Help}
}

func (k branchKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Open, k.ShowCommit, k.Refresh, k.Help, k.Quit},
	}
}

// --- Commit panel keybindings ---

type commitKeyMap struct {
	Help key.Binding
	Quit key.Binding
}

func newCommitKeyMap() commitKeyMap {
	return commitKeyMap{
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q"),
			key.WithHelp("q", "quit"),
		),
	}
}

func (k commitKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help}
}

func (k commitKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Help, k.Quit}}
}

// --- Command pane keybindings ---

type cmdKeyMap struct {
	Clear key.Binding
	Help  key.Binding
	Quit  key.Binding
}

func newCmdKeyMap() cmdKeyMap {
	return cmdKeyMap{
		Clear: key.NewBinding(
			key.WithKeys("C"),
			key.WithHelp("C", "clear"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q"),
			key.WithHelp("q", "quit"),
		),
	}
}

func (k cmdKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Clear, k.Help}
}

func (k cmdKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Clear, k.Help, k.Quit}}
}
