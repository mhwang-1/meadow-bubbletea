package service

import (
	"fmt"
	"time"

	"github.com/mhwang-1/meadow-bubbletea/internal/domain"
)

// CreateTimebox creates a new unassigned timebox on the given date.
// It validates the time range (start < end, >= 15 min) and checks for
// overlap with existing timeboxes.
func (s *Service) CreateTimebox(date time.Time, start, end time.Time) error {
	if err := validateTimeRange(start, end); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	dt, err := s.store.ReadDailyTimeboxes(date)
	if err != nil {
		return fmt.Errorf("reading daily timeboxes: %w", err)
	}

	if err := checkOverlap(start, end, dt.Timeboxes, -1); err != nil {
		return err
	}

	dt.Timeboxes = append(dt.Timeboxes, domain.Timebox{
		Start:  start,
		End:    end,
		Status: domain.StatusUnassigned,
	})
	dt.Date = domain.StripTimeForDate(date)
	dt.SortByStart()

	if err := s.store.WriteDailyTimeboxes(dt); err != nil {
		return fmt.Errorf("writing daily timeboxes: %w", err)
	}

	return nil
}

// EditTimeboxTime updates the time range of an existing timebox. It validates
// the new range and checks for overlap excluding the timebox being edited.
func (s *Service) EditTimeboxTime(date time.Time, idx int, start, end time.Time) error {
	if err := validateTimeRange(start, end); err != nil {
		return err
	}

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

	if err := checkOverlap(start, end, dt.Timeboxes, idx); err != nil {
		return err
	}

	dt.Timeboxes[idx].Start = start
	dt.Timeboxes[idx].End = end
	dt.SortByStart()

	if err := s.store.WriteDailyTimeboxes(dt); err != nil {
		return fmt.Errorf("writing daily timeboxes: %w", err)
	}

	return nil
}

// DeleteTimebox removes a timebox at the given index from the daily file.
// Tasks remain in their task list; no redistribution is needed.
func (s *Service) DeleteTimebox(date time.Time, idx int) error {
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

	dt.Timeboxes = append(dt.Timeboxes[:idx], dt.Timeboxes[idx+1:]...)

	if err := s.store.WriteDailyTimeboxes(dt); err != nil {
		return fmt.Errorf("writing daily timeboxes: %w", err)
	}

	return nil
}

// AssignTaskList assigns a task list to an unassigned timebox and sets its
// status to active.
func (s *Service) AssignTaskList(date time.Time, idx int, slug string) error {
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

	if (tb.Status != domain.StatusUnassigned && tb.TaskListSlug != "") || tb.IsReserved() {
		return fmt.Errorf("timebox is not assignable (status: %s, slug: %q, reserved: %v)",
			tb.Status, tb.TaskListSlug, tb.IsReserved())
	}

	// Verify the task list exists.
	if _, err := s.store.ReadTaskList(slug); err != nil {
		return fmt.Errorf("task list %q not found: %w", slug, err)
	}

	tb.TaskListSlug = slug
	tb.Status = domain.StatusActive

	if err := s.store.WriteDailyTimeboxes(dt); err != nil {
		return fmt.Errorf("writing daily timeboxes: %w", err)
	}

	return nil
}

// SetReserved marks an unassigned timebox as reserved with an optional note.
func (s *Service) SetReserved(date time.Time, idx int, note string) error {
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

	if tb.TaskListSlug != "" || tb.Status == domain.StatusArchived {
		return fmt.Errorf("cannot reserve: timebox has task list or is archived")
	}

	tb.Tag = "reserved"
	tb.Status = domain.StatusActive
	tb.Note = note

	if err := s.store.WriteDailyTimeboxes(dt); err != nil {
		return fmt.Errorf("writing daily timeboxes: %w", err)
	}

	return nil
}

// UnsetReserved clears the reserved tag on a timebox and sets it back to
// unassigned.
func (s *Service) UnsetReserved(date time.Time, idx int) error {
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

	if !tb.IsReserved() {
		return fmt.Errorf("timebox is not reserved")
	}

	tb.Tag = ""
	tb.Note = ""
	tb.Status = domain.StatusUnassigned

	if err := s.store.WriteDailyTimeboxes(dt); err != nil {
		return fmt.Errorf("writing daily timeboxes: %w", err)
	}

	return nil
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
