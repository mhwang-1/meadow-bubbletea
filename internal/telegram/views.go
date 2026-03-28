package telegram

import (
	"fmt"
	"log"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/hwang/meadow-bubbletea/internal/domain"
	"github.com/hwang/meadow-bubbletea/internal/store"
)

// formatTodayView renders the /today response for a given date.
func formatTodayView(dt *domain.DailyTimeboxes, s *store.Store, date time.Time) string {
	wi := domain.WeekForDate(date)

	var b strings.Builder
	b.WriteString(fmt.Sprintf("\U0001f4c5 %s \u00b7 W%d\n", domain.FormatDate(date), wi.Week))

	if len(dt.Timeboxes) == 0 {
		b.WriteString("\nNo timeboxes scheduled.")
		return b.String()
	}

	listTasks := make(map[string][]domain.Task)
	listNames := make(map[string]string)
	for _, tb := range dt.Timeboxes {
		if tb.TaskListSlug == "" {
			continue
		}
		if _, ok := listTasks[tb.TaskListSlug]; ok {
			continue
		}
		tl, err := s.ReadTaskList(tb.TaskListSlug)
		if err != nil {
			listTasks[tb.TaskListSlug] = nil
			listNames[tb.TaskListSlug] = tb.TaskListSlug + " (error loading list)"
			continue
		}
		listTasks[tb.TaskListSlug] = tl.ActiveTasks()
		listNames[tb.TaskListSlug] = tl.Name
	}

	scheduledByIndex := sequenceDailyTimeboxes(dt.Timeboxes, listTasks)

	for i, tb := range dt.Timeboxes {
		b.WriteString("\n")

		if tb.IsReserved() {
			line := fmt.Sprintf("\U0001f512 %s-%s Reserved",
				tb.Start.Format("15:04"),
				tb.End.Format("15:04"),
			)
			if tb.Note != "" {
				line += "  " + tb.Note
			}
			b.WriteString(line + "\n")
			continue
		}

		if tb.TaskListSlug == "" {
			// Unassigned timebox.
			b.WriteString(fmt.Sprintf("\u23f0 %s-%s (unassigned) 0/%d\n",
				tb.Start.Format("15:04"),
				tb.End.Format("15:04"),
				tb.DurationMinutes(),
			))
			continue
		}

		// Compute completed minutes.
		completedMins := 0
		for _, ct := range tb.CompletedTasks {
			completedMins += int(ct.Duration.Minutes())
		}

		b.WriteString(fmt.Sprintf("\u23f0 %s-%s %s (%d/%d)\n",
			tb.Start.Format("15:04"),
			tb.End.Format("15:04"),
			listNames[tb.TaskListSlug],
			completedMins,
			tb.DurationMinutes(),
		))

		// Show completed tasks.
		for _, ct := range tb.CompletedTasks {
			b.WriteString(fmt.Sprintf("\u2705 %s ~%s\n",
				ct.Description,
				domain.FormatDuration(ct.Duration),
			))
		}

		for _, st := range scheduledByIndex[i] {
			if isCompletedTask(tb.CompletedTasks, st.Task) {
				continue
			}
			if st.IsBreak {
				b.WriteString(fmt.Sprintf("\u23f8 %s ~%s\n",
					st.Task.Description,
					domain.FormatDuration(st.Task.Duration),
				))
			} else {
				b.WriteString(fmt.Sprintf("\u2b1a %s ~%s\n",
					st.Task.Description,
					domain.FormatDuration(st.Task.Duration),
				))
			}
		}
	}

	return b.String()
}

func sequenceDailyTimeboxes(timeboxes []domain.Timebox, listTasks map[string][]domain.Task) [][]domain.ScheduledTask {
	result := make([][]domain.ScheduledTask, len(timeboxes))
	indicesBySlug := make(map[string][]int)

	for i, tb := range timeboxes {
		if tb.Status != domain.StatusActive || tb.TaskListSlug == "" {
			continue
		}
		indicesBySlug[tb.TaskListSlug] = append(indicesBySlug[tb.TaskListSlug], i)
	}

	for slug, indices := range indicesBySlug {
		group := make([]domain.Timebox, len(indices))
		for i, idx := range indices {
			group[i] = timeboxes[idx]
		}
		groupScheduled := domain.SequenceTasksAcrossTimeboxes(group, listTasks[slug])
		for i, idx := range indices {
			result[idx] = groupScheduled[i]
		}
	}

	return result
}

