package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Colors
	primaryColor   = lipgloss.Color("#7C3AED") // Purple
	secondaryColor = lipgloss.Color("#10B981") // Green
	mutedColor     = lipgloss.Color("#6B7280") // Gray
	accentColor    = lipgloss.Color("#F59E0B") // Amber
	errorColor     = lipgloss.Color("#EF4444") // Red
	bgColor        = lipgloss.Color("#1F2937") // Dark gray
	fgColor        = lipgloss.Color("#F9FAFB") // Light

	// Layout styles
	AppStyle    = lipgloss.NewStyle().Padding(1, 2)
	HeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(primaryColor).MarginBottom(1)

	// List panel (left side)
	ListPanelStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(mutedColor).Padding(0, 1)

	// Detail panel (right side)
	DetailPanelStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(primaryColor).Padding(1, 2)

	// Event list item styles
	SelectedItemStyle = lipgloss.NewStyle().Background(primaryColor).Foreground(fgColor).Bold(true).Padding(0, 1)
	SelectedPastStyle = lipgloss.NewStyle().Background(lipgloss.Color("#374151")).Foreground(lipgloss.Color("#9CA3AF")).Padding(0, 1)
	NormalItemStyle   = lipgloss.NewStyle().Foreground(fgColor).Padding(0, 1)
	PastItemStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#52525B")).Faint(true).Padding(0, 1)
	TimeStyle         = lipgloss.NewStyle().Foreground(secondaryColor).Width(12)
	PastTimeStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#52525B")).Faint(true).Width(12)
	DurationStyle     = lipgloss.NewStyle().Foreground(mutedColor).Width(6)

	// Detail panel styles
	TitleStyle          = lipgloss.NewStyle().Bold(true).Foreground(primaryColor).MarginBottom(1)
	LabelStyle          = lipgloss.NewStyle().Foreground(accentColor).Bold(true).Width(14)
	ValueStyle          = lipgloss.NewStyle().Foreground(fgColor)
	LinkStyle           = lipgloss.NewStyle().Foreground(lipgloss.Color("#60A5FA")).Underline(true)
	StatusAcceptedStyle = lipgloss.NewStyle().Foreground(secondaryColor)
	StatusDeclinedStyle = lipgloss.NewStyle().Foreground(errorColor)
	StatusPendingStyle  = lipgloss.NewStyle().Foreground(accentColor)

	// Help bar
	HelpStyle    = lipgloss.NewStyle().Foreground(mutedColor).MarginTop(1)
	HelpKeyStyle = lipgloss.NewStyle().Foreground(primaryColor).Bold(true)

	// In progress indicator
	InProgressStyle = lipgloss.NewStyle().Background(secondaryColor).Foreground(fgColor).Bold(true).Padding(0, 1)

	// Calendar badge
	CalendarBadgeStyle = lipgloss.NewStyle().Foreground(mutedColor).Italic(true)
)
