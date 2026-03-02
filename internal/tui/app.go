package tui

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
	"github.com/mbency/lazyworktree/internal/config"
	"github.com/mbency/lazyworktree/internal/git"
	"github.com/mbency/lazyworktree/internal/hooks"
	"github.com/mbency/lazyworktree/internal/model"
	"github.com/mbency/lazyworktree/internal/version"

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
	repoPath      string // bare repo or git dir (for git commands)
	projectRoot   string // lw project root (for placing worktrees)
	hookExec      *hooks.Executor
	textInput     textinput.Model
	confirmPrompt string
	commitLabel   string // branch/worktree name shown in the commit pane title
	lastCursor    int
	showCommit     bool
	lastClickTime  time.Time // for double-click detection
	lastClickPanel Panel
	lastClickRow   int

	// Keymaps per panel
	wtKeys     worktreeKeyMap
	branchKeys branchKeyMap
	commitKeys commitKeyMap
	cmdKeys    cmdKeyMap

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

type outputLineMsg struct {
	line hooks.OutputLine
	ch   <-chan tea.Msg // yields outputLineMsg or hookDoneMsg; nil for direct sendOutput calls
}

type hookDoneMsg struct {
	hookName string
	exitCode int
	refresh  bool // whether to reload worktrees/branches after
}

func NewApp(cfg *config.Config, repoPath, projectRoot string) App {
	ti := textinput.New()
	ti.Placeholder = "branch name"
	ti.Prompt = "branch> "

	return App{
		list:          NewWorktreeList(),
		branchList:    NewBranchList(),
		commitView:    NewCommitView(),
		cmdPane:       NewCommandPane(),
		focused:       WorktreesPanel,
		state:         StateNormal,
		cfg:           cfg,
		repoPath:      repoPath,
		projectRoot:   projectRoot,
		hookExec:      hooks.NewExecutor(cfg.ShellCmd()),
		textInput:     ti,
		confirmPrompt: "y/n",
		lastCursor:    -1,
		wtKeys:        newWorktreeKeyMap(),
		branchKeys:    newBranchKeyMap(),
		commitKeys:    newCommitKeyMap(),
		cmdKeys:       newCmdKeyMap(),
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
		cmd := a.list.SetItems(msg.worktrees)
		a.branchList.SetWorktrees(msg.worktrees)
		var cmds []tea.Cmd
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
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
		a.cmdPane.Append(msg.line)
		if msg.ch != nil {
			return a, listenForHookOutput(msg.ch)
		}
		return a, nil

	case hookDoneMsg:
		if msg.refresh {
			return a, tea.Batch(a.loadWorktrees(), a.loadBranches())
		}
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

	// Border color per panel
	col1Color := blurredBorderColor
	col2Color := blurredBorderColor
	cmdColor := blurredBorderColor

	switch a.focused {
	case WorktreesPanel:
		col1Color = focusedBorderColor
	case BranchesPanel:
		col2Color = focusedBorderColor
	case CmdPanel:
		cmdColor = focusedBorderColor
	}

	// Determine titles (show filter prompt when filtering)
	wtTitle := "Worktrees"
	if a.focused == WorktreesPanel && a.list.IsFiltering() {
		wtTitle = "Filter: " + a.list.FilterValue()
	}
	brTitle := "Branches"
	if a.focused == BranchesPanel && a.branchList.IsFiltering() {
		brTitle = "Filter: " + a.branchList.FilterValue()
	}

	col1 := renderTitledPanel(col1Color, wtTitle, a.list.View(a.focused == WorktreesPanel), col1W, topHeight)
	col2 := renderTitledPanel(col2Color, brTitle, a.branchList.View(a.focused == BranchesPanel), col2W, topHeight)

	var topRow string
	if a.showCommit {
		col3Color := blurredBorderColor
		if a.focused == CommitPanel {
			col3Color = focusedBorderColor
		}
		col3 := renderTitledPanel(col3Color, "HEAD: "+a.commitLabel, a.commitView.View(), col3W, topHeight)
		topRow = lipgloss.JoinHorizontal(lipgloss.Top, col1, col2, col3)
	} else {
		topRow = lipgloss.JoinHorizontal(lipgloss.Top, col1, col2)
	}

	cmdPanel := renderTitledPanel(cmdColor, "Command Output", a.cmdPane.View(), a.width, cmdHeight)

	status := a.statusLine()

	return lipgloss.JoinVertical(lipgloss.Left, topRow, cmdPanel, status)
}

// statusLine renders the project path (left) and version (right).
func (a *App) statusLine() string {
	left := " " + a.projectRoot
	right := version.Version + " "
	gap := a.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return dimStyle.Render(left + strings.Repeat(" ", gap) + right)
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

	// When a list is filtering, forward all keys to that list
	if a.focused == WorktreesPanel && a.list.IsFiltering() {
		return a.forwardToWorktreeList(msg)
	}
	if a.focused == BranchesPanel && a.branchList.IsFiltering() {
		return a.forwardToBranchList(msg)
	}

	if msg.Type == tea.KeyTab {
		cmd := a.cycleFocus()
		return a, cmd
	}
	if msg.Type == tea.KeyShiftTab {
		cmd := a.cycleFocusReverse()
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
	km := a.wtKeys

	switch {
	case key.Matches(msg, km.Open):
		return a, a.handleOpen()
	case key.Matches(msg, km.Quit):
		return a, tea.Quit
	case key.Matches(msg, km.Refresh):
		return a, tea.Batch(a.loadWorktrees(), a.loadBranches())
	case key.Matches(msg, km.New):
		a.textInput.SetValue("")
		a.state = StateCreating
		a.textInput.Focus()
		return a, nil
	case key.Matches(msg, km.Delete):
		return a.handleDeleteRequest()
	case key.Matches(msg, km.Prune):
		a.state = StateConfirmingPrune
		a.confirmPrompt = "prune stale worktrees? (y/n)"
		return a, nil
	case key.Matches(msg, km.ShowCommit):
		return a.toggleCommitPane()
	case key.Matches(msg, km.Details):
		a.state = StateViewingDetails
		return a, nil
	case key.Matches(msg, km.Help):
		a.state = StateHelp
		return a, nil
	}

	// Forward to list for cursor movement, filtering, pagination
	return a.forwardToWorktreeList(msg)
}

func (a *App) forwardToWorktreeList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	oldIdx := a.list.Index()
	cmd := a.list.Update(msg)
	newIdx := a.list.Index()
	if newIdx != oldIdx {
		return a, tea.Batch(cmd, a.fireOnSwitch(), a.loadCommitForSelectedWorktree())
	}
	return a, cmd
}

func (a *App) handleBranchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	km := a.branchKeys

	switch {
	case key.Matches(msg, km.Open):
		return a.handleBranchOpen()
	case key.Matches(msg, km.Quit):
		return a, tea.Quit
	case key.Matches(msg, km.Refresh):
		return a, tea.Batch(a.loadWorktrees(), a.loadBranches())
	case key.Matches(msg, km.ShowCommit):
		return a.toggleCommitPane()
	case key.Matches(msg, km.Help):
		a.state = StateHelp
		return a, nil
	}

	// Forward to list for cursor movement, filtering, pagination
	return a.forwardToBranchList(msg)
}