func isCompletedTask(completed []domain.Task, task domain.Task) bool {
	for _, ct := range completed {
		if ct.Description == task.Description && ct.Duration == task.Duration {
			return true
		}
	}
	return false
}

// formatWeekView renders the /week response.
func formatWeekView(wi domain.WeekInfo, s *store.Store) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("\U0001f4c5 W%d \u00b7 %s\n", wi.Week, domain.FormatWeekRange(wi)))

	days := domain.DaysOfWeek(wi)
	dayNames := [7]string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}

	for i, date := range days {
		dt, err := s.ReadDailyTimeboxes(date)
		if err != nil {
			continue
		}
		if len(dt.Timeboxes) == 0 {
			continue
		}

		dt.SortByStart()
		b.WriteString(fmt.Sprintf("\n%s %d:", dayNames[i], date.Day()))

		for j, tb := range dt.Timeboxes {
			if j > 0 {
				b.WriteString(" |")
			}

			timeStr := fmt.Sprintf(" %s-%s",
				tb.Start.Format("15:04"),
				tb.End.Format("15:04"),
			)

			if tb.IsReserved() {
				label := "Reserved"
				if tb.Note != "" {
					label = tb.Note
				}
				b.WriteString(fmt.Sprintf("%s \U0001f512 %s", timeStr, label))
				continue
			}

			if tb.TaskListSlug == "" {
				b.WriteString(fmt.Sprintf("%s (unassigned)", timeStr))
				continue
			}

			tl, err := s.ReadTaskList(tb.TaskListSlug)
			listLabel := tb.TaskListSlug
			if err == nil {
				listLabel = tl.Name
			}

			completedMins := 0
			for _, ct := range tb.CompletedTasks {
				completedMins += int(ct.Duration.Minutes())
			}

			b.WriteString(fmt.Sprintf("%s %s (%d/%d)",
				timeStr,
				listLabel,
				completedMins,
				tb.DurationMinutes(),
			))
		}
	}

	return b.String()
}

// formatTaskListOverview renders the /lists response.
func formatTaskListOverview(lists []*domain.TaskList) string {
	if len(lists) == 0 {
		return "No task lists found."
	}

	var b strings.Builder
	b.WriteString("<b>Task Lists</b>\n")

	for _, tl := range lists {
		active := tl.ActiveTasks()
		totalMins := int(tl.TotalDuration().Minutes())
		b.WriteString(fmt.Sprintf("\n\U0001f4cb %s\n   %d tasks \u00b7 %s\n",
			tl.Name,
			len(active),
			domain.FormatDuration(time.Duration(totalMins)*time.Minute),
		))
	}

	return b.String()
}

// formatTaskListDetail renders the /list response.
func formatTaskListDetail(tl *domain.TaskList) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("<b>%s</b>\n", tl.Name))

	active := tl.ActiveTasks()
	if len(active) == 0 {
		b.WriteString("\nNo active tasks.")
		return b.String()
	}

	for _, t := range active {
		b.WriteString(fmt.Sprintf("\n\u2022 %s ~%s", t.Description, domain.FormatDuration(t.Duration)))
	}

	totalMins := int(tl.TotalDuration().Minutes())
	b.WriteString(fmt.Sprintf("\n\n%d tasks \u00b7 %s total",
		len(active),
		domain.FormatDuration(time.Duration(totalMins)*time.Minute),
	))

	return b.String()
}

// todayKeyboard builds the inline keyboard for the /today view.
func todayKeyboard(date time.Time, dt *domain.DailyTimeboxes, s *store.Store) tgbotapi.InlineKeyboardMarkup {
	prev := date.AddDate(0, 0, -1).Format("2006-01-02")
	next := date.AddDate(0, 0, 1).Format("2006-01-02")

	rows := [][]tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("\u2190 Prev", "prev:"+prev),
			tgbotapi.NewInlineKeyboardButtonData("Next \u2192", "next:"+next),
		),
	}

	rows = append(rows, todayActionRows(dt, s, date)...)
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

// taskDoneButton creates an inline button to mark the next task as done
// for a specific date/timebox.
func taskDoneButton(taskDesc, dateStr, startHHMM string) tgbotapi.InlineKeyboardButton {
	label := "\u2713 Done: " + taskDesc
	if len([]rune(label)) > 30 {
		label = string([]rune(label)[:29]) + "…"
	}

	// Format: done:{YYYY-MM-DD}:{HHMM}
	data := "done:" + dateStr + ":" + startHHMM

	return tgbotapi.NewInlineKeyboardButtonData(label, data)
}

