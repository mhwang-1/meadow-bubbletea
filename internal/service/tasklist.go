package service

import (
	"fmt"
	"time"

	"github.com/mhwang-1/meadow-bubbletea/internal/domain"
)

// CreateTaskList creates a new task list with the given display name.
// The slug is generated using domain.Slugify. Returns an error if a task list
// with the same slug already exists.
func (s *Service) CreateTaskList(name string) (*domain.TaskList, error) {
	slug := domain.Slugify(name)
	if slug == "" {
		return nil, fmt.Errorf("invalid task list name: produces empty slug")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check for existing list with the same slug.
	if _, err := s.store.ReadTaskList(slug); err == nil {
		return nil, fmt.Errorf("task list %q already exists", slug)
	}

	tl := &domain.TaskList{
		Name: name,
		Slug: slug,
	}

	if err := s.store.WriteTaskList(tl); err != nil {
		return nil, fmt.Errorf("writing task list: %w", err)
	}

	return tl, nil
}

// UpdateTaskList replaces the tasks in an existing task list, preserving
// the list's name.
func (s *Service) UpdateTaskList(slug string, tasks []domain.Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tl, err := s.store.ReadTaskList(slug)
	if err != nil {
		return fmt.Errorf("reading task list %q: %w", slug, err)
	}

	tl.Tasks = tasks

	if err := s.store.WriteTaskList(tl); err != nil {
		return fmt.Errorf("writing task list: %w", err)
	}

	return nil
}

// DeleteTaskList deletes a task list. It returns an error if the list has
// completed tasks in the archive or is currently assigned to active timeboxes.
func (s *Service) DeleteTaskList(slug string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if assigned to active timeboxes.
	assigned, err := s.store.IsTaskListAssigned(slug)
	if err != nil {
		return fmt.Errorf("checking assignment: %w", err)
	}
	if assigned {
		return fmt.Errorf("cannot delete: list is assigned to active timeboxes")
	}

	// Check for completed tasks in any archive.
	has, err := s.store.HasCompletedTasks(slug)
	if err != nil {
		return fmt.Errorf("checking completed tasks: %w", err)
	}
	if has {
		return fmt.Errorf("cannot delete: has archived completed tasks")
	}

	if err := s.store.DeleteTaskList(slug); err != nil {
		return fmt.Errorf("deleting task list: %w", err)
	}

	return nil
}

// ArchiveTaskList archives a task list for the current week.
func (s *Service) ArchiveTaskList(slug string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if assigned to active timeboxes.
	assigned, err := s.store.IsTaskListAssigned(slug)
	if err != nil {
		return fmt.Errorf("checking assignment: %w", err)
	}
	if assigned {
		return fmt.Errorf("cannot archive: list is assigned to active timeboxes")
	}

	wi := domain.WeekForDate(time.Now())

	if err := s.store.ArchiveTaskList(slug, wi.Year, wi.Week); err != nil {
		return fmt.Errorf("archiving task list: %w", err)
	}

	return nil
}

// ListTaskLists returns all active task lists, sorted alphabetically by name.
func (s *Service) ListTaskLists() ([]*domain.TaskList, error) {
	return s.store.ListTaskLists()
}

// GetTaskList reads a single task list by slug.
func (s *Service) GetTaskList(slug string) (*domain.TaskList, error) {
	return s.store.ReadTaskList(slug)
}
