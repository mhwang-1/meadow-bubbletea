package domain

import (
	"testing"
	"time"
)

func TestIsReserved(t *testing.T) {
	t.Run("reserved_tag", func(t *testing.T) {
		tb := &Timebox{Tag: "reserved"}
		if !tb.IsReserved() {
			t.Error("IsReserved() = false, want true")
		}
	})

	t.Run("empty_tag", func(t *testing.T) {
		tb := &Timebox{}
		if tb.IsReserved() {
			t.Error("IsReserved() = true, want false")
		}
	})

	t.Run("other_tag", func(t *testing.T) {
		tb := &Timebox{Tag: "other"}
		if tb.IsReserved() {
			t.Error("IsReserved() = true for tag 'other', want false")
		}
	})
}

func TestTimeboxDurationMinutes(t *testing.T) {
	tb := &Timebox{
		Start: time.Date(2026, 3, 27, 9, 0, 0, 0, time.UTC),
		End:   time.Date(2026, 3, 27, 11, 0, 0, 0, time.UTC),
	}

	got := tb.DurationMinutes()
	if got != 120 {
		t.Errorf("DurationMinutes() = %d, want 120", got)
	}
}

func TestSequenceTasks(t *testing.T) {
	t.Run("three_tasks_with_remaining_break", func(t *testing.T) {
		tb := &Timebox{
			Start: time.Date(2026, 3, 27, 9, 0, 0, 0, time.UTC),
			End:   time.Date(2026, 3, 27, 11, 0, 0, 0, time.UTC),
		}
		tasks := []Task{
			{Description: "Task A", Duration: 24 * time.Minute},
			{Description: "Task B", Duration: 24 * time.Minute},
			{Description: "Task C", Duration: 24 * time.Minute},
		}

		scheduled := SequenceTasks(tb, tasks)

		// 3 tasks of 24m = 72m total in a 120m timebox.
		// All three should fit. The 4th slot does not exist because there are
		// only 3 tasks; SequenceTasks only inserts a break when the *next* task
		// doesn't fit, and after placing all 3 tasks it simply stops.
		if len(scheduled) != 3 {
			t.Fatalf("got %d scheduled items, want 3", len(scheduled))
		}

		for i, s := range scheduled {
			if s.IsBreak {
				t.Errorf("scheduled[%d] is a break, expected task", i)
			}
		}

		// Verify start times.
		expectedStarts := []time.Time{
			time.Date(2026, 3, 27, 9, 0, 0, 0, time.UTC),
			time.Date(2026, 3, 27, 9, 24, 0, 0, time.UTC),
			time.Date(2026, 3, 27, 9, 48, 0, 0, time.UTC),
		}
		for i, s := range scheduled {
			if !s.StartTime.Equal(expectedStarts[i]) {
				t.Errorf("scheduled[%d].StartTime = %v, want %v", i, s.StartTime, expectedStarts[i])
			}
		}
	})

	t.Run("tasks_exactly_fill_timebox", func(t *testing.T) {
		tb := &Timebox{
			Start: time.Date(2026, 3, 27, 9, 0, 0, 0, time.UTC),
			End:   time.Date(2026, 3, 27, 10, 0, 0, 0, time.UTC),
		}
		tasks := []Task{
			{Description: "Task A", Duration: 30 * time.Minute},
			{Description: "Task B", Duration: 30 * time.Minute},
		}

		scheduled := SequenceTasks(tb, tasks)

		if len(scheduled) != 2 {
			t.Fatalf("got %d scheduled items, want 2", len(scheduled))
		}
		for i, s := range scheduled {
			if s.IsBreak {
				t.Errorf("scheduled[%d] is a break, expected task", i)
			}
		}
	})

	t.Run("single_task_longer_than_remaining", func(t *testing.T) {
		tb := &Timebox{
			Start: time.Date(2026, 3, 27, 9, 0, 0, 0, time.UTC),
			End:   time.Date(2026, 3, 27, 9, 30, 0, 0, time.UTC),
		}
		tasks := []Task{
			{Description: "Long task", Duration: 60 * time.Minute},
		}

		scheduled := SequenceTasks(tb, tasks)

		if len(scheduled) != 1 {
			t.Fatalf("got %d scheduled items, want 1", len(scheduled))
		}
		if !scheduled[0].IsBreak {
			t.Error("expected a break, got a task")
		}
		if scheduled[0].Task.Duration != 30*time.Minute {
			t.Errorf("break duration = %v, want %v", scheduled[0].Task.Duration, 30*time.Minute)
		}
	})

	t.Run("empty_task_list", func(t *testing.T) {
		tb := &Timebox{
			Start: time.Date(2026, 3, 27, 9, 0, 0, 0, time.UTC),
			End:   time.Date(2026, 3, 27, 11, 0, 0, 0, time.UTC),
		}

		scheduled := SequenceTasks(tb, []Task{})

		if len(scheduled) != 0 {
			t.Errorf("got %d scheduled items, want 0", len(scheduled))
		}
	})

	t.Run("completed_tasks_reduce_available_time", func(t *testing.T) {
		tb := &Timebox{
			Start: time.Date(2026, 3, 27, 9, 0, 0, 0, time.UTC),
			End:   time.Date(2026, 3, 27, 10, 0, 0, 0, time.UTC),
			CompletedTasks: []Task{
				{Description: "Done task", Duration: 30 * time.Minute},
			},
		}
		tasks := []Task{
			{Description: "Task A", Duration: 24 * time.Minute},
			{Description: "Task B", Duration: 24 * time.Minute},
		}

		scheduled := SequenceTasks(tb, tasks)

		// 60m timebox with 30m completed → 30m remaining.
		// Task A (24m) fits, Task B (24m) doesn't → break of 6m.
		if len(scheduled) != 2 {
			t.Fatalf("got %d scheduled items, want 2", len(scheduled))
		}

		if scheduled[0].Task.Description != "Task A" || scheduled[0].IsBreak {
			t.Errorf("scheduled[0] = %q (break=%v), want Task A (break=false)",
				scheduled[0].Task.Description, scheduled[0].IsBreak)
		}
		// Task A should start at 09:30 (after completed task's 30m).
		wantStart := time.Date(2026, 3, 27, 9, 30, 0, 0, time.UTC)
		if !scheduled[0].StartTime.Equal(wantStart) {
			t.Errorf("scheduled[0].StartTime = %v, want %v", scheduled[0].StartTime, wantStart)
		}

		if !scheduled[1].IsBreak {
			t.Errorf("scheduled[1] should be a break")
		}
		if scheduled[1].Task.Duration != 6*time.Minute {
			t.Errorf("break duration = %v, want %v", scheduled[1].Task.Duration, 6*time.Minute)
		}
	})
}

