package model

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/mhwang-1/meadow-bubbletea/internal/domain"
	"github.com/mhwang-1/meadow-bubbletea/internal/ui"
)

// renderDayView renders the day view for either Execute or Plan mode.
func (m RootModel) renderDayView() string {
	dt, err := m.store.ReadDailyTimeboxes(m.currentDate)
	if err != nil {
		return ui.TaskPendingStyle.Render(fmt.Sprintf("  Error loading timeboxes: %v", err))
	}

	dt.SortByStart()

	if len(dt.Timeboxes) == 0 {
		return ui.TaskPendingStyle.Render("  No timeboxes for this day. Press n to create one.")
	}

	listNames := make(map[string]string)
	listTasks := make(map[string][]domain.Task)
	for _, tb := range dt.Timeboxes {
		if tb.TaskListSlug == "" {
			continue
		}
		if _, ok := listNames[tb.TaskListSlug]; ok {
			continue
		}
		tl, err := m.store.ReadTaskList(tb.TaskListSlug)
		if err != nil {
			listNames[tb.TaskListSlug] = tb.TaskListSlug + " (error)"
			listTasks[tb.TaskListSlug] = nil
			continue
		}
		listNames[tb.TaskListSlug] = tl.Name
		listTasks[tb.TaskListSlug] = tl.ActiveTasks()
	}

	scheduledByIndex, scheduledMinByIndex := sequenceDailyTimeboxes(dt, listTasks)

	var sections []string
	var prevEnd time.Time

	for i, tb := range dt.Timeboxes {
		// Show gap indicator if there's a gap between timeboxes.
		if i > 0 && tb.Start.After(prevEnd) {
			sections = append(sections, ui.TaskPendingStyle.Render("  (no timebox)"))
		}
		prevEnd = tb.End

		taskListName := listNames[tb.TaskListSlug]
		tasks := listTasks[tb.TaskListSlug]
		scheduled := scheduledByIndex[i]
		scheduledMin := scheduledMinByIndex[i]

		switch m.mode {
		case ModeExecute:
			section := m.renderTimebox(tb, scheduled, taskListName, scheduledMin, i)
			sections = append(sections, section)
		case ModePlan:
			section := m.renderTimeboxCompact(tb, taskListName, len(tasks), scheduledMin, i)
			sections = append(sections, section)
		}
	}

	return strings.Join(sections, "\n")
}

