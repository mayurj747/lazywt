package tui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mbency/lazyworktree/internal/git"
	"github.com/mbency/lazyworktree/internal/model"
)

// BranchItem represents a branch in the list.
type BranchItem struct {
	Name        string
	Ref         string
	LocalName   string
	HasWorktree bool
	IsRemote    bool
}

func (b BranchItem) FilterValue() string { return b.Name }

// branchDelegate renders branch items in the list.
type branchDelegate struct {
	focused bool
}

func (d *branchDelegate) Height() int                             { return 1 }
func (d *branchDelegate) Spacing() int                            { return 0 }
func (d *branchDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d *branchDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	bi, ok := item.(BranchItem)
	if !ok {
		return
	}

	isSelected := index == m.Index()

	if isSelected {
		label := "  " + bi.Name
		if bi.IsRemote {
			label += " [remote]"
		}
		if bi.HasWorktree {
			label += " [wt]"
		}
		if d.focused {
			fmt.Fprint(w, highlightStyle.Width(m.Width()).Render(label))
		} else {
			fmt.Fprint(w, inactiveHighlightStyle.Width(m.Width()).Render(label))
		}
	} else {
		tags := ""
		if bi.IsRemote {
			tags += " " + dimStyle.Render("[remote]")
		}
		if bi.HasWorktree {
			tags += " " + dimStyle.Render("[wt]")
		}
		fmt.Fprintf(w, "  %s%s", bi.Name, tags)
	}
}

// BranchList is a thin wrapper around list.Model for branches.
type BranchList struct {
	list       list.Model
	delegate   *branchDelegate
	wtBranches map[string]bool
	remoteRefs map[string]string
	branches   []git.Branch
}

func NewBranchList() BranchList {
	d := &branchDelegate{}
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
	l.KeyMap.NextPage.SetKeys("right", "l", "pgdown", "f")
	l.KeyMap.PrevPage.SetKeys("left", "h", "pgup", "b", "u")
	l.Styles.NoItems = dimStyle
	return BranchList{list: l, delegate: d, wtBranches: make(map[string]bool), remoteRefs: make(map[string]string)}
}

func (b *BranchList) SetBranches(branches []git.Branch) {
	b.branches = branches
	b.remoteRefs = make(map[string]string)
	for _, branch := range branches {
		if !branch.IsRemote {
			continue
		}
		if existing, ok := b.remoteRefs[branch.Name]; ok {
			if strings.HasPrefix(existing, "origin/") {
				continue
			}
			if strings.HasPrefix(branch.Ref, "origin/") {
				b.remoteRefs[branch.Name] = branch.Ref
			}
			continue
		}
		b.remoteRefs[branch.Name] = branch.Ref
	}
	b.rebuildItems()
}

func (b *BranchList) SetWorktrees(worktrees []model.Worktree) {
	b.wtBranches = make(map[string]bool)
	for _, wt := range worktrees {
		if wt.Branch != "" {
			b.wtBranches[wt.Branch] = true
		}
	}
	b.rebuildItems()
}

func (b *BranchList) rebuildItems() {
	items := make([]list.Item, len(b.branches))
	for i, branch := range b.branches {
		items[i] = BranchItem{
			Name:        branch.Display,
			Ref:         branch.Ref,
			LocalName:   branch.Name,
			HasWorktree: b.wtBranches[branch.Name],
			IsRemote:    branch.IsRemote,
		}
	}
	b.list.SetItems(items)
}

func (b *BranchList) SetSize(width, height int) {
	b.list.SetSize(width, height)
}

func (b *BranchList) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	b.list, cmd = b.list.Update(msg)
	return cmd
}

func (b *BranchList) Index() int {
	return b.list.Index()
}

func (b *BranchList) SelectedRef() string {
	item := b.list.SelectedItem()
	if item == nil {
		return ""
	}
	if bi, ok := item.(BranchItem); ok {
		return bi.Ref
	}
	return ""
}

func (b *BranchList) SelectedCreateName() string {
	item := b.list.SelectedItem()
	if item == nil {
		return ""
	}
	if bi, ok := item.(BranchItem); ok {
		return bi.LocalName
	}
	return ""
}

func (b *BranchList) RemoteRef(name string) string {
	return b.remoteRefs[name]
}

func (b *BranchList) HasWorktree(branch string) bool {
	return b.wtBranches[branch]
}

func (b *BranchList) Select(index int) {
	b.list.Select(index)
}

func (b *BranchList) IsFiltering() bool {
	return b.list.SettingFilter()
}

func (b *BranchList) FilterValue() string {
	return b.list.FilterInput.Value()
}

func (b *BranchList) View(focused bool) string {
	b.delegate.focused = focused
	if len(b.list.Items()) == 0 && !b.list.SettingFilter() {
		empty := dimStyle.Render("  No branches found")
		return lipgloss.NewStyle().Width(b.list.Width()).Height(b.list.Height()).Render(empty)
	}
	return b.list.View()
}