func TestScheduledMinutes(t *testing.T) {
	scheduled := []ScheduledTask{
		{
			Task:      Task{Description: "Task A", Duration: 24 * time.Minute},
			StartTime: time.Date(2026, 3, 27, 9, 0, 0, 0, time.UTC),
			IsBreak:   false,
		},
		{
			Task:      Task{Description: "Task B", Duration: 24 * time.Minute},
			StartTime: time.Date(2026, 3, 27, 9, 24, 0, 0, time.UTC),
			IsBreak:   false,
		},
		{
			Task:      Task{Description: "Break", Duration: 12 * time.Minute},
			StartTime: time.Date(2026, 3, 27, 9, 48, 0, 0, time.UTC),
			IsBreak:   true,
		},
	}

	got := ScheduledMinutes(scheduled)
	if got != 48 {
		t.Errorf("ScheduledMinutes() = %d, want 48", got)
	}
}

func TestSequenceTasksAcrossTimeboxes(t *testing.T) {
	tb1 := Timebox{
		Start: time.Date(2026, 3, 27, 9, 0, 0, 0, time.UTC),
		End:   time.Date(2026, 3, 27, 10, 0, 0, 0, time.UTC),
	}
	tb2 := Timebox{
		Start: time.Date(2026, 3, 27, 10, 0, 0, 0, time.UTC),
		End:   time.Date(2026, 3, 27, 11, 0, 0, 0, time.UTC),
	}

	tasks := []Task{
		{Description: "Task A", Duration: 30 * time.Minute},
		{Description: "Task B", Duration: 30 * time.Minute},
		{Description: "Task C", Duration: 30 * time.Minute},
	}

	got := SequenceTasksAcrossTimeboxes([]Timebox{tb1, tb2}, tasks)
	if len(got) != 2 {
		t.Fatalf("got %d timebox schedules, want 2", len(got))
	}

	if len(got[0]) != 2 {
		t.Fatalf("first timebox got %d items, want 2", len(got[0]))
	}
	if got[0][0].Task.Description != "Task A" || got[0][1].Task.Description != "Task B" {
		t.Fatalf("first timebox tasks = [%s, %s], want [Task A, Task B]",
			got[0][0].Task.Description, got[0][1].Task.Description)
	}

	if len(got[1]) != 1 {
		t.Fatalf("second timebox got %d items, want 1", len(got[1]))
	}
	if got[1][0].Task.Description != "Task C" {
		t.Fatalf("second timebox first task = %q, want %q", got[1][0].Task.Description, "Task C")
	}
}

func TestSequenceTasksAcrossTimeboxesCarriesOversizedTask(t *testing.T) {
	tb1 := Timebox{
		Start: time.Date(2026, 3, 27, 9, 0, 0, 0, time.UTC),
		End:   time.Date(2026, 3, 27, 9, 20, 0, 0, time.UTC),
	}
	tb2 := Timebox{
		Start: time.Date(2026, 3, 27, 9, 20, 0, 0, time.UTC),
		End:   time.Date(2026, 3, 27, 10, 0, 0, 0, time.UTC),
	}

	tasks := []Task{{Description: "Deep work", Duration: 30 * time.Minute}}
	got := SequenceTasksAcrossTimeboxes([]Timebox{tb1, tb2}, tasks)

	if len(got[0]) != 1 || !got[0][0].IsBreak {
		t.Fatalf("first timebox should contain one break")
	}
	if len(got[1]) != 1 || got[1][0].IsBreak {
		t.Fatalf("second timebox should contain the carried task")
	}
	if got[1][0].Task.Description != "Deep work" {
		t.Fatalf("carried task = %q, want %q", got[1][0].Task.Description, "Deep work")
	}
}
