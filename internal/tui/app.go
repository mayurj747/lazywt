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
	WorktreesPanel Panel = iota
	BranchesPanel
	CommitPanel
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
	branchList    BranchList
	commitView    CommitView
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

type branchesLoadedMsg struct {
	branches []string
}

type commitLoadedMsg struct {
	content string
}

type outputLineMsg hooks.OutputLine

func NewApp(cfg *config.Config, repoPath string) App {
	ti := textinput.New()
	ti.Placeholder = "branch name"
	ti.Prompt = "branch> "

	return App{
		list:          NewWorktreeList(cfg),
		branchList:    NewBranchList(),
		commitView:    NewCommitView(),
		cmdPane:       NewCommandPane(),
		focused:       WorktreesPanel,
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
	return tea.Batch(a.loadWorktrees(), a.loadBranches())
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

	case tea.MouseMsg:
		return a.handleMouse(msg)

	case worktreesLoadedMsg:
		a.list.SetItems(msg.worktrees)
		a.branchList.SetWorktrees(msg.worktrees)
		var cmds []tea.Cmd
		if a.lastCursor == -1 && len(msg.worktrees) > 0 {
			a.lastCursor = 0
			cmds = append(cmds, a.fireOnSwitch())
		}
		cmds = append(cmds, a.loadCommitForSelectedWorktree())
		return a, tea.Batch(cmds...)

	case branchesLoadedMsg:
		a.branchList.SetBranches(msg.branches)
		return a, nil

	case commitLoadedMsg:
		a.commitView.SetContent(msg.content)
		return a, nil

	case outputLineMsg:
		a.cmdPane.Append(hooks.OutputLine(msg))
		return a, nil
	}

	return a, nil
}

func (a App) View() string {
	// Overlays render full-screen
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

	topHeight, cmdHeight := a.panelHeights()
	col1W, col2W, col3W := a.colWidths()

	// Border selection
	col1Border := blurredBorder
	col2Border := blurredBorder
	col3Border := blurredBorder
	cmdBorder := blurredBorder

	switch a.focused {
	case WorktreesPanel:
		col1Border = focusedBorder
	case BranchesPanel:
		col2Border = focusedBorder
	case CommitPanel:
		col3Border = focusedBorder
	case CmdPanel:
		cmdBorder = focusedBorder
	}

	col1 := col1Border.
		Width(col1W-2).
		Height(topHeight).
		Render(titleStyle.Render(" Worktrees") + "\n" + a.list.View())

	col2 := col2Border.
		Width(col2W-2).
		Height(topHeight).
		Render(titleStyle.Render(" Branches") + "\n" + a.branchList.View())

	col3 := col3Border.
		Width(col3W-2).
		Height(topHeight).
		Render(titleStyle.Render(" HEAD Commit") + "\n" + a.commitView.View())

	topRow := lipgloss.JoinHorizontal(lipgloss.Top, col1, col2, col3)

	cmdPanel := cmdBorder.
		Width(a.width-2).
		Height(cmdHeight).
		Render(titleStyle.Render(" Command Output") + "\n" + a.cmdPane.View())

	return lipgloss.JoinVertical(lipgloss.Left, topRow, cmdPanel)
}

// --- Key handling ---

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

	if msg.Type == tea.KeyTab {
		cmd := a.cycleFocus()
		return a, cmd
	}

	switch a.focused {
	case WorktreesPanel:
		return a.handleListKey(msg)
	case BranchesPanel:
		return a.handleBranchKey(msg)
	case CommitPanel:
		return a.handleCommitKey(msg)
	case CmdPanel:
		return a.handleCmdPaneKey(msg)
	}
	return a, nil
}