// renderTimebox renders a single timebox in day execute view.
func (m RootModel) renderTimebox(tb domain.Timebox, tasks []domain.ScheduledTask, taskListName string, scheduledMin int, index int) string {
	var b strings.Builder

	totalMin := tb.DurationMinutes()
	timeRange := fmt.Sprintf("%s-%s", tb.Start.Format("15:04"), tb.End.Format("15:04"))

	// Time marker line: "09:00 ─────────────"
	timeStr := tb.Start.Format("15:04")
	lineWidth := 50 - lipgloss.Width(timeStr) - 1
	if m.width > 0 && m.width-2 > lipgloss.Width(timeStr)+1 {
		lineWidth = m.width - 2 - lipgloss.Width(timeStr) - 1
	}
	if lineWidth < 4 {
		lineWidth = 4
	}
	marker := timeStr + " " + strings.Repeat("─", lineWidth)
	if index == m.selectedTimebox {
		b.WriteString(ui.ActiveTimeboxStyle.Render(marker))
	} else {
		b.WriteString(ui.SeparatorStyle.Render(timeStr+" ") + ui.SeparatorStyle.Render(strings.Repeat("─", lineWidth)))
	}
	b.WriteString("\n")
	b.WriteString("\n")

	// Header line: │ 09:00-11:00 Work 03/2026 96/120
	var headerStyle lipgloss.Style
	switch tb.Status {
	case domain.StatusArchived:
		headerStyle = ui.ArchivedTimeboxStyle
	case domain.StatusUnassigned:
		headerStyle = ui.UnassignedTimeboxStyle
	default:
		headerStyle = ui.ActiveTimeboxStyle
	}

	if tb.IsReserved() {
		headerStyle = ui.ReservedTimeboxStyle
		header := fmt.Sprintf("│ %s %s Reserved %d min", timeRange, ui.TimeMarker("reserved"), totalMin)
		if tb.Note != "" {
			header += "  " + tb.Note
		}
		b.WriteString(headerStyle.Render(header))
		b.WriteString("\n")
		return b.String()
	}

	if tb.Status == domain.StatusUnassigned || tb.TaskListSlug == "" {
		header := fmt.Sprintf("│ %s (unassigned) 0/%d", timeRange, totalMin)
		b.WriteString(headerStyle.Render(header))
		b.WriteString("\n")
		b.WriteString(ui.TaskPendingStyle.Render("│   Press / to assign a task list"))
		b.WriteString("\n")
		return b.String()
	}

	// Include completed tasks' time in the scheduled total.
	completedMin := 0
	for _, ct := range tb.CompletedTasks {
		completedMin += int(ct.Duration.Minutes())
	}
	scheduledMin += completedMin

	header := fmt.Sprintf("│ %s %s %d/%d", timeRange, taskListName, scheduledMin, totalMin)
	if tb.Status == domain.StatusArchived {
		header += " ✓ archived"
	}
	b.WriteString(headerStyle.Render(header))
	b.WriteString("\n")

	// For archived timeboxes, show completed tasks.
	if tb.Status == domain.StatusArchived {
		for _, ct := range tb.CompletedTasks {
			taskLine := fmt.Sprintf("│   %s %s %s ~%s",
				ui.TimeMarker("done"),
				"",
				ct.Description,
				domain.FormatDuration(ct.Duration))
			b.WriteString(ui.ArchivedTimeboxStyle.Render(taskLine))
			b.WriteString("\n")
		}
		return b.String()
	}

	// For active timeboxes, show sequenced tasks.
	foundCurrent := false
	for _, st := range tasks {
		var marker string
		var taskStyle lipgloss.Style

		if st.IsBreak {
			marker = ui.TimeMarker("break")
			taskStyle = ui.BreakStyle
		} else if !foundCurrent {
			marker = ui.TimeMarker("current")
			taskStyle = ui.TaskCurrentStyle
			foundCurrent = true
		} else {
			marker = ui.TimeMarker("pending")
			taskStyle = ui.TaskPendingStyle
		}

		taskLine := fmt.Sprintf("│   %s %s %s ~%s",
			marker,
			st.StartTime.Format("15:04"),
			st.Task.Description,
			domain.FormatDuration(st.Task.Duration))
		b.WriteString(taskStyle.Render(taskLine))
		b.WriteString("\n")
	}

	// Completed tasks at the bottom with strikethrough.
	for _, ct := range tb.CompletedTasks {
		taskLine := fmt.Sprintf("│   ✓       %s ~%s",
			ct.Description,
			domain.FormatDuration(ct.Duration))
		b.WriteString(ui.TaskCompletedStyle.Render(taskLine))
		b.WriteString("\n")
	}

	return b.String()
}

