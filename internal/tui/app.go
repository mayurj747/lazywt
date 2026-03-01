package tui

import (
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
	"github.com/mbency/lazyworktree/internal/config"
	"github.com/mbency/lazyworktree/internal/git"
	"github.com/mbency/lazyworktree/internal/hooks"
	"github.com/mbency/lazyworktree/internal/model"

	tea "github.com/charmbracelet/bubbletea"
)

type Panel int

const (
	ListPanel Panel = iota
	CmdPanel
)

type AppState int

const (
	StateNormal AppState = iota
	StateCreating
	StateConfirmingDelete
	StateConfirmingPrune
	StateViewingDetails
	StateHelp
)

type App struct {
	list          WorktreeList
	cmdPane       CommandPane
	focused       Panel
	state         AppState
	width         int
	height        int
	cfg           *config.Config
	repoPath      string
	hookExec      *hooks.Executor
	textInput     textinput.Model
	confirmPrompt string
	lastCursor    int
}

type worktreesLoadedMsg struct {
	worktrees []model.Worktree
}

type outputLineMsg hooks.OutputLine

func NewApp(cfg *config.Config, repoPath string) App {
	ti := textinput.New()
	ti.Placeholder = "branch name"
	ti.Prompt = "branch> "

	return App{
		list:          NewWorktreeList(cfg),
		cmdPane:       NewCommandPane(),
		focused:       ListPanel,
		state:         StateNormal,
		cfg:           cfg,
		repoPath:      repoPath,
		hookExec:      hooks.NewExecutor(cfg.ShellCmd()),
		textInput:     ti,
		confirmPrompt: "y/n",
		lastCursor:    -1,
	}
}

func (a App) Init() tea.Cmd {
	return a.loadWorktrees()
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.redistributePanels()
		return a, nil

	case tea.KeyMsg:
		return a.handleKey(msg)

	case worktreesLoadedMsg:
		a.list.SetItems(msg.worktrees)
		// Fire on_switch for initial selection
		if a.lastCursor == -1 && len(msg.worktrees) > 0 {
			a.lastCursor = 0
			return a, a.fireOnSwitch()
		}
		return a, nil

	case outputLineMsg:
		a.cmdPane.Append(hooks.OutputLine(msg))
		return a, nil
	}

	return a, nil
}

func (a App) View() string {
	// Handle overlays first
	if a.state == StateHelp {
		return a.helpOverlay()
	}
	if a.state == StateViewingDetails {
		return a.detailsOverlay()
	}
	if a.state == StateCreating {
		return a.creatingView()
	}
	if a.state == StateConfirmingDelete || a.state == StateConfirmingPrune {
		return a.confirmView()
	}

	// Normal two-panel view
	listBorder := blurredBorder
	cmdBorder := blurredBorder

	if a.focused == ListPanel {
		listBorder = focusedBorder
	} else {
		cmdBorder = focusedBorder
	}

	listHeight, cmdHeight := a.panelHeights()

	listPanel := listBorder.
		Width(a.width - 2).
		Height(listHeight).
		Render(a.list.View())

	cmdPanel := cmdBorder.
		Width(a.width - 2).
		Height(cmdHeight).
		Render(a.cmdPane.View())

	return lipgloss.JoinVertical(lipgloss.Left, listPanel, cmdPanel)
}

func (a *App) handleKey(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return a, nil
	}

	switch a.state {
	case StateNormal:
		return a.handleNormalKey(keyMsg)
	case StateCreating:
		return a.handleCreatingKey(keyMsg)
	case StateConfirmingDelete:
		return a.handleConfirmDeleteKey(keyMsg)
	case StateConfirmingPrune:
		return a.handleConfirmPruneKey(keyMsg)
	case StateViewingDetails:
		return a.handleDetailsKey(keyMsg)
	case StateHelp:
		return a.handleHelpKey(keyMsg)
	}
	return a, nil
}

func (a *App) handleNormalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyCtrlC {
		return a, tea.Quit
	}

	switch msg.Type {
	case tea.KeyTab:
		a.toggleFocus()
		return a, nil
	}

	if a.focused == CmdPanel {
		return a.handleCmdPaneKey(msg)
	}

	return a.handleListKey(msg)
}