func (a *App) forwardToBranchList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	oldIdx := a.branchList.Index()
	cmd := a.branchList.Update(msg)
	newIdx := a.branchList.Index()
	if newIdx != oldIdx {
		return a, tea.Batch(cmd, a.loadCommitForSelectedBranch())
	}
	return a, cmd
}

func (a *App) handleCommitKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	km := a.commitKeys

	if msg.Type == tea.KeyUp || msg.String() == "k" {
		a.commitView.ScrollUp()
		return a, nil
	}
	if msg.Type == tea.KeyDown || msg.String() == "j" {
		a.commitView.ScrollDown()
		return a, nil
	}

	switch {
	case key.Matches(msg, km.Quit):
		return a, tea.Quit
	case key.Matches(msg, km.Help):
		a.state = StateHelp
		return a, nil
	}

	return a, nil
}

func (a *App) handleCmdPaneKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	km := a.cmdKeys

	if msg.Type == tea.KeyUp || msg.String() == "k" {
		a.cmdPane.ScrollUp()
		return a, nil
	}
	if msg.Type == tea.KeyDown || msg.String() == "j" {
		a.cmdPane.ScrollDown()
		return a, nil
	}

	switch {
	case key.Matches(msg, km.Clear):
		a.cmdPane.Clear()
		return a, nil
	case key.Matches(msg, km.Quit):
		return a, tea.Quit
	case key.Matches(msg, km.Help):
		a.state = StateHelp
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

	switch msg.String() {
	case "y", "Y":
		_, cmd := a.runDeleteAction()
		return a, cmd
	case "n", "N":
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

	switch msg.String() {
	case "y", "Y":
		_, cmd := a.runPruneAction()
		return a, cmd
	case "n", "N":
		a.state = StateNormal
		a.confirmPrompt = "y/n"
	}
	return a, nil
}

func (a *App) handleDetailsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyCtrlC || msg.Type == tea.KeyEsc || msg.String() == "q" {
		a.state = StateNormal
		return a, nil
	}
	return a, nil
}

