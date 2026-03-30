package model

import (
	"fmt"
	"strings"
	"time"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mhwang-1/meadow-bubbletea/internal/domain"
	"github.com/mhwang-1/meadow-bubbletea/internal/store"
)

// InputMode tracks what kind of text input the user is providing.
type InputMode int

const (
	InputNone           InputMode = iota
	InputTimeboxCreate            // typing start-end time for new timebox
	InputTimeboxEdit              // editing start or end time of selected timebox
	InputTaskListCreate           // typing name for new task list
	InputReservedNote             // typing optional note for reserved timebox
)

// handleTimeboxCreate is called when 'n' is pressed in Plan mode, Day view.
// It enters input mode for creating a new timebox.
func (m *RootModel) handleTimeboxCreate() {
	m.inputMode = InputTimeboxCreate
	m.inputPrompt = "New timebox (HH:MM-HH:MM): "
	m.inputBuffer = ""
}

// handleTimeboxCreateConfirm is called when Enter is pressed during
// InputTimeboxCreate. It parses and validates the time range, then creates
// a new unassigned timebox.
func (m *RootModel) handleTimeboxCreateConfirm() {
	start, end, err := parseInputTimeRange(m.inputBuffer, m.currentDate)
	if err != nil {
		m.inputPrompt = fmt.Sprintf("Error: %v. Try again (HH:MM-HH:MM): ", err)
		m.inputBuffer = ""
		return
	}

	if err := validateTimeRange(start, end); err != nil {
		m.inputPrompt = fmt.Sprintf("Error: %v. Try again (HH:MM-HH:MM): ", err)
		m.inputBuffer = ""
		return
	}

	// Read current daily timeboxes to check for overlap.
	dt, err := m.store.ReadDailyTimeboxes(m.currentDate)
	if err != nil {
		m.inputPrompt = fmt.Sprintf("Error reading timeboxes: %v. Press Escape.", err)
		m.inputBuffer = ""
		return
	}

	if err := checkOverlap(start, end, dt.Timeboxes, -1); err != nil {
		m.inputPrompt = fmt.Sprintf("Error: %v. Try again (HH:MM-HH:MM): ", err)
		m.inputBuffer = ""
		return
	}

	// Create the new timebox.
	tb := domain.Timebox{
		Start:  start,
		End:    end,
		Status: domain.StatusUnassigned,
	}

	// Read-modify-write: re-read, append, sort, write back.
	dt, err = m.store.ReadDailyTimeboxes(m.currentDate)
	if err != nil {
		m.inputPrompt = fmt.Sprintf("Error reading timeboxes: %v. Press Escape.", err)
		m.inputBuffer = ""
		return
	}

	dt.Timeboxes = append(dt.Timeboxes, tb)
	dt.Date = domain.StripTimeForDate(m.currentDate)
	dt.SortByStart()

	if err := m.store.WriteDailyTimeboxes(dt); err != nil {
		m.inputPrompt = fmt.Sprintf("Error writing timeboxes: %v. Press Escape.", err)
		m.inputBuffer = ""
		return
	}

	m.clearInput()
}

// handleToggleReserved is called when 'r' is pressed in Plan mode, Day view.
// It toggles the reserved tag on the selected timebox.
func (m *RootModel) handleToggleReserved() {
	dt, err := m.store.ReadDailyTimeboxes(m.currentDate)
	if err != nil || len(dt.Timeboxes) == 0 {
		return
	}

	dt.SortByStart()

	if m.selectedTimebox < 0 || m.selectedTimebox >= len(dt.Timeboxes) {
		return
	}

	tb := &dt.Timeboxes[m.selectedTimebox]

	if tb.IsReserved() {
		// Unreserve: clear tag/note, return to unassigned.
		tb.Tag = ""
		tb.Note = ""
		tb.Status = domain.StatusUnassigned
		_ = m.store.WriteDailyTimeboxes(dt)
		return
	}

	// Only allow reserving unassigned timeboxes with no task list.
	if tb.TaskListSlug != "" || tb.Status == domain.StatusArchived {
		return
	}

	// Reserve: set tag and status, then prompt for note.
	tb.Tag = "reserved"
	tb.Status = domain.StatusActive
	_ = m.store.WriteDailyTimeboxes(dt)

	m.inputMode = InputReservedNote
	m.inputPrompt = "Note (optional, Enter to skip): "
	m.inputBuffer = ""
}

