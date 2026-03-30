package domain

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"
)

// Task represents a single task entry in a task list.
type Task struct {
	Description string
	Duration    time.Duration
	Commented   bool
}

// TaskList represents a named collection of tasks.
type TaskList struct {
	Name  string // display name, e.g. "Work 03/2026"
	Slug  string // filename slug, e.g. "work-03-2026"
	Tasks []Task
}

// durationRe matches duration strings like ~24m, ~1h, ~1h30m.
var durationRe = regexp.MustCompile(`^~(\d+h)?(\d+m)?$`)

// ParseDuration parses a duration string with a required ~ prefix.
// Supported formats: ~{N}m, ~{N}h, ~{N}h{M}m.
func ParseDuration(s string) (time.Duration, error) {
	if !strings.HasPrefix(s, "~") {
		return 0, fmt.Errorf("duration must start with ~: %q", s)
	}

	if !durationRe.MatchString(s) {
		return 0, fmt.Errorf("invalid duration format: %q (expected ~Nm, ~Nh, or ~NhMm)", s)
	}

	// Strip the ~ and parse with time.ParseDuration.
	raw := s[1:]
	if raw == "" {
		return 0, fmt.Errorf("empty duration after ~: %q", s)
	}

	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q: %w", s, err)
	}

	return d, nil
}

// FormatDuration formats a duration for display without the ~ prefix.
// Returns formats like "24m", "2h", or "1h30m".
func FormatDuration(d time.Duration) string {
	totalMinutes := int(d.Minutes())
	hours := totalMinutes / 60
	minutes := totalMinutes % 60

	switch {
	case hours == 0:
		return fmt.Sprintf("%dm", minutes)
	case minutes == 0:
		return fmt.Sprintf("%dh", hours)
	default:
		return fmt.Sprintf("%dh%dm", hours, minutes)
	}
}

// ParseTaskLine parses a single task line into a Task.
// Lines look like "Visit Report - Create maps ~24m" or "# Blossom - Call John ~24m".
func ParseTaskLine(line string) (Task, error) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return Task{}, fmt.Errorf("empty task line")
	}

	commented := false
	working := trimmed

	// Check if the line is commented out.
	if strings.HasPrefix(working, "#") {
		commented = true
		working = strings.TrimSpace(strings.TrimPrefix(working, "#"))
	}

	// Find the last occurrence of " ~" to split description from duration.
	idx := strings.LastIndex(working, " ~")
	if idx < 0 {
		return Task{}, fmt.Errorf("no duration found in task line: %q", line)
	}

	description := working[:idx]
	durationStr := working[idx+1:] // includes the ~

	dur, err := ParseDuration(durationStr)
	if err != nil {
		return Task{}, fmt.Errorf("invalid duration in task line %q: %w", line, err)
	}

	if description == "" {
		return Task{}, fmt.Errorf("empty description in task line: %q", line)
	}

	return Task{
		Description: description,
		Duration:    dur,
		Commented:   commented,
	}, nil
}

// FormatTaskLine formats a Task back into a line string.
// Commented tasks are prefixed with "# ".
func FormatTaskLine(t Task) string {
	line := fmt.Sprintf("%s ~%s", t.Description, FormatDuration(t.Duration))
	if t.Commented {
		line = "# " + line
	}
	return line
}

// Slugify converts a display name to a filename-safe slug.
// It lowercases the string, replaces spaces with hyphens, and removes
// characters that are not alphanumeric, hyphens, or periods.
func Slugify(name string) string {
	s := strings.ToLower(name)
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "/", "-")

	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '.' {
			b.WriteRune(r)
		}
	}

	// Collapse multiple hyphens.
	result := b.String()
	for strings.Contains(result, "--") {
		result = strings.ReplaceAll(result, "--", "-")
	}
	return strings.Trim(result, "-")
}

// TotalDuration returns the sum of durations for all non-commented tasks.
func (tl *TaskList) TotalDuration() time.Duration {
	var total time.Duration
	for _, t := range tl.Tasks {
		if !t.Commented {
			total += t.Duration
		}
	}
	return total
}

// ActiveTasks returns all tasks that are not commented out.
func (tl *TaskList) ActiveTasks() []Task {
	var active []Task
	for _, t := range tl.Tasks {
		if !t.Commented {
			active = append(active, t)
		}
	}
	return active
}
