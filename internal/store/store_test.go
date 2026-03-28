package store

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/hwang/meadow-bubbletea/internal/domain"
)

func TestTaskListRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	s := NewStore(tmpDir)

	original := &domain.TaskList{
		Name: "Work 03/2026",
		Slug: "work-03-2026",
		Tasks: []domain.Task{
			{Description: "Review pull requests", Duration: 30 * time.Minute, Commented: false},
			{Description: "Write documentation", Duration: 1 * time.Hour, Commented: false},
			{Description: "Old meeting notes", Duration: 15 * time.Minute, Commented: true},
		},
	}

	if err := s.WriteTaskList(original); err != nil {
		t.Fatalf("WriteTaskList: %v", err)
	}

	got, err := s.ReadTaskList("work-03-2026")
	if err != nil {
		t.Fatalf("ReadTaskList: %v", err)
	}

	if got.Name != original.Name {
		t.Errorf("Name: got %q, want %q", got.Name, original.Name)
	}
	if got.Slug != original.Slug {
		t.Errorf("Slug: got %q, want %q", got.Slug, original.Slug)
	}
	if !reflect.DeepEqual(got.Tasks, original.Tasks) {
		t.Errorf("Tasks mismatch:\n  got:  %+v\n  want: %+v", got.Tasks, original.Tasks)
	}

	// Verify ListTaskLists returns it.
	lists, err := s.ListTaskLists()
	if err != nil {
		t.Fatalf("ListTaskLists: %v", err)
	}
	if len(lists) != 1 {
		t.Fatalf("ListTaskLists: got %d lists, want 1", len(lists))
	}
	if lists[0].Slug != "work-03-2026" {
		t.Errorf("ListTaskLists[0].Slug: got %q, want %q", lists[0].Slug, "work-03-2026")
	}
	if !reflect.DeepEqual(lists[0].Tasks, original.Tasks) {
		t.Errorf("ListTaskLists[0].Tasks mismatch:\n  got:  %+v\n  want: %+v", lists[0].Tasks, original.Tasks)
	}
}

func TestDailyTimeboxesRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	s := NewStore(tmpDir)

	date := time.Date(2026, 3, 27, 0, 0, 0, 0, time.UTC)

	original := &domain.DailyTimeboxes{
		Date: date,
		Timeboxes: []domain.Timebox{
			{
				Start:        time.Date(2026, 3, 27, 9, 0, 0, 0, time.UTC),
				End:          time.Date(2026, 3, 27, 11, 0, 0, 0, time.UTC),
				TaskListSlug: "work-03-2026",
				Status:       domain.StatusActive,
			},
			{
				Start:        time.Date(2026, 3, 27, 14, 0, 0, 0, time.UTC),
				End:          time.Date(2026, 3, 27, 15, 30, 0, 0, time.UTC),
				TaskListSlug: "journal-12-2025",
				Status:       domain.StatusArchived,
				CompletedTasks: []domain.Task{
					{Description: "Write morning entry", Duration: 30 * time.Minute, Commented: false},
					{Description: "Review weekly goals", Duration: 20 * time.Minute, Commented: false},
				},
			},
		},
	}

	if err := s.WriteDailyTimeboxes(original); err != nil {
		t.Fatalf("WriteDailyTimeboxes: %v", err)
	}

	got, err := s.ReadDailyTimeboxes(date)
	if err != nil {
		t.Fatalf("ReadDailyTimeboxes: %v", err)
	}

	if !got.Date.Equal(original.Date) {
		t.Errorf("Date: got %v, want %v", got.Date, original.Date)
	}
	if len(got.Timeboxes) != len(original.Timeboxes) {
		t.Fatalf("Timeboxes count: got %d, want %d", len(got.Timeboxes), len(original.Timeboxes))
	}

	for i, wantTB := range original.Timeboxes {
		gotTB := got.Timeboxes[i]

		if !gotTB.Start.Equal(wantTB.Start) {
			t.Errorf("Timebox[%d].Start: got %v, want %v", i, gotTB.Start, wantTB.Start)
		}
		if !gotTB.End.Equal(wantTB.End) {
			t.Errorf("Timebox[%d].End: got %v, want %v", i, gotTB.End, wantTB.End)
		}
		if gotTB.TaskListSlug != wantTB.TaskListSlug {
			t.Errorf("Timebox[%d].TaskListSlug: got %q, want %q", i, gotTB.TaskListSlug, wantTB.TaskListSlug)
		}
		if gotTB.Status != wantTB.Status {
			t.Errorf("Timebox[%d].Status: got %q, want %q", i, gotTB.Status, wantTB.Status)
		}
		if !reflect.DeepEqual(gotTB.CompletedTasks, wantTB.CompletedTasks) {
			t.Errorf("Timebox[%d].CompletedTasks mismatch:\n  got:  %+v\n  want: %+v", i, gotTB.CompletedTasks, wantTB.CompletedTasks)
		}
	}
}

func TestCompletedRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	s := NewStore(tmpDir)

	year, week := 2026, 13

	original := []CompletedTask{
		{
			Task:          domain.Task{Description: "Draft proposal", Duration: 45 * time.Minute},
			CompletedDate: time.Date(2026, 3, 23, 0, 0, 0, 0, time.UTC),
			TaskListSlug:  "work-03-2026",
		},
		{
			Task:          domain.Task{Description: "Send invoices", Duration: 20 * time.Minute},
			CompletedDate: time.Date(2026, 3, 24, 0, 0, 0, 0, time.UTC),
			TaskListSlug:  "work-03-2026",
		},
	}

	if err := s.WriteCompleted(year, week, original); err != nil {
		t.Fatalf("WriteCompleted: %v", err)
	}

	got, err := s.ReadCompleted(year, week)
	if err != nil {
		t.Fatalf("ReadCompleted: %v", err)
	}

	if !reflect.DeepEqual(got, original) {
		t.Errorf("ReadCompleted mismatch:\n  got:  %+v\n  want: %+v", got, original)
	}

	// Test AddCompleted appends correctly.
	extra := CompletedTask{
		Task:          domain.Task{Description: "Update README", Duration: 15 * time.Minute},
		CompletedDate: time.Date(2026, 3, 25, 0, 0, 0, 0, time.UTC),
		TaskListSlug:  "docs-03-2026",
	}

	if err := s.AddCompleted(year, week, extra); err != nil {
		t.Fatalf("AddCompleted: %v", err)
	}

	got2, err := s.ReadCompleted(year, week)
	if err != nil {
		t.Fatalf("ReadCompleted after AddCompleted: %v", err)
	}

	// AddCompleted + WriteCompleted regroups by slug alphabetically, so the
	// order may differ from simple append. Build the expected set.
	expected := append(original, extra)

	if len(got2) != len(expected) {
		t.Fatalf("ReadCompleted after AddCompleted: got %d tasks, want %d", len(got2), len(expected))
	}

	// Build maps keyed by description for order-independent comparison.
	gotMap := make(map[string]CompletedTask)
	for _, ct := range got2 {
		gotMap[ct.Task.Description] = ct
	}
	for _, want := range expected {
		g, ok := gotMap[want.Task.Description]
		if !ok {
			t.Errorf("missing task %q after AddCompleted", want.Task.Description)
			continue
		}
		if g.Task.Duration != want.Task.Duration {
			t.Errorf("task %q duration: got %v, want %v", want.Task.Description, g.Task.Duration, want.Task.Duration)
		}
		if !g.CompletedDate.Equal(want.CompletedDate) {
			t.Errorf("task %q CompletedDate: got %v, want %v", want.Task.Description, g.CompletedDate, want.CompletedDate)
		}
		if g.TaskListSlug != want.TaskListSlug {
			t.Errorf("task %q TaskListSlug: got %q, want %q", want.Task.Description, g.TaskListSlug, want.TaskListSlug)
		}
	}
}

func TestArchivedTimeboxesRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	s := NewStore(tmpDir)

	year, week := 2026, 13
	date := time.Date(2026, 3, 27, 0, 0, 0, 0, time.UTC)

	original := []ArchivedTimebox{
		{
			Date:         date,
			Start:        time.Date(2026, 3, 27, 9, 0, 0, 0, time.UTC),
			End:          time.Date(2026, 3, 27, 11, 0, 0, 0, time.UTC),
			TaskListSlug: "work-03-2026",
			CompletedTasks: []domain.Task{
				{Description: "Code review", Duration: 30 * time.Minute},
				{Description: "Fix CI pipeline", Duration: 45 * time.Minute},
			},
		},
		{
			Date:         date,
			Start:        time.Date(2026, 3, 27, 14, 0, 0, 0, time.UTC),
			End:          time.Date(2026, 3, 27, 16, 0, 0, 0, time.UTC),
			TaskListSlug: "personal-03-2026",
		},
	}

	if err := s.WriteArchivedTimeboxes(year, week, original); err != nil {
		t.Fatalf("WriteArchivedTimeboxes: %v", err)
	}

	got, err := s.ReadArchivedTimeboxes(year, week)
	if err != nil {
		t.Fatalf("ReadArchivedTimeboxes: %v", err)
	}

	if len(got) != len(original) {
		t.Fatalf("ArchivedTimeboxes count: got %d, want %d", len(got), len(original))
	}

	for i, want := range original {
		g := got[i]
		if !g.Date.Equal(want.Date) {
			t.Errorf("[%d].Date: got %v, want %v", i, g.Date, want.Date)
		}
		if !g.Start.Equal(want.Start) {
			t.Errorf("[%d].Start: got %v, want %v", i, g.Start, want.Start)
		}
		if !g.End.Equal(want.End) {
			t.Errorf("[%d].End: got %v, want %v", i, g.End, want.End)
		}
		if g.TaskListSlug != want.TaskListSlug {
			t.Errorf("[%d].TaskListSlug: got %q, want %q", i, g.TaskListSlug, want.TaskListSlug)
		}
		if !reflect.DeepEqual(g.CompletedTasks, want.CompletedTasks) {
			t.Errorf("[%d].CompletedTasks mismatch:\n  got:  %+v\n  want: %+v", i, g.CompletedTasks, want.CompletedTasks)
		}
	}
}

func TestAddCompletedDeduplicates(t *testing.T) {
	tmpDir := t.TempDir()
	s := NewStore(tmpDir)

	year, week := 2026, 13
	task := CompletedTask{
		Task:          domain.Task{Description: "Draft proposal", Duration: 45 * time.Minute},
		CompletedDate: time.Date(2026, 3, 23, 0, 0, 0, 0, time.UTC),
		TaskListSlug:  "work-03-2026",
	}

	if err := s.AddCompleted(year, week, task); err != nil {
		t.Fatalf("AddCompleted first call: %v", err)
	}
	if err := s.AddCompleted(year, week, task); err != nil {
		t.Fatalf("AddCompleted duplicate call: %v", err)
	}

	got, err := s.ReadCompleted(year, week)
	if err != nil {
		t.Fatalf("ReadCompleted: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("ReadCompleted count: got %d tasks, want 1", len(got))
	}
	if got[0].Task.Description != task.Task.Description ||
		got[0].Task.Duration != task.Task.Duration ||
		got[0].TaskListSlug != task.TaskListSlug ||
		!got[0].CompletedDate.Equal(task.CompletedDate) {
		t.Fatalf("ReadCompleted mismatch:\n  got:  %+v\n  want: %+v", got[0], task)
	}
}

func TestReservedTimeboxRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	s := NewStore(tmpDir)

	date := time.Date(2026, 3, 28, 0, 0, 0, 0, time.UTC)

	original := &domain.DailyTimeboxes{
		Date: date,
		Timeboxes: []domain.Timebox{
			{
				Start:  time.Date(2026, 3, 28, 9, 0, 0, 0, time.UTC),
				End:    time.Date(2026, 3, 28, 10, 0, 0, 0, time.UTC),
				Status: domain.StatusActive,
				Tag:    "reserved",
				Note:   "Bring Summer to visit doctor",
			},
			{
				Start:        time.Date(2026, 3, 28, 10, 0, 0, 0, time.UTC),
				End:          time.Date(2026, 3, 28, 11, 0, 0, 0, time.UTC),
				TaskListSlug: "work",
				Status:       domain.StatusActive,
			},
			{
				Start:  time.Date(2026, 3, 28, 14, 0, 0, 0, time.UTC),
				End:    time.Date(2026, 3, 28, 15, 0, 0, 0, time.UTC),
				Status: domain.StatusUnassigned,
			},
		},
	}

	if err := s.WriteDailyTimeboxes(original); err != nil {
		t.Fatalf("WriteDailyTimeboxes: %v", err)
	}

	got, err := s.ReadDailyTimeboxes(date)
	if err != nil {
		t.Fatalf("ReadDailyTimeboxes: %v", err)
	}

	if len(got.Timeboxes) != 3 {
		t.Fatalf("Timeboxes count: got %d, want 3", len(got.Timeboxes))
	}

	// Reserved timebox.
	tb0 := got.Timeboxes[0]
	if tb0.Tag != "reserved" {
		t.Errorf("[0].Tag: got %q, want %q", tb0.Tag, "reserved")
	}
	if tb0.Note != "Bring Summer to visit doctor" {
		t.Errorf("[0].Note: got %q, want %q", tb0.Note, "Bring Summer to visit doctor")
	}
	if !tb0.IsReserved() {
		t.Error("[0].IsReserved() = false, want true")
	}
	if tb0.TaskListSlug != "" {
		t.Errorf("[0].TaskListSlug: got %q, want empty", tb0.TaskListSlug)
	}

	// Normal timebox — tag and note should be zero values.
	tb1 := got.Timeboxes[1]
	if tb1.Tag != "" {
		t.Errorf("[1].Tag: got %q, want empty", tb1.Tag)
	}
	if tb1.Note != "" {
		t.Errorf("[1].Note: got %q, want empty", tb1.Note)
	}
	if tb1.TaskListSlug != "work" {
		t.Errorf("[1].TaskListSlug: got %q, want %q", tb1.TaskListSlug, "work")
	}

	// Unassigned timebox — tag and note should be zero values.
	tb2 := got.Timeboxes[2]
	if tb2.Tag != "" {
		t.Errorf("[2].Tag: got %q, want empty", tb2.Tag)
	}
	if tb2.IsReserved() {
		t.Error("[2].IsReserved() = true, want false")
	}
}

func TestArchivedReservedTimeboxRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	s := NewStore(tmpDir)

	year, week := 2026, 13
	date := time.Date(2026, 3, 28, 0, 0, 0, 0, time.UTC)

	original := []ArchivedTimebox{
		{
			Date:  date,
			Start: time.Date(2026, 3, 28, 12, 0, 0, 0, time.UTC),
			End:   time.Date(2026, 3, 28, 13, 0, 0, 0, time.UTC),
			Tag:   "reserved",
			Note:  "Doctor appointment",
		},
		{
			Date:         date,
			Start:        time.Date(2026, 3, 28, 14, 0, 0, 0, time.UTC),
			End:          time.Date(2026, 3, 28, 16, 0, 0, 0, time.UTC),
			TaskListSlug: "work",
			CompletedTasks: []domain.Task{
				{Description: "Fix bug", Duration: 30 * time.Minute},
			},
		},
	}

	if err := s.WriteArchivedTimeboxes(year, week, original); err != nil {
		t.Fatalf("WriteArchivedTimeboxes: %v", err)
	}

	got, err := s.ReadArchivedTimeboxes(year, week)
	if err != nil {
		t.Fatalf("ReadArchivedTimeboxes: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("count: got %d, want 2", len(got))
	}

	// Reserved archived timebox.
	g0 := got[0]
	if g0.Tag != "reserved" {
		t.Errorf("[0].Tag: got %q, want %q", g0.Tag, "reserved")
	}
	if g0.Note != "Doctor appointment" {
		t.Errorf("[0].Note: got %q, want %q", g0.Note, "Doctor appointment")
	}
	if g0.TaskListSlug != "" {
		t.Errorf("[0].TaskListSlug: got %q, want empty", g0.TaskListSlug)
	}

	// Normal archived timebox.
	g1 := got[1]
	if g1.Tag != "" {
		t.Errorf("[1].Tag: got %q, want empty", g1.Tag)
	}
	if g1.TaskListSlug != "work" {
		t.Errorf("[1].TaskListSlug: got %q, want %q", g1.TaskListSlug, "work")
	}
	if len(g1.CompletedTasks) != 1 {
		t.Fatalf("[1].CompletedTasks: got %d, want 1", len(g1.CompletedTasks))
	}
}