// handleTimeboxUnassign clears the assigned task list from the selected
// timebox, returning it to unassigned state.
func (m *RootModel) handleTimeboxUnassign() tea.Cmd {
	dt, err := m.store.ReadDailyTimeboxes(m.currentDate)
	if err != nil || len(dt.Timeboxes) == 0 {
		return nil
	}

	dt.SortByStart()

	if m.selectedTimebox < 0 || m.selectedTimebox >= len(dt.Timeboxes) {
		return nil
	}

	tb := &dt.Timeboxes[m.selectedTimebox]
	if tb.Status == domain.StatusArchived {
		return m.showToast("Cannot unassign archived timebox", 3*time.Second)
	}
	if tb.IsReserved() {
		return m.showToast("Cannot unassign reserved timebox", 3*time.Second)
	}
	if len(tb.CompletedTasks) > 0 {
		return m.showToast("Cannot unassign: unmark done tasks first", 3*time.Second)
	}
	if tb.TaskListSlug == "" {
		return nil
	}

	tb.TaskListSlug = ""
	tb.Status = domain.StatusUnassigned

	if err := m.store.WriteDailyTimeboxes(dt); err != nil {
		return m.showToast("Failed to unassign timebox", 3*time.Second)
	}

	return nil
}

// handleReservedNoteConfirm saves the optional note for a reserved timebox.
func (m *RootModel) handleReservedNoteConfirm() {
	note := strings.TrimSpace(m.inputBuffer)
	m.clearInput()

	if note == "" {
		return
	}

	dt, err := m.store.ReadDailyTimeboxes(m.currentDate)
	if err != nil || len(dt.Timeboxes) == 0 {
		return
	}

	dt.SortByStart()

	if m.selectedTimebox < 0 || m.selectedTimebox >= len(dt.Timeboxes) {
		return
	}

	tb := &dt.Timeboxes[m.selectedTimebox]
	if !tb.IsReserved() {
		return
	}

	tb.Note = note
	_ = m.store.WriteDailyTimeboxes(dt)
}

// handleTimeboxEdit is called when Enter is pressed on a selected timebox
// in Plan mode. It enters input mode for editing the timebox's time range.
func (m *RootModel) handleTimeboxEdit() {
	dt, err := m.store.ReadDailyTimeboxes(m.currentDate)
	if err != nil || len(dt.Timeboxes) == 0 {
		return
	}

	dt.SortByStart()

	if m.selectedTimebox < 0 || m.selectedTimebox >= len(dt.Timeboxes) {
		return
	}

	tb := dt.Timeboxes[m.selectedTimebox]
	m.inputMode = InputTimeboxEdit
	m.inputBuffer = fmt.Sprintf("%s-%s", tb.Start.Format("15:04"), tb.End.Format("15:04"))
	m.inputPrompt = "Edit times (HH:MM-HH:MM): "
}

// handleTimeboxEditConfirm is called when Enter is pressed during
// InputTimeboxEdit. It parses, validates, and saves the updated time range.
func (m *RootModel) handleTimeboxEditConfirm() {
	start, end, err := parseInputTimeRange(m.inputBuffer, m.currentDate)
	if err != nil {
		m.inputPrompt = fmt.Sprintf("Error: %v. Try again (HH:MM-HH:MM): ", err)
		m.inputBuffer = ""
		return
	}

	if err := validateTimeRange(start, end); err != nil {
		m.inputPrompt = fmt.Sprintf("Error: %v. Try again (HH:MM-HH:MM): ", err)
		m.inputBuffer = ""
		return
	}

	// Read-modify-write.
	dt, err := m.store.ReadDailyTimeboxes(m.currentDate)
	if err != nil {
		m.inputPrompt = fmt.Sprintf("Error reading timeboxes: %v. Press Escape.", err)
		m.inputBuffer = ""
		return
	}

	dt.SortByStart()

	if m.selectedTimebox < 0 || m.selectedTimebox >= len(dt.Timeboxes) {
		m.clearInput()
		return
	}

	// Check overlap excluding the timebox being edited.
	if err := checkOverlap(start, end, dt.Timeboxes, m.selectedTimebox); err != nil {
		m.inputPrompt = fmt.Sprintf("Error: %v. Try again (HH:MM-HH:MM): ", err)
		m.inputBuffer = ""
		return
	}

	dt.Timeboxes[m.selectedTimebox].Start = start
	dt.Timeboxes[m.selectedTimebox].End = end
	dt.SortByStart()

	if err := m.store.WriteDailyTimeboxes(dt); err != nil {
		m.inputPrompt = fmt.Sprintf("Error writing timeboxes: %v. Press Escape.", err)
		m.inputBuffer = ""
		return
	}

	m.clearInput()
}

