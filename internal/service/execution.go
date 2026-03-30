package service

import (
	"fmt"
	"time"

	"github.com/mhwang-1/meadow-bubbletea/internal/domain"
	"github.com/mhwang-1/meadow-bubbletea/internal/store"
)

// MarkTaskDone marks the first uncompleted task in the timebox at timeboxIdx
// as done. It performs a 5-step atomic operation:
//  1. Read daily timeboxes, sort, validate index
//  2. Read task list for the timebox's slug
//  3. Find pending task at taskIdx (skip already-completed tasks)
//  4. Add to timebox CompletedTasks, remove from active task list
//  5. Write task list, write daily timeboxes, add to archive completed.md
func (s *Service) MarkTaskDone(date time.Time, timeboxIdx, taskIdx int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 1. Read daily timeboxes.
	dt, err := s.store.ReadDailyTimeboxes(date)
	if err != nil {
		return fmt.Errorf("reading daily timeboxes: %w", err)
	}

	dt.SortByStart()

	if timeboxIdx < 0 || timeboxIdx >= len(dt.Timeboxes) {
		return fmt.Errorf("timebox index %d out of range", timeboxIdx)
	}

	tb := &dt.Timeboxes[timeboxIdx]

	if tb.TaskListSlug == "" || tb.Status != domain.StatusActive {
		return fmt.Errorf("timebox is not active or has no task list assigned")
	}

	// 2. Read the task list.
	tl, err := s.store.ReadTaskList(tb.TaskListSlug)
	if err != nil {
		return fmt.Errorf("reading task list %q: %w", tb.TaskListSlug, err)
	}

	activeTasks := tl.ActiveTasks()
	if len(activeTasks) == 0 {
		return fmt.Errorf("no active tasks in task list %q", tb.TaskListSlug)
	}

	// 3. Find the pending task at taskIdx (skip already-completed tasks).
	var currentTask *domain.Task
	pendingIdx := 0
	for _, at := range activeTasks {
		if isTaskCompleted(*tb, at) {
			continue
		}
		if pendingIdx == taskIdx {
			currentTask = &at
			break
		}
		pendingIdx++
	}

	if currentTask == nil {
		return fmt.Errorf("no pending task at index %d", taskIdx)
	}

	// 4. Add to timebox CompletedTasks, remove from active task list.
	tb.CompletedTasks = append(tb.CompletedTasks, *currentTask)

	var updatedTasks []domain.Task
	removed := false
	for _, t := range tl.Tasks {
		if !removed && !t.Commented && t.Description == currentTask.Description && t.Duration == currentTask.Duration {
			removed = true
			continue
		}
		updatedTasks = append(updatedTasks, t)
	}
	tl.Tasks = updatedTasks

	// 5. Write task list, write daily timeboxes, add to archive.
	if err := s.store.WriteTaskList(tl); err != nil {
		return fmt.Errorf("writing task list: %w", err)
	}

	if err := s.store.WriteDailyTimeboxes(dt); err != nil {
		return fmt.Errorf("writing daily timeboxes: %w", err)
	}

	// Best effort: add to archive completed.md.
	wi := domain.WeekForDate(date)
	completedTask := store.CompletedTask{
		Task:          *currentTask,
		CompletedDate: domain.StripTimeForDate(date),
		TaskListSlug:  tb.TaskListSlug,
	}
	_ = s.store.AddCompleted(wi.Year, wi.Week, completedTask)

	return nil
}

