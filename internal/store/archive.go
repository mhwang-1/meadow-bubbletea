package store

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mhwang-1/meadow-bubbletea/internal/domain"
)

// CompletedTask pairs a task with its completion date and originating task list.
type CompletedTask struct {
	Task          domain.Task
	CompletedDate time.Time
	TaskListSlug  string
}

// ArchivedTimebox is a timebox record stored in the weekly archive.
type ArchivedTimebox struct {
	Date           time.Time
	Start          time.Time
	End            time.Time
	TaskListSlug   string
	Tag            string
	Note           string
	CompletedTasks []domain.Task
}

// ArchiveDir returns the archive directory path for the given year and week.
func (s *Store) ArchiveDir(year, week int) string {
	return fmt.Sprintf("%s/archive/%d/%02d", s.DataDir, year, week)
}

// ReadCompleted reads the completed.md file for the given year and week,
// returning the list of completed tasks. Returns an empty slice (not an error)
// if the file does not exist.
func (s *Store) ReadCompleted(year, week int) ([]CompletedTask, error) {
	path := filepath.Join(s.ArchiveDir(year, week), "completed.md")

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []CompletedTask{}, nil
		}
		return nil, fmt.Errorf("reading completed file: %w", err)
	}

	return parseCompleted(string(data))
}

// parseCompleted parses the contents of a completed.md file.
func parseCompleted(content string) ([]CompletedTask, error) {
	var tasks []CompletedTask
	var currentSlug string

	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// Section header: ## {slug}
		if strings.HasPrefix(trimmed, "## ") {
			currentSlug = strings.TrimSpace(strings.TrimPrefix(trimmed, "## "))
			continue
		}

		if currentSlug == "" {
			continue
		}

		// Task line: {description} ~{duration} | {YYYY-MM-DD}
		pipeIdx := strings.LastIndex(trimmed, " | ")
		if pipeIdx < 0 {
			return nil, fmt.Errorf("invalid completed task line (no date separator): %q", trimmed)
		}

		taskPart := trimmed[:pipeIdx]
		datePart := strings.TrimSpace(trimmed[pipeIdx+3:])

		task, err := domain.ParseTaskLine(taskPart)
		if err != nil {
			return nil, fmt.Errorf("parsing completed task: %w", err)
		}

		completedDate, err := time.Parse("2006-01-02", datePart)
		if err != nil {
			return nil, fmt.Errorf("parsing completed date %q: %w", datePart, err)
		}

		tasks = append(tasks, CompletedTask{
			Task:          task,
			CompletedDate: completedDate,
			TaskListSlug:  currentSlug,
		})
	}

	return tasks, nil
}

// WriteCompleted writes the completed.md file for the given year and week,
// grouping tasks by TaskListSlug in alphabetical order.
func (s *Store) WriteCompleted(year, week int, tasks []CompletedTask) error {
	dir := s.ArchiveDir(year, week)
	path := filepath.Join(dir, "completed.md")

	return WithLock(path, func() error {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("creating archive directory: %w", err)
		}

		content := formatCompleted(tasks)

		tmp, err := os.CreateTemp(dir, ".completed-*.md.tmp")
		if err != nil {
			return fmt.Errorf("creating temp file: %w", err)
		}
		tmpPath := tmp.Name()

		if _, err := tmp.WriteString(content); err != nil {
			tmp.Close()
			os.Remove(tmpPath)
			return fmt.Errorf("writing temp file: %w", err)
		}

		if err := tmp.Close(); err != nil {
			os.Remove(tmpPath)
			return fmt.Errorf("closing temp file: %w", err)
		}

		if err := os.Rename(tmpPath, path); err != nil {
			os.Remove(tmpPath)
			return fmt.Errorf("renaming temp file: %w", err)
		}

		return nil
	})
}

// formatCompleted formats completed tasks into the completed.md format,
// grouped by TaskListSlug in alphabetical order.
func formatCompleted(tasks []CompletedTask) string {
	// Group by slug.
	groups := make(map[string][]CompletedTask)
	for _, t := range tasks {
		groups[t.TaskListSlug] = append(groups[t.TaskListSlug], t)
	}

	// Sort slugs alphabetically.
	slugs := make([]string, 0, len(groups))
	for slug := range groups {
		slugs = append(slugs, slug)
	}
	sort.Strings(slugs)

	var b strings.Builder
	for i, slug := range slugs {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString("## " + slug + "\n")
		for _, ct := range groups[slug] {
			b.WriteString(fmt.Sprintf("%s ~%s | %s\n",
				ct.Task.Description,
				domain.FormatDuration(ct.Task.Duration),
				ct.CompletedDate.Format("2006-01-02"),
			))
		}
	}

	return b.String()
}

