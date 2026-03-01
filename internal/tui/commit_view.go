package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
)

var (
	diffAddStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))   // green
	diffRemoveStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))   // red
	diffHunkStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))   // cyan
	diffHeaderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("244")) // mid-gray
	commitHashStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("62"))
)

// CommitView is a scrollable panel showing the output of `git show HEAD`.
type CommitView struct {
	viewport viewport.Model
	empty    bool
	width    int
	height   int
}

func NewCommitView() CommitView {
	vp := viewport.New(0, 0)
	return CommitView{viewport: vp, empty: true}
}

func (c *CommitView) SetSize(width, height int) {
	c.width = width
	c.height = height
	c.viewport.Width = width
	c.viewport.Height = height
}

func (c *CommitView) SetContent(raw string) {
	c.empty = strings.TrimSpace(raw) == ""
	c.viewport.SetContent(c.colorize(raw))
	c.viewport.GotoTop()
}

func (c *CommitView) ScrollUp() {
	c.viewport.LineUp(3)
}

func (c *CommitView) ScrollDown() {
	c.viewport.LineDown(3)
}

func (c *CommitView) colorize(raw string) string {
	lines := strings.Split(raw, "\n")
	result := make([]string, len(lines))
	for i, line := range lines {
		switch {
		case strings.HasPrefix(line, "commit "):
			result[i] = commitHashStyle.Render(line)
		case strings.HasPrefix(line, "Author:") || strings.HasPrefix(line, "Date:") || strings.HasPrefix(line, "Merge:"):
			result[i] = dimStyle.Render(line)
		case strings.HasPrefix(line, "diff --git") || strings.HasPrefix(line, "index ") ||
			strings.HasPrefix(line, "new file") || strings.HasPrefix(line, "deleted file") ||
			strings.HasPrefix(line, "--- ") || strings.HasPrefix(line, "+++ "):
			result[i] = diffHeaderStyle.Render(line)
		case strings.HasPrefix(line, "@@"):
			result[i] = diffHunkStyle.Render(line)
		case len(line) > 0 && line[0] == '+':
			result[i] = diffAddStyle.Render(line)
		case len(line) > 0 && line[0] == '-':
			result[i] = diffRemoveStyle.Render(line)
		default:
			result[i] = line
		}
	}
	return strings.Join(result, "\n")
}

func (c *CommitView) View() string {
	if c.empty || c.viewport.Height == 0 {
		return lipgloss.NewStyle().Width(c.width).Height(c.height).Render(
			dimStyle.Render("  Select a worktree or branch to view HEAD commit"),
		)
	}
	return c.viewport.View()
}
