package telegram

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/mhwang-1/meadow-bubbletea/internal/domain"
	"github.com/mhwang-1/meadow-bubbletea/internal/service"
)

// sendMainView sends or edits the main view message for a given date.
func sendMainView(b *Bot, chatID int64, date time.Time, messageID int) error {
	dv, err := b.svc.GetDayView(date)
	if err != nil {
		return fmt.Errorf("getting day view: %w", err)
	}

	text := formatMainView(dv, date)
	kb := mainViewKeyboard(date)
	sendOrEdit(b, chatID, messageID, text, &kb)
	return nil
}

// handleNav updates the main view to a new date.
func handleNav(b *Bot, chatID int64, msgID int, date time.Time) {
	if err := sendMainView(b, chatID, date, msgID); err != nil {
		log.Printf("Error navigating: %v", err)
	}
}

// --- Done wizard ---

// handleDoneStart shows active timeboxes with pending tasks.
func handleDoneStart(b *Bot, chatID int64, msgID int, date time.Time) {
	dv, err := b.svc.GetDayView(date)
	if err != nil {
		log.Printf("Error getting day view: %v", err)
		editMessage(b, chatID, msgID, "Error loading timeboxes.", nil)
		return
	}

	// Filter to active timeboxes with pending tasks.
	var eligible []service.DayViewTimebox
	for _, dvtb := range dv.Timeboxes {
		if dvtb.Timebox.Status != domain.StatusActive || dvtb.Timebox.TaskListSlug == "" || dvtb.Timebox.IsReserved() {
			continue
		}
		if countPendingTasks(dvtb) == 0 {
			continue
		}
		eligible = append(eligible, dvtb)
	}

	if len(eligible) == 0 {
		editMessage(b, chatID, msgID, "No pending tasks to mark done.", nil)
		return
	}

	dateStr := fmtDate(date)

	// Skip timebox selection if only one.
	if len(eligible) == 1 {
		handleDoneSelectTimebox(b, chatID, msgID, date, eligible[0].Index)
		return
	}

	text := fmt.Sprintf("<b>Mark Done -- %s</b>\n\nSelect timebox:", dayLabel(date))
	kb := timeboxSelectKeyboard(eligible, "done", dateStr)
	editMessage(b, chatID, msgID, text, &kb)
}

// handleDoneSelectTimebox shows pending tasks in the selected timebox.
func handleDoneSelectTimebox(b *Bot, chatID int64, msgID int, date time.Time, tbIdx int) {
	dv, err := b.svc.GetDayView(date)
	if err != nil {
		log.Printf("Error getting day view: %v", err)
		return
	}

	if tbIdx < 0 || tbIdx >= len(dv.Timeboxes) {
		editMessage(b, chatID, msgID, "Timebox not found.", nil)
		return
	}

	dvtb := dv.Timeboxes[tbIdx]

	// Filter to only pending (non-completed, non-break) tasks.
	var pending []domain.ScheduledTask
	for _, st := range dvtb.ScheduledTasks {
		if st.IsBreak || isCompletedInTimebox(dvtb.Timebox.CompletedTasks, st.Task) {
			continue
		}
		pending = append(pending, st)
	}

	if len(pending) == 0 {
		editMessage(b, chatID, msgID, "No pending tasks in this timebox.", nil)
		return
	}

	// Skip task selection if only one.
	if len(pending) == 1 {
		handleDoneSelectTask(b, chatID, msgID, date, tbIdx, 0)
		return
	}

	dateStr := fmtDate(date)
	text := fmt.Sprintf("<b>Mark Done -- %s-%s %s</b>\n\nSelect task:",
		dvtb.Timebox.Start.Format("15:04"),
		dvtb.Timebox.End.Format("15:04"),
		dvtb.TaskListName,
	)
	kb := taskSelectKeyboard(pending, dateStr, tbIdx)
	editMessage(b, chatID, msgID, text, &kb)
}

// handleDoneSelectTask marks a task as done and returns to the main view.
func handleDoneSelectTask(b *Bot, chatID int64, msgID int, date time.Time, tbIdx, taskIdx int) {
	err := b.svc.MarkTaskDone(date, tbIdx, taskIdx)
	if err != nil {
		log.Printf("Error marking task done: %v", err)
		editMessage(b, chatID, msgID, "Error: "+err.Error(), nil)
		return
	}

	// Return to main view.
	if err := sendMainView(b, chatID, date, msgID); err != nil {
		log.Printf("Error returning to main view: %v", err)
	}
}