// AddCompleted reads the existing completed tasks for the given week, appends
// the new task, and writes them back.
func (s *Store) AddCompleted(year, week int, task CompletedTask) error {
	dir := s.ArchiveDir(year, week)
	path := filepath.Join(dir, "completed.md")

	return WithLock(path, func() error {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("creating archive directory: %w", err)
		}

		var existing []CompletedTask
		data, err := os.ReadFile(path)
		if err != nil {
			if !os.IsNotExist(err) {
				return fmt.Errorf("reading completed file: %w", err)
			}
			// File doesn't exist yet; start with empty slice.
		} else {
			existing, err = parseCompleted(string(data))
			if err != nil {
				return fmt.Errorf("parsing completed file: %w", err)
			}
		}

		// Avoid duplicate completed records for the same task/date/list.
		targetDate := domain.StripTimeForDate(task.CompletedDate)
		for _, ct := range existing {
			if ct.TaskListSlug == task.TaskListSlug &&
				domain.StripTimeForDate(ct.CompletedDate).Equal(targetDate) &&
				ct.Task.Description == task.Task.Description &&
				ct.Task.Duration == task.Task.Duration {
				return nil
			}
		}

		existing = append(existing, task)
		content := formatCompleted(existing)

		tmp, err := os.CreateTemp(dir, ".completed-*.md.tmp")
		if err != nil {
			return fmt.Errorf("creating temp file: %w", err)
		}
		tmpPath := tmp.Name()

		if _, err := tmp.WriteString(content); err != nil {
			tmp.Close()
			os.Remove(tmpPath)
			return fmt.Errorf("writing temp file: %w", err)
		}

		if err := tmp.Close(); err != nil {
			os.Remove(tmpPath)
			return fmt.Errorf("closing temp file: %w", err)
		}

		if err := os.Rename(tmpPath, path); err != nil {
			os.Remove(tmpPath)
			return fmt.Errorf("renaming temp file: %w", err)
		}

		return nil
	})
}

// ReadArchivedTimeboxes reads the timeboxes.md file for the given year and
// week. Returns an empty slice (not an error) if the file does not exist.
func (s *Store) ReadArchivedTimeboxes(year, week int) ([]ArchivedTimebox, error) {
	path := filepath.Join(s.ArchiveDir(year, week), "timeboxes.md")

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []ArchivedTimebox{}, nil
		}
		return nil, fmt.Errorf("reading timeboxes file: %w", err)
	}

	return parseArchivedTimeboxes(string(data))
}

// parseArchivedTimeboxes parses the contents of a timeboxes.md file.
func parseArchivedTimeboxes(content string) ([]ArchivedTimebox, error) {
	var timeboxes []ArchivedTimebox
	var currentDate time.Time
	var currentTimebox *ArchivedTimebox
	inCompleted := false

	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)

		// Date header: ## YYYY-MM-DD
		if strings.HasPrefix(trimmed, "## ") && !strings.HasPrefix(trimmed, "### ") {
			dateStr := strings.TrimSpace(strings.TrimPrefix(trimmed, "## "))
			d, err := time.Parse("2006-01-02", dateStr)
			if err != nil {
				return nil, fmt.Errorf("parsing date header %q: %w", dateStr, err)
			}
			currentDate = d
			// Flush any in-progress timebox.
			if currentTimebox != nil {
				timeboxes = append(timeboxes, *currentTimebox)
				currentTimebox = nil
			}
			inCompleted = false
			continue
		}

		// Time range header: ### HH:MM-HH:MM
		if strings.HasPrefix(trimmed, "### ") {
			// Flush any in-progress timebox.
			if currentTimebox != nil {
				timeboxes = append(timeboxes, *currentTimebox)
			}
			inCompleted = false

			timeRange := strings.TrimSpace(strings.TrimPrefix(trimmed, "### "))
			parts := strings.SplitN(timeRange, "-", 2)
			if len(parts) != 2 {
				return nil, fmt.Errorf("invalid time range: %q", timeRange)
			}

			startTime, err := parseTimeOnDate(currentDate, strings.TrimSpace(parts[0]))
			if err != nil {
				return nil, fmt.Errorf("parsing start time %q: %w", parts[0], err)
			}
			endTime, err := parseTimeOnDate(currentDate, strings.TrimSpace(parts[1]))
			if err != nil {
				return nil, fmt.Errorf("parsing end time %q: %w", parts[1], err)
			}

			currentTimebox = &ArchivedTimebox{
				Date:  currentDate,
				Start: startTime,
				End:   endTime,
			}
			continue
		}

		if currentTimebox == nil {
			continue
		}

		// Metadata lines inside a timebox.
		if strings.HasPrefix(trimmed, "tasklist:") {
			currentTimebox.TaskListSlug = strings.TrimSpace(strings.TrimPrefix(trimmed, "tasklist:"))
			continue
		}

		if strings.HasPrefix(trimmed, "tag:") {
			currentTimebox.Tag = strings.TrimSpace(strings.TrimPrefix(trimmed, "tag:"))
			continue
		}

		if strings.HasPrefix(trimmed, "note:") {
			currentTimebox.Note = strings.TrimSpace(strings.TrimPrefix(trimmed, "note:"))
			continue
		}

		if trimmed == "completed:" {
			inCompleted = true
			continue
		}

		// Completed task line: "  - {description} ~{duration}"
		if inCompleted && strings.HasPrefix(trimmed, "- ") {
			taskLine := strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
			task, err := domain.ParseTaskLine(taskLine)
			if err != nil {
				return nil, fmt.Errorf("parsing archived task: %w", err)
			}
			currentTimebox.CompletedTasks = append(currentTimebox.CompletedTasks, task)
			continue
		}

		if trimmed == "" {
			continue
		}
	}

	// Flush the last timebox.
	if currentTimebox != nil {
		timeboxes = append(timeboxes, *currentTimebox)
	}

	return timeboxes, nil
}