func (a *App) handleListKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyUp || (msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == keyK) {
		oldCursor := a.list.Cursor()
		a.list.MoveUp()
		if a.list.Cursor() != oldCursor {
			return a, tea.Batch(a.fireOnSwitch(), a.loadCommitForSelectedWorktree())
		}
		return a, nil
	}
	if msg.Type == tea.KeyDown || (msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == keyJ) {
		oldCursor := a.list.Cursor()
		a.list.MoveDown()
		if a.list.Cursor() != oldCursor {
			return a, tea.Batch(a.fireOnSwitch(), a.loadCommitForSelectedWorktree())
		}
		return a, nil
	}

	if msg.Type == tea.KeyEnter {
		return a, a.handleOpen()
	}

	if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 {
		switch msg.Runes[0] {
		case keyQ:
			return a, tea.Quit
		case keyR:
			return a, tea.Batch(a.loadWorktrees(), a.loadBranches())
		case keyN:
			a.textInput.SetValue("")
			a.state = StateCreating
			a.textInput.Focus()
			return a, nil
		case keyD:
			return a.handleDeleteRequest()
		case keyO:
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

func (a *App) handleBranchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyUp || (msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == keyK) {
		oldCursor := a.branchList.Cursor()
		a.branchList.MoveUp()
		if a.branchList.Cursor() != oldCursor {
			return a, a.loadCommitForSelectedBranch()
		}
		return a, nil
	}
	if msg.Type == tea.KeyDown || (msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == keyJ) {
		oldCursor := a.branchList.Cursor()
		a.branchList.MoveDown()
		if a.branchList.Cursor() != oldCursor {
			return a, a.loadCommitForSelectedBranch()
		}
		return a, nil
	}

	if msg.Type == tea.KeyEnter {
		return a.handleBranchOpen()
	}

	if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 {
		switch msg.Runes[0] {
		case keyQ:
			return a, tea.Quit
		case keyR:
			return a, tea.Batch(a.loadWorktrees(), a.loadBranches())
		case keyO:
			return a.handleBranchOpen()
		case keyQuestion:
			a.state = StateHelp
			return a, nil
		}
	}

	return a, nil
}

func (a *App) handleCommitKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyUp || (msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == keyK) {
		a.commitView.ScrollUp()
		return a, nil
	}
	if msg.Type == tea.KeyDown || (msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == keyJ) {
		a.commitView.ScrollDown()
		return a, nil
	}

	if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 {
		switch msg.Runes[0] {
		case keyQ:
			return a, tea.Quit
		case keyQuestion:
			a.state = StateHelp
			return a, nil
		}
	}

	return a, nil
}