// UnmarkTaskDone reverses a mark-done operation. It removes the completed
// task at taskIdx from the timebox's CompletedTasks and adds it back to the
// task list.
func (s *Service) UnmarkTaskDone(date time.Time, timeboxIdx, taskIdx int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 1. Read daily timeboxes.
	dt, err := s.store.ReadDailyTimeboxes(date)
	if err != nil {
		return fmt.Errorf("reading daily timeboxes: %w", err)
	}

	dt.SortByStart()

	if timeboxIdx < 0 || timeboxIdx >= len(dt.Timeboxes) {
		return fmt.Errorf("timebox index %d out of range", timeboxIdx)
	}

	tb := &dt.Timeboxes[timeboxIdx]

	if tb.TaskListSlug == "" || len(tb.CompletedTasks) == 0 {
		return fmt.Errorf("timebox has no task list or no completed tasks")
	}

	if taskIdx < 0 || taskIdx >= len(tb.CompletedTasks) {
		return fmt.Errorf("completed task index %d out of range", taskIdx)
	}

	// 2. Remove the completed task.
	reopenedTask := tb.CompletedTasks[taskIdx]
	tb.CompletedTasks = append(tb.CompletedTasks[:taskIdx], tb.CompletedTasks[taskIdx+1:]...)

	// 3. Add back to task list.
	tl, err := s.store.ReadTaskList(tb.TaskListSlug)
	if err != nil {
		return fmt.Errorf("reading task list %q: %w", tb.TaskListSlug, err)
	}

	tl.Tasks = append(tl.Tasks, reopenedTask)

	if err := s.store.WriteTaskList(tl); err != nil {
		return fmt.Errorf("writing task list: %w", err)
	}

	if err := s.store.WriteDailyTimeboxes(dt); err != nil {
		return fmt.Errorf("writing daily timeboxes: %w", err)
	}

	// 4. Best effort: remove from archive completed.md.
	wi := domain.WeekForDate(date)
	_ = s.removeCompletedArchiveEntry(wi.Year, wi.Week, store.CompletedTask{
		Task:          reopenedTask,
		CompletedDate: domain.StripTimeForDate(date),
		TaskListSlug:  tb.TaskListSlug,
	})

	return nil
}

// PendingTasksError is returned when ArchiveTimebox is called without force
// and there are pending tasks remaining.
type PendingTasksError struct {
	Count int
}

func (e *PendingTasksError) Error() string {
	return fmt.Sprintf("%d task(s) undone — archive anyway with force=true", e.Count)
}

// ArchiveTimebox archives a timebox at the given index. It validates that the
// timebox is active, assigned, and not reserved. If force is false and there
// are pending tasks, it returns a PendingTasksError with the count.
func (s *Service) ArchiveTimebox(date time.Time, idx int, force bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	dt, err := s.store.ReadDailyTimeboxes(date)
	if err != nil {
		return fmt.Errorf("reading daily timeboxes: %w", err)
	}

	dt.SortByStart()

	if idx < 0 || idx >= len(dt.Timeboxes) {
		return fmt.Errorf("timebox index %d out of range", idx)
	}

	tb := &dt.Timeboxes[idx]

	if tb.Status == domain.StatusArchived {
		return fmt.Errorf("timebox is already archived")
	}

	if tb.IsReserved() {
		return fmt.Errorf("cannot archive a reserved timebox")
	}

	if tb.TaskListSlug == "" {
		return fmt.Errorf("cannot archive an unassigned timebox")
	}

	// Count pending tasks if not forcing.
	if !force {
		tl, err := s.store.ReadTaskList(tb.TaskListSlug)
		if err != nil {
			return fmt.Errorf("reading task list %q: %w", tb.TaskListSlug, err)
		}

		listTasks := map[string][]domain.Task{
			tb.TaskListSlug: tl.ActiveTasks(),
		}
		scheduledByIndex := sequenceDailyTimeboxes(dt, listTasks)
		pendingCount := 0
		for _, st := range scheduledByIndex[idx] {
			if st.IsBreak || isTaskCompleted(*tb, st.Task) {
				continue
			}
			pendingCount++
		}

		if pendingCount > 0 {
			return &PendingTasksError{Count: pendingCount}
		}
	}

	wi := domain.WeekForDate(date)

	// 1. Archive completed tasks to weekly completed.md.
	if len(tb.CompletedTasks) > 0 {
		for _, ct := range tb.CompletedTasks {
			completedTask := store.CompletedTask{
				Task:          ct,
				CompletedDate: domain.StripTimeForDate(date),
				TaskListSlug:  tb.TaskListSlug,
			}
			// Best effort — continue archiving.
			_ = s.store.AddCompleted(wi.Year, wi.Week, completedTask)
		}
	}

	// 2. Add timebox record to weekly timeboxes.md.
	archivedTimeboxes, err := s.store.ReadArchivedTimeboxes(wi.Year, wi.Week)
	if err != nil {
		archivedTimeboxes = []store.ArchivedTimebox{}
	}

	archivedTimeboxes = append(archivedTimeboxes, store.ArchivedTimebox{
		Date:           domain.StripTimeForDate(date),
		Start:          tb.Start,
		End:            tb.End,
		TaskListSlug:   tb.TaskListSlug,
		Tag:            tb.Tag,
		Note:           tb.Note,
		CompletedTasks: tb.CompletedTasks,
	})

	// Best effort write.
	_ = s.store.WriteArchivedTimeboxes(wi.Year, wi.Week, archivedTimeboxes)

	// 3. Mark the timebox as archived in the daily file.
	tb.Status = domain.StatusArchived

	if err := s.store.WriteDailyTimeboxes(dt); err != nil {
		return fmt.Errorf("writing daily timeboxes: %w", err)
	}

	return nil
}