func TestReadMissingFiles(t *testing.T) {
	tmpDir := t.TempDir()
	s := NewStore(tmpDir)

	// ReadDailyTimeboxes for a non-existent date returns empty DailyTimeboxes (no error).
	date := time.Date(2026, 3, 27, 0, 0, 0, 0, time.UTC)
	dt, err := s.ReadDailyTimeboxes(date)
	if err != nil {
		t.Fatalf("ReadDailyTimeboxes for missing file: %v", err)
	}
	if dt == nil {
		t.Fatal("ReadDailyTimeboxes returned nil, want empty DailyTimeboxes")
	}
	if len(dt.Timeboxes) != 0 {
		t.Errorf("ReadDailyTimeboxes: got %d timeboxes, want 0", len(dt.Timeboxes))
	}

	// ReadCompleted for non-existent week returns empty slice (no error).
	completed, err := s.ReadCompleted(2026, 13)
	if err != nil {
		t.Fatalf("ReadCompleted for missing file: %v", err)
	}
	if len(completed) != 0 {
		t.Errorf("ReadCompleted: got %d tasks, want 0", len(completed))
	}

	// ReadArchivedTimeboxes for non-existent week returns empty slice (no error).
	archived, err := s.ReadArchivedTimeboxes(2026, 13)
	if err != nil {
		t.Fatalf("ReadArchivedTimeboxes for missing file: %v", err)
	}
	if len(archived) != 0 {
		t.Errorf("ReadArchivedTimeboxes: got %d timeboxes, want 0", len(archived))
	}
}

func TestArchiveTaskList(t *testing.T) {
	tmpDir := t.TempDir()
	s := NewStore(tmpDir)

	// Create a task list first.
	tl := &domain.TaskList{
		Name: "Work",
		Slug: "work",
		Tasks: []domain.Task{
			{Description: "Review PRs", Duration: 30 * time.Minute},
		},
	}
	if err := s.WriteTaskList(tl); err != nil {
		t.Fatalf("WriteTaskList: %v", err)
	}

	// Verify it exists.
	if _, err := s.ReadTaskList("work"); err != nil {
		t.Fatalf("ReadTaskList before archive: %v", err)
	}

	// Archive it.
	if err := s.ArchiveTaskList("work", 2026, 13); err != nil {
		t.Fatalf("ArchiveTaskList: %v", err)
	}

	// Verify source is gone.
	srcPath := filepath.Join(s.TaskListDir(), "work.md")
	if _, err := os.Stat(srcPath); !os.IsNotExist(err) {
		t.Errorf("source file still exists after archive")
	}

	// Verify destination exists and is readable.
	got, err := s.ReadArchivedTaskList(2026, 13, "work")
	if err != nil {
		t.Fatalf("ReadArchivedTaskList: %v", err)
	}
	if got.Name != "Work" {
		t.Errorf("Name: got %q, want %q", got.Name, "Work")
	}
	if len(got.Tasks) != 1 {
		t.Fatalf("Tasks count: got %d, want 1", len(got.Tasks))
	}
	if got.Tasks[0].Description != "Review PRs" {
		t.Errorf("Task description: got %q, want %q", got.Tasks[0].Description, "Review PRs")
	}
}

func TestListArchivedTaskLists(t *testing.T) {
	tmpDir := t.TempDir()
	s := NewStore(tmpDir)

	// Create and archive two task lists in different weeks.
	for _, tc := range []struct {
		slug string
		name string
		year int
		week int
	}{
		{"work", "Work", 2026, 13},
		{"personal", "Personal", 2026, 13},
		{"old-work", "Old Work", 2026, 10},
	} {
		tl := &domain.TaskList{Name: tc.name, Slug: tc.slug}
		if err := s.WriteTaskList(tl); err != nil {
			t.Fatalf("WriteTaskList(%s): %v", tc.slug, err)
		}
		if err := s.ArchiveTaskList(tc.slug, tc.year, tc.week); err != nil {
			t.Fatalf("ArchiveTaskList(%s): %v", tc.slug, err)
		}
	}

	result, err := s.ListArchivedTaskLists()
	if err != nil {
		t.Fatalf("ListArchivedTaskLists: %v", err)
	}

	// Should have year 2026.
	yearData, ok := result[2026]
	if !ok {
		t.Fatal("missing year 2026 in result")
	}

	// Week 13 should have 2 lists (sorted alphabetically).
	w13 := yearData[13]
	if len(w13) != 2 {
		t.Fatalf("week 13: got %d lists, want 2", len(w13))
	}
	if w13[0].Name != "Personal" || w13[1].Name != "Work" {
		t.Errorf("week 13 names: got [%q, %q], want [Personal, Work]", w13[0].Name, w13[1].Name)
	}

	// Week 10 should have 1 list.
	w10 := yearData[10]
	if len(w10) != 1 {
		t.Fatalf("week 10: got %d lists, want 1", len(w10))
	}
	if w10[0].Name != "Old Work" {
		t.Errorf("week 10 name: got %q, want %q", w10[0].Name, "Old Work")
	}
}

