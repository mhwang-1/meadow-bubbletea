package telegram

import (
	"fmt"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/hwang/meadow-bubbletea/internal/domain"
	"github.com/hwang/meadow-bubbletea/internal/store"
)

// handleToday shows today's timeboxes (or a date offset via prev/next).
func (b *Bot) handleToday(chatID int64, args string) (tgbotapi.MessageConfig, error) {
	date := time.Now()
	if args != "" {
		parsed, err := time.Parse("2006-01-02", args)
		if err == nil {
			date = parsed
		}
	}

	dt, err := b.store.ReadDailyTimeboxes(date)
	if err != nil {
		return tgbotapi.MessageConfig{}, fmt.Errorf("reading timeboxes: %w", err)
	}
	dt.SortByStart()

	text := formatTodayView(dt, b.store, date)
	msg := tgbotapi.NewMessage(chatID, text)
	keyboard := todayKeyboard(date, dt, b.store)
	msg.ReplyMarkup = keyboard
	return msg, nil
}

// handleWeek shows a summary of the current week.
func (b *Bot) handleWeek(chatID int64) (tgbotapi.MessageConfig, error) {
	wi := domain.WeekForDate(time.Now())
	text := formatWeekView(wi, b.store)
	msg := tgbotapi.NewMessage(chatID, text)
	return msg, nil
}

// handleLists shows all task lists with stats.
func (b *Bot) handleLists(chatID int64) (tgbotapi.MessageConfig, error) {
	lists, err := b.store.ListTaskLists()
	if err != nil {
		return tgbotapi.MessageConfig{}, fmt.Errorf("listing task lists: %w", err)
	}

	text := formatTaskListOverview(lists)
	msg := tgbotapi.NewMessage(chatID, text)
	return msg, nil
}

// handleList shows tasks in a specific list (fuzzy match on name).
func (b *Bot) handleList(chatID int64, args string) (tgbotapi.MessageConfig, error) {
	if args == "" {
		return tgbotapi.NewMessage(chatID, "Usage: /list {name}"), nil
	}

	lists, err := b.store.ListTaskLists()
	if err != nil {
		return tgbotapi.MessageConfig{}, fmt.Errorf("listing task lists: %w", err)
	}

	tl := fuzzyMatchList(lists, args)
	if tl == nil {
		return tgbotapi.NewMessage(chatID, fmt.Sprintf("No task list matching %q", args)), nil
	}

	text := formatTaskListDetail(tl)
	msg := tgbotapi.NewMessage(chatID, text)
	return msg, nil
}

// handleDone marks a task as done by fuzzy-matching its description.
func (b *Bot) handleDone(chatID int64, args string) (tgbotapi.MessageConfig, error) {
	if args == "" {
		return tgbotapi.NewMessage(chatID, "Usage: /done {task description}"), nil
	}

	lists, err := b.store.ListTaskLists()
	if err != nil {
		return tgbotapi.MessageConfig{}, fmt.Errorf("listing task lists: %w", err)
	}

	// Search across all task lists for a matching active task.
	var matchedTask *domain.Task
	var matchedList *domain.TaskList
	var matchedIdx int
	query := strings.ToLower(args)

	for _, tl := range lists {
		for i, t := range tl.Tasks {
			if t.Commented {
				continue
			}
			if strings.Contains(strings.ToLower(t.Description), query) {
				matchedTask = &tl.Tasks[i]
				matchedList = tl
				matchedIdx = i
				break
			}
		}
		if matchedTask != nil {
			break
		}
	}

	if matchedTask == nil {
		return tgbotapi.NewMessage(chatID, fmt.Sprintf("No active task matching %q", args)), nil
	}

	now := time.Now()
	today := domain.StripTimeForDate(now)

	// Find the timebox this task might be in and add to completed.
	dt, err := b.store.ReadDailyTimeboxes(today)
	if err != nil {
		return tgbotapi.MessageConfig{}, fmt.Errorf("reading daily timeboxes: %w", err)
	}

	tbUpdated := false
	for i := range dt.Timeboxes {
		tb := &dt.Timeboxes[i]
		if tb.TaskListSlug == matchedList.Slug && tb.Status == domain.StatusActive {
			tb.CompletedTasks = append(tb.CompletedTasks, *matchedTask)
			tbUpdated = true
			break
		}
	}

	if tbUpdated {
		if err := b.store.WriteDailyTimeboxes(dt); err != nil {
			return tgbotapi.MessageConfig{}, fmt.Errorf("writing daily timeboxes: %w", err)
		}
	}

	// Remove from task list.
	matchedList.Tasks = append(matchedList.Tasks[:matchedIdx], matchedList.Tasks[matchedIdx+1:]...)
	if err := b.store.WriteTaskList(matchedList); err != nil {
		return tgbotapi.MessageConfig{}, fmt.Errorf("writing task list: %w", err)
	}

	// Add to archive.
	wi := domain.WeekForDate(today)
	ct := store.CompletedTask{
		Task:          *matchedTask,
		CompletedDate: today,
		TaskListSlug:  matchedList.Slug,
	}
	if err := b.store.AddCompleted(wi.Year, wi.Week, ct); err != nil {
		return tgbotapi.MessageConfig{}, fmt.Errorf("archiving completed task: %w", err)
	}

	text := fmt.Sprintf("Done: %s ~%s\nFrom: %s",
		matchedTask.Description,
		domain.FormatDuration(matchedTask.Duration),
		matchedList.Name,
	)
	return tgbotapi.NewMessage(chatID, text), nil
}

