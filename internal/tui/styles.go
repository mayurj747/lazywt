package tui

import "github.com/charmbracelet/lipgloss"

var (
	focusedBorder  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("62"))
	blurredBorder  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("240"))
	titleStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("62"))
	dimStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	highlightStyle = lipgloss.NewStyle().Background(lipgloss.Color("62")).Foreground(lipgloss.Color("230"))
	dirtyStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	stderrStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	errorStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	currentMarker  = lipgloss.NewStyle().Foreground(lipgloss.Color("62")).SetString("●")
	blankMarker    = lipgloss.NewStyle().SetString(" ")
)
