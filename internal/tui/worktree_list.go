package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mbency/lazyworktree/internal/config"
	"github.com/mbency/lazyworktree/internal/model"
)

type WorktreeList struct {
	items  []model.Worktree
	cursor int
	width  int
	height int
	cfg    *config.Config
}

func NewWorktreeList(cfg *config.Config) WorktreeList {
	return WorktreeList{cfg: cfg}
}

func (w *WorktreeList) SetItems(items []model.Worktree) {
	w.items = items
	if w.cursor >= len(items) && len(items) > 0 {
		w.cursor = len(items) - 1
	}
}

func (w *WorktreeList) SetSize(width, height int) {
	w.width = width
	w.height = height
}

func (w *WorktreeList) MoveUp() {
	if w.cursor > 0 {
		w.cursor--
	}
}

func (w *WorktreeList) MoveDown() {
	if w.cursor < len(w.items)-1 {
		w.cursor++
	}
}

func (w *WorktreeList) SelectedIndex() int {
	return w.cursor
}

func (w *WorktreeList) Cursor() int {
	return w.cursor
}

func (w *WorktreeList) Selected() *model.Worktree {
	if len(w.items) == 0 {
		return nil
	}
	return &w.items[w.cursor]
}

func (w *WorktreeList) FindByBranch(branch string) *model.Worktree {
	for i := range w.items {
		if w.items[i].Branch == branch {
			return &w.items[i]
		}
	}
	return nil
}

func (w *WorktreeList) View() string {
	if len(w.items) == 0 {
		return dimStyle.Render("  No worktrees found. Press 'n' to create one.")
	}

	var rows []string
	showPath := w.cfg.ShowPath()

	for i, wt := range w.items {
		marker := blankMarker.String()
		if wt.IsCurrent {
			marker = currentMarker.String()
		}

		row := fmt.Sprintf("%s %-20s", marker, wt.Branch)

		if showPath {
			row += fmt.Sprintf("  %-30s", wt.Name)
		}

		if wt.LastCommitHash != "" {
			row += fmt.Sprintf("  %s %s", dimStyle.Render(wt.LastCommitHash), wt.LastCommitSubject)
		}

		if wt.IsDirty {
			row += "  " + dirtyStyle.Render("[dirty]")
		}

		if i == w.cursor {
			row = highlightStyle.Width(w.width).Render(row)
		}

		rows = append(rows, row)
	}

	content := strings.Join(rows, "\n")

	visibleHeight := w.height
	lineCount := len(rows)
	if lineCount < visibleHeight {
		content += strings.Repeat("\n", visibleHeight-lineCount)
	}

	return lipgloss.NewStyle().Width(w.width).Render(content)
}