// --- Archive wizard ---

// handleArchiveStart shows archivable timeboxes.
func handleArchiveStart(b *Bot, chatID int64, msgID int, date time.Time) {
	dv, err := b.svc.GetDayView(date)
	if err != nil {
		log.Printf("Error getting day view: %v", err)
		editMessage(b, chatID, msgID, "Error loading timeboxes.", nil)
		return
	}

	// Filter to active, assigned, non-reserved timeboxes.
	var eligible []service.DayViewTimebox
	for _, dvtb := range dv.Timeboxes {
		tb := dvtb.Timebox
		if tb.Status != domain.StatusActive || tb.TaskListSlug == "" || tb.IsReserved() {
			continue
		}
		eligible = append(eligible, dvtb)
	}

	if len(eligible) == 0 {
		editMessage(b, chatID, msgID, "No timeboxes to archive.", nil)
		return
	}

	dateStr := fmtDate(date)

	// Skip selection if only one.
	if len(eligible) == 1 {
		handleArchiveSelect(b, chatID, msgID, date, eligible[0].Index)
		return
	}

	text := fmt.Sprintf("<b>Archive -- %s</b>\n\nSelect timebox:", dayLabel(date))
	kb := timeboxSelectKeyboard(eligible, "arch", dateStr)
	editMessage(b, chatID, msgID, text, &kb)
}

// handleArchiveSelect tries to archive and asks for confirmation if pending.
func handleArchiveSelect(b *Bot, chatID int64, msgID int, date time.Time, tbIdx int) {
	dateStr := fmtDate(date)

	err := b.svc.ArchiveTimebox(date, tbIdx, false)
	if err != nil {
		// Check if PendingTasksError.
		if pe, ok := err.(*service.PendingTasksError); ok {
			text := fmt.Sprintf("%d task(s) still pending. Archive anyway?", pe.Count)
			yesData := fmt.Sprintf("arch:%s:%d:f", dateStr, tbIdx)
			noData := fmt.Sprintf("cancel:%s", dateStr)
			kb := confirmKeyboard(yesData, noData)
			editMessage(b, chatID, msgID, text, &kb)
			return
		}
		log.Printf("Error archiving timebox: %v", err)
		editMessage(b, chatID, msgID, "Error: "+err.Error(), nil)
		return
	}

	// Success, return to main view.
	if err := sendMainView(b, chatID, date, msgID); err != nil {
		log.Printf("Error returning to main view: %v", err)
	}
}

// handleArchiveConfirm archives with force if confirmed.
func handleArchiveConfirm(b *Bot, chatID int64, msgID int, date time.Time, tbIdx int, force bool) {
	if !force {
		// User said no, return to main view.
		if err := sendMainView(b, chatID, date, msgID); err != nil {
			log.Printf("Error returning to main view: %v", err)
		}
		return
	}

	err := b.svc.ArchiveTimebox(date, tbIdx, true)
	if err != nil {
		log.Printf("Error force-archiving timebox: %v", err)
		editMessage(b, chatID, msgID, "Error: "+err.Error(), nil)
		return
	}

	if err := sendMainView(b, chatID, date, msgID); err != nil {
		log.Printf("Error returning to main view: %v", err)
	}
}

// --- Lists wizard ---

// handleListsStart shows task lists with tab buttons.
func handleListsStart(b *Bot, chatID int64, msgID int, tab string) {
	var rows [][]tgbotapi.InlineKeyboardButton

	// Tab buttons.
	rows = append(rows, listsTabKeyboard(tab))

	switch tab {
	case "active":
		lists, err := b.svc.ListTaskLists()
		if err != nil {
			log.Printf("Error listing task lists: %v", err)
			editMessage(b, chatID, msgID, "Error loading lists.", nil)
			return
		}

		text := formatTaskListOverviewTab(lists)

		// Add list buttons.
		for _, tl := range lists {
			label := tl.Name
			if len([]rune(label)) > 40 {
				label = string([]rune(label)[:39]) + "..."
			}
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(label, "list:"+tl.Slug),
			))
		}
		rows = append(rows, cancelRow(""))
		kb := tgbotapi.NewInlineKeyboardMarkup(rows...)
		editMessage(b, chatID, msgID, text, &kb)

	case "history":
		wi := domain.WeekForDate(time.Now())
		handleHistoryBrowse(b, chatID, msgID, wi.Year, wi.Week)

	case "archived":
		wi := domain.WeekForDate(time.Now())
		handleArchivedBrowse(b, chatID, msgID, wi.Year, wi.Week)
	}
}

