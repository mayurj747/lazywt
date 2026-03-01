package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mbency/lazyworktree/internal/model"
)

// BranchList is a navigable list of local git branches.
// Branches with an active worktree show a ● marker and [wt] tag.
type BranchList struct {
	branches   []string
	wtBranches map[string]bool // branch name → has active worktree
	cursor     int
	width      int
	height     int
}

func NewBranchList() BranchList {
	return BranchList{wtBranches: make(map[string]bool)}
}

func (b *BranchList) SetBranches(branches []string) {
	b.branches = branches
	if b.cursor >= len(branches) && len(branches) > 0 {
		b.cursor = len(branches) - 1
	}
}

// SetWorktrees updates which branches have active worktrees.
func (b *BranchList) SetWorktrees(worktrees []model.Worktree) {
	b.wtBranches = make(map[string]bool)
	for _, wt := range worktrees {
		if wt.Branch != "" {
			b.wtBranches[wt.Branch] = true
		}
	}
}

func (b *BranchList) SetSize(width, height int) {
	b.width = width
	b.height = height
}

func (b *BranchList) MoveUp() {
	if b.cursor > 0 {
		b.cursor--
	}
}

func (b *BranchList) MoveDown() {
	if b.cursor < len(b.branches)-1 {
		b.cursor++
	}
}

func (b *BranchList) Cursor() int {
	return b.cursor
}

func (b *BranchList) Selected() string {
	if len(b.branches) == 0 {
		return ""
	}
	return b.branches[b.cursor]
}

func (b *BranchList) HasWorktree(branch string) bool {
	return b.wtBranches[branch]
}

func (b *BranchList) View() string {
	if len(b.branches) == 0 {
		return dimStyle.Render("  No branches found")
	}

	var rows []string
	for i, branch := range b.branches {
		hasWT := b.wtBranches[branch]

		marker := "  "
		if hasWT {
			marker = currentMarker.String() + " "
		}

		wtTag := ""
		if hasWT {
			wtTag = " " + dimStyle.Render("[wt]")
		}

		row := fmt.Sprintf("%s%s%s", marker, branch, wtTag)

		if i == b.cursor {
			row = highlightStyle.Width(b.width).Render(row)
		}

		rows = append(rows, row)
	}

	content := strings.Join(rows, "\n")
	if len(rows) < b.height {
		content += strings.Repeat("\n", b.height-len(rows))
	}

	return lipgloss.NewStyle().Width(b.width).Render(content)
}