// handleCallback processes inline button presses.
func handleCallback(b *Bot, query *tgbotapi.CallbackQuery) {
	data := query.Data
	chatID := query.Message.Chat.ID

	// Acknowledge the callback.
	callback := tgbotapi.NewCallback(query.ID, "")
	if _, err := b.api.Request(callback); err != nil {
		log.Printf("Error acknowledging callback: %v", err)
	}

	if strings.HasPrefix(data, "prev:") || strings.HasPrefix(data, "next:") {
		// Navigate to a different date.
		dateStr := strings.SplitN(data, ":", 2)[1]
		date, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			log.Printf("Error parsing callback date: %v", err)
			return
		}

		dt, err := b.store.ReadDailyTimeboxes(date)
		if err != nil {
			log.Printf("Error reading timeboxes: %v", err)
			return
		}
		dt.SortByStart()

		text := formatTodayView(dt, b.store, date)
		keyboard := todayKeyboard(date, dt, b.store)

		edit := tgbotapi.NewEditMessageText(chatID, query.Message.MessageID, text)
		edit.ParseMode = "HTML"
		markup := keyboard
		edit.ReplyMarkup = &markup

		if _, err := b.api.Send(edit); err != nil {
			log.Printf("Error editing message: %v", err)
		}
		return
	}

	if strings.HasPrefix(data, "done:") {
		rest := strings.SplitN(data, ":", 3)
		if len(rest) < 3 {
			log.Printf("Invalid done callback data: %q", data)
			return
		}
		args := rest[1] + " " + rest[2]
		msg, err := b.handleDoneForTimebox(chatID, args)
		if err != nil {
			log.Printf("Error handling done callback: %v", err)
			msg = tgbotapi.NewMessage(chatID, "Error: "+err.Error())
		}
		msg.ParseMode = "HTML"
		if _, err := b.api.Send(msg); err != nil {
			log.Printf("Error sending done confirmation: %v", err)
		}
		return
	}

	if strings.HasPrefix(data, "archive:") {
		// Format: archive:{date}:{time}
		rest := strings.SplitN(data, ":", 3)
		if len(rest) < 3 {
			log.Printf("Invalid archive callback data: %q", data)
			return
		}
		args := rest[1] + " " + rest[2]
		msg, err := b.handleArchive(chatID, args)
		if err != nil {
			log.Printf("Error handling archive callback: %v", err)
			msg = tgbotapi.NewMessage(chatID, "Error: "+err.Error())
		}
		msg.ParseMode = "HTML"
		if _, err := b.api.Send(msg); err != nil {
			log.Printf("Error sending archive confirmation: %v", err)
		}
		return
	}
}

func todayActionRows(dt *domain.DailyTimeboxes, s *store.Store, date time.Time) [][]tgbotapi.InlineKeyboardButton {
	var rows [][]tgbotapi.InlineKeyboardButton
	if dt == nil || len(dt.Timeboxes) == 0 {
		return rows
	}

	listTasks := make(map[string][]domain.Task)
	for _, tb := range dt.Timeboxes {
		if tb.TaskListSlug == "" {
			continue
		}
		if _, ok := listTasks[tb.TaskListSlug]; ok {
			continue
		}
		tl, err := s.ReadTaskList(tb.TaskListSlug)
		if err != nil {
			listTasks[tb.TaskListSlug] = nil
			continue
		}
		listTasks[tb.TaskListSlug] = tl.ActiveTasks()
	}

	scheduledByIndex := sequenceDailyTimeboxes(dt.Timeboxes, listTasks)
	dateStr := date.Format("2006-01-02")

	for i, tb := range dt.Timeboxes {
		if tb.TaskListSlug == "" || tb.Status != domain.StatusActive || tb.IsReserved() {
			continue
		}

		ok, _, err := canArchiveTimebox(dt, i, s)
		if err == nil && ok {
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(
					"Archive "+tb.Start.Format("15:04"),
					"archive:"+dateStr+":"+tb.Start.Format("15:04"),
				),
			))
		}

		for _, st := range scheduledByIndex[i] {
			if st.IsBreak || isCompletedTask(tb.CompletedTasks, st.Task) {
				continue
			}
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(taskDoneButton(st.Task.Description, dateStr, tb.Start.Format("1504"))))
			break
		}
	}

	return rows
}