// handleListSelect shows a specific task list with action buttons.
func handleListSelect(b *Bot, chatID int64, msgID int, slug string) {
	tl, err := b.svc.GetTaskList(slug)
	if err != nil {
		log.Printf("Error reading task list: %v", err)
		editMessage(b, chatID, msgID, fmt.Sprintf("Task list %q not found.", slug), nil)
		return
	}

	text := formatTaskListView(tl)
	kb := listViewKeyboard(slug, "")
	editMessage(b, chatID, msgID, text, &kb)
}

// handleListAssign shows today's assignable timeboxes for a list.
func handleListAssign(b *Bot, chatID int64, msgID int, slug string) {
	date := today()
	dv, err := b.svc.GetDayView(date)
	if err != nil {
		log.Printf("Error getting day view: %v", err)
		editMessage(b, chatID, msgID, "Error loading timeboxes.", nil)
		return
	}

	// Filter to unassigned, non-reserved timeboxes.
	var eligible []service.DayViewTimebox
	for _, dvtb := range dv.Timeboxes {
		tb := dvtb.Timebox
		if tb.Status == domain.StatusUnassigned && tb.TaskListSlug == "" && !tb.IsReserved() {
			eligible = append(eligible, dvtb)
		}
	}

	if len(eligible) == 0 {
		text := "No unassigned timeboxes available today."
		rows := [][]tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("Back to List", "list:"+slug),
			),
		}
		kb := tgbotapi.NewInlineKeyboardMarkup(rows...)
		editMessage(b, chatID, msgID, text, &kb)
		return
	}

	dateStr := fmtDate(date)
	text := fmt.Sprintf("<b>Assign %s to timebox:</b>", slug)

	var rows [][]tgbotapi.InlineKeyboardButton
	for _, dvtb := range eligible {
		tb := dvtb.Timebox
		label := fmt.Sprintf("%s-%s", tb.Start.Format("15:04"), tb.End.Format("15:04"))
		data := fmt.Sprintf("list:%s:at:%s:%d", slug, dateStr, dvtb.Index)
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(label, data),
		))
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("Back to List", "list:"+slug),
	))
	kb := tgbotapi.NewInlineKeyboardMarkup(rows...)
	editMessage(b, chatID, msgID, text, &kb)
}

// handleListAssignTimebox assigns a task list to a timebox.
func handleListAssignTimebox(b *Bot, chatID int64, msgID int, slug string, date time.Time, tbIdx int) {
	err := b.svc.AssignTaskList(date, tbIdx, slug)
	if err != nil {
		log.Printf("Error assigning task list: %v", err)
		editMessage(b, chatID, msgID, "Error: "+err.Error(), nil)
		return
	}

	// Return to main view.
	if err := sendMainView(b, chatID, date, msgID); err != nil {
		log.Printf("Error returning to main view: %v", err)
	}
}

// handleListEdit shows the list in edit mode.
func handleListEdit(b *Bot, chatID int64, msgID int, slug string) {
	tl, err := b.svc.GetTaskList(slug)
	if err != nil {
		log.Printf("Error reading task list: %v", err)
		editMessage(b, chatID, msgID, fmt.Sprintf("Task list %q not found.", slug), nil)
		return
	}

	text := formatTaskListEdit(tl)

	// Build keyboard with remove buttons for each task.
	var rows [][]tgbotapi.InlineKeyboardButton
	active := tl.ActiveTasks()
	for i, t := range active {
		label := fmt.Sprintf("Remove: %s", t.Description)
		if len([]rune(label)) > 40 {
			label = string([]rune(label)[:39]) + "..."
		}
		data := fmt.Sprintf("list:%s:rm:%d", slug, i)
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(label, data),
		))
	}

	// Add task and back buttons.
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("Add Task", "list:"+slug+":add"),
	))
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("Back to List", "list:"+slug),
	))
	kb := tgbotapi.NewInlineKeyboardMarkup(rows...)
	editMessage(b, chatID, msgID, text, &kb)
}