// handleDoneForTimebox marks the next pending scheduled task as done for a
// specific date/timebox start.
// Format args: {YYYY-MM-DD} {HHMM}
func (b *Bot) handleDoneForTimebox(chatID int64, args string) (tgbotapi.MessageConfig, error) {
	parts := strings.SplitN(strings.TrimSpace(args), " ", 2)
	if len(parts) < 2 {
		return tgbotapi.NewMessage(chatID, "Invalid done action context."), nil
	}

	date, err := time.Parse("2006-01-02", strings.TrimSpace(parts[0]))
	if err != nil {
		return tgbotapi.NewMessage(chatID, "Invalid done action date."), nil
	}

	startToken := strings.TrimSpace(parts[1])
	if len(startToken) != 4 {
		return tgbotapi.NewMessage(chatID, "Invalid done action time."), nil
	}
	startTime, err := time.Parse("1504", startToken)
	if err != nil {
		return tgbotapi.NewMessage(chatID, "Invalid done action time."), nil
	}
	targetStart := fmt.Sprintf("%02d:%02d", startTime.Hour(), startTime.Minute())

	dt, err := b.store.ReadDailyTimeboxes(date)
	if err != nil {
		return tgbotapi.MessageConfig{}, fmt.Errorf("reading daily timeboxes: %w", err)
	}
	dt.SortByStart()

	matchIdx := -1
	for i := range dt.Timeboxes {
		if dt.Timeboxes[i].Start.Format("15:04") == targetStart {
			matchIdx = i
			break
		}
	}
	if matchIdx < 0 {
		return tgbotapi.NewMessage(chatID, "That timebox is no longer available."), nil
	}

	tb := &dt.Timeboxes[matchIdx]
	if tb.TaskListSlug == "" || tb.Status != domain.StatusActive || tb.IsReserved() {
		return tgbotapi.NewMessage(chatID, "That timebox cannot be marked done."), nil
	}

	listTasks := make(map[string][]domain.Task)
	for _, box := range dt.Timeboxes {
		if box.TaskListSlug == "" {
			continue
		}
		if _, ok := listTasks[box.TaskListSlug]; ok {
			continue
		}
		tl, readErr := b.store.ReadTaskList(box.TaskListSlug)
		if readErr != nil {
			listTasks[box.TaskListSlug] = nil
			continue
		}
		listTasks[box.TaskListSlug] = tl.ActiveTasks()
	}

	scheduledByIndex := sequenceDailyTimeboxes(dt.Timeboxes, listTasks)
	var selectedTask *domain.Task
	for _, st := range scheduledByIndex[matchIdx] {
		if st.IsBreak || isCompletedTask(tb.CompletedTasks, st.Task) {
			continue
		}
		task := st.Task
		selectedTask = &task
		break
	}

	if selectedTask == nil {
		return tgbotapi.NewMessage(chatID, "No pending task in that timebox."), nil
	}

	tb.CompletedTasks = append(tb.CompletedTasks, *selectedTask)
	if err := b.store.WriteDailyTimeboxes(dt); err != nil {
		return tgbotapi.MessageConfig{}, fmt.Errorf("writing daily timeboxes: %w", err)
	}

	tl, err := b.store.ReadTaskList(tb.TaskListSlug)
	if err != nil {
		return tgbotapi.MessageConfig{}, fmt.Errorf("reading task list: %w", err)
	}

	updatedTasks := make([]domain.Task, 0, len(tl.Tasks))
	removed := false
	for _, t := range tl.Tasks {
		if !removed && !t.Commented && t.Description == selectedTask.Description && t.Duration == selectedTask.Duration {
			removed = true
			continue
		}
		updatedTasks = append(updatedTasks, t)
	}
	tl.Tasks = updatedTasks
	if err := b.store.WriteTaskList(tl); err != nil {
		return tgbotapi.MessageConfig{}, fmt.Errorf("writing task list: %w", err)
	}

	wi := domain.WeekForDate(date)
	ct := store.CompletedTask{
		Task:          *selectedTask,
		CompletedDate: domain.StripTimeForDate(date),
		TaskListSlug:  tb.TaskListSlug,
	}
	if err := b.store.AddCompleted(wi.Year, wi.Week, ct); err != nil {
		return tgbotapi.MessageConfig{}, fmt.Errorf("archiving completed task: %w", err)
	}

	text := fmt.Sprintf("Done: %s ~%s\nFrom: %s (%s-%s)",
		selectedTask.Description,
		domain.FormatDuration(selectedTask.Duration),
		tl.Name,
		tb.Start.Format("15:04"),
		tb.End.Format("15:04"),
	)
	return tgbotapi.NewMessage(chatID, text), nil
}