// handleTimeboxDelete is called when 'd' is pressed on a selected timebox
// in Plan mode. It sets up a confirmation dialog.
func (m *RootModel) handleTimeboxDelete() {
	dt, err := m.store.ReadDailyTimeboxes(m.currentDate)
	if err != nil || len(dt.Timeboxes) == 0 {
		return
	}

	dt.SortByStart()

	if m.selectedTimebox < 0 || m.selectedTimebox >= len(dt.Timeboxes) {
		return
	}

	m.showConfirm = true
	m.confirmAction = func() {
		m.executeTimeboxDelete()
	}
}

// executeTimeboxDelete performs the actual timebox deletion, redistributing
// any open tasks to the next timebox for the same task list.
func (m *RootModel) executeTimeboxDelete() {
	dt, err := m.store.ReadDailyTimeboxes(m.currentDate)
	if err != nil || len(dt.Timeboxes) == 0 {
		return
	}

	dt.SortByStart()

	if m.selectedTimebox < 0 || m.selectedTimebox >= len(dt.Timeboxes) {
		return
	}

	deleted := dt.Timeboxes[m.selectedTimebox]

	// If the deleted timebox has a task list, redistribute open tasks.
	if deleted.TaskListSlug != "" {
		tl, err := m.store.ReadTaskList(deleted.TaskListSlug)
		if err == nil {
			openTasks := tl.ActiveTasks()
			if len(openTasks) > 0 {
				m.redistributeTasks(deleted, openTasks)
			}
		}
	}

	// Remove the timebox from the list.
	dt.Timeboxes = append(dt.Timeboxes[:m.selectedTimebox], dt.Timeboxes[m.selectedTimebox+1:]...)

	if err := m.store.WriteDailyTimeboxes(dt); err != nil {
		return
	}

	// Adjust cursor.
	if m.selectedTimebox >= len(dt.Timeboxes) && m.selectedTimebox > 0 {
		m.selectedTimebox--
	}
}

// handleTimeboxArchive is called when 'a' is pressed on a selected timebox.
// It archives the timebox: marks it as archived, moves completed tasks to the
// weekly archive, records the timebox in the archive, and redistributes open
// tasks.
func (m *RootModel) handleTimeboxArchive() {
	dt, err := m.store.ReadDailyTimeboxes(m.currentDate)
	if err != nil || len(dt.Timeboxes) == 0 {
		return
	}

	dt.SortByStart()

	if m.selectedTimebox < 0 || m.selectedTimebox >= len(dt.Timeboxes) {
		return
	}

	tb := &dt.Timeboxes[m.selectedTimebox]

	// Already archived — nothing to do.
	if tb.Status == domain.StatusArchived {
		return
	}

	ok, pendingCount, _ := m.canArchiveTimebox(dt, m.selectedTimebox)
	if !ok {
		return
	}

	if pendingCount > 0 {
		m.showConfirm = true
		m.confirmMessage = fmt.Sprintf("%d task(s) undone — archive anyway?", pendingCount)
		m.confirmAction = func() {
			m.executeTimeboxArchive()
		}
		return
	}

	m.executeTimeboxArchive()
}