// handleListRemoveTask removes a task from the list.
func handleListRemoveTask(b *Bot, chatID int64, msgID int, slug string, taskIdx int) {
	tl, err := b.svc.GetTaskList(slug)
	if err != nil {
		log.Printf("Error reading task list: %v", err)
		editMessage(b, chatID, msgID, "Error: "+err.Error(), nil)
		return
	}

	active := tl.ActiveTasks()
	if taskIdx < 0 || taskIdx >= len(active) {
		editMessage(b, chatID, msgID, "Task index out of range.", nil)
		return
	}

	targetTask := active[taskIdx]

	// Remove the task from the full task list (including commented).
	var updatedTasks []domain.Task
	removed := false
	for _, t := range tl.Tasks {
		if !removed && !t.Commented && t.Description == targetTask.Description && t.Duration == targetTask.Duration {
			removed = true
			continue
		}
		updatedTasks = append(updatedTasks, t)
	}

	err = b.svc.UpdateTaskList(slug, updatedTasks)
	if err != nil {
		log.Printf("Error updating task list: %v", err)
		editMessage(b, chatID, msgID, "Error: "+err.Error(), nil)
		return
	}

	// Return to edit view.
	handleListEdit(b, chatID, msgID, slug)
}

// handleListAddTask sets wizard state to capture text input for a new task.
func handleListAddTask(b *Bot, chatID int64, msgID int, slug string) {
	b.sessions.set(chatID, &WizardState{
		Action: "add_task",
		Params: map[string]string{
			"slug":  slug,
			"msgID": strconv.Itoa(msgID),
		},
	})

	text := fmt.Sprintf("Send a task to add to <b>%s</b>:\n\nFormat: description ~duration\nExample: Review report ~24m", slug)
	rows := [][]tgbotapi.InlineKeyboardButton{cancelRow("")}
	kb := tgbotapi.NewInlineKeyboardMarkup(rows...)
	editMessage(b, chatID, msgID, text, &kb)
}

// handleAddTaskText processes text input for adding a task.
func handleAddTaskText(b *Bot, chatID int64, ws *WizardState, text string) {
	slug := ws.Params["slug"]
	msgIDStr := ws.Params["msgID"]
	b.sessions.clear(chatID)

	task, err := domain.ParseTaskLine(text)
	if err != nil {
		msg := tgbotapi.NewMessage(chatID, "Invalid task format. Use: description ~duration\nExample: Review report ~24m")
		if _, sendErr := b.api.Send(msg); sendErr != nil {
			log.Printf("Error sending message: %v", sendErr)
		}
		return
	}

	tl, err := b.svc.GetTaskList(slug)
	if err != nil {
		msg := tgbotapi.NewMessage(chatID, "Error: task list not found.")
		if _, sendErr := b.api.Send(msg); sendErr != nil {
			log.Printf("Error sending message: %v", sendErr)
		}
		return
	}

	updatedTasks := make([]domain.Task, len(tl.Tasks)+1)
	copy(updatedTasks, tl.Tasks)
	updatedTasks[len(tl.Tasks)] = task
	err = b.svc.UpdateTaskList(slug, updatedTasks)
	if err != nil {
		msg := tgbotapi.NewMessage(chatID, "Error: "+err.Error())
		if _, sendErr := b.api.Send(msg); sendErr != nil {
			log.Printf("Error sending message: %v", sendErr)
		}
		return
	}

	// Send confirmation as a new message.
	confirmText := fmt.Sprintf("Added to %s:\n%s ~%s", tl.Name, task.Description, domain.FormatDuration(task.Duration))
	msg := tgbotapi.NewMessage(chatID, confirmText)
	if _, sendErr := b.api.Send(msg); sendErr != nil {
		log.Printf("Error sending message: %v", sendErr)
	}

	// Edit the original message back to list edit view.
	msgID, _ := strconv.Atoi(msgIDStr)
	if msgID > 0 {
		handleListEdit(b, chatID, msgID, slug)
	}
}