// renderTimeboxCompact renders a timebox in day plan view (compact summary).
func (m RootModel) renderTimeboxCompact(tb domain.Timebox, taskListName string, taskCount int, scheduledMin int, index int) string {
	var b strings.Builder

	totalMin := tb.DurationMinutes()
	timeRange := fmt.Sprintf("%s-%s", tb.Start.Format("15:04"), tb.End.Format("15:04"))
	breakMin := totalMin - scheduledMin

	var headerStyle lipgloss.Style
	switch tb.Status {
	case domain.StatusArchived:
		headerStyle = ui.ArchivedTimeboxStyle
	case domain.StatusUnassigned:
		headerStyle = ui.UnassignedTimeboxStyle
	default:
		headerStyle = ui.ActiveTimeboxStyle
	}

	// Highlight selected timebox.
	headerPrefix := "│"
	if index == m.selectedTimebox {
		headerPrefix = "▸"
		headerStyle = headerStyle.Bold(true)
	}

	if tb.IsReserved() {
		headerStyle = ui.ReservedTimeboxStyle
		if index == m.selectedTimebox {
			headerStyle = headerStyle.Bold(true)
		}
		header := fmt.Sprintf("%s %s %s Reserved %d min", headerPrefix, timeRange, ui.TimeMarker("reserved"), totalMin)
		b.WriteString(headerStyle.Render(header))
		b.WriteString("\n")
		hint := "│   "
		if tb.Note != "" {
			hint += tb.Note
		} else {
			hint += "Press r to unreserve · Enter to edit times · d to delete"
		}
		b.WriteString(ui.ReservedTimeboxStyle.Render(hint))
		b.WriteString("\n")
		return b.String()
	}

	if tb.Status == domain.StatusUnassigned || tb.TaskListSlug == "" {
		header := fmt.Sprintf("%s %s (unassigned) 0/%d", headerPrefix, timeRange, totalMin)
		b.WriteString(headerStyle.Render(header))
		b.WriteString("\n")
		b.WriteString(ui.TaskPendingStyle.Render("│   Press / to assign · Enter to edit times · d to delete"))
		b.WriteString("\n")
		return b.String()
	}

	header := fmt.Sprintf("%s %s %s %d/%d", headerPrefix, timeRange, taskListName, scheduledMin, totalMin)
	b.WriteString(headerStyle.Render(header))
	b.WriteString("\n")

	summary := fmt.Sprintf("│   %d tasks · %dm scheduled", taskCount, scheduledMin)
	if breakMin > 0 {
		summary += fmt.Sprintf(" · %dm break", breakMin)
	}
	b.WriteString(ui.TaskPendingStyle.Render(summary))
	b.WriteString("\n")
	b.WriteString(ui.TaskPendingStyle.Render("│   / change list · u unassign · Enter edit times"))
	b.WriteString("\n")

	return b.String()
}

