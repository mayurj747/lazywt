package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
	"github.com/mbency/lazyworktree/internal/hooks"
)

type CommandPane struct {
	lines    []hooks.OutputLine
	viewport viewport.Model
	focused  bool
	width    int
	height   int
}

func NewCommandPane() CommandPane {
	vp := viewport.New(0, 0)
	return CommandPane{viewport: vp}
}

func (c *CommandPane) SetSize(width, height int) {
	c.width = width
	c.height = height
	c.viewport.Width = width
	c.viewport.Height = height
	c.refreshContent()
}

func (c *CommandPane) SetFocused(focused bool) {
	c.focused = focused
}

func (c *CommandPane) Append(line hooks.OutputLine) {
	c.lines = append(c.lines, line)
	c.refreshContent()
	c.viewport.GotoBottom()
}

func (c *CommandPane) Clear() {
	c.lines = nil
	c.refreshContent()
}

func (c *CommandPane) ScrollUp() {
	c.viewport.LineUp(1)
}

func (c *CommandPane) ScrollDown() {
	c.viewport.LineDown(1)
}

func (c *CommandPane) refreshContent() {
	var lines []string
	for _, line := range c.lines {
		prefix := dimStyle.Render(fmt.Sprintf("[%s %s]", line.Hook, line.Timestamp.Format("15:04:05")))

		var styled string
		switch {
		case line.Stream == "stderr":
			styled = stderrStyle.Render(line.Text)
		default:
			styled = line.Text
		}

		entry := fmt.Sprintf("%s %s", prefix, styled)
		if c.width > 0 {
			entry = lipgloss.NewStyle().Width(c.width).Render(entry)
		}
		lines = append(lines, entry)
	}
	c.viewport.SetContent(strings.Join(lines, "\n"))
}

const banner = ` ██╗       █████╗  ███████╗ ██╗    ██╗  ██████╗  ██████╗  ██╗  ██╗ ████████╗
 ██║      ██╔══██╗ ╚══███╔╝ ██║    ██║ ██╔═══██╗ ██╔══██╗ ██║ ██╔╝ ╚══██╔══╝
 ██║      ███████║   ███╔╝  ██║ █╗ ██║ ██║   ██║ ██████╔╝ █████╔╝     ██║
 ██║      ██╔══██║  ███╔╝   ██║███╗██║ ██║   ██║ ██╔══██╗ ██╔═██╗     ██║
 ███████╗ ██║  ██║ ███████╗ ╚███╔███╔╝ ╚██████╔╝ ██║  ██║ ██║  ██╗    ██║
 ╚══════╝ ╚═╝  ╚═╝ ╚══════╝  ╚══╝╚══╝   ╚═════╝  ╚═╝  ╚═╝ ╚═╝  ╚═╝    ╚═╝
 ██████╗  ███████╗ ███████╗
 ██╔══██╗ ██╔════╝ ██╔════╝
 ██████╔╝ █████╗   █████╗
 ██╔══██╗ ██╔══╝   ██╔══╝
 ██║  ██║ ███████╗ ███████╗
 ╚═╝  ╚═╝ ╚══════╝ ╚══════╝`

func (c *CommandPane) View() string {
	if len(c.lines) == 0 {
		art := dimStyle.Render(banner)
		padded := lipgloss.NewStyle().Width(c.width).Height(c.height).Render(art)
		return padded
	}
	return c.viewport.View()
}