func (a *App) handleListKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyUp || (msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == keyK) {
		oldCursor := a.list.Cursor()
		a.list.MoveUp()
		if a.list.Cursor() != oldCursor {
			return a, a.fireOnSwitch()
		}
		return a, nil
	}
	if msg.Type == tea.KeyDown || (msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == keyJ) {
		oldCursor := a.list.Cursor()
		a.list.MoveDown()
		if a.list.Cursor() != oldCursor {
			return a, a.fireOnSwitch()
		}
		return a, nil
	}

	if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 {
		switch msg.Runes[0] {
		case keyQ:
			return a, tea.Quit
		case keyR:
			return a, a.loadWorktrees()
		case keyN:
			a.state = StateCreating
			a.textInput.Focus()
			return a, nil
		case keyD:
			return a.handleDeleteRequest()
		case keyO:
			return a, a.handleOpen()
		case keyEnter:
			return a, a.handleOpen()
		case keyP:
			a.state = StateConfirmingPrune
			a.confirmPrompt = "prune stale worktrees? (y/n)"
			return a, nil
		case keyV:
			a.state = StateViewingDetails
			return a, nil
		case keyQuestion:
			a.state = StateHelp
			return a, nil
		}
	}

	return a, nil
}

func (a *App) handleCmdPaneKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 {
		switch msg.Runes[0] {
		case keyQ:
			return a, tea.Quit
		case keyK:
			a.cmdPane.ScrollUp()
			return a, nil
		case keyJ:
			a.cmdPane.ScrollDown()
			return a, nil
		case keyC:
			a.cmdPane.Clear()
			return a, nil
		}
	}

	switch msg.Type {
	case tea.KeyUp:
		a.cmdPane.ScrollUp()
		return a, nil
	case tea.KeyDown:
		a.cmdPane.ScrollDown()
		return a, nil
	}

	return a, nil
}

func (a *App) handleCreatingKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC, tea.KeyEsc:
		a.state = StateNormal
		a.textInput.Reset()
		return a, nil
	case tea.KeyEnter:
		branch := a.textInput.Value()
		if branch == "" {
			a.state = StateNormal
			a.textInput.Reset()
			return a, nil
		}
		_, cmd := a.runCreateAction(branch)
		return a, cmd
	}

	var cmd tea.Cmd
	a.textInput, cmd = a.textInput.Update(msg)
	return a, cmd
}

func (a *App) handleConfirmDeleteKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyCtrlC || msg.Type == tea.KeyEsc {
		a.state = StateNormal
		a.confirmPrompt = "y/n"
		return a, nil
	}

	if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 {
		if msg.Runes[0] == 'y' || msg.Runes[0] == 'Y' {
			_, cmd := a.runDeleteAction()
			return a, cmd
		}
		a.state = StateNormal
		a.confirmPrompt = "y/n"
	}
	return a, nil
}

func (a *App) handleConfirmPruneKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyCtrlC || msg.Type == tea.KeyEsc {
		a.state = StateNormal
		a.confirmPrompt = "y/n"
		return a, nil
	}

	if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 {
		if msg.Runes[0] == 'y' || msg.Runes[0] == 'Y' {
			_, cmd := a.runPruneAction()
			return a, cmd
		}
		a.state = StateNormal
		a.confirmPrompt = "y/n"
	}
	return a, nil
}

func (a *App) handleDetailsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyCtrlC || msg.Type == tea.KeyEsc || (msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == keyQ) {
		a.state = StateNormal
		return a, nil
	}
	return a, nil
}

func (a *App) handleHelpKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyCtrlC || msg.Type == tea.KeyEsc || (msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == keyQ) {
		a.state = StateNormal
		return a, nil
	}
	return a, nil
}

func (a *App) toggleFocus() {
	if a.focused == ListPanel {
		a.focused = CmdPanel
		a.cmdPane.SetFocused(true)
	} else {
		a.focused = ListPanel
		a.cmdPane.SetFocused(false)
	}
}

func (a *App) redistributePanels() {
	listHeight, cmdHeight := a.panelHeights()
	a.list.SetSize(a.width-4, listHeight)
	a.cmdPane.SetSize(a.width-4, cmdHeight)
}

func (a *App) panelHeights() (int, int) {
	usable := a.height - 4
	if usable < 2 {
		return 1, 1
	}
	listHeight := usable * 70 / 100
	cmdHeight := usable - listHeight
	return listHeight, cmdHeight
}