// executeTimeboxArchive performs the actual timebox archive: moves completed
// tasks to the weekly archive, records the timebox, redistributes open tasks,
// and marks the timebox as archived.
func (m *RootModel) executeTimeboxArchive() {
	dt, err := m.store.ReadDailyTimeboxes(m.currentDate)
	if err != nil || len(dt.Timeboxes) == 0 {
		return
	}

	dt.SortByStart()

	if m.selectedTimebox < 0 || m.selectedTimebox >= len(dt.Timeboxes) {
		return
	}

	tb := &dt.Timeboxes[m.selectedTimebox]

	if tb.Status == domain.StatusArchived {
		return
	}

	wi := domain.WeekForDate(m.currentDate)

	// 1. Archive completed tasks to archive/{YYYY}/{WW}/completed.md.
	if len(tb.CompletedTasks) > 0 {
		for _, ct := range tb.CompletedTasks {
			completedTask := store.CompletedTask{
				Task:          ct,
				CompletedDate: domain.StripTimeForDate(m.currentDate),
				TaskListSlug:  tb.TaskListSlug,
			}
			if err := m.store.AddCompleted(wi.Year, wi.Week, completedTask); err != nil {
				// Best effort — continue archiving.
				continue
			}
		}
	}

	// 2. Add timebox record to archive/{YYYY}/{WW}/timeboxes.md.
	archivedTimeboxes, err := m.store.ReadArchivedTimeboxes(wi.Year, wi.Week)
	if err != nil {
		archivedTimeboxes = []store.ArchivedTimebox{}
	}

	archivedTimeboxes = append(archivedTimeboxes, store.ArchivedTimebox{
		Date:           domain.StripTimeForDate(m.currentDate),
		Start:          tb.Start,
		End:            tb.End,
		TaskListSlug:   tb.TaskListSlug,
		Tag:            tb.Tag,
		Note:           tb.Note,
		CompletedTasks: tb.CompletedTasks,
	})

	// Best effort write — don't block the archive on failure.
	_ = m.store.WriteArchivedTimeboxes(wi.Year, wi.Week, archivedTimeboxes)

	// 3. Redistribute open tasks to the next timebox for the same task list.
	if tb.TaskListSlug != "" {
		tl, err := m.store.ReadTaskList(tb.TaskListSlug)
		if err == nil {
			openTasks := tl.ActiveTasks()
			if len(openTasks) > 0 {
				m.redistributeTasks(*tb, openTasks)
			}
		}
	}

	// 4. Mark the timebox as archived in the daily file.
	tb.Status = domain.StatusArchived

	if err := m.store.WriteDailyTimeboxes(dt); err != nil {
		return
	}
}

func (m *RootModel) canArchiveTimebox(dt *domain.DailyTimeboxes, idx int) (bool, int, string) {
	if dt == nil || idx < 0 || idx >= len(dt.Timeboxes) {
		return false, 0, "timebox not found"
	}

	tb := dt.Timeboxes[idx]
	if tb.Status == domain.StatusArchived {
		return false, 0, "already archived"
	}
	if tb.IsReserved() {
		return false, 0, "reserved timebox"
	}
	if tb.TaskListSlug == "" {
		return false, 0, "unassigned timebox"
	}

	tl, err := m.store.ReadTaskList(tb.TaskListSlug)
	if err != nil {
		return false, 0, "task list unavailable"
	}

	listTasks := map[string][]domain.Task{
		tb.TaskListSlug: tl.ActiveTasks(),
	}
	scheduledByIndex, _ := sequenceDailyTimeboxes(dt, listTasks)
	pendingCount := 0
	for _, st := range scheduledByIndex[idx] {
		if st.IsBreak || isTaskCompleted(tb, st.Task) {
			continue
		}
		pendingCount++
	}

	return true, pendingCount, ""
}

// handleTimeboxUnarchive is called when 'U' is pressed on an archived timebox.
// It reactivates the timebox and removes the matching record from the weekly
// archive. Completed tasks remain in the archive.
func (m *RootModel) handleTimeboxUnarchive() {
	dt, err := m.store.ReadDailyTimeboxes(m.currentDate)
	if err != nil || len(dt.Timeboxes) == 0 {
		return
	}

	dt.SortByStart()

	if m.selectedTimebox < 0 || m.selectedTimebox >= len(dt.Timeboxes) {
		return
	}

	tb := &dt.Timeboxes[m.selectedTimebox]

	// Only archived timeboxes can be unarchived.
	if tb.Status != domain.StatusArchived {
		return
	}

	// 1. Set status back to active.
	tb.Status = domain.StatusActive

	if err := m.store.WriteDailyTimeboxes(dt); err != nil {
		return
	}

	// 2. Remove the matching timebox record from archive/{YYYY}/{WW}/timeboxes.md.
	wi := domain.WeekForDate(m.currentDate)
	m.removeArchivedTimeboxRecord(wi.Year, wi.Week, *tb)
}