// UnarchiveTimebox sets a timebox's status back to active and removes the
// matching record from the weekly archived timeboxes file.
func (s *Service) UnarchiveTimebox(date time.Time, idx int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	dt, err := s.store.ReadDailyTimeboxes(date)
	if err != nil {
		return fmt.Errorf("reading daily timeboxes: %w", err)
	}

	dt.SortByStart()

	if idx < 0 || idx >= len(dt.Timeboxes) {
		return fmt.Errorf("timebox index %d out of range", idx)
	}

	tb := &dt.Timeboxes[idx]

	if tb.Status != domain.StatusArchived {
		return fmt.Errorf("timebox is not archived")
	}

	// 1. Set status back to active.
	tb.Status = domain.StatusActive

	if err := s.store.WriteDailyTimeboxes(dt); err != nil {
		return fmt.Errorf("writing daily timeboxes: %w", err)
	}

	// 2. Remove matching record from archived timeboxes (best effort).
	wi := domain.WeekForDate(date)
	s.removeArchivedTimeboxRecord(wi.Year, wi.Week, date, *tb)

	return nil
}

// removeArchivedTimeboxRecord removes the first matching timebox record from
// the weekly archived timeboxes file. Best effort — silently no-ops on failure.
func (s *Service) removeArchivedTimeboxRecord(year, week int, date time.Time, tb domain.Timebox) {
	archivedTimeboxes, err := s.store.ReadArchivedTimeboxes(year, week)
	if err != nil || len(archivedTimeboxes) == 0 {
		return
	}

	tbDate := domain.StripTimeForDate(date)
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
		_ = s.store.WriteArchivedTimeboxes(year, week, updated)
	}
}

// removeCompletedArchiveEntry removes one matching completed task from
// the weekly completed.md archive. Best effort.
func (s *Service) removeCompletedArchiveEntry(year, week int, target store.CompletedTask) error {
	completed, err := s.store.ReadCompleted(year, week)
	if err != nil {
		return err
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
		return nil
	}

	return s.store.WriteCompleted(year, week, updated)
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

// sequenceDailyTimeboxes sequences tasks across timeboxes grouped by slug,
// replicating the TUI's scheduling logic. Returns scheduled tasks indexed
// by timebox position.
func sequenceDailyTimeboxes(dt *domain.DailyTimeboxes, listTasks map[string][]domain.Task) [][]domain.ScheduledTask {
	scheduledByIndex := make([][]domain.ScheduledTask, len(dt.Timeboxes))
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
		}
	}

	return scheduledByIndex
}