func (a *App) loadWorktrees() tea.Cmd {
	repoPath := a.repoPath
	return func() tea.Msg {
		worktrees, err := git.ListWorktrees(repoPath)
		if err != nil {
			return worktreesLoadedMsg{worktrees: nil}
		}
		git.EnrichWorktreesConcurrent(worktrees)
		return worktreesLoadedMsg{worktrees: worktrees}
	}
}

// --- Action Helpers ---

func (a *App) buildHookEnv(worktree *model.Worktree, action string) map[string]string {
	wtPath := ""
	branch := ""
	if worktree != nil {
		wtPath = worktree.Path
		branch = worktree.Branch
	}

	env := map[string]string{
		"LW_ACTION":    action,
		"LW_REPO_PATH": a.repoPath,
		"LW_PATH":      wtPath,
		"LW_BRANCH":    branch,
	}

	// Add LW_IS_DIRTY only for on_open and on_switch
	if action == "open" || action == "switch" {
		if worktree != nil {
			if worktree.IsDirty {
				env["LW_IS_DIRTY"] = "1"
			} else {
				env["LW_IS_DIRTY"] = "0"
			}
		}
	}

	return env
}

func (a *App) sendOutput(stream, text, hook string) {
	a.cmdPane.Append(hooks.OutputLine{
		Stream: stream,
		Text:   text,
		Hook:   hook,
	})
}

func (a *App) runHook(cmd string, env map[string]string, hookName string) {
	result := a.hookExec.Run(cmd, env)
	if result.Stdout != "" {
		for _, line := range strings.Split(result.Stdout, "\n") {
			if line != "" {
				a.sendOutput("stdout", line, hookName)
			}
		}
	}
	if result.Stderr != "" {
		for _, line := range strings.Split(result.Stderr, "\n") {
			if line != "" {
				a.sendOutput("stderr", line, hookName)
			}
		}
	}
}

func (a *App) fireOnSwitch() tea.Cmd {
	worktree := a.list.Selected()
	if worktree == nil {
		return nil
	}

	hook := a.cfg.Hooks.OnSwitch
	if hook == "" {
		return nil
	}

	env := a.buildHookEnv(worktree, "switch")

	return func() tea.Msg {
		result := a.hookExec.Run(hook, env)
		if result.Stdout != "" {
			for _, line := range strings.Split(result.Stdout, "\n") {
				if line != "" {
					return outputLineMsg{Stream: "stdout", Text: line, Hook: "on_switch"}
				}
			}
		}
		if result.Stderr != "" {
			for _, line := range strings.Split(result.Stderr, "\n") {
				if line != "" {
					return outputLineMsg{Stream: "stderr", Text: line, Hook: "on_switch"}
				}
			}
		}
		return nil
	}
}

func (a *App) handleOpen() tea.Cmd {
	worktree := a.list.Selected()
	if worktree == nil {
		return nil
	}

	hook := a.cfg.Hooks.OnOpen
	if hook == "" {
		a.sendOutput("stdout", "No on_open hook configured", "info")
		return nil
	}

	env := a.buildHookEnv(worktree, "open")

	return func() tea.Msg {
		result := a.hookExec.Run(hook, env)
		if result.Stdout != "" {
			for _, line := range strings.Split(result.Stdout, "\n") {
				if line != "" {
					return outputLineMsg{Stream: "stdout", Text: line, Hook: "on_open"}
				}
			}
		}
		if result.Stderr != "" {
			for _, line := range strings.Split(result.Stderr, "\n") {
				if line != "" {
					return outputLineMsg{Stream: "stderr", Text: line, Hook: "on_open"}
				}
			}
		}
		return nil
	}
}

func (a *App) handleDeleteRequest() (tea.Model, tea.Cmd) {
	worktree := a.list.Selected()
	if worktree == nil {
		return a, nil
	}

	// Block deletion of main worktree
	if worktree.IsMain {
		a.sendOutput("stderr", "Cannot delete main worktree", "info")
		return a, nil
	}

	a.state = StateConfirmingDelete
	a.confirmPrompt = "delete " + worktree.Name + "? (y/n)"
	return a, nil
}