func TestReadArchivedTaskList(t *testing.T) {
	tmpDir := t.TempDir()
	s := NewStore(tmpDir)

	tl := &domain.TaskList{
		Name: "Test",
		Slug: "test",
		Tasks: []domain.Task{
			{Description: "Task A", Duration: 15 * time.Minute},
			{Description: "Task B", Duration: 30 * time.Minute, Commented: true},
		},
	}
	if err := s.WriteTaskList(tl); err != nil {
		t.Fatalf("WriteTaskList: %v", err)
	}
	if err := s.ArchiveTaskList("test", 2026, 13); err != nil {
		t.Fatalf("ArchiveTaskList: %v", err)
	}

	got, err := s.ReadArchivedTaskList(2026, 13, "test")
	if err != nil {
		t.Fatalf("ReadArchivedTaskList: %v", err)
	}

	if !reflect.DeepEqual(got.Tasks, tl.Tasks) {
		t.Errorf("Tasks mismatch:\n  got:  %+v\n  want: %+v", got.Tasks, tl.Tasks)
	}

	// Non-existent archived list should return error.
	_, err = s.ReadArchivedTaskList(2026, 13, "nonexistent")
	if err == nil {
		t.Error("expected error for non-existent archived task list")
	}
}

func TestHasCompletedTasks(t *testing.T) {
	tmpDir := t.TempDir()
	s := NewStore(tmpDir)

	// No completed tasks — should return false.
	has, err := s.HasCompletedTasks("work")
	if err != nil {
		t.Fatalf("HasCompletedTasks (empty): %v", err)
	}
	if has {
		t.Error("HasCompletedTasks: got true, want false (no archive)")
	}

	// Add a completed task for "work".
	ct := CompletedTask{
		Task:          domain.Task{Description: "Do stuff", Duration: 30 * time.Minute},
		CompletedDate: time.Date(2026, 3, 25, 0, 0, 0, 0, time.UTC),
		TaskListSlug:  "work",
	}
	if err := s.AddCompleted(2026, 13, ct); err != nil {
		t.Fatalf("AddCompleted: %v", err)
	}

	// Now should return true for "work".
	has, err = s.HasCompletedTasks("work")
	if err != nil {
		t.Fatalf("HasCompletedTasks (with data): %v", err)
	}
	if !has {
		t.Error("HasCompletedTasks: got false, want true")
	}

	// Should return false for a different slug.
	has, err = s.HasCompletedTasks("personal")
	if err != nil {
		t.Fatalf("HasCompletedTasks (different slug): %v", err)
	}
	if has {
		t.Error("HasCompletedTasks: got true, want false for different slug")
	}
}

func TestReadWriteNotes(t *testing.T) {
	tmpDir := t.TempDir()
	s := NewStore(tmpDir)

	// Reading non-existent notes should return empty string.
	content, err := s.ReadNotes()
	if err != nil {
		t.Fatalf("ReadNotes (empty): %v", err)
	}
	if content != "" {
		t.Errorf("ReadNotes (empty): got %q, want empty", content)
	}

	// Write and read back.
	original := "# My Notes\n\nSome content here.\n"
	if err := s.WriteNotes(original); err != nil {
		t.Fatalf("WriteNotes: %v", err)
	}

	got, err := s.ReadNotes()
	if err != nil {
		t.Fatalf("ReadNotes: %v", err)
	}
	if got != original {
		t.Errorf("ReadNotes: got %q, want %q", got, original)
	}

	// Overwrite with new content.
	updated := "# Updated\n\nNew content.\n"
	if err := s.WriteNotes(updated); err != nil {
		t.Fatalf("WriteNotes (update): %v", err)
	}

	got, err = s.ReadNotes()
	if err != nil {
		t.Fatalf("ReadNotes (after update): %v", err)
	}
	if got != updated {
		t.Errorf("ReadNotes (after update): got %q, want %q", got, updated)
	}
}
