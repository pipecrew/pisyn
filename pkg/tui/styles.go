package tui

import "charm.land/lipgloss/v2"

var (
	colorGreen  = lipgloss.Color("#22c55e")
	colorRed    = lipgloss.Color("#ef4444")
	colorOrange = lipgloss.Color("#f97316")
	colorYellow = lipgloss.Color("#eab308")
	colorGray   = lipgloss.Color("#6b7280")
	colorDim    = lipgloss.Color("#4b5563")
	colorWhite  = lipgloss.Color("#f9fafb")
	colorBorder = lipgloss.Color("#374151")

	stylePassed       = lipgloss.NewStyle().Foreground(colorGreen)
	styleFailed       = lipgloss.NewStyle().Foreground(colorRed).Bold(true)
	styleAllowFailure = lipgloss.NewStyle().Foreground(colorOrange)
	styleRunning      = lipgloss.NewStyle().Foreground(colorYellow)
	stylePending      = lipgloss.NewStyle().Foreground(colorGray)
	styleSkipped      = lipgloss.NewStyle().Foreground(colorDim)

	styleSelected = lipgloss.NewStyle().Bold(true).Background(lipgloss.Color("#1f2937"))

	styleJobPanel = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Padding(0, 1)

	styleLogPanel = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Padding(0, 1)

	styleStatusBar = lipgloss.NewStyle().
			Foreground(colorWhite).
			Background(lipgloss.Color("#1f2937")).
			Padding(0, 1)

	styleTitle = lipgloss.NewStyle().Bold(true).Foreground(colorWhite)
)