func (a *App) handleHelpKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyCtrlC || msg.Type == tea.KeyEsc || msg.String() == "q" {
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
		if a.showCommit {
			a.focused = CommitPanel
		} else {
			a.focused = CmdPanel
			a.cmdPane.SetFocused(true)
		}
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

// cycleFocusReverse moves focus backward through panels, skipping commit if hidden.
func (a *App) cycleFocusReverse() tea.Cmd {
	switch a.focused {
	case WorktreesPanel:
		a.focused = CmdPanel
		a.cmdPane.SetFocused(true)
		return nil
	case BranchesPanel:
		a.focused = WorktreesPanel
		a.cmdPane.SetFocused(false)
		return a.loadCommitForSelectedWorktree()
	case CommitPanel:
		a.focused = BranchesPanel
		a.cmdPane.SetFocused(false)
		return a.loadCommitForSelectedBranch()
	case CmdPanel:
		if a.showCommit {
			a.focused = CommitPanel
		} else {
			a.focused = BranchesPanel
		}
		a.cmdPane.SetFocused(false)
		return nil
	}
	return nil
}

func (a *App) toggleCommitPane() (tea.Model, tea.Cmd) {
	a.showCommit = !a.showCommit
	if !a.showCommit && a.focused == CommitPanel {
		a.focused = WorktreesPanel
	}
	a.redistributePanels()
	if a.showCommit {
		if a.focused == WorktreesPanel {
			return a, a.loadCommitForSelectedWorktree()
		}
		return a, a.loadCommitForSelectedBranch()
	}
	return a, nil
}

// --- Layout ---

func (a *App) colWidths() (int, int, int) {
	if a.width < 12 {
		if a.showCommit {
			return 4, 4, 4
		}
		return 6, 6, 0
	}
	if !a.showCommit {
		col1W := a.width / 2
		col2W := a.width - col1W
		return col1W, col2W, 0
	}
	col1W := a.width * 22 / 100
	col2W := a.width * 22 / 100
	col3W := a.width - col1W - col2W
	return col1W, col2W, col3W
}

func (a *App) redistributePanels() {
	topHeight, cmdHeight := a.panelHeights()
	col1W, col2W, col3W := a.colWidths()

	// Title is embedded in the border, so content gets the full height.
	a.list.SetSize(col1W-4, topHeight)
	a.branchList.SetSize(col2W-4, topHeight)
	if a.showCommit {
		a.commitView.SetSize(col3W-4, topHeight)
	}
	a.cmdPane.SetSize(a.width-4, cmdHeight)
}

func (a *App) panelHeights() (int, int) {
	usable := a.height - 4 - 1 // -1 for status line
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
	a.commitLabel = wt.Branch
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
	a.commitLabel = branch
	repoPath := a.repoPath
	return func() tea.Msg {
		content, err := git.ShowHead(repoPath, "", branch)
		if err != nil {
			return commitLoadedMsg{content: "(error loading commit: " + err.Error() + ")"}
		}
		return commitLoadedMsg{content: content}
	}
}

// --- Streaming hook helpers ---

func listenForHookOutput(ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return nil
		}
		return msg
	}
}

func (a *App) runHookStreaming(hookCmds []string, hookName string, env map[string]string, refresh bool) tea.Cmd {
	if len(hookCmds) == 0 {
		return nil
	}
	ch := make(chan tea.Msg, 64)
	hookExec := a.hookExec

	go func() {
		var lastExit int
		for _, cmd := range hookCmds {
			result := hookExec.RunStreaming(cmd, env, func(ol hooks.OutputLine) {
				ol.Hook = hookName
				ch <- outputLineMsg{line: ol, ch: ch}
			})
			lastExit = result.ExitCode
		}
		ch <- hookDoneMsg{hookName: hookName, exitCode: lastExit, refresh: refresh}
		close(ch)
	}()

	return listenForHookOutput(ch)
}

// runPreHookChain runs pre-hooks synchronously, aborting on first failure.
func (a *App) runPreHookChain(cmds []string, env map[string]string, hookName string) bool {
	for _, cmd := range cmds {
		result := a.hookExec.Run(cmd, env)
		a.sendHookOutput(result, hookName)
		if result.ExitCode != 0 {
			a.sendOutput("stderr", hookName+" hook failed, aborting", hookName)
			return false
		}
	}
	return true
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
		Stream:    stream,
		Text:      text,
		Hook:      hook,
		Timestamp: time.Now(),
	})
}

