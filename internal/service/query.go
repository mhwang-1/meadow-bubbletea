package service

import (
	"time"

	"github.com/mhwang-1/meadow-bubbletea/internal/domain"
	"github.com/mhwang-1/meadow-bubbletea/internal/store"
)

// DayViewTimebox combines a timebox with its scheduled tasks and task list name.
type DayViewTimebox struct {
	Index          int
	Timebox        domain.Timebox
	ScheduledTasks []domain.ScheduledTask
	TaskListName   string
}

// DayView is a structured view of a single day's timeboxes with sequenced tasks.
type DayView struct {
	Date      time.Time
	Timeboxes []DayViewTimebox
}

// DaySummary is a day entry within a week summary.
type DaySummary struct {
	Date      time.Time
	Timeboxes []DayViewTimebox
}

// WeekSummary provides an overview of all seven days in a week.
type WeekSummary struct {
	Week domain.WeekInfo
	Days [7]DaySummary
}

// GetDayView reads the daily timeboxes and, for each active timebox with a
// task list, sequences tasks using domain.SequenceTasks. Returns a structured
// view with timeboxes and their scheduled tasks.
func (s *Service) GetDayView(date time.Time) (*DayView, error) {
	dt, err := s.store.ReadDailyTimeboxes(date)
	if err != nil {
		return nil, err
	}

	dt.SortByStart()

	// Build the list of tasks per slug, reading each task list once.
	listTasks := make(map[string][]domain.Task)
	listNames := make(map[string]string)

	for _, tb := range dt.Timeboxes {
		if tb.TaskListSlug == "" {
			continue
		}
		if _, ok := listTasks[tb.TaskListSlug]; ok {
			continue
		}
		tl, err := s.store.ReadTaskList(tb.TaskListSlug)
		if err != nil {
			listTasks[tb.TaskListSlug] = nil
			continue
		}
		listTasks[tb.TaskListSlug] = tl.ActiveTasks()
		listNames[tb.TaskListSlug] = tl.Name
	}

	// Sequence tasks across timeboxes.
	scheduledByIndex := sequenceDailyTimeboxes(dt, listTasks)

	// Build the view.
	view := &DayView{
		Date:      date,
		Timeboxes: make([]DayViewTimebox, len(dt.Timeboxes)),
	}

	for i, tb := range dt.Timeboxes {
		view.Timeboxes[i] = DayViewTimebox{
			Index:          i,
			Timebox:        tb,
			ScheduledTasks: scheduledByIndex[i],
			TaskListName:   listNames[tb.TaskListSlug],
		}
	}

	return view, nil
}

// GetWeekSummary builds a summary of all seven days in the week containing
// the given date.
func (s *Service) GetWeekSummary(date time.Time) (*WeekSummary, error) {
	wi := domain.WeekForDate(date)
	days := domain.DaysOfWeek(wi)

	summary := &WeekSummary{
		Week: wi,
	}

	for i, day := range days {
		dayView, err := s.GetDayView(day)
		if err != nil {
			// Use empty day on error.
			summary.Days[i] = DaySummary{Date: day}
			continue
		}
		summary.Days[i] = DaySummary{
			Date:      day,
			Timeboxes: dayView.Timeboxes,
		}
	}

	return summary, nil
}

// GetHistory returns completed tasks for the given year and week.
func (s *Service) GetHistory(year, week int) ([]store.CompletedTask, error) {
	return s.store.ReadCompleted(year, week)
}

// GetArchivedTaskLists returns all archived task lists grouped by year and week.
func (s *Service) GetArchivedTaskLists() (map[int]map[int][]*domain.TaskList, error) {
	return s.store.ListArchivedTaskLists()
}

// GetWeeklyStats returns the total duration of completed tasks grouped by
// task list slug for the given year and week.
func (s *Service) GetWeeklyStats(year, week int) (map[string]time.Duration, error) {
	return s.store.CompletedDurationsBySlug(year, week)
}

// GetArchivedTimeboxes returns the archived timebox records for the given
// year and week.
func (s *Service) GetArchivedTimeboxes(year, week int) ([]store.ArchivedTimebox, error) {
	return s.store.ReadArchivedTimeboxes(year, week)
}
