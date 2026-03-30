package domain

import (
	"testing"
	"time"
)

func TestParseDuration(t *testing.T) {
	valid := []struct {
		input    string
		expected time.Duration
	}{
		{"~24m", 24 * time.Minute},
		{"~1h30m", 90 * time.Minute},
		{"~2h", 2 * time.Hour},
	}

	for _, tc := range valid {
		t.Run("valid_"+tc.input, func(t *testing.T) {
			got, err := ParseDuration(tc.input)
			if err != nil {
				t.Fatalf("ParseDuration(%q) unexpected error: %v", tc.input, err)
			}
			if got != tc.expected {
				t.Errorf("ParseDuration(%q) = %v, want %v", tc.input, got, tc.expected)
			}
		})
	}

	invalid := []struct {
		name  string
		input string
	}{
		{"no_prefix", "24m"},
		{"empty", ""},
		{"tilde_only", "~"},
		{"tilde_abc", "~abc"},
	}

	for _, tc := range invalid {
		t.Run("invalid_"+tc.name, func(t *testing.T) {
			_, err := ParseDuration(tc.input)
			if err == nil {
				t.Errorf("ParseDuration(%q) expected error, got nil", tc.input)
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		input    time.Duration
		expected string
	}{
		{24 * time.Minute, "24m"},
		{120 * time.Minute, "2h"},
		{90 * time.Minute, "1h30m"},
	}

	for _, tc := range tests {
		t.Run(tc.expected, func(t *testing.T) {
			got := FormatDuration(tc.input)
			if got != tc.expected {
				t.Errorf("FormatDuration(%v) = %q, want %q", tc.input, got, tc.expected)
			}
		})
	}
}

func TestParseTaskLine(t *testing.T) {
	t.Run("normal_line", func(t *testing.T) {
		task, err := ParseTaskLine("Visit Report - Create maps ~24m")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if task.Description != "Visit Report - Create maps" {
			t.Errorf("Description = %q, want %q", task.Description, "Visit Report - Create maps")
		}
		if task.Duration != 24*time.Minute {
			t.Errorf("Duration = %v, want %v", task.Duration, 24*time.Minute)
		}
		if task.Commented {
			t.Error("Commented = true, want false")
		}
	})

	t.Run("commented_line", func(t *testing.T) {
		task, err := ParseTaskLine("# Blossom - Call John ~24m")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if task.Description != "Blossom - Call John" {
			t.Errorf("Description = %q, want %q", task.Description, "Blossom - Call John")
		}
		if task.Duration != 24*time.Minute {
			t.Errorf("Duration = %v, want %v", task.Duration, 24*time.Minute)
		}
		if !task.Commented {
			t.Error("Commented = false, want true")
		}
	})

	t.Run("no_duration", func(t *testing.T) {
		_, err := ParseTaskLine("Some task without duration")
		if err == nil {
			t.Error("expected error for line with no duration, got nil")
		}
	})
}

func TestFormatTaskLine(t *testing.T) {
	t.Run("normal_task", func(t *testing.T) {
		task := Task{
			Description: "Visit Report - Create maps",
			Duration:    24 * time.Minute,
			Commented:   false,
		}
		got := FormatTaskLine(task)
		expected := "Visit Report - Create maps ~24m"
		if got != expected {
			t.Errorf("FormatTaskLine() = %q, want %q", got, expected)
		}
	})

	t.Run("commented_task", func(t *testing.T) {
		task := Task{
			Description: "Blossom - Call John",
			Duration:    24 * time.Minute,
			Commented:   true,
		}
		got := FormatTaskLine(task)
		expected := "# Blossom - Call John ~24m"
		if got != expected {
			t.Errorf("FormatTaskLine() = %q, want %q", got, expected)
		}
	})
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Work 03/2026", "work-03-2026"},
		{"Journal 12/2025", "journal-12-2025"},
		{"Gitea", "gitea"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := Slugify(tc.input)
			if got != tc.expected {
				t.Errorf("Slugify(%q) = %q, want %q", tc.input, got, tc.expected)
			}
		})
	}
}

func TestTaskListTotalDuration(t *testing.T) {
	tl := &TaskList{
		Name: "Test",
		Slug: "test",
		Tasks: []Task{
			{Description: "Task A", Duration: 30 * time.Minute, Commented: false},
			{Description: "Task B", Duration: 20 * time.Minute, Commented: true},
			{Description: "Task C", Duration: 10 * time.Minute, Commented: false},
		},
	}

	got := tl.TotalDuration()
	expected := 40 * time.Minute
	if got != expected {
		t.Errorf("TotalDuration() = %v, want %v", got, expected)
	}
}

func TestTaskListActiveTasks(t *testing.T) {
	tl := &TaskList{
		Name: "Test",
		Slug: "test",
		Tasks: []Task{
			{Description: "Task A", Duration: 30 * time.Minute, Commented: false},
			{Description: "Task B", Duration: 20 * time.Minute, Commented: true},
			{Description: "Task C", Duration: 10 * time.Minute, Commented: false},
		},
	}

	active := tl.ActiveTasks()
	if len(active) != 2 {
		t.Fatalf("ActiveTasks() returned %d tasks, want 2", len(active))
	}
	if active[0].Description != "Task A" {
		t.Errorf("active[0].Description = %q, want %q", active[0].Description, "Task A")
	}
	if active[1].Description != "Task C" {
		t.Errorf("active[1].Description = %q, want %q", active[1].Description, "Task C")
	}
}