// handleListArchive archives a task list.
func handleListArchive(b *Bot, chatID int64, msgID int, slug string) {
	err := b.svc.ArchiveTaskList(slug)
	if err != nil {
		log.Printf("Error archiving task list: %v", err)
		editMessage(b, chatID, msgID, "Error: "+err.Error(), nil)
		return
	}

	text := fmt.Sprintf("Task list %q archived.", slug)
	rows := [][]tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Back to Lists", "lists"),
		),
	}
	kb := tgbotapi.NewInlineKeyboardMarkup(rows...)
	editMessage(b, chatID, msgID, text, &kb)
}

// handleListDelete deletes a task list.
func handleListDelete(b *Bot, chatID int64, msgID int, slug string) {
	err := b.svc.DeleteTaskList(slug)
	if err != nil {
		log.Printf("Error deleting task list: %v", err)
		editMessage(b, chatID, msgID, "Error: "+err.Error(), nil)
		return
	}

	text := fmt.Sprintf("Task list %q deleted.", slug)
	rows := [][]tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Back to Lists", "lists"),
		),
	}
	kb := tgbotapi.NewInlineKeyboardMarkup(rows...)
	editMessage(b, chatID, msgID, text, &kb)
}

// --- New Timebox wizard ---

// handleNewTimeboxStart shows day selection.
func handleNewTimeboxStart(b *Bot, chatID int64, msgID int) {
	text := "<b>New Timebox</b>\n\nSelect day:"
	kb := daySelectKeyboard("tb:new")
	editMessage(b, chatID, msgID, text, &kb)
}

// handleNewTimeboxDay shows start time selection.
func handleNewTimeboxDay(b *Bot, chatID int64, msgID int, dateStr string) {
	if dateStr == "custom" {
		// Shouldn't happen for day selection, but handle gracefully.
		handleNewTimeboxStart(b, chatID, msgID)
		return
	}

	text := fmt.Sprintf("<b>New Timebox -- %s</b>\n\nSelect start time:", dateStr)
	kb := timeSelectKeyboard(dateStr, "tb:new", 8)
	editMessage(b, chatID, msgID, text, &kb)
}

// handleNewTimeboxStartTime shows end time selection.
func handleNewTimeboxStartTime(b *Bot, chatID int64, msgID int, dateStr, startTime string) {
	if startTime == "custom" {
		// Set wizard state for custom time input.
		b.sessions.set(chatID, &WizardState{
			Action: "custom_time",
			Params: map[string]string{
				"phase":   "start",
				"date":    dateStr,
				"msgID":   strconv.Itoa(msgID),
			},
		})
		text := "Enter start time (HH:MM):"
		rows := [][]tgbotapi.InlineKeyboardButton{cancelRow("")}
		kb := tgbotapi.NewInlineKeyboardMarkup(rows...)
		editMessage(b, chatID, msgID, text, &kb)
		return
	}

	sh, sm := parseHHMM(startTime)
	text := fmt.Sprintf("<b>New Timebox -- %s %02d:%02d</b>\n\nSelect end time:", dateStr, sh, sm)
	kb := endTimeSelectKeyboard(dateStr, startTime, "tb:new")
	editMessage(b, chatID, msgID, text, &kb)
}