// removeArchivedTimeboxRecord removes the matching timebox record from the
// weekly archived timeboxes file. Matches on date, start/end times, and task
// list slug. Best-effort — silently no-ops if the record is missing.
func (m *RootModel) removeArchivedTimeboxRecord(year, week int, tb domain.Timebox) {
	archivedTimeboxes, err := m.store.ReadArchivedTimeboxes(year, week)
	if err != nil || len(archivedTimeboxes) == 0 {
		return
	}

	tbDate := domain.StripTimeForDate(m.currentDate)
	var updated []store.ArchivedTimebox
	removed := false

	for _, at := range archivedTimeboxes {
		if !removed &&
			domain.StripTimeForDate(at.Date).Equal(tbDate) &&
			at.Start.Hour() == tb.Start.Hour() && at.Start.Minute() == tb.Start.Minute() &&
			at.End.Hour() == tb.End.Hour() && at.End.Minute() == tb.End.Minute() &&
			at.TaskListSlug == tb.TaskListSlug {
			removed = true
			continue
		}
		updated = append(updated, at)
	}

	if removed {
		_ = m.store.WriteArchivedTimeboxes(year, week, updated)
	}
}

// handleMarkDone is called when 'x' is pressed in Execute mode on the
// current task. It marks the first uncompleted task in the current timebox
// as done.
func (m *RootModel) handleMarkDone() {
	dt, err := m.store.ReadDailyTimeboxes(m.currentDate)
	if err != nil || len(dt.Timeboxes) == 0 {
		return
	}

	dt.SortByStart()

	if m.selectedTimebox < 0 || m.selectedTimebox >= len(dt.Timeboxes) {
		return
	}

	tb := &dt.Timeboxes[m.selectedTimebox]

	// Must be an active timebox with an assigned task list.
	if tb.TaskListSlug == "" || tb.Status != domain.StatusActive {
		return
	}

	// Read the task list.
	tl, err := m.store.ReadTaskList(tb.TaskListSlug)
	if err != nil {
		return
	}

	activeTasks := tl.ActiveTasks()
	if len(activeTasks) == 0 {
		return
	}

	// Find the first uncompleted task (not already in CompletedTasks).
	var currentTask *domain.Task
	for _, at := range activeTasks {
		if !isTaskCompleted(*tb, at) {
			currentTask = &at
			break
		}
	}

	if currentTask == nil {
		return
	}

	// 1. Add the task to the timebox's completed list.
	tb.CompletedTasks = append(tb.CompletedTasks, *currentTask)

	// 2. Remove the task from the active task list file.
	var updatedTasks []domain.Task
	removed := false
	for _, t := range tl.Tasks {
		if !removed && !t.Commented && t.Description == currentTask.Description && t.Duration == currentTask.Duration {
			removed = true
			continue // skip this task
		}
		updatedTasks = append(updatedTasks, t)
	}
	tl.Tasks = updatedTasks

	// 3. Write the updated task list.
	if err := m.store.WriteTaskList(tl); err != nil {
		return
	}

	// 4. Write the updated daily timeboxes.
	if err := m.store.WriteDailyTimeboxes(dt); err != nil {
		return
	}

	// 5. Add to archive completed.md for the current week.
	wi := domain.WeekForDate(m.currentDate)
	completedTask := store.CompletedTask{
		Task:          *currentTask,
		CompletedDate: domain.StripTimeForDate(m.currentDate),
		TaskListSlug:  tb.TaskListSlug,
	}
	// Best effort — don't fail the mark-done if archive write fails.
	_ = m.store.AddCompleted(wi.Year, wi.Week, completedTask)
}