// handleAdd adds a task to a task list.
// Format: /add {list} {description} ~{time}
func (b *Bot) handleAdd(chatID int64, args string) (tgbotapi.MessageConfig, error) {
	if args == "" {
		return tgbotapi.NewMessage(chatID, "Usage: /add {list} {description} ~{time}"), nil
	}

	parts := strings.SplitN(args, " ", 2)
	if len(parts) < 2 {
		return tgbotapi.NewMessage(chatID, "Usage: /add {list} {description} ~{time}"), nil
	}

	listName := parts[0]
	taskLine := parts[1]

	// Parse the task line to validate it.
	task, err := domain.ParseTaskLine(taskLine)
	if err != nil {
		return tgbotapi.MessageConfig{}, fmt.Errorf("parsing task: %w", err)
	}

	// Fuzzy match the list.
	lists, err := b.store.ListTaskLists()
	if err != nil {
		return tgbotapi.MessageConfig{}, fmt.Errorf("listing task lists: %w", err)
	}

	tl := fuzzyMatchList(lists, listName)
	if tl == nil {
		return tgbotapi.NewMessage(chatID, fmt.Sprintf("No task list matching %q", listName)), nil
	}

	tl.Tasks = append(tl.Tasks, task)
	if err := b.store.WriteTaskList(tl); err != nil {
		return tgbotapi.MessageConfig{}, fmt.Errorf("writing task list: %w", err)
	}

	text := fmt.Sprintf("Added to %s:\n%s ~%s",
		tl.Name,
		task.Description,
		domain.FormatDuration(task.Duration),
	)
	return tgbotapi.NewMessage(chatID, text), nil
}