// handleNewTimeboxEnd creates the timebox and shows post-creation options.
func handleNewTimeboxEnd(b *Bot, chatID int64, msgID int, dateStr, startTime, endTime string) {
	if endTime == "custom" {
		// Set wizard state for custom end time input.
		b.sessions.set(chatID, &WizardState{
			Action: "custom_time",
			Params: map[string]string{
				"phase": "end",
				"date":  dateStr,
				"start": startTime,
				"msgID": strconv.Itoa(msgID),
			},
		})
		text := "Enter end time (HH:MM):"
		rows := [][]tgbotapi.InlineKeyboardButton{cancelRow("")}
		kb := tgbotapi.NewInlineKeyboardMarkup(rows...)
		editMessage(b, chatID, msgID, text, &kb)
		return
	}

	date := parseDateOrToday(dateStr)
	sh, sm := parseHHMM(startTime)
	eh, em := parseHHMM(endTime)

	y, mo, d := date.Date()
	loc := date.Location()
	start := time.Date(y, mo, d, sh, sm, 0, 0, loc)
	end := time.Date(y, mo, d, eh, em, 0, 0, loc)

	err := b.svc.CreateTimebox(date, start, end)
	if err != nil {
		log.Printf("Error creating timebox: %v", err)
		editMessage(b, chatID, msgID, "Error: "+err.Error(), nil)
		return
	}

	// Show post-creation options.
	text := fmt.Sprintf("Timebox created: %s %02d:%02d-%02d:%02d", dateStr, sh, sm, eh, em)

	// We need to find the index of the newly created timebox.
	dv, err := b.svc.GetDayView(date)
	tbIdxStr := ""
	if err == nil {
		for _, dvtb := range dv.Timeboxes {
			if dvtb.Timebox.Start.Hour() == sh && dvtb.Timebox.Start.Minute() == sm &&
				dvtb.Timebox.End.Hour() == eh && dvtb.Timebox.End.Minute() == em &&
				dvtb.Timebox.Status == domain.StatusUnassigned {
				tbIdxStr = strconv.Itoa(dvtb.Index)
				break
			}
		}
	}

	var rows [][]tgbotapi.InlineKeyboardButton
	if tbIdxStr != "" {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Assign List", fmt.Sprintf("tb:post:assign:%s:%s", dateStr, tbIdxStr)),
			tgbotapi.NewInlineKeyboardButtonData("Mark Reserved", fmt.Sprintf("tb:post:reserve:%s:%s", dateStr, tbIdxStr)),
		))
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("Leave Unassigned", "main:"+dateStr),
	))
	kb := tgbotapi.NewInlineKeyboardMarkup(rows...)
	editMessage(b, chatID, msgID, text, &kb)
}

// handleTimeboxPostAction handles post-creation timebox actions.
func handleTimeboxPostAction(b *Bot, chatID int64, msgID int, parts []string) {
	if len(parts) < 1 {
		return
	}

	action := parts[0]
	switch action {
	case "assign":
		// tb:post:assign:{date}:{tbIdx}
		if len(parts) < 3 {
			return
		}
		dateStr := parts[1]
		tbIdx, _ := strconv.Atoi(parts[2])
		handlePostAssign(b, chatID, msgID, dateStr, tbIdx)
	case "reserve":
		// tb:post:reserve:{date}:{tbIdx}
		if len(parts) < 3 {
			return
		}
		dateStr := parts[1]
		date := parseDateOrToday(dateStr)
		tbIdx, _ := strconv.Atoi(parts[2])
		handlePostReserve(b, chatID, msgID, date, tbIdx)
	}
}

// handlePostAssign shows task lists to assign to the new timebox.
func handlePostAssign(b *Bot, chatID int64, msgID int, dateStr string, tbIdx int) {
	lists, err := b.svc.ListTaskLists()
	if err != nil {
		log.Printf("Error listing task lists: %v", err)
		editMessage(b, chatID, msgID, "Error loading lists.", nil)
		return
	}

	if len(lists) == 0 {
		editMessage(b, chatID, msgID, "No task lists available.", nil)
		return
	}

	text := "<b>Assign task list to timebox:</b>"
	var rows [][]tgbotapi.InlineKeyboardButton
	for _, tl := range lists {
		label := tl.Name
		if len([]rune(label)) > 40 {
			label = string([]rune(label)[:39]) + "..."
		}
		// Reuse the list assign-to-timebox callback.
		data := fmt.Sprintf("list:%s:at:%s:%d", tl.Slug, dateStr, tbIdx)
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(label, data),
		))
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("Leave Unassigned", "main:"+dateStr),
	))
	kb := tgbotapi.NewInlineKeyboardMarkup(rows...)
	editMessage(b, chatID, msgID, text, &kb)
}

// handlePostReserve marks the timebox as reserved.
func handlePostReserve(b *Bot, chatID int64, msgID int, date time.Time, tbIdx int) {
	// Set wizard state to capture optional note.
	b.sessions.set(chatID, &WizardState{
		Action: "reserve_note",
		Params: map[string]string{
			"date":  fmtDate(date),
			"tbIdx": strconv.Itoa(tbIdx),
			"msgID": strconv.Itoa(msgID),
		},
	})

	text := "Enter a note for the reserved timebox (or send a dot '.' to skip):"
	rows := [][]tgbotapi.InlineKeyboardButton{cancelRow(fmtDate(date))}
	kb := tgbotapi.NewInlineKeyboardMarkup(rows...)
	editMessage(b, chatID, msgID, text, &kb)
}

