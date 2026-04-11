package tui

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mbency/lazyworktree/internal/model"
)

// WorktreeItem wraps a model.Worktree for use with the bubbles list.
type WorktreeItem struct {
	Worktree model.Worktree
}

func (w WorktreeItem) FilterValue() string { return w.Worktree.Name }

// worktreeDelegate renders worktree items in the list.
type worktreeDelegate struct {
	focused      bool
	spinnerFrame string
	activePaths  map[string]bool
	showPath     bool
	pathStyle    string // "relative" or "absolute"
	projectRoot  string // used to compute relative paths
}

func (d *worktreeDelegate) Height() int                             { return 1 }
func (d *worktreeDelegate) Spacing() int                            { return 0 }
func (d *worktreeDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d *worktreeDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	wt, ok := item.(WorktreeItem)
	if !ok {
		return
	}

	name := wt.Worktree.Name
	if wt.Worktree.IsMain {
		name = "● " + name
	} else {
		name = "  " + name
	}

	var parts []string
	parts = append(parts, name)

	if d.showPath && wt.Worktree.Path != "" {
		p := wt.Worktree.Path
		if d.pathStyle != "absolute" && d.projectRoot != "" {
			if rel, err := filepath.Rel(d.projectRoot, p); err == nil {
				p = rel
			}
		}
		parts = append(parts, dimStyle.Render(p))
	}

	if wt.Worktree.LastCommitHash != "" {
		commit := wt.Worktree.LastCommitHash
		if wt.Worktree.LastCommitSubject != "" {
			commit += " " + wt.Worktree.LastCommitSubject
		}
		parts = append(parts, dimStyle.Render(commit))
	}

	row := strings.Join(parts, "  ")

	if d.activePaths[wt.Worktree.Path] {
		row += " " + d.spinnerFrame
	}
	if wt.Worktree.IsDirty {
		row += " " + dirtyStyle.Render("*")
	}
	if wt.Worktree.IsPathMissing {
		row += " " + dimStyle.Render("[missing]")
	}

	isSelected := index == m.Index()
	if isSelected {
		if d.focused {
			row = highlightStyle.Width(m.Width()).Render(row)
		} else {
			row = inactiveHighlightStyle.Width(m.Width()).Render(row)
		}
	}

	fmt.Fprint(w, row)
}

// WorktreeList is a thin wrapper around list.Model for worktrees.
type WorktreeList struct {
	list     list.Model
	delegate *worktreeDelegate
	items    []model.Worktree // keep original items for lookup
}

func NewWorktreeList() WorktreeList {
	d := &worktreeDelegate{activePaths: make(map[string]bool)}
	l := list.New(nil, d, 0, 0)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetShowFilter(false)
	l.SetFilteringEnabled(true)
	l.SetShowPagination(false)
	l.DisableQuitKeybindings()
	l.KeyMap.ShowFullHelp.SetEnabled(false)
	l.KeyMap.CloseFullHelp.SetEnabled(false)
	// Remove conflicting key bindings (d=next page conflicts with delete)
	l.KeyMap.NextPage.SetKeys("right", "l", "pgdown", "f")
	l.KeyMap.PrevPage.SetKeys("left", "h", "pgup", "b", "u")
	l.Styles.NoItems = dimStyle
	return WorktreeList{list: l, delegate: d}
}

// SetDisplayConfig propagates display settings to the delegate.
func (w *WorktreeList) SetDisplayConfig(showPath bool, pathStyle, projectRoot string) {
	w.delegate.showPath = showPath
	w.delegate.pathStyle = pathStyle
	w.delegate.projectRoot = projectRoot
}

func (w *WorktreeList) SetSpinnerFrame(frame string, activePaths map[string]bool) {
	w.delegate.spinnerFrame = frame
	w.delegate.activePaths = activePaths
}

func (w *WorktreeList) SetItems(items []model.Worktree) tea.Cmd {
	w.items = items
	listItems := make([]list.Item, len(items))
	for i, wt := range items {
		listItems[i] = WorktreeItem{Worktree: wt}
	}
	return w.list.SetItems(listItems)
}

func (w *WorktreeList) SetSize(width, height int) {
	w.list.SetSize(width, height)
}

func (w *WorktreeList) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	w.list, cmd = w.list.Update(msg)
	return cmd
}

func (w *WorktreeList) Index() int {
	return w.list.Index()
}

func (w *WorktreeList) Selected() *model.Worktree {
	item := w.list.SelectedItem()
	if item == nil {
		return nil
	}
	if wt, ok := item.(WorktreeItem); ok {
		return &wt.Worktree
	}
	return nil
}

func (w *WorktreeList) FindByBranch(branch string) *model.Worktree {
	for i := range w.items {
		if w.items[i].Branch == branch {
			return &w.items[i]
		}
	}
	return nil
}

func (w *WorktreeList) Select(index int) {
	w.list.Select(index)
}

func (w *WorktreeList) IsFiltering() bool {
	return w.list.SettingFilter()
}

func (w *WorktreeList) FilterValue() string {
	return w.list.FilterInput.Value()
}

func (w *WorktreeList) View(focused bool) string {
	w.delegate.focused = focused
	if len(w.list.Items()) == 0 && !w.list.SettingFilter() {
		empty := dimStyle.Render("  No worktrees found. Press 'n' to create one.")
		return lipgloss.NewStyle().Width(w.list.Width()).Height(w.list.Height()).Render(empty)
	}
	return w.list.View()
}