// handlePlan creates a new timebox.
// Format: /plan {day} {start}-{end}
func (b *Bot) handlePlan(chatID int64, args string) (tgbotapi.MessageConfig, error) {
	if args == "" {
		return tgbotapi.NewMessage(chatID, "Usage: /plan {day} {start}-{end}"), nil
	}

	parts := strings.SplitN(args, " ", 2)
	if len(parts) < 2 {
		return tgbotapi.NewMessage(chatID, "Usage: /plan {day} {start}-{end}"), nil
	}

	date, err := resolveDay(parts[0])
	if err != nil {
		return tgbotapi.MessageConfig{}, fmt.Errorf("resolving day: %w", err)
	}

	timeRange := strings.TrimSpace(parts[1])
	timeParts := strings.SplitN(timeRange, "-", 2)
	if len(timeParts) != 2 {
		return tgbotapi.NewMessage(chatID, "Invalid time range. Use HH:MM-HH:MM"), nil
	}

	startTime, err := time.Parse("15:04", strings.TrimSpace(timeParts[0]))
	if err != nil {
		return tgbotapi.MessageConfig{}, fmt.Errorf("parsing start time: %w", err)
	}
	endTime, err := time.Parse("15:04", strings.TrimSpace(timeParts[1]))
	if err != nil {
		return tgbotapi.MessageConfig{}, fmt.Errorf("parsing end time: %w", err)
	}

	y, m, d := date.Date()
	loc := date.Location()
	start := time.Date(y, m, d, startTime.Hour(), startTime.Minute(), 0, 0, loc)
	end := time.Date(y, m, d, endTime.Hour(), endTime.Minute(), 0, 0, loc)

	if err := validateTimeboxRange(start, end); err != nil {
		return tgbotapi.NewMessage(chatID, "Invalid timebox: "+err.Error()), nil
	}

	dt, err := b.store.ReadDailyTimeboxes(date)
	if err != nil {
		return tgbotapi.MessageConfig{}, fmt.Errorf("reading daily timeboxes: %w", err)
	}

	// If this is a brand new file, set the date.
	if dt.Date.IsZero() {
		dt.Date = date
	}

	if err := checkTimeboxOverlap(start, end, dt.Timeboxes); err != nil {
		return tgbotapi.NewMessage(chatID, "Invalid timebox: "+err.Error()), nil
	}

	tb := domain.Timebox{
		Start:  start,
		End:    end,
		Status: domain.StatusUnassigned,
	}
	dt.Timeboxes = append(dt.Timeboxes, tb)
	dt.SortByStart()

	if err := b.store.WriteDailyTimeboxes(dt); err != nil {
		return tgbotapi.MessageConfig{}, fmt.Errorf("writing daily timeboxes: %w", err)
	}

	text := fmt.Sprintf("Planned: %s %s-%s (unassigned)",
		domain.FormatDate(date),
		start.Format("15:04"),
		end.Format("15:04"),
	)
	return tgbotapi.NewMessage(chatID, text), nil
}

// handleAssign assigns a task list to a timebox.
// Format: /assign {day} {time} {list}
func (b *Bot) handleAssign(chatID int64, args string) (tgbotapi.MessageConfig, error) {
	if args == "" {
		return tgbotapi.NewMessage(chatID, "Usage: /assign {day} {time} {list}"), nil
	}

	parts := strings.SplitN(args, " ", 3)
	if len(parts) < 3 {
		return tgbotapi.NewMessage(chatID, "Usage: /assign {day} {time} {list}"), nil
	}

	date, err := resolveDay(parts[0])
	if err != nil {
		return tgbotapi.MessageConfig{}, fmt.Errorf("resolving day: %w", err)
	}

	targetTime := strings.TrimSpace(parts[1])
	listName := strings.TrimSpace(parts[2])

	dt, err := b.store.ReadDailyTimeboxes(date)
	if err != nil {
		return tgbotapi.MessageConfig{}, fmt.Errorf("reading daily timeboxes: %w", err)
	}

	// Find the timebox starting at targetTime.
	var matched *domain.Timebox
	for i := range dt.Timeboxes {
		if dt.Timeboxes[i].Start.Format("15:04") == targetTime {
			matched = &dt.Timeboxes[i]
			break
		}
	}
	if matched == nil {
		return tgbotapi.NewMessage(chatID, fmt.Sprintf("No timebox starting at %s on %s",
			targetTime, domain.FormatDate(date))), nil
	}

	if matched.IsReserved() {
		return tgbotapi.NewMessage(chatID, "That timebox is reserved and cannot be assigned a task list."), nil
	}

	// Fuzzy match the list.
	lists, err := b.store.ListTaskLists()
	if err != nil {
		return tgbotapi.MessageConfig{}, fmt.Errorf("listing task lists: %w", err)
	}

	tl := fuzzyMatchList(lists, listName)
	if tl == nil {
		return tgbotapi.NewMessage(chatID, fmt.Sprintf("No task list matching %q", listName)), nil
	}

	matched.TaskListSlug = tl.Slug
	matched.Status = domain.StatusActive

	if err := b.store.WriteDailyTimeboxes(dt); err != nil {
		return tgbotapi.MessageConfig{}, fmt.Errorf("writing daily timeboxes: %w", err)
	}

	text := fmt.Sprintf("Assigned: %s-%s on %s\nList: %s",
		matched.Start.Format("15:04"),
		matched.End.Format("15:04"),
		domain.FormatDate(date),
		tl.Name,
	)
	return tgbotapi.NewMessage(chatID, text), nil
}