// handleUnmarkDone is called when 'u' is pressed in Execute mode on the
// selected timebox. It reopens the latest completed task in that timebox.
func (m *RootModel) handleUnmarkDone() {
	dt, err := m.store.ReadDailyTimeboxes(m.currentDate)
	if err != nil || len(dt.Timeboxes) == 0 {
		return
	}

	dt.SortByStart()

	if m.selectedTimebox < 0 || m.selectedTimebox >= len(dt.Timeboxes) {
		return
	}

	tb := &dt.Timeboxes[m.selectedTimebox]

	// Must be an assigned timebox with at least one completed task.
	if tb.TaskListSlug == "" || len(tb.CompletedTasks) == 0 {
		return
	}

	// Reopen the most recently completed task in this timebox.
	reopenedTask := tb.CompletedTasks[len(tb.CompletedTasks)-1]
	tb.CompletedTasks = tb.CompletedTasks[:len(tb.CompletedTasks)-1]

	// Add the task back to the active task list file.
	tl, err := m.store.ReadTaskList(tb.TaskListSlug)
	if err != nil {
		return
	}
	tl.Tasks = append(tl.Tasks, reopenedTask)
	if err := m.store.WriteTaskList(tl); err != nil {
		return
	}

	// Persist the updated daily timeboxes.
	if err := m.store.WriteDailyTimeboxes(dt); err != nil {
		return
	}

	// Best effort: remove a matching archive entry for this week/day.
	wi := domain.WeekForDate(m.currentDate)
	_, _ = m.removeCompletedArchiveEntry(wi.Year, wi.Week, store.CompletedTask{
		Task:          reopenedTask,
		CompletedDate: domain.StripTimeForDate(m.currentDate),
		TaskListSlug:  tb.TaskListSlug,
	})
}

// removeCompletedArchiveEntry removes one matching completed task from
// archive/{year}/{week}/completed.md.
func (m *RootModel) removeCompletedArchiveEntry(year, week int, target store.CompletedTask) (bool, error) {
	completed, err := m.store.ReadCompleted(year, week)
	if err != nil {
		return false, err
	}

	targetDate := domain.StripTimeForDate(target.CompletedDate)
	var updated []store.CompletedTask
	removed := false

	for _, ct := range completed {
		if !removed &&
			ct.TaskListSlug == target.TaskListSlug &&
			domain.StripTimeForDate(ct.CompletedDate).Equal(targetDate) &&
			ct.Task.Description == target.Task.Description &&
			ct.Task.Duration == target.Task.Duration {
			removed = true
			continue
		}
		updated = append(updated, ct)
	}

	if !removed {
		return false, nil
	}

	if err := m.store.WriteCompleted(year, week, updated); err != nil {
		return false, err
	}

	return true, nil
}

// removeTaskFromDailyCompleted removes one matching completed task from the
// daily timebox file for the given date and task list slug.
func (m *RootModel) removeTaskFromDailyCompleted(date time.Time, slug string, task domain.Task) (bool, error) {
	dt, err := m.store.ReadDailyTimeboxes(date)
	if err != nil {
		return false, err
	}

	removed := false
	for i := range dt.Timeboxes {
		tb := &dt.Timeboxes[i]
		if tb.TaskListSlug != slug || len(tb.CompletedTasks) == 0 {
			continue
		}

		updatedTasks, didRemove := removeFirstMatchingTask(tb.CompletedTasks, task)
		if didRemove {
			tb.CompletedTasks = updatedTasks
			removed = true
			break
		}
	}

	if !removed {
		return false, nil
	}

	if err := m.store.WriteDailyTimeboxes(dt); err != nil {
		return false, err
	}

	return true, nil
}

func removeFirstMatchingTask(tasks []domain.Task, target domain.Task) ([]domain.Task, bool) {
	for i, t := range tasks {
		if t.Description == target.Description && t.Duration == target.Duration {
			updated := make([]domain.Task, 0, len(tasks)-1)
			updated = append(updated, tasks[:i]...)
			updated = append(updated, tasks[i+1:]...)
			return updated, true
		}
	}

	return tasks, false
}

// isTaskCompleted checks whether a task appears in the timebox's completed list.
func isTaskCompleted(tb domain.Timebox, task domain.Task) bool {
	for _, ct := range tb.CompletedTasks {
		if ct.Description == task.Description && ct.Duration == task.Duration {
			return true
		}
	}
	return false
}

// redistributeTasks moves open tasks to the next chronological timebox
// assigned to the same task list. If no such timebox exists, the tasks
// remain unscheduled in their task list (no action needed since they are
// already there).
func (m *RootModel) redistributeTasks(fromTimebox domain.Timebox, openTasks []domain.Task) {
	if fromTimebox.TaskListSlug == "" || len(openTasks) == 0 {
		return
	}

	dt, err := m.store.ReadDailyTimeboxes(m.currentDate)
	if err != nil {
		return
	}

	dt.SortByStart()

	// Find the next timebox (after fromTimebox.End) with the same task list slug.
	for i := range dt.Timeboxes {
		tb := &dt.Timeboxes[i]
		if tb.TaskListSlug != fromTimebox.TaskListSlug {
			continue
		}
		if !tb.Start.Before(fromTimebox.End) && tb.Status != domain.StatusArchived {
			// Found the next timebox for the same task list.
			// The tasks are already in the task list file, so the next timebox
			// will automatically pick them up via SequenceTasks. No file
			// manipulation is needed — the tasks live in the task list, not
			// in the timebox.
			return
		}
	}

	// No future timebox found for this task list. Tasks stay unscheduled
	// in their task list file — nothing to do.
}