func (a *App) runDeleteAction() (tea.Model, tea.Cmd) {
	worktree := a.list.Selected()
	if worktree == nil {
		a.state = StateNormal
		return a, nil
	}

	a.state = StateNormal

	// Pre-delete hook
	preHook := a.cfg.Hooks.PreDelete
	if preHook != "" {
		env := a.buildHookEnv(worktree, "delete")
		result := a.hookExec.Run(preHook, env)
		for _, line := range strings.Split(result.Stdout, "\n") {
			if line != "" {
				a.sendOutput("stdout", line, "pre_delete")
			}
		}
		for _, line := range strings.Split(result.Stderr, "\n") {
			if line != "" {
				a.sendOutput("stderr", line, "pre_delete")
			}
		}
		if result.ExitCode != 0 {
			a.sendOutput("stderr", "pre_delete hook failed, aborting", "pre_delete")
			return a, nil
		}
	}

	// Git delete
	a.sendOutput("stdout", "git worktree remove "+worktree.Path, "git")
	err := git.Delete(a.repoPath, worktree.Path, false)
	if err != nil {
		a.sendOutput("stderr", err.Error(), "git")
		// Try force delete
		a.sendOutput("stdout", "git worktree remove --force "+worktree.Path, "git")
		err = git.Delete(a.repoPath, worktree.Path, true)
		if err != nil {
			a.sendOutput("stderr", err.Error(), "git")
			return a, nil
		}
	}

	// Post-delete hook
	postHook := a.cfg.Hooks.PostDelete
	if postHook != "" {
		env := a.buildHookEnv(worktree, "delete")
		result := a.hookExec.Run(postHook, env)
		for _, line := range strings.Split(result.Stdout, "\n") {
			if line != "" {
				a.sendOutput("stdout", line, "post_delete")
			}
		}
		for _, line := range strings.Split(result.Stderr, "\n") {
			if line != "" {
				a.sendOutput("stderr", line, "post_delete")
			}
		}
	}

	return a, a.loadWorktrees()
}

func (a *App) runCreateAction(branch string) (tea.Model, tea.Cmd) {
	a.state = StateNormal

	// Build worktree path: repoPath/defaultPathDir/branchName
	defaultPath := a.cfg.DefaultPathDir()
	wtPath := filepath.Join(a.repoPath, defaultPath, branch)

	worktree := &model.Worktree{
		Path:   wtPath,
		Branch: branch,
		Name:   branch,
	}

	// Pre-create hook
	preHook := a.cfg.Hooks.PreCreate
	if preHook != "" {
		env := a.buildHookEnv(worktree, "create")
		result := a.hookExec.Run(preHook, env)
		for _, line := range strings.Split(result.Stdout, "\n") {
			if line != "" {
				a.sendOutput("stdout", line, "pre_create")
			}
		}
		for _, line := range strings.Split(result.Stderr, "\n") {
			if line != "" {
				a.sendOutput("stderr", line, "pre_create")
			}
		}
		if result.ExitCode != 0 {
			a.sendOutput("stderr", "pre_create hook failed, aborting", "pre_create")
			a.textInput.Reset()
			return a, nil
		}
	}

	// Git create
	a.sendOutput("stdout", "git worktree add -b "+branch+" "+wtPath, "git")
	err := git.Create(a.repoPath, wtPath, branch, "")
	if err != nil {
		a.sendOutput("stderr", err.Error(), "git")
		a.textInput.Reset()
		return a, nil
	}

	// Post-create hook
	postHook := a.cfg.Hooks.PostCreate
	if postHook != "" {
		env := a.buildHookEnv(worktree, "create")
		result := a.hookExec.Run(postHook, env)
		for _, line := range strings.Split(result.Stdout, "\n") {
			if line != "" {
				a.sendOutput("stdout", line, "post_create")
			}
		}
		for _, line := range strings.Split(result.Stderr, "\n") {
			if line != "" {
				a.sendOutput("stderr", line, "post_create")
			}
		}
	}

	a.textInput.Reset()
	return a, a.loadWorktrees()
}

