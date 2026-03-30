package store

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mhwang-1/meadow-bubbletea/internal/domain"
)

// TimeboxDir returns the directory where daily timebox files are stored.
func (s *Store) TimeboxDir() string {
	return s.DataDir + "/active/timeboxes"
}

// ReadDailyTimeboxes reads the timebox file for a given date and returns
// the parsed DailyTimeboxes. Returns an empty DailyTimeboxes (not an error)
// if the file does not exist.
func (s *Store) ReadDailyTimeboxes(date time.Time) (*domain.DailyTimeboxes, error) {
	filename := domain.DayFilename(date) + ".md"
	path := filepath.Join(s.TimeboxDir(), filename)

	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &domain.DailyTimeboxes{
				Date: date,
			}, nil
		}
		return nil, err
	}
	defer f.Close()

	dt := &domain.DailyTimeboxes{}
	scanner := bufio.NewScanner(f)

	// Parse YAML frontmatter.
	if !scanner.Scan() {
		return dt, nil
	}
	if strings.TrimSpace(scanner.Text()) != "---" {
		return nil, fmt.Errorf("expected frontmatter opening '---', got %q", scanner.Text())
	}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "---" {
			break
		}
		if strings.HasPrefix(line, "date:") {
			dateStr := strings.TrimSpace(strings.TrimPrefix(line, "date:"))
			parsed, err := time.Parse("2006-01-02", dateStr)
			if err != nil {
				return nil, fmt.Errorf("invalid date in frontmatter: %w", err)
			}
			dt.Date = parsed
		}
	}

	// Parse timebox sections.
	var current *domain.Timebox
	inCompleted := false

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "## ") {
			// Save any previous timebox.
			if current != nil {
				dt.Timeboxes = append(dt.Timeboxes, *current)
			}
			inCompleted = false

			// Parse time range from header: ## HH:MM-HH:MM
			timeRange := strings.TrimPrefix(trimmed, "## ")
			start, end, err := parseTimeRange(timeRange, dt.Date)
			if err != nil {
				return nil, fmt.Errorf("invalid timebox header %q: %w", trimmed, err)
			}

			current = &domain.Timebox{
				Start:  start,
				End:    end,
				Status: domain.StatusUnassigned,
			}
			continue
		}

		if current == nil {
			continue
		}

		if strings.HasPrefix(trimmed, "tasklist:") {
			current.TaskListSlug = strings.TrimSpace(strings.TrimPrefix(trimmed, "tasklist:"))
			continue
		}

		if strings.HasPrefix(trimmed, "status:") {
			current.Status = domain.TimeboxStatus(strings.TrimSpace(strings.TrimPrefix(trimmed, "status:")))
			continue
		}

		if strings.HasPrefix(trimmed, "tag:") {
			current.Tag = strings.TrimSpace(strings.TrimPrefix(trimmed, "tag:"))
			continue
		}

		if strings.HasPrefix(trimmed, "note:") {
			current.Note = strings.TrimSpace(strings.TrimPrefix(trimmed, "note:"))
			continue
		}

		if trimmed == "completed:" {
			inCompleted = true
			continue
		}

		if inCompleted && strings.HasPrefix(trimmed, "- ") {
			taskLine := strings.TrimPrefix(trimmed, "- ")
			task, err := domain.ParseTaskLine(taskLine)
			if err != nil {
				return nil, fmt.Errorf("invalid completed task %q: %w", taskLine, err)
			}
			current.CompletedTasks = append(current.CompletedTasks, task)
			continue
		}

		// A non-indented, non-empty line that isn't a known field ends the completed block.
		if trimmed != "" {
			inCompleted = false
		}
	}

	// Save the last timebox.
	if current != nil {
		dt.Timeboxes = append(dt.Timeboxes, *current)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return dt, nil
}

// WriteDailyTimeboxes writes the timebox file atomically under a file lock.
func (s *Store) WriteDailyTimeboxes(dt *domain.DailyTimeboxes) error {
	filename := domain.DayFilename(dt.Date) + ".md"
	path := filepath.Join(s.TimeboxDir(), filename)

	return WithLock(path, func() error {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}

		content := formatDailyTimeboxes(dt)

		tmpFile := path + ".tmp"
		if err := os.WriteFile(tmpFile, []byte(content), 0o644); err != nil {
			return err
		}

		return os.Rename(tmpFile, path)
	})
}

// formatDailyTimeboxes renders a DailyTimeboxes to the markdown file format.
func formatDailyTimeboxes(dt *domain.DailyTimeboxes) string {
	var b strings.Builder

	// Frontmatter.
	b.WriteString("---\n")
	b.WriteString(fmt.Sprintf("date: %s\n", dt.Date.Format("2006-01-02")))
	b.WriteString("---\n")

	for _, tb := range dt.Timeboxes {
		b.WriteString(fmt.Sprintf("\n## %s-%s\n",
			tb.Start.Format("15:04"),
			tb.End.Format("15:04"),
		))

		if tb.TaskListSlug != "" {
			b.WriteString(fmt.Sprintf("tasklist: %s\n", tb.TaskListSlug))
		}

		b.WriteString(fmt.Sprintf("status: %s\n", tb.Status))

		if tb.Tag != "" {
			b.WriteString(fmt.Sprintf("tag: %s\n", tb.Tag))
		}
		if tb.Note != "" {
			b.WriteString(fmt.Sprintf("note: %s\n", tb.Note))
		}

		if len(tb.CompletedTasks) > 0 {
			b.WriteString("completed:\n")
			for _, task := range tb.CompletedTasks {
				b.WriteString(fmt.Sprintf("  - %s\n", domain.FormatTaskLine(task)))
			}
		}
	}

	return b.String()
}

// parseTimeRange parses a string like "09:00-11:00" and combines it with
// the given date to produce full time.Time values for start and end.
func parseTimeRange(s string, date time.Time) (time.Time, time.Time, error) {
	parts := strings.SplitN(s, "-", 2)
	if len(parts) != 2 {
		return time.Time{}, time.Time{}, fmt.Errorf("expected HH:MM-HH:MM, got %q", s)
	}

	startHour, startMin, err := parseHHMM(parts[0])
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid start time: %w", err)
	}

	endHour, endMin, err := parseHHMM(parts[1])
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid end time: %w", err)
	}

	y, m, d := date.Date()
	loc := date.Location()
	start := time.Date(y, m, d, startHour, startMin, 0, 0, loc)
	end := time.Date(y, m, d, endHour, endMin, 0, 0, loc)

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