func (a *App) handleCmdPaneKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyUp || (msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == keyK) {
		a.cmdPane.ScrollUp()
		return a, nil
	}
	if msg.Type == tea.KeyDown || (msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == keyJ) {
		a.cmdPane.ScrollDown()
		return a, nil
	}

	if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 {
		switch msg.Runes[0] {
		case keyQ:
			return a, tea.Quit
		case keyC:
			a.cmdPane.Clear()
			return a, nil
		}
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

// cycleFocus advances focus through: Worktrees → Branches → Commit → CmdPanel → Worktrees.
// It also loads the appropriate commit when focus enters Worktrees or Branches.
func (a *App) cycleFocus() tea.Cmd {
	switch a.focused {
	case WorktreesPanel:
		a.focused = BranchesPanel
		a.cmdPane.SetFocused(false)
		return a.loadCommitForSelectedBranch()
	case BranchesPanel:
		a.focused = CommitPanel
		a.cmdPane.SetFocused(false)
		return nil
	case CommitPanel:
		a.focused = CmdPanel
		a.cmdPane.SetFocused(true)
		return nil
	case CmdPanel:
		a.focused = WorktreesPanel
		a.cmdPane.SetFocused(false)
		return a.loadCommitForSelectedWorktree()
	}
	return nil
}

// --- Layout ---

func (a *App) colWidths() (int, int, int) {
	if a.width < 12 {
		return 4, 4, 4
	}
	col1W := a.width * 22 / 100
	col2W := a.width * 22 / 100
	col3W := a.width - col1W - col2W
	return col1W, col2W, col3W
}

func (a *App) redistributePanels() {
	topHeight, cmdHeight := a.panelHeights()
	col1W, col2W, col3W := a.colWidths()

	// Each panel has 1 title line, so subtract 1 from the content height.
	a.list.SetSize(col1W-4, topHeight-1)
	a.branchList.SetSize(col2W-4, topHeight-1)
	a.commitView.SetSize(col3W-4, topHeight-1)
	a.cmdPane.SetSize(a.width-4, cmdHeight-1)
}

func (a *App) panelHeights() (int, int) {
	usable := a.height - 4
	if usable < 2 {
		return 1, 1
	}
	topHeight := usable * 70 / 100
	cmdHeight := usable - topHeight
	return topHeight, cmdHeight
}

// --- Data loading ---

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

func (a *App) loadBranches() tea.Cmd {
	repoPath := a.repoPath
	return func() tea.Msg {
		branches, err := git.ListBranches(repoPath)
		if err != nil {
			return branchesLoadedMsg{branches: nil}
		}
		return branchesLoadedMsg{branches: branches}
	}
}

func (a *App) loadCommitForSelectedWorktree() tea.Cmd {
	wt := a.list.Selected()
	if wt == nil {
		return nil
	}
	path := wt.Path
	repoPath := a.repoPath
	return func() tea.Msg {
		content, err := git.ShowHead(repoPath, path, "")
		if err != nil {
			return commitLoadedMsg{content: "(error loading commit: " + err.Error() + ")"}
		}
		return commitLoadedMsg{content: content}
	}
}

func (a *App) loadCommitForSelectedBranch() tea.Cmd {
	branch := a.branchList.Selected()
	if branch == "" {
		return nil
	}
	repoPath := a.repoPath
	return func() tea.Msg {
		content, err := git.ShowHead(repoPath, "", branch)
		if err != nil {
			return commitLoadedMsg{content: "(error loading commit: " + err.Error() + ")"}
		}
		return commitLoadedMsg{content: content}
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

// openWorktree fires the on_open hook for the given worktree.
func (a *App) openWorktree(wt *model.Worktree) tea.Cmd {
	hook := a.cfg.Hooks.OnOpen
	if hook == "" {
		a.sendOutput("stdout", "No on_open hook configured", "info")
		return nil
	}

	env := a.buildHookEnv(wt, "open")

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

func (a *App) handleOpen() tea.Cmd {
	wt := a.list.Selected()
	if wt == nil {
		return nil
	}
	return a.openWorktree(wt)
}

// handleBranchOpen implements smart open in the branch panel:
//   - branch has a worktree → fire on_open for that worktree
//   - branch has no worktree → pre-fill create dialog with branch name
func (a *App) handleBranchOpen() (tea.Model, tea.Cmd) {
	branch := a.branchList.Selected()
	if branch == "" {
		return a, nil
	}

	if a.branchList.HasWorktree(branch) {
		wt := a.list.FindByBranch(branch)
		if wt != nil {
			return a, a.openWorktree(wt)
		}
	}

	// No worktree for this branch — open create dialog pre-filled
	a.textInput.SetValue(branch)
	a.state = StateCreating
	a.textInput.Focus()
	return a, nil
}

func (a *App) handleDeleteRequest() (tea.Model, tea.Cmd) {
	worktree := a.list.Selected()
	if worktree == nil {
		return a, nil
	}

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

	a.sendOutput("stdout", "git worktree remove "+worktree.Path, "git")
	err := git.Delete(a.repoPath, worktree.Path, false)
	if err != nil {
		a.sendOutput("stderr", err.Error(), "git")
		a.sendOutput("stdout", "git worktree remove --force "+worktree.Path, "git")
		err = git.Delete(a.repoPath, worktree.Path, true)
		if err != nil {
			a.sendOutput("stderr", err.Error(), "git")
			return a, nil
		}
	}

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

	return a, tea.Batch(a.loadWorktrees(), a.loadBranches())
}

func (a *App) runCreateAction(branch string) (tea.Model, tea.Cmd) {
	a.state = StateNormal

	defaultPath := a.cfg.DefaultPathDir()
	wtPath := filepath.Join(a.repoPath, defaultPath, branch)

	worktree := &model.Worktree{
		Path:   wtPath,
		Branch: branch,
		Name:   branch,
	}

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

	a.sendOutput("stdout", "git worktree add -b "+branch+" "+wtPath, "git")
	err := git.Create(a.repoPath, wtPath, branch, "")
	if err != nil {
		a.sendOutput("stderr", err.Error(), "git")
		a.textInput.Reset()
		return a, nil
	}

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
	return a, tea.Batch(a.loadWorktrees(), a.loadBranches())
}

func (a *App) runPruneAction() (tea.Model, tea.Cmd) {
	a.state = StateNormal

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

	a.sendOutput("stdout", "git worktree prune", "git")
	err := git.Prune(a.repoPath)
	if err != nil {
		a.sendOutput("stderr", err.Error(), "git")
		return a, nil
	}

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

	return a, tea.Batch(a.loadWorktrees(), a.loadBranches())
}

// --- Mouse handling ---

// handleMouse routes mouse events to the appropriate panel action.
// Only active in StateNormal.
func (a *App) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if a.state != StateNormal {
		return a, nil
	}

	col1W, col2W, _ := a.colWidths()
	topHeight, _ := a.panelHeights()

	// Determine which panel the mouse is over.
	// Top row outer height = topHeight + 2 (top border + bottom border).
	var hovered Panel
	if msg.Y < topHeight+2 {
		switch {
		case msg.X < col1W:
			hovered = WorktreesPanel
		case msg.X < col1W+col2W:
			hovered = BranchesPanel
		default:
			hovered = CommitPanel
		}
	} else {
		hovered = CmdPanel
	}

	switch {
	case msg.Button == tea.MouseButtonWheelUp:
		return a.mouseScroll(hovered, -1)
	case msg.Button == tea.MouseButtonWheelDown:
		return a.mouseScroll(hovered, 1)
	case msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress:
		return a.mouseClick(hovered, msg.Y)
	}

	return a, nil
}

// mouseScroll scrolls a panel by dir (-1 = up, +1 = down).
// For list panels the cursor moves; for viewport panels the content scrolls.
func (a *App) mouseScroll(panel Panel, dir int) (tea.Model, tea.Cmd) {
	switch panel {
	case WorktreesPanel:
		old := a.list.cursor
		if dir < 0 {
			a.list.MoveUp()
		} else {
			a.list.MoveDown()
		}
		if a.list.cursor != old {
			return a, tea.Batch(a.fireOnSwitch(), a.loadCommitForSelectedWorktree())
		}
	case BranchesPanel:
		old := a.branchList.cursor
		if dir < 0 {
			a.branchList.MoveUp()
		} else {
			a.branchList.MoveDown()
		}
		if a.branchList.cursor != old {
			return a, a.loadCommitForSelectedBranch()
		}
	case CommitPanel:
		if dir < 0 {
			a.commitView.ScrollUp()
		} else {
			a.commitView.ScrollDown()
		}
	case CmdPanel:
		if dir < 0 {
			a.cmdPane.ScrollUp()
		} else {
			a.cmdPane.ScrollDown()
		}
	}
	return a, nil
}

// mouseClick focuses the clicked panel and moves the cursor to the clicked row
// for list panels. contentRow = y - 2 (1 for top border + 1 for title line).
func (a *App) mouseClick(panel Panel, y int) (tea.Model, tea.Cmd) {
	prevFocused := a.focused
	a.focused = panel
	a.cmdPane.SetFocused(panel == CmdPanel)

	contentRow := y - 2

	switch panel {
	case WorktreesPanel:
		old := a.list.cursor
		if contentRow >= 0 && contentRow < len(a.list.items) {
			a.list.cursor = contentRow
		}
		if a.list.cursor != old || prevFocused != WorktreesPanel {
			return a, tea.Batch(a.fireOnSwitch(), a.loadCommitForSelectedWorktree())
		}
	case BranchesPanel:
		old := a.branchList.cursor
		if contentRow >= 0 && contentRow < len(a.branchList.branches) {
			a.branchList.cursor = contentRow
		}
		if a.branchList.cursor != old || prevFocused != BranchesPanel {
			return a, a.loadCommitForSelectedBranch()
		}
	}

	return a, nil
}

// --- Overlay Views ---

func (a *App) creatingView() string {
	border := focusedBorder.Width(a.width - 2).Height(a.height - 4)
	content := lipgloss.NewStyle().
		Width(a.width - 6).
		Height(a.height - 8).
		Render("Create new worktree\n\n" + a.textInput.View())

	return border.Render(content)
}

func (a *App) confirmView() string {
	border := focusedBorder.Width(a.width - 2).Height(5)
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
		border := focusedBorder.Width(a.width - 2).Height(5)
		content := lipgloss.NewStyle().
			Width(a.width - 6).
			Height(3).
			Render("No worktree selected")
		return border.Render(content)
	}

	border := focusedBorder.Width(a.width - 2).Height(a.height - 4)

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
	border := focusedBorder.Width(a.width - 2).Height(a.height - 4)

	lines := []string{
		"Keybindings",
		"",
		"Navigation:",
		"  j/k or ↑/↓   Move up/down in focused panel",
		"  Tab           Cycle focus: Worktrees → Branches → Commit → Output",
		"  Ctrl+C        Quit",
		"",
		"Worktree Actions (Worktrees panel):",
		"  n             Create new worktree",
		"  d             Delete worktree",
		"  o or Enter    Open worktree (run on_open hook)",
		"  p             Prune stale worktrees",
		"  v             View worktree details",
		"  r             Refresh",
		"",
		"Branch Actions (Branches panel):",
		"  o or Enter    Open worktree if exists, else create",
		"  r             Refresh",
		"",
		"Command Output panel:",
		"  j/k or ↑/↓   Scroll",
		"  C             Clear output",
		"",
		"  ?             Toggle this help",
		"  q             Quit",
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