// handleReserveNoteText processes text input for reserve note.
func handleReserveNoteText(b *Bot, chatID int64, ws *WizardState, text string) {
	dateStr := ws.Params["date"]
	tbIdx, _ := strconv.Atoi(ws.Params["tbIdx"])
	msgIDStr := ws.Params["msgID"]
	msgID, _ := strconv.Atoi(msgIDStr)
	b.sessions.clear(chatID)

	date := parseDateOrToday(dateStr)
	note := strings.TrimSpace(text)
	if note == "." {
		note = ""
	}

	err := b.svc.SetReserved(date, tbIdx, note)
	if err != nil {
		log.Printf("Error setting reserved: %v", err)
		msg := tgbotapi.NewMessage(chatID, "Error: "+err.Error())
		if _, sendErr := b.api.Send(msg); sendErr != nil {
			log.Printf("Error sending message: %v", sendErr)
		}
		return
	}

	if err := sendMainView(b, chatID, date, msgID); err != nil {
		log.Printf("Error returning to main view: %v", err)
	}
}

// handleCustomTimeText processes text input for custom time entry.
func handleCustomTimeText(b *Bot, chatID int64, ws *WizardState, text string) {
	phase := ws.Params["phase"]
	dateStr := ws.Params["date"]
	msgIDStr := ws.Params["msgID"]
	msgID, _ := strconv.Atoi(msgIDStr)

	// Parse HH:MM input.
	text = strings.TrimSpace(text)
	t, err := time.Parse("15:04", text)
	if err != nil {
		msg := tgbotapi.NewMessage(chatID, "Invalid time format. Use HH:MM (e.g. 09:30)")
		if _, sendErr := b.api.Send(msg); sendErr != nil {
			log.Printf("Error sending message: %v", sendErr)
		}
		return
	}

	b.sessions.clear(chatID)
	timeStr := fmt.Sprintf("%02d%02d", t.Hour(), t.Minute())

	switch phase {
	case "start":
		// Show end time selection.
		handleNewTimeboxStartTime(b, chatID, msgID, dateStr, timeStr)
	case "end":
		startTime := ws.Params["start"]
		handleNewTimeboxEnd(b, chatID, msgID, dateStr, startTime, timeStr)
	}
}

// --- Notes wizard ---

// handleNotesStart shows notes with edit/back buttons.
func handleNotesStart(b *Bot, chatID int64, msgID int) {
	content, err := b.svc.ReadNotes()
	if err != nil {
		log.Printf("Error reading notes: %v", err)
		editMessage(b, chatID, msgID, "Error loading notes.", nil)
		return
	}

	text := formatNotesView(content)
	rows := [][]tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Edit", "notes:edit"),
		),
		cancelRow(""),
	}
	kb := tgbotapi.NewInlineKeyboardMarkup(rows...)
	editMessage(b, chatID, msgID, text, &kb)
}

// handleNotesEdit sets wizard state to capture note text.
func handleNotesEdit(b *Bot, chatID int64, msgID int) {
	b.sessions.set(chatID, &WizardState{
		Action: "edit_notes",
		Params: map[string]string{
			"msgID": strconv.Itoa(msgID),
		},
	})

	text := "Send the new notes content (replaces existing):"
	rows := [][]tgbotapi.InlineKeyboardButton{cancelRow("")}
	kb := tgbotapi.NewInlineKeyboardMarkup(rows...)
	editMessage(b, chatID, msgID, text, &kb)
}

// handleEditNotesText processes text input for notes editing.
func handleEditNotesText(b *Bot, chatID int64, ws *WizardState, text string) {
	msgIDStr := ws.Params["msgID"]
	msgID, _ := strconv.Atoi(msgIDStr)
	b.sessions.clear(chatID)

	err := b.svc.WriteNotes(text)
	if err != nil {
		log.Printf("Error writing notes: %v", err)
		msg := tgbotapi.NewMessage(chatID, "Error: "+err.Error())
		if _, sendErr := b.api.Send(msg); sendErr != nil {
			log.Printf("Error sending message: %v", sendErr)
		}
		return
	}

	// Show updated notes.
	if msgID > 0 {
		handleNotesStart(b, chatID, msgID)
	}
}