// handleArchive archives a timebox.
// Format: /archive {day} {time}
func (b *Bot) handleArchive(chatID int64, args string) (tgbotapi.MessageConfig, error) {
	if args == "" {
		return tgbotapi.NewMessage(chatID, "Usage: /archive {day} {time}"), nil
	}

	parts := strings.SplitN(args, " ", 2)
	if len(parts) < 2 {
		return tgbotapi.NewMessage(chatID, "Usage: /archive {day} {time}"), nil
	}

	date, err := resolveDay(parts[0])
	if err != nil {
		return tgbotapi.MessageConfig{}, fmt.Errorf("resolving day: %w", err)
	}

	targetTime := strings.TrimSpace(parts[1])

	dt, err := b.store.ReadDailyTimeboxes(date)
	if err != nil {
		return tgbotapi.MessageConfig{}, fmt.Errorf("reading daily timeboxes: %w", err)
	}

	// Find the timebox starting at targetTime.
	matchIdx := -1
	for i := range dt.Timeboxes {
		if dt.Timeboxes[i].Start.Format("15:04") == targetTime {
			matchIdx = i
			break
		}
	}
	if matchIdx < 0 {
		return tgbotapi.NewMessage(chatID, fmt.Sprintf("No timebox starting at %s on %s",
			targetTime, domain.FormatDate(date))), nil
	}

	tb := dt.Timeboxes[matchIdx]
	if tb.Status == domain.StatusArchived {
		return tgbotapi.NewMessage(chatID, "That timebox is already archived."), nil
	}

	ok, reason, err := canArchiveTimebox(dt, matchIdx, b.store)
	if err != nil {
		return tgbotapi.MessageConfig{}, fmt.Errorf("validating archive eligibility: %w", err)
	}
	if !ok {
		return tgbotapi.NewMessage(chatID, "Cannot archive: "+reason), nil
	}

	// Write to the archive.
	wi := domain.WeekForDate(date)
	existing, err := b.store.ReadArchivedTimeboxes(wi.Year, wi.Week)
	if err != nil {
		return tgbotapi.MessageConfig{}, fmt.Errorf("reading archived timeboxes: %w", err)
	}

	archived := store.ArchivedTimebox{
		Date:           date,
		Start:          tb.Start,
		End:            tb.End,
		TaskListSlug:   tb.TaskListSlug,
		Tag:            tb.Tag,
		Note:           tb.Note,
		CompletedTasks: tb.CompletedTasks,
	}
	existing = append(existing, archived)
	if err := b.store.WriteArchivedTimeboxes(wi.Year, wi.Week, existing); err != nil {
		return tgbotapi.MessageConfig{}, fmt.Errorf("writing archived timeboxes: %w", err)
	}

	for _, ct := range tb.CompletedTasks {
		completedTask := store.CompletedTask{
			Task:          ct,
			CompletedDate: domain.StripTimeForDate(date),
			TaskListSlug:  tb.TaskListSlug,
		}
		if err := b.store.AddCompleted(wi.Year, wi.Week, completedTask); err != nil {
			return tgbotapi.MessageConfig{}, fmt.Errorf("archiving completed tasks: %w", err)
		}
	}

	// Keep the timebox in the daily file and mark it archived.
	dt.Timeboxes[matchIdx].Status = domain.StatusArchived
	if err := b.store.WriteDailyTimeboxes(dt); err != nil {
		return tgbotapi.MessageConfig{}, fmt.Errorf("writing daily timeboxes: %w", err)
	}

	text := fmt.Sprintf("Archived: %s-%s on %s",
		tb.Start.Format("15:04"),
		tb.End.Format("15:04"),
		domain.FormatDate(date),
	)
	return tgbotapi.NewMessage(chatID, text), nil
}

