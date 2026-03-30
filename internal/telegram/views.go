package telegram

import (
	"fmt"
	"strings"
	"time"

	"github.com/mhwang-1/meadow-bubbletea/internal/domain"
	"github.com/mhwang-1/meadow-bubbletea/internal/service"
	"github.com/mhwang-1/meadow-bubbletea/internal/store"
)

// formatMainView renders the day schedule view.
// No emojis. Uses [done], [ ], Break, -- separators.
func formatMainView(dv *service.DayView, date time.Time) string {
	wi := domain.WeekForDate(date)

	var b strings.Builder
	b.WriteString(fmt.Sprintf("<b>%s -- %s -- W%d</b>\n",
		dayLabel(date),
		date.Format("02 Jan 2006"),
		wi.Week,
	))

	if len(dv.Timeboxes) == 0 {
		b.WriteString("\nNo timeboxes scheduled.")
		return b.String()
	}

	for _, dvtb := range dv.Timeboxes {
		tb := dvtb.Timebox
		b.WriteString("\n")

		if tb.IsReserved() {
			line := fmt.Sprintf("%s-%s Reserved",
				tb.Start.Format("15:04"),
				tb.End.Format("15:04"),
			)
			if tb.Note != "" {
				line += " -- " + tb.Note
			}
			b.WriteString(line + "\n")
			continue
		}

		if tb.TaskListSlug == "" {
			b.WriteString(fmt.Sprintf("%s-%s (unassigned)\n",
				tb.Start.Format("15:04"),
				tb.End.Format("15:04"),
			))
			continue
		}

		if tb.Status == domain.StatusArchived {
			completedMins := 0
			for _, ct := range tb.CompletedTasks {
				completedMins += int(ct.Duration.Minutes())
			}
			b.WriteString(fmt.Sprintf("%s-%s %s (%d/%d) [archived]\n",
				tb.Start.Format("15:04"),
				tb.End.Format("15:04"),
				dvtb.TaskListName,
				completedMins,
				tb.DurationMinutes(),
			))
			for _, ct := range tb.CompletedTasks {
				b.WriteString(fmt.Sprintf("  [done] %s ~%s\n",
					ct.Description,
					domain.FormatDuration(ct.Duration),
				))
			}
			continue
		}

		// Active timebox with task list.
		completedMins := 0
		for _, ct := range tb.CompletedTasks {
			completedMins += int(ct.Duration.Minutes())
		}
		b.WriteString(fmt.Sprintf("%s-%s %s (%d/%d)\n",
			tb.Start.Format("15:04"),
			tb.End.Format("15:04"),
			dvtb.TaskListName,
			completedMins,
			tb.DurationMinutes(),
		))

		// Show completed tasks.
		for _, ct := range tb.CompletedTasks {
			b.WriteString(fmt.Sprintf("  [done] %s ~%s\n",
				ct.Description,
				domain.FormatDuration(ct.Duration),
			))
		}

		// Show scheduled (pending) tasks.
		for _, st := range dvtb.ScheduledTasks {
			if isCompletedInTimebox(tb.CompletedTasks, st.Task) {
				continue
			}
			if st.IsBreak {
				b.WriteString(fmt.Sprintf("  Break ~%s\n",
					domain.FormatDuration(st.Task.Duration),
				))
			} else {
				b.WriteString(fmt.Sprintf("  [ ] %s ~%s\n",
					st.Task.Description,
					domain.FormatDuration(st.Task.Duration),
				))
			}
		}
	}

	return b.String()
}

// formatTaskListView renders a task list with stats.
func formatTaskListView(tl *domain.TaskList) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("<b>%s</b>\n", tl.Name))

	active := tl.ActiveTasks()
	if len(active) == 0 {
		b.WriteString("\nNo active tasks.")
		return b.String()
	}

	for i, t := range active {
		b.WriteString(fmt.Sprintf("\n%d. %s ~%s", i+1, t.Description, domain.FormatDuration(t.Duration)))
	}

	totalMins := int(tl.TotalDuration().Minutes())
	b.WriteString(fmt.Sprintf("\n\n%d tasks -- %s total",
		len(active),
		domain.FormatDuration(time.Duration(totalMins)*time.Minute),
	))

	return b.String()
}