// renderWeekView renders the week view for either Execute or Plan mode.
func (m RootModel) renderWeekView() string {
	wi := domain.WeekForDate(m.currentDate)
	days := domain.DaysOfWeek(wi)

	// Calculate column width: distribute evenly across the terminal width.
	colWidth := m.width / 7
	if colWidth < 14 {
		colWidth = 14
	}

	// Build header row and separator row.
	var headerCols []string
	var sepCols []string

	dayNames := [7]string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}

	today := time.Now()
	for i, day := range days {
		isToday := day.Year() == today.Year() && day.Month() == today.Month() && day.Day() == today.Day()
		label := fmt.Sprintf("%s %d", dayNames[i], day.Day())
		padded := padOrTruncate(label, colWidth)
		if i == m.selectedWeekday && isToday {
			headerCols = append(headerCols, ui.TodayHighlightStyle.Underline(true).Render(padded))
		} else if i == m.selectedWeekday {
			headerCols = append(headerCols, ui.ActiveTimeboxStyle.Underline(true).Render(padded))
		} else if isToday {
			headerCols = append(headerCols, ui.TodayHighlightStyle.Render(padded))
		} else {
			headerCols = append(headerCols, ui.HeaderStyle.Render(padded))
		}

		sep := strings.Repeat("─", colWidth-1)
		sepCols = append(sepCols, ui.TaskPendingStyle.Render(sep))
	}

	headerRow := lipgloss.JoinHorizontal(lipgloss.Top, headerCols...)
	sepRow := lipgloss.JoinHorizontal(lipgloss.Top, sepCols...)

	// Build content rows per day.
	var dayCols []string

	for _, day := range days {
		dt, err := m.store.ReadDailyTimeboxes(day)
		if err != nil {
			dayCols = append(dayCols, padOrTruncate("  err", colWidth))
			continue
		}

		dt.SortByStart()

		listNames := make(map[string]string)
		listTasks := make(map[string][]domain.Task)
		for _, tb := range dt.Timeboxes {
			if tb.TaskListSlug == "" {
				continue
			}
			if _, ok := listNames[tb.TaskListSlug]; ok {
				continue
			}
			tl, err := m.store.ReadTaskList(tb.TaskListSlug)
			if err != nil {
				listNames[tb.TaskListSlug] = tb.TaskListSlug
				listTasks[tb.TaskListSlug] = nil
				continue
			}
			listNames[tb.TaskListSlug] = tl.Name
			listTasks[tb.TaskListSlug] = tl.ActiveTasks()
		}

		_, scheduledMinByIndex := sequenceDailyTimeboxes(dt, listTasks)

		if len(dt.Timeboxes) == 0 {
			dayCols = append(dayCols, padOrTruncate(" —", colWidth))
			continue
		}

		var lines []string
		for i, tb := range dt.Timeboxes {
			timeRange := fmt.Sprintf("%s-%s", tb.Start.Format("15:04"), tb.End.Format("15:04"))

			var name string
			if tb.IsReserved() {
				name = "■ Rsvd"
			} else if tb.TaskListSlug == "" || tb.Status == domain.StatusUnassigned {
				name = "(unasgn)"
			} else {
				name = truncate(listNames[tb.TaskListSlug], colWidth-14)
			}

			// Add status indicator.
			tbLine := fmt.Sprintf("%s %s", timeRange, name)
			if !tb.IsReserved() {
				switch tb.Status {
				case domain.StatusArchived:
					tbLine += " ✓"
				case domain.StatusActive:
					tbLine += " ▶"
				}
			}

			// Apply status-based style.
			var tbStyle lipgloss.Style
			if tb.IsReserved() {
				tbStyle = ui.ReservedTimeboxStyle
			} else {
				switch tb.Status {
				case domain.StatusArchived:
					tbStyle = ui.ArchivedTimeboxStyle
				case domain.StatusUnassigned:
					tbStyle = ui.UnassignedTimeboxStyle
				default:
					tbStyle = ui.ActiveTimeboxStyle
				}
			}

			lines = append(lines, tbStyle.Render(padOrTruncate(tbLine, colWidth)))

			// Show scheduled/total on next line, or note for reserved.
			if tb.IsReserved() {
				noteLine := "Reserved"
				if tb.Note != "" {
					noteLine = truncate(tb.Note, colWidth)
				}
				lines = append(lines, tbStyle.Render(padOrTruncate(noteLine, colWidth)))
			} else {
				scheduledMin := 0
				if tb.Status == domain.StatusActive {
					scheduledMin = scheduledMinByIndex[i]
				}
				totalMin := tb.DurationMinutes()
				statsLine := fmt.Sprintf("%d/%d", scheduledMin, totalMin)
				lines = append(lines, tbStyle.Render(padOrTruncate(statsLine, colWidth)))
			}
		}

		dayCols = append(dayCols, strings.Join(lines, "\n"))
	}

	// To render columns side by side, we need to use lipgloss.
	// Split each column into lines and pad to equal height.
	maxLines := 0
	var colLines [][]string
	for _, col := range dayCols {
		lines := strings.Split(col, "\n")
		if len(lines) > maxLines {
			maxLines = len(lines)
		}
		colLines = append(colLines, lines)
	}

	// Pad all columns to the same height.
	for i := range colLines {
		for len(colLines[i]) < maxLines {
			colLines[i] = append(colLines[i], strings.Repeat(" ", colWidth))
		}
	}

	// Build rows.
	var contentRows []string
	for row := 0; row < maxLines; row++ {
		var rowParts []string
		for col := 0; col < 7; col++ {
			rowParts = append(rowParts, colLines[col][row])
		}
		contentRows = append(contentRows, lipgloss.JoinHorizontal(lipgloss.Top, rowParts...))
	}
	gridContent := strings.Join(contentRows, "\n")

	// Task list overview section.
	taskListOverview := m.renderTaskListOverview(wi)

	return lipgloss.JoinVertical(lipgloss.Left,
		headerRow,
		sepRow,
		gridContent,
		"",
		taskListOverview,
	)
}