// handleInputChar appends a printable character to the input buffer.
func (m *RootModel) handleInputChar(ch rune) {
	if unicode.IsPrint(ch) {
		m.inputBuffer += string(ch)
	}
}

// handleInputBackspace removes the last character from the input buffer.
func (m *RootModel) handleInputBackspace() {
	if len(m.inputBuffer) > 0 {
		runes := []rune(m.inputBuffer)
		m.inputBuffer = string(runes[:len(runes)-1])
	}
}

// clearInput resets input mode and related fields.
func (m *RootModel) clearInput() {
	m.inputMode = InputNone
	m.inputBuffer = ""
	m.inputPrompt = ""
}

// clearConfirm resets the confirmation dialog state.
func (m *RootModel) clearConfirm() {
	m.showConfirm = false
	m.confirmAction = nil
	m.confirmMessage = ""
}

// renderInputLine renders the input prompt and buffer for display.
func (m *RootModel) renderInputLine() string {
	return m.inputPrompt + m.inputBuffer + "█"
}

// renderConfirmLine renders the confirmation prompt.
func (m *RootModel) renderConfirmLine() string {
	if m.confirmMessage != "" {
		return m.confirmMessage + " (y/n)"
	}
	return "Are you sure? (y/n)"
}

// parseInputTimeRange parses a string like "09:00-11:00" into start and end
// time.Time values on the given date.
func parseInputTimeRange(input string, date time.Time) (time.Time, time.Time, error) {
	input = strings.TrimSpace(input)
	parts := strings.SplitN(input, "-", 2)
	if len(parts) != 2 {
		return time.Time{}, time.Time{}, fmt.Errorf("expected HH:MM-HH:MM, got %q", input)
	}

	startStr := strings.TrimSpace(parts[0])
	endStr := strings.TrimSpace(parts[1])

	startHour, startMin, err := parseHHMM(startStr)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid start time: %w", err)
	}

	endHour, endMin, err := parseHHMM(endStr)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid end time: %w", err)
	}

	y, mo, d := date.Date()
	loc := date.Location()
	start := time.Date(y, mo, d, startHour, startMin, 0, 0, loc)
	end := time.Date(y, mo, d, endHour, endMin, 0, 0, loc)

	return start, end, nil
}

// parseHHMM parses a time string like "09:00" or "14:30" into hour and minute.
func parseHHMM(s string) (int, int, error) {
	s = strings.TrimSpace(s)
	var hour, min int
	n, err := fmt.Sscanf(s, "%d:%d", &hour, &min)
	if err != nil || n != 2 {
		return 0, 0, fmt.Errorf("expected HH:MM, got %q", s)
	}
	if hour < 0 || hour > 23 || min < 0 || min > 59 {
		return 0, 0, fmt.Errorf("time out of range: %q", s)
	}
	return hour, min, nil
}

// validateTimeRange checks that start < end and the duration is at least
// 15 minutes.
func validateTimeRange(start, end time.Time) error {
	if !start.Before(end) {
		return fmt.Errorf("start time must be before end time")
	}
	if end.Sub(start) < 15*time.Minute {
		return fmt.Errorf("timebox must be at least 15 minutes")
	}
	return nil
}

// checkOverlap verifies that the proposed [start, end) range does not overlap
// with any existing timebox. excludeIndex can be set to skip a specific timebox
// (e.g. the one being edited); pass -1 to check all.
func checkOverlap(start, end time.Time, timeboxes []domain.Timebox, excludeIndex int) error {
	for i, tb := range timeboxes {
		if i == excludeIndex {
			continue
		}
		// Two intervals [s1,e1) and [s2,e2) overlap iff s1 < e2 && s2 < e1.
		if start.Before(tb.End) && tb.Start.Before(end) {
			return fmt.Errorf("overlaps with existing timebox %s-%s",
				tb.Start.Format("15:04"), tb.End.Format("15:04"))
		}
	}
	return nil
}
