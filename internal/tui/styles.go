package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	focusedBorderColor lipgloss.Color = "62"
	blurredBorderColor lipgloss.Color = "240"

	focusedBorder  = lipgloss.NewStyle().Border(lipgloss.ThickBorder()).BorderForeground(focusedBorderColor)
	blurredBorder  = lipgloss.NewStyle().Border(lipgloss.ThickBorder()).BorderForeground(blurredBorderColor)
	titleStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("62"))
	dimStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	highlightStyle         = lipgloss.NewStyle().Background(lipgloss.Color("62")).Foreground(lipgloss.Color("230"))
	inactiveHighlightStyle = lipgloss.NewStyle().Background(lipgloss.Color("238")).Foreground(lipgloss.Color("250"))
	dirtyStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	stderrStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	errorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
)

// renderTitledPanel renders content inside a rounded border with the title
// embedded in the top border line (e.g. ╭─ Worktrees ──────╮).
// w is the total outer width, h is the inner content height.
func renderTitledPanel(borderColor lipgloss.Color, title, content string, w, h int) string {
	bd := lipgloss.ThickBorder()
	boxStyle := lipgloss.NewStyle().
		Border(bd).
		BorderForeground(borderColor).
		Width(w - 2).Height(h).
		MaxWidth(w).MaxHeight(h + 2)

	rendered := boxStyle.Render(content)

	if title == "" {
		return rendered
	}

	lines := strings.Split(rendered, "\n")
	if len(lines) == 0 {
		return rendered
	}

	bStyle := lipgloss.NewStyle().Foreground(borderColor)
	styledTitle := bStyle.Bold(true).Render(title)
	titleW := lipgloss.Width(styledTitle)

	// ╭──Title──...──╮
	// 1 for ╭, 2 for ── before title, 1 for ╮
	fillW := w - 4 - titleW
	if fillW < 0 {
		fillW = 0
	}

	lines[0] = bStyle.Render(bd.TopLeft+bd.Top+bd.Top) +
		styledTitle +
		bStyle.Render(strings.Repeat(bd.Top, fillW)+bd.TopRight)

	return strings.Join(lines, "\n")
}