// renderTaskListOverview renders the task list summary for the current week.
func (m RootModel) renderTaskListOverview(wi domain.WeekInfo) string {
	var b strings.Builder

	overviewHeader := fmt.Sprintf("── Task Lists (W%d) ──", wi.Week)
	b.WriteString(ui.HeaderStyle.Render(overviewHeader))
	b.WriteString("\n")

	lists, err := m.store.ListTaskLists()
	if err != nil {
		b.WriteString(ui.TaskPendingStyle.Render(fmt.Sprintf("  Error loading task lists: %v", err)))
		return b.String()
	}

	if len(lists) == 0 {
		b.WriteString(ui.TaskPendingStyle.Render("  No task lists found."))
		return b.String()
	}

	// For each task list, calculate week stats by scanning all 7 days of timeboxes.
	days := domain.DaysOfWeek(wi)

	type taskListStats struct {
		name         string
		totalMin     int
		completedMin int
		scheduledMin int
	}

	statsMap := make(map[string]*taskListStats)
	for _, tl := range lists {
		statsMap[tl.Slug] = &taskListStats{
			name:     tl.Name,
			totalMin: int(tl.TotalDuration().Minutes()),
		}
	}

	completed, err := m.store.ReadCompleted(wi.Year, wi.Week)
	if err == nil {
		for _, ct := range completed {
			if stats, ok := statsMap[ct.TaskListSlug]; ok {
				stats.completedMin += int(ct.Task.Duration.Minutes())
			}
		}
	}

	// Scan each day's timeboxes for scheduled and completed time.
	for _, day := range days {
		dt, err := m.store.ReadDailyTimeboxes(day)
		if err != nil {
			continue
		}

		listTasks := make(map[string][]domain.Task)
		for _, tb := range dt.Timeboxes {
			if tb.TaskListSlug == "" {
				continue
			}
			if _, ok := listTasks[tb.TaskListSlug]; ok {
				continue
			}
			tl, err := m.store.ReadTaskList(tb.TaskListSlug)
			if err != nil {
				listTasks[tb.TaskListSlug] = nil
				continue
			}
			listTasks[tb.TaskListSlug] = tl.ActiveTasks()
		}

		_, scheduledMinByIndex := sequenceDailyTimeboxes(dt, listTasks)

		for i, tb := range dt.Timeboxes {
			if tb.TaskListSlug == "" {
				continue
			}
			stats, ok := statsMap[tb.TaskListSlug]
			if !ok {
				continue
			}

			// Scheduled tasks (active timeboxes).
			if tb.Status == domain.StatusActive {
				stats.scheduledMin += scheduledMinByIndex[i]
			}
		}
	}

	// Sort by name.
	sort.Slice(lists, func(i, j int) bool {
		return lists[i].Name < lists[j].Name
	})

	// Find the widest name and widest value in each column for alignment.
	maxNameWidth := 0
	for _, tl := range lists {
		if w := lipgloss.Width(tl.Name); w > maxNameWidth {
			maxNameWidth = w
		}
	}

	// Pre-compute stats strings and find max widths for right-alignment.
	type statsStrings struct {
		total       string
		completed   string
		scheduled   string
		unscheduled string
	}

	allStats := make([]statsStrings, len(lists))
	maxTotalW, maxCompW, maxSchedW, maxUnschedW := 0, 0, 0, 0

	for i, tl := range lists {
		stats := statsMap[tl.Slug]
		unscheduledMin := stats.totalMin - stats.completedMin - stats.scheduledMin
		if unscheduledMin < 0 {
			unscheduledMin = 0
		}

		s := statsStrings{
			total:       formatMinutesAsHours(stats.totalMin),
			completed:   formatMinutesAsHours(stats.completedMin),
			scheduled:   formatMinutesAsHours(stats.scheduledMin),
			unscheduled: formatMinutesAsHours(unscheduledMin),
		}
		allStats[i] = s

		if w := lipgloss.Width(s.total); w > maxTotalW {
			maxTotalW = w
		}
		if w := lipgloss.Width(s.completed); w > maxCompW {
			maxCompW = w
		}
		if w := lipgloss.Width(s.scheduled); w > maxSchedW {
			maxSchedW = w
		}
		if w := lipgloss.Width(s.unscheduled); w > maxUnschedW {
			maxUnschedW = w
		}
	}

	for i, tl := range lists {
		s := allStats[i]

		// Pad name to maxNameWidth using visual width (handles wide characters).
		namePad := maxNameWidth - lipgloss.Width(tl.Name)
		if namePad < 0 {
			namePad = 0
		}
		paddedName := tl.Name + strings.Repeat(" ", namePad)

		// Right-align each value within its column.
		totalPad := maxTotalW - lipgloss.Width(s.total)
		compPad := maxCompW - lipgloss.Width(s.completed)
		schedPad := maxSchedW - lipgloss.Width(s.scheduled)
		unschedPad := maxUnschedW - lipgloss.Width(s.unscheduled)

		nameStr := lipgloss.NewStyle().Foreground(ui.ColorTaskText).Render(paddedName)
		statsStr := lipgloss.NewStyle().Foreground(ui.ColorDimmed).Render(fmt.Sprintf(
			"%s%s   %s%s✓   %s%s⏱   %s%s○",
			strings.Repeat(" ", totalPad), s.total,
			strings.Repeat(" ", compPad), s.completed,
			strings.Repeat(" ", schedPad), s.scheduled,
			strings.Repeat(" ", unschedPad), s.unscheduled))
		b.WriteString(fmt.Sprintf(" %s  %s", nameStr, statsStr))
		b.WriteString("\n")
	}

	return b.String()
}

