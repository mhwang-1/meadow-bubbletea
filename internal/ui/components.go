package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// RenderHeader renders the top header bar distributed across the given width.
//
// Layout: Left ("meadow" + "bubbletea") | Centre (mode pills, view pills) | Right (date).
// A separator line is rendered below.
func RenderHeader(width int, mode string, view string, dateStr string) string {
	left := HeaderStyle.Render("meadow") + HeaderSecondaryStyle.Render("bubbletea")

	modePills := lipgloss.JoinHorizontal(lipgloss.Top,
		ModePill("Execute", mode == "Execute"),
		ModePill("Plan", mode == "Plan"),
	)
	viewPills := lipgloss.JoinHorizontal(lipgloss.Top,
		ModePill("Day", view == "Day"),
		ModePill("Week", view == "Week"),
	)
	centre := lipgloss.JoinHorizontal(lipgloss.Top, modePills, "  ", viewPills)

	right := DateStyle.Render(dateStr)

	// Place centre at the absolute middle of the terminal width so it
	// stays fixed regardless of left/right content length.
	leftWidth := lipgloss.Width(left)
	centreWidth := lipgloss.Width(centre)
	rightWidth := lipgloss.Width(right)

	usedWidth := leftWidth + centreWidth + rightWidth

	var headerLine string
	if usedWidth >= width {
		// Not enough room — just join with single spaces.
		headerLine = lipgloss.JoinHorizontal(lipgloss.Top, left, " ", centre, " ", right)
	} else {
		// Centre pills at absolute middle; left and right fill their sides.
		centreStart := (width - centreWidth) / 2
		leftGap := centreStart - leftWidth
		if leftGap < 1 {
			leftGap = 1
		}
		rightGap := width - leftWidth - leftGap - centreWidth - rightWidth
		if rightGap < 1 {
			rightGap = 1
		}

		headerLine = lipgloss.JoinHorizontal(lipgloss.Top,
			left,
			strings.Repeat(" ", leftGap),
			centre,
			strings.Repeat(" ", rightGap),
			right,
		)
	}

	// Separator line below header.
	sep := SeparatorStyle.Render(strings.Repeat("─", width))

	return headerLine + "\n" + sep
}

// RenderStatusBar renders a full-width status bar containing the given text.
func RenderStatusBar(width int, text string) string {
	return StatusBarStyle.Width(width).Render(text)
}

// RenderShortcutBar renders the bottom keybinding hints bar, centred.
func RenderShortcutBar(width int, shortcuts string) string {
	styled := ShortcutBarStyle.Render(shortcuts)
	return lipgloss.PlaceHorizontal(width, lipgloss.Center, styled)
}

// ModePill renders a single pill label with the appropriate active/inactive style.
func ModePill(label string, active bool) string {
	if active {
		return PillActiveStyle.Render(label)
	}
	return PillInactiveStyle.Render(label)
}

// TimeMarker returns a styled marker character for the given task status.
//
// Recognised statuses: "done", "current", "pending", "break".
func TimeMarker(status string) string {
	switch status {
	case "done":
		return TaskDoneStyle.Render("✓")
	case "current":
		return TaskCurrentStyle.Render("●")
	case "pending":
		return lipgloss.NewStyle().Foreground(ColorPending).Render("○")
	case "break":
		return BreakStyle.Render("·")
	case "reserved":
		return ReservedTimeboxStyle.Render("■")
	default:
		return lipgloss.NewStyle().Foreground(ColorPending).Render("○")
	}
}