func (a *App) runPruneAction() (tea.Model, tea.Cmd) {
	a.state = StateNormal

	// Pre-prune hook
	preHook := a.cfg.Hooks.PrePrune
	if preHook != "" {
		env := a.buildHookEnv(nil, "prune")
		result := a.hookExec.Run(preHook, env)
		for _, line := range strings.Split(result.Stdout, "\n") {
			if line != "" {
				a.sendOutput("stdout", line, "pre_prune")
			}
		}
		for _, line := range strings.Split(result.Stderr, "\n") {
			if line != "" {
				a.sendOutput("stderr", line, "pre_prune")
			}
		}
		if result.ExitCode != 0 {
			a.sendOutput("stderr", "pre_prune hook failed, aborting", "pre_prune")
			return a, nil
		}
	}

	// Git prune
	a.sendOutput("stdout", "git worktree prune", "git")
	err := git.Prune(a.repoPath)
	if err != nil {
		a.sendOutput("stderr", err.Error(), "git")
		return a, nil
	}

	// Post-prune hook
	postHook := a.cfg.Hooks.PostPrune
	if postHook != "" {
		env := a.buildHookEnv(nil, "prune")
		result := a.hookExec.Run(postHook, env)
		for _, line := range strings.Split(result.Stdout, "\n") {
			if line != "" {
				a.sendOutput("stdout", line, "post_prune")
			}
		}
		for _, line := range strings.Split(result.Stderr, "\n") {
			if line != "" {
				a.sendOutput("stderr", line, "post_prune")
			}
		}
	}

	return a, a.loadWorktrees()
}

// --- Overlay Views ---

func (a *App) creatingView() string {
	border := focusedBorder.Copy().Width(a.width - 2).Height(a.height - 4)
	content := lipgloss.NewStyle().
		Width(a.width - 6).
		Height(a.height - 8).
		Render("Create new worktree\n\n" + a.textInput.View())

	return border.Render(content)
}

func (a *App) confirmView() string {
	border := focusedBorder.Copy().Width(a.width - 2).Height(5)
	content := lipgloss.NewStyle().
		Width(a.width - 6).
		Height(3).
		Align(lipgloss.Center).
		Render(a.confirmPrompt)

	return border.Render(content)
}

func (a *App) detailsOverlay() string {
	worktree := a.list.Selected()
	if worktree == nil {
		border := focusedBorder.Copy().Width(a.width - 2).Height(5)
		content := lipgloss.NewStyle().
			Width(a.width - 6).
			Height(3).
			Render("No worktree selected")
		return border.Render(content)
	}

	border := focusedBorder.Copy().Width(a.width - 2).Height(a.height - 4)

	lines := []string{
		"Details",
		"",
		"Branch:        " + worktree.Branch,
		"Path:          " + worktree.Path,
		"Main worktree: " + boolToStr(worktree.IsMain),
		"Current:       " + boolToStr(worktree.IsCurrent),
		"Dirty:         " + boolToStr(worktree.IsDirty),
		"",
		"Last Commit:",
		"  Hash:    " + worktree.LastCommitHash,
		"  Subject: " + worktree.LastCommitSubject,
		"  Author:  " + worktree.LastCommitAuthor,
		"  Date:    " + worktree.LastCommitDate.Format("2006-01-02 15:04:05"),
		"",
		"Tracking: " + worktree.TrackingBranch,
		"",
		"Press q or Esc to close",
	}

	content := lipgloss.NewStyle().
		Width(a.width - 6).
		Height(a.height - 8).
		Render(strings.Join(lines, "\n"))

	return border.Render(content)
}

func (a *App) helpOverlay() string {
	border := focusedBorder.Copy().Width(a.width - 2).Height(a.height - 4)

	lines := []string{
		"Keybindings",
		"",
		"Navigation:",
		"  j/k or ↑/↓   Move up/down",
		"  Tab          Switch panels",
		"  Ctrl+C       Quit",
		"",
		"Worktree Actions:",
		"  n             Create new worktree",
		"  d             Delete worktree",
		"  o or Enter    Open worktree (run on_open hook)",
		"  p             Prune stale worktrees",
		"  v             View worktree details",
		"",
		"Other:",
		"  r             Refresh worktrees",
		"  ?             Toggle this help",
		"  q             Quit (in list panel)",
		"",
		"Press q or Esc to close",
	}

	content := lipgloss.NewStyle().
		Width(a.width - 6).
		Height(a.height - 8).
		Render(strings.Join(lines, "\n"))

	return border.Render(content)
}

func boolToStr(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}