// formatMinutesAsHours formats minutes as hours and minutes, e.g. 168 -> "2h48m".
func formatMinutesAsHours(minutes int) string {
	if minutes <= 0 {
		return "0h"
	}
	h := minutes / 60
	m := minutes % 60
	if m == 0 {
		return fmt.Sprintf("%dh", h)
	}
	if h == 0 {
		return fmt.Sprintf("%dm", m)
	}
	return fmt.Sprintf("%dh%dm", h, m)
}

// sequenceDailyTimeboxes sequences active timeboxes per task list in
// chronological order for a single day. It returns per-timebox scheduled items
// and scheduled minutes aligned by index with dt.Timeboxes.
func sequenceDailyTimeboxes(dt *domain.DailyTimeboxes, listTasks map[string][]domain.Task) ([][]domain.ScheduledTask, []int) {
	scheduledByIndex := make([][]domain.ScheduledTask, len(dt.Timeboxes))
	scheduledMinByIndex := make([]int, len(dt.Timeboxes))
	indicesBySlug := make(map[string][]int)

	for i, tb := range dt.Timeboxes {
		if tb.Status != domain.StatusActive || tb.TaskListSlug == "" {
			continue
		}
		indicesBySlug[tb.TaskListSlug] = append(indicesBySlug[tb.TaskListSlug], i)
	}

	for slug, indices := range indicesBySlug {
		group := make([]domain.Timebox, len(indices))
		for i, idx := range indices {
			group[i] = dt.Timeboxes[idx]
		}

		groupScheduled := domain.SequenceTasksAcrossTimeboxes(group, listTasks[slug])
		for i, idx := range indices {
			scheduledByIndex[idx] = groupScheduled[i]
			scheduledMinByIndex[idx] = domain.ScheduledMinutes(groupScheduled[i])
		}
	}

	return scheduledByIndex, scheduledMinByIndex
}

// padOrTruncate ensures a string is exactly the given width, padding with
// spaces or truncating as needed.
func padOrTruncate(s string, width int) string {
	visWidth := lipgloss.Width(s)
	if visWidth >= width {
		// Truncate: take runes until we hit the width.
		return truncate(s, width)
	}
	return s + strings.Repeat(" ", width-visWidth)
}

// truncate truncates a string to a maximum visual width, appending "…" if needed.
func truncate(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= maxWidth {
		return s
	}

	runes := []rune(s)
	for i := len(runes); i > 0; i-- {
		candidate := string(runes[:i]) + "…"
		if lipgloss.Width(candidate) <= maxWidth {
			return candidate
		}
	}
	return "…"
}

// maxTimeboxCount returns the maximum number of timeboxes for the given date,
// used to clamp selectedTimebox.
func (m RootModel) maxTimeboxCount() int {
	dt, err := m.store.ReadDailyTimeboxes(m.currentDate)
	if err != nil || len(dt.Timeboxes) == 0 {
		return 0
	}
	return len(dt.Timeboxes)
}