func (a *App) sendHookOutput(result hooks.HookResult, hookName string) {
	for _, line := range strings.Split(result.Stdout, "\n") {
		if line != "" {
			a.sendOutput("stdout", line, hookName)
		}
	}
	for _, line := range strings.Split(result.Stderr, "\n") {
		if line != "" {
			a.sendOutput("stderr", line, hookName)
		}
	}
}

func (a *App) fireOnSwitch() tea.Cmd {
	worktree := a.list.Selected()
	if worktree == nil {
		return nil
	}

	hooks := a.cfg.Hooks.OnSwitch
	if len(hooks) == 0 {
		return nil
	}

	env := a.buildHookEnv(worktree, "switch")
	return a.runHookStreaming(hooks, "on_switch", env, false)
}

// openWorktree fires the on_open hook for the given worktree.
func (a *App) openWorktree(wt *model.Worktree) tea.Cmd {
	hooks := a.cfg.Hooks.OnOpen
	if len(hooks) == 0 {
		a.sendOutput("stdout", "No on_open hook configured", "info")
		return nil
	}

	env := a.buildHookEnv(wt, "open")
	return a.runHookStreaming(hooks, "on_open", env, false)
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

	if len(a.cfg.Hooks.PreDelete) > 0 {
		env := a.buildHookEnv(worktree, "delete")
		if !a.runPreHookChain(a.cfg.Hooks.PreDelete, env, "pre_delete") {
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

	if len(a.cfg.Hooks.PostDelete) > 0 {
		env := a.buildHookEnv(worktree, "delete")
		return a, tea.Batch(a.loadWorktrees(), a.loadBranches(), a.runHookStreaming(a.cfg.Hooks.PostDelete, "post_delete", env, true))
	}

	return a, tea.Batch(a.loadWorktrees(), a.loadBranches())
}

func (a *App) runCreateAction(branch string) (tea.Model, tea.Cmd) {
	a.state = StateNormal

	defaultPath := a.cfg.DefaultPathDir()
	wtPath := filepath.Join(a.projectRoot, defaultPath, branch)

	worktree := &model.Worktree{
		Path:   wtPath,
		Branch: branch,
		Name:   branch,
	}

	if len(a.cfg.Hooks.PreCreate) > 0 {
		env := a.buildHookEnv(worktree, "create")
		if !a.runPreHookChain(a.cfg.Hooks.PreCreate, env, "pre_create") {
			a.textInput.Reset()
			return a, nil
		}
	}

	// If branch already exists, check it out into the worktree; otherwise create it.
	var err error
	if git.BranchExists(a.repoPath, branch) {
		a.sendOutput("stdout", "git worktree add "+wtPath+" "+branch, "git")
		err = git.Create(a.repoPath, wtPath, "", branch)
	} else {
		a.sendOutput("stdout", "git worktree add -b "+branch+" "+wtPath, "git")
		err = git.Create(a.repoPath, wtPath, branch, "")
	}
	if err != nil {
		a.sendOutput("stderr", err.Error(), "git")
		a.textInput.Reset()
		return a, nil
	}

	a.textInput.Reset()

	if len(a.cfg.Hooks.PostCreate) > 0 {
		env := a.buildHookEnv(worktree, "create")
		return a, tea.Batch(a.loadWorktrees(), a.loadBranches(), a.runHookStreaming(a.cfg.Hooks.PostCreate, "post_create", env, true))
	}

	return a, tea.Batch(a.loadWorktrees(), a.loadBranches())
}

func (a *App) runPruneAction() (tea.Model, tea.Cmd) {
	a.state = StateNormal

	if len(a.cfg.Hooks.PrePrune) > 0 {
		env := a.buildHookEnv(nil, "prune")
		if !a.runPreHookChain(a.cfg.Hooks.PrePrune, env, "pre_prune") {
			return a, nil
		}
	}

	a.sendOutput("stdout", "git worktree prune", "git")
	err := git.Prune(a.repoPath)
	if err != nil {
		a.sendOutput("stderr", err.Error(), "git")
		return a, nil
	}

	if len(a.cfg.Hooks.PostPrune) > 0 {
		env := a.buildHookEnv(nil, "prune")
		return a, tea.Batch(a.loadWorktrees(), a.loadBranches(), a.runHookStreaming(a.cfg.Hooks.PostPrune, "post_prune", env, true))
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
			if a.showCommit {
				hovered = CommitPanel
			} else {
				hovered = BranchesPanel
			}
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
// For list panels the cursor moves via CursorUp/CursorDown; for viewport panels the content scrolls.
func (a *App) mouseScroll(panel Panel, dir int) (tea.Model, tea.Cmd) {
	switch panel {
	case WorktreesPanel:
		oldIdx := a.list.Index()
		if dir < 0 {
			a.list.list.CursorUp()
		} else {
			a.list.list.CursorDown()
		}
		if a.list.Index() != oldIdx {
			return a, tea.Batch(a.fireOnSwitch(), a.loadCommitForSelectedWorktree())
		}
	case BranchesPanel:
		oldIdx := a.branchList.Index()
		if dir < 0 {
			a.branchList.list.CursorUp()
		} else {
			a.branchList.list.CursorDown()
		}
		if a.branchList.Index() != oldIdx {
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
// Double-clicking a worktree opens it; double-clicking a branch opens its
// worktree or creates one if none exists.
func (a *App) mouseClick(panel Panel, y int) (tea.Model, tea.Cmd) {
	prevFocused := a.focused
	a.focused = panel
	a.cmdPane.SetFocused(panel == CmdPanel)

	contentRow := y - 2
	now := time.Now()
	doubleClick := panel == a.lastClickPanel &&
		contentRow == a.lastClickRow &&
		now.Sub(a.lastClickTime) < 400*time.Millisecond
	a.lastClickTime = now
	a.lastClickPanel = panel
	a.lastClickRow = contentRow

	switch panel {
	case WorktreesPanel:
		oldIdx := a.list.Index()
		absRow := a.list.list.Paginator.Page*a.list.list.Paginator.PerPage + contentRow
		if contentRow >= 0 && absRow < len(a.list.list.Items()) {
			a.list.Select(absRow)
		}
		if doubleClick {
			return a, a.handleOpen()
		}
		if a.list.Index() != oldIdx || prevFocused != WorktreesPanel {
			return a, tea.Batch(a.fireOnSwitch(), a.loadCommitForSelectedWorktree())
		}
	case BranchesPanel:
		oldIdx := a.branchList.Index()
		absRow := a.branchList.list.Paginator.Page*a.branchList.list.Paginator.PerPage + contentRow
		if contentRow >= 0 && absRow < len(a.branchList.list.Items()) {
			a.branchList.Select(absRow)
		}
		if doubleClick {
			return a.handleBranchOpen()
		}
		if a.branchList.Index() != oldIdx || prevFocused != BranchesPanel {
			return a, a.loadCommitForSelectedBranch()
		}
	}

	return a, nil
}

// --- Overlay Views ---

func (a *App) creatingView() string {
	content := lipgloss.NewStyle().
		Width(a.width - 6).
		Height(a.height - 6).
		Render("\n" + a.textInput.View())

	return renderTitledPanel(focusedBorderColor, "Create Worktree", content, a.width, a.height-4)
}

func (a *App) confirmView() string {
	content := lipgloss.NewStyle().
		Width(a.width - 6).
		Height(3).
		Align(lipgloss.Center).
		Render(a.confirmPrompt)

	return renderTitledPanel(focusedBorderColor, "Confirm", content, a.width, 5)
}

func (a *App) detailsOverlay() string {
	worktree := a.list.Selected()
	if worktree == nil {
		content := lipgloss.NewStyle().
			Width(a.width - 6).
			Height(3).
			Render("No worktree selected")
		return renderTitledPanel(focusedBorderColor, "Details", content, a.width, 5)
	}

	lines := []string{
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
		Height(a.height - 6).
		Render(strings.Join(lines, "\n"))

	return renderTitledPanel(focusedBorderColor, "Details", content, a.width, a.height-4)
}

func (a *App) helpOverlay() string {
	lines := []string{
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
		"Filtering:",
		"  /             Start filtering in current list",
		"  Enter         Apply filter",
		"  Esc           Cancel/clear filter",
		"",
		"  ?             Toggle this help",
		"  q             Quit",
		"",
		"Press q or Esc to close",
	}

	content := lipgloss.NewStyle().
		Width(a.width - 6).
		Height(a.height - 6).
		Render(strings.Join(lines, "\n"))

	return renderTitledPanel(focusedBorderColor, "Keybindings", content, a.width, a.height-4)
}

func boolToStr(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}