// --- Undo / Unarchive ---

// handleUndoDone unmarks a completed task.
func handleUndoDone(b *Bot, chatID int64, msgID int, date time.Time, tbIdx, taskIdx int) {
	err := b.svc.UnmarkTaskDone(date, tbIdx, taskIdx)
	if err != nil {
		log.Printf("Error unmarking task done: %v", err)
		editMessage(b, chatID, msgID, "Error: "+err.Error(), nil)
		return
	}

	if err := sendMainView(b, chatID, date, msgID); err != nil {
		log.Printf("Error returning to main view: %v", err)
	}
}

// handleUnarchive unarchives a timebox.
func handleUnarchive(b *Bot, chatID int64, msgID int, date time.Time, tbIdx int) {
	err := b.svc.UnarchiveTimebox(date, tbIdx)
	if err != nil {
		log.Printf("Error unarchiving timebox: %v", err)
		editMessage(b, chatID, msgID, "Error: "+err.Error(), nil)
		return
	}

	if err := sendMainView(b, chatID, date, msgID); err != nil {
		log.Printf("Error returning to main view: %v", err)
	}
}

// --- History browsing ---

// handleHistoryBrowse shows completed tasks for a year/week with navigation.
func handleHistoryBrowse(b *Bot, chatID int64, msgID int, year, week int) {
	tasks, err := b.svc.GetHistory(year, week)
	if err != nil {
		log.Printf("Error reading history: %v", err)
		tasks = nil
	}

	text := formatHistoryView(tasks, year, week)

	// Navigation: prev/next week.
	prevWeek := week - 1
	prevYear := year
	if prevWeek < 1 {
		prevYear--
		prevWeek = 53
	}
	nextWeek := week + 1
	nextYear := year
	if nextWeek > 53 {
		nextYear++
		nextWeek = 1
	}

	rows := [][]tgbotapi.InlineKeyboardButton{
		listsTabKeyboard("history"),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("<< W%d", prevWeek), fmt.Sprintf("hist:%d:%d", prevYear, prevWeek)),
			tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("W%d >>", nextWeek), fmt.Sprintf("hist:%d:%d", nextYear, nextWeek)),
		),
		cancelRow(""),
	}
	kb := tgbotapi.NewInlineKeyboardMarkup(rows...)
	editMessage(b, chatID, msgID, text, &kb)
}

// handleArchivedBrowse shows archived task lists for a year/week.
func handleArchivedBrowse(b *Bot, chatID int64, msgID int, year, week int) {
	allArchived, err := b.svc.GetArchivedTaskLists()
	if err != nil {
		log.Printf("Error reading archived lists: %v", err)
		allArchived = nil
	}

	var lists []*domain.TaskList
	if yearMap, ok := allArchived[year]; ok {
		if weekLists, ok := yearMap[week]; ok {
			lists = weekLists
		}
	}

	text := formatArchivedListsView(lists, year, week)

	// Navigation: prev/next week.
	prevWeek := week - 1
	prevYear := year
	if prevWeek < 1 {
		prevYear--
		prevWeek = 53
	}
	nextWeek := week + 1
	nextYear := year
	if nextWeek > 53 {
		nextYear++
		nextWeek = 1
	}

	rows := [][]tgbotapi.InlineKeyboardButton{
		listsTabKeyboard("archived"),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("<< W%d", prevWeek), fmt.Sprintf("arcd:%d:%d", prevYear, prevWeek)),
			tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("W%d >>", nextWeek), fmt.Sprintf("arcd:%d:%d", nextYear, nextWeek)),
		),
		cancelRow(""),
	}
	kb := tgbotapi.NewInlineKeyboardMarkup(rows...)
	editMessage(b, chatID, msgID, text, &kb)
}

// --- Helpers ---

// countPendingTasks counts non-completed, non-break tasks in a DayViewTimebox.
func countPendingTasks(dvtb service.DayViewTimebox) int {
	count := 0
	for _, st := range dvtb.ScheduledTasks {
		if st.IsBreak || isCompletedInTimebox(dvtb.Timebox.CompletedTasks, st.Task) {
			continue
		}
		count++
	}
	return count
}