func canArchiveTimebox(dt *domain.DailyTimeboxes, idx int, s *store.Store) (bool, string, error) {
	if dt == nil || idx < 0 || idx >= len(dt.Timeboxes) {
		return false, "timebox not found", nil
	}

	tb := dt.Timeboxes[idx]
	if tb.Status == domain.StatusArchived {
		return false, "already archived", nil
	}
	if tb.IsReserved() {
		return false, "reserved timebox", nil
	}
	if tb.TaskListSlug == "" {
		return false, "unassigned timebox", nil
	}

	listTasks := make(map[string][]domain.Task)
	for _, box := range dt.Timeboxes {
		if box.TaskListSlug == "" {
			continue
		}
		if _, ok := listTasks[box.TaskListSlug]; ok {
			continue
		}
		tl, err := s.ReadTaskList(box.TaskListSlug)
		if err != nil {
			return false, "", err
		}
		listTasks[box.TaskListSlug] = tl.ActiveTasks()
	}

	scheduledByIndex := sequenceDailyTimeboxes(dt.Timeboxes, listTasks)
	for _, st := range scheduledByIndex[idx] {
		if st.IsBreak || isCompletedTask(tb.CompletedTasks, st.Task) {
			continue
		}
		return false, "pending tasks remain", nil
	}

	return true, "", nil
}

// handleHelp returns a summary of available commands.
func (b *Bot) handleHelp(chatID int64) (tgbotapi.MessageConfig, error) {
	text := `<b>Meadow Commands</b>

/today - Show today's timeboxes
/week - Show current week summary
/lists - Show all task lists
/list {name} - Show tasks in a list
/done {task} - Mark a task as done
/add {list} {desc} ~{time} - Add a task
/plan {day} {start}-{end} - Create a timebox
/assign {day} {time} {list} - Assign list to timebox
/archive {day} {time} - Archive a timebox
/help - Show this message

<b>Day formats:</b> today, tomorrow, mon, tue, wed, thu, fri, sat, sun`
	return tgbotapi.NewMessage(chatID, text), nil
}

// resolveDay parses a day reference into a date.
// Supported: "today", "tomorrow", "mon"..."sun", or "YYYY-MM-DD".
func resolveDay(s string) (time.Time, error) {
	now := time.Now()
	today := domain.StripTimeForDate(now)

	switch strings.ToLower(s) {
	case "today":
		return today, nil
	case "tomorrow":
		return today.AddDate(0, 0, 1), nil
	}

	// Try weekday abbreviation.
	dayMap := map[string]time.Weekday{
		"sun": time.Sunday,
		"mon": time.Monday,
		"tue": time.Tuesday,
		"wed": time.Wednesday,
		"thu": time.Thursday,
		"fri": time.Friday,
		"sat": time.Saturday,
	}

	if wd, ok := dayMap[strings.ToLower(s)]; ok {
		// Find the next occurrence of this weekday (including today).
		diff := int(wd) - int(now.Weekday())
		if diff < 0 {
			diff += 7
		}
		return today.AddDate(0, 0, diff), nil
	}

	// Try ISO date.
	parsed, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, fmt.Errorf("unrecognised day %q (use today, tomorrow, mon-sun, or YYYY-MM-DD)", s)
	}
	return parsed, nil
}

// fuzzyMatchList finds the first task list whose name or slug contains the
// query (case-insensitive).
func fuzzyMatchList(lists []*domain.TaskList, query string) *domain.TaskList {
	q := strings.ToLower(query)
	for _, tl := range lists {
		if strings.Contains(strings.ToLower(tl.Name), q) ||
			strings.Contains(strings.ToLower(tl.Slug), q) {
			return tl
		}
	}
	return nil
}

func validateTimeboxRange(start, end time.Time) error {
	if !start.Before(end) {
		return fmt.Errorf("start time must be before end time")
	}
	if end.Sub(start) < 15*time.Minute {
		return fmt.Errorf("timebox must be at least 15 minutes")
	}
	return nil
}

func checkTimeboxOverlap(start, end time.Time, timeboxes []domain.Timebox) error {
	for _, tb := range timeboxes {
		if start.Before(tb.End) && tb.Start.Before(end) {
			return fmt.Errorf("overlaps with existing timebox %s-%s", tb.Start.Format("15:04"), tb.End.Format("15:04"))
		}
	}
	return nil
}
