package ui

import "github.com/charmbracelet/lipgloss"

// Colour constants — Lavender Haze palette.
var (
	ColorAccent     = lipgloss.Color("#7C3AED") // Active/accent (bright violet)
	ColorDone       = lipgloss.Color("#C4B5FD") // Completed (light lavender)
	ColorUnassigned = lipgloss.Color("#8B5CF6") // Unassigned (mid violet)
	ColorDimmed     = lipgloss.Color("#A78BFA") // Dimmed/pending/date
	ColorText       = lipgloss.Color("#6D28D9") // Primary text (deep violet)
	ColorTaskText   = lipgloss.Color("#581C87") // Regular task text (very dark purple)
	ColorBg         = lipgloss.Color("#FAFAFA") // Background (near-white)
	ColorBorder     = lipgloss.Color("#E9D5FF") // Borders/separators (pale lavender)
	ColorLogoLight  = lipgloss.Color("#C084FC") // Logo secondary
	ColorPending    = lipgloss.Color("#DDD6FE") // Pending markers (very light violet)
	ColorReserved   = lipgloss.Color("#D97706") // Reserved timeboxes (amber/gold)
)

// Styles — Lip Gloss styles for the Lavender Haze theme.
var (
	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorAccent)

	HeaderSecondaryStyle = lipgloss.NewStyle().
				Foreground(ColorLogoLight)

	DateStyle = lipgloss.NewStyle().
			Foreground(ColorDimmed).
			Faint(true)

	ActiveTimeboxStyle = lipgloss.NewStyle().
				Foreground(ColorAccent).
				Bold(true)

	ArchivedTimeboxStyle = lipgloss.NewStyle().
				Foreground(ColorDone).
				Faint(true)

	UnassignedTimeboxStyle = lipgloss.NewStyle().
				Foreground(ColorUnassigned)

	TaskDoneStyle = lipgloss.NewStyle().
			Foreground(ColorDone)

	TaskCompletedStyle = lipgloss.NewStyle().
				Foreground(ColorDone).
				Strikethrough(true)

	TaskCurrentStyle = lipgloss.NewStyle().
				Foreground(ColorAccent).
				Bold(true)

	TaskPendingStyle = lipgloss.NewStyle().
				Foreground(ColorDimmed)

	StatusBarStyle = lipgloss.NewStyle().
			Background(ColorBorder).
			Foreground(ColorText).
			PaddingLeft(1)

	PillActiveStyle = lipgloss.NewStyle().
			Bold(true).
			Background(ColorAccent).
			Foreground(ColorBg).
			Padding(0, 1)

	PillInactiveStyle = lipgloss.NewStyle().
				Foreground(ColorDimmed).
				Padding(0, 1)

	ShortcutBarStyle = lipgloss.NewStyle().
				Foreground(ColorDone).
				Faint(true)

	BreakStyle = lipgloss.NewStyle().
			Foreground(ColorDone).
			Italic(true)

	SeparatorStyle = lipgloss.NewStyle().
			Foreground(ColorBorder)

	TodayHighlightStyle = lipgloss.NewStyle().
				Background(ColorAccent).
				Foreground(lipgloss.Color("#FFFFFF")).
				Bold(true)

	ReservedTimeboxStyle = lipgloss.NewStyle().
				Foreground(ColorReserved).
				Italic(true)
)
