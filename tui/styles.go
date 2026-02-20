package tui

import "github.com/charmbracelet/lipgloss"

var (
	colorPrimary  = lipgloss.Color("#F97316") // orange
	colorSuccess  = lipgloss.Color("#22C55E") // green
	colorMuted    = lipgloss.Color("#6B7280") // gray
	colorSubtle   = lipgloss.Color("#374151") // dark gray
	colorBg       = lipgloss.Color("#111827") // very dark
	colorText      = lipgloss.Color("#F9FAFB") // near white
	colorProtein  = lipgloss.Color("#60A5FA") // blue
	colorCarbs    = lipgloss.Color("#FBBF24") // amber
	colorFat      = lipgloss.Color("#F472B6") // pink
	colorCalories = lipgloss.Color("#F97316") // orange

	styleBase = lipgloss.NewStyle().
			Background(colorBg).
			Foreground(colorText)

	styleHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary).
			PaddingBottom(1)

	styleDateNav = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorText).
			Padding(0, 1)

	styleMealHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary).
			MarginTop(1)

	styleItemName = lipgloss.NewStyle().
			Foreground(colorText)

	styleItemMuted = lipgloss.NewStyle().
			Foreground(colorMuted)

	styleKcal = lipgloss.NewStyle().
			Foreground(colorCalories)

	styleProtein = lipgloss.NewStyle().
			Foreground(colorProtein)

	styleCarbs = lipgloss.NewStyle().
			Foreground(colorCarbs)

	styleFat = lipgloss.NewStyle().
			Foreground(colorFat)

	styleHelp = lipgloss.NewStyle().
			Foreground(colorMuted).
			MarginTop(1)

	styleError = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#EF4444")).
			Bold(true)

	styleSelected = lipgloss.NewStyle().
			Background(colorSubtle).
			Foreground(colorPrimary).
			Bold(true)

	styleTab = lipgloss.NewStyle().
			Padding(0, 2).
			Foreground(colorMuted)

	styleTabActive = lipgloss.NewStyle().
			Padding(0, 2).
			Foreground(colorPrimary).
			Bold(true).
			Underline(true)

	styleBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorSubtle).
			Padding(1, 2)

	styleInput = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorPrimary).
			Padding(0, 1)

	styleDimmed = lipgloss.NewStyle().
			Foreground(colorMuted)
)

func padRight(s string, width int) string {
	for len(s) < width {
		s += " "
	}
	return s
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