// parseTimeOnDate parses a time string like "14:00" and combines it with a date.
func parseTimeOnDate(date time.Time, timeStr string) (time.Time, error) {
	t, err := time.Parse("15:04", timeStr)
	if err != nil {
		return time.Time{}, err
	}
	return time.Date(
		date.Year(), date.Month(), date.Day(),
		t.Hour(), t.Minute(), 0, 0,
		date.Location(),
	), nil
}

// WriteArchivedTimeboxes writes the timeboxes.md file for the given year and week.
func (s *Store) WriteArchivedTimeboxes(year, week int, timeboxes []ArchivedTimebox) error {
	dir := s.ArchiveDir(year, week)
	path := filepath.Join(dir, "timeboxes.md")

	return WithLock(path, func() error {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("creating archive directory: %w", err)
		}

		content := formatArchivedTimeboxes(timeboxes)

		tmp, err := os.CreateTemp(dir, ".timeboxes-*.md.tmp")
		if err != nil {
			return fmt.Errorf("creating temp file: %w", err)
		}
		tmpPath := tmp.Name()

		if _, err := tmp.WriteString(content); err != nil {
			tmp.Close()
			os.Remove(tmpPath)
			return fmt.Errorf("writing temp file: %w", err)
		}

		if err := tmp.Close(); err != nil {
			os.Remove(tmpPath)
			return fmt.Errorf("closing temp file: %w", err)
		}

		if err := os.Rename(tmpPath, path); err != nil {
			os.Remove(tmpPath)
			return fmt.Errorf("renaming temp file: %w", err)
		}

		return nil
	})
}

// formatArchivedTimeboxes formats archived timeboxes into the timeboxes.md format.
func formatArchivedTimeboxes(timeboxes []ArchivedTimebox) string {
	// Group timeboxes by date.
	type dateGroup struct {
		date      time.Time
		timeboxes []ArchivedTimebox
	}

	dateMap := make(map[string]*dateGroup)
	var dateOrder []string

	for _, tb := range timeboxes {
		key := tb.Date.Format("2006-01-02")
		if _, ok := dateMap[key]; !ok {
			dateMap[key] = &dateGroup{date: tb.Date}
			dateOrder = append(dateOrder, key)
		}
		dateMap[key].timeboxes = append(dateMap[key].timeboxes, tb)
	}

	// Sort dates chronologically.
	sort.Strings(dateOrder)

	var b strings.Builder
	for i, key := range dateOrder {
		if i > 0 {
			b.WriteString("\n")
		}
		group := dateMap[key]
		b.WriteString("## " + key + "\n")

		for _, tb := range group.timeboxes {
			b.WriteString(fmt.Sprintf("### %s-%s\n",
				tb.Start.Format("15:04"),
				tb.End.Format("15:04"),
			))
			if tb.TaskListSlug != "" {
				b.WriteString("tasklist: " + tb.TaskListSlug + "\n")
			}
			if tb.Tag != "" {
				b.WriteString(fmt.Sprintf("tag: %s\n", tb.Tag))
			}
			if tb.Note != "" {
				b.WriteString(fmt.Sprintf("note: %s\n", tb.Note))
			}
			if len(tb.CompletedTasks) > 0 {
				b.WriteString("completed:\n")
				for _, task := range tb.CompletedTasks {
					b.WriteString(fmt.Sprintf("  - %s ~%s\n",
						task.Description,
						domain.FormatDuration(task.Duration),
					))
				}
			}
		}
	}

	return b.String()
}

// CompletedDurationsBySlug reads completed.md for the given year/week and
// returns the total duration of completed tasks grouped by task list slug.
// Returns an empty map (not an error) if the file does not exist.
func (s *Store) CompletedDurationsBySlug(year, week int) (map[string]time.Duration, error) {
	tasks, err := s.ReadCompleted(year, week)
	if err != nil {
		return nil, err
	}

	result := make(map[string]time.Duration)
	for _, ct := range tasks {
		result[ct.TaskListSlug] += ct.Task.Duration
	}
	return result, nil
}