// formatTaskListEdit renders a task list in edit mode with remove buttons.
func formatTaskListEdit(tl *domain.TaskList) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("<b>Edit: %s</b>\n", tl.Name))

	active := tl.ActiveTasks()
	if len(active) == 0 {
		b.WriteString("\nNo active tasks.")
		return b.String()
	}

	for i, t := range active {
		b.WriteString(fmt.Sprintf("\n%d. %s ~%s", i+1, t.Description, domain.FormatDuration(t.Duration)))
	}

	return b.String()
}

// formatHistoryView renders completed tasks grouped by list.
func formatHistoryView(tasks []store.CompletedTask, year, week int) string {
	if len(tasks) == 0 {
		return fmt.Sprintf("<b>History -- W%d %d</b>\n\nNo completed tasks.", week, year)
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("<b>History -- W%d %d</b>\n", week, year))

	// Group by slug.
	grouped := make(map[string][]store.CompletedTask)
	var slugOrder []string
	for _, ct := range tasks {
		if _, seen := grouped[ct.TaskListSlug]; !seen {
			slugOrder = append(slugOrder, ct.TaskListSlug)
		}
		grouped[ct.TaskListSlug] = append(grouped[ct.TaskListSlug], ct)
	}

	for _, slug := range slugOrder {
		cts := grouped[slug]
		b.WriteString(fmt.Sprintf("\n<b>%s</b>\n", slug))
		for _, ct := range cts {
			b.WriteString(fmt.Sprintf("  %s ~%s (%s)\n",
				ct.Task.Description,
				domain.FormatDuration(ct.Task.Duration),
				ct.CompletedDate.Format("02 Jan"),
			))
		}
	}

	return b.String()
}

// formatNotesView renders the notes content.
func formatNotesView(content string) string {
	if content == "" {
		return "<b>Notes</b>\n\n(empty)"
	}

	var b strings.Builder
	b.WriteString("<b>Notes</b>\n\n")
	// Escape HTML in notes content.
	escaped := strings.ReplaceAll(content, "&", "&amp;")
	escaped = strings.ReplaceAll(escaped, "<", "&lt;")
	escaped = strings.ReplaceAll(escaped, ">", "&gt;")
	b.WriteString(escaped)
	return b.String()
}

// formatArchivedListsView renders archived task lists for a given week.
func formatArchivedListsView(lists []*domain.TaskList, year, week int) string {
	if len(lists) == 0 {
		return fmt.Sprintf("<b>Archived Lists -- W%d %d</b>\n\nNo archived lists.", week, year)
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("<b>Archived Lists -- W%d %d</b>\n", week, year))

	for _, tl := range lists {
		active := tl.ActiveTasks()
		totalMins := int(tl.TotalDuration().Minutes())
		b.WriteString(fmt.Sprintf("\n%s -- %d tasks -- %s\n",
			tl.Name,
			len(active),
			domain.FormatDuration(time.Duration(totalMins)*time.Minute),
		))
	}

	return b.String()
}

// formatTaskListOverviewTab renders the lists overview for a tab.
func formatTaskListOverviewTab(lists []*domain.TaskList) string {
	if len(lists) == 0 {
		return "<b>Task Lists</b>\n\nNo task lists found."
	}

	var b strings.Builder
	b.WriteString("<b>Task Lists</b>\n")

	for _, tl := range lists {
		active := tl.ActiveTasks()
		totalMins := int(tl.TotalDuration().Minutes())
		b.WriteString(fmt.Sprintf("\n%s\n   %d tasks -- %s\n",
			tl.Name,
			len(active),
			domain.FormatDuration(time.Duration(totalMins)*time.Minute),
		))
	}

	return b.String()
}

// isCompletedInTimebox checks whether a task appears in the timebox's completed list.
func isCompletedInTimebox(completed []domain.Task, task domain.Task) bool {
	for _, ct := range completed {
		if ct.Description == task.Description && ct.Duration == task.Duration {
			return true
		}
	}
	return false
}

