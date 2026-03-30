package domain

import (
	"sort"
	"time"
)

// TimeboxStatus represents the lifecycle state of a timebox.
type TimeboxStatus string

const (
	StatusUnassigned TimeboxStatus = "unassigned"
	StatusActive     TimeboxStatus = "active"
	StatusArchived   TimeboxStatus = "archived"
)

// ScheduledTask is a task placed at a calculated position within a timebox.
type ScheduledTask struct {
	Task      Task
	StartTime time.Time
	IsBreak   bool
}

// Timebox is a fixed block of time that may be assigned to a task list.
type Timebox struct {
	Start          time.Time
	End            time.Time
	TaskListSlug   string
	Status         TimeboxStatus
	Tag            string // "" (normal) or "reserved"
	Note           string // short note, used when Tag == "reserved"
	CompletedTasks []Task
}

// IsReserved returns true if the timebox is tagged as reserved.
func (tb *Timebox) IsReserved() bool { return tb.Tag == "reserved" }

// DurationMinutes returns the total number of minutes from Start to End.
func (tb *Timebox) DurationMinutes() int {
	return int(tb.End.Sub(tb.Start).Minutes())
}

// SequenceTasks places tasks sequentially into a timebox. If the remaining
// time is less than the next task's duration, a break is inserted for the
// remaining time and scheduling stops.
func SequenceTasks(tb *Timebox, tasks []Task) []ScheduledTask {
	var scheduled []ScheduledTask
	cursor := tb.Start

	// Completed tasks still occupy their time in the timebox.
	for _, ct := range tb.CompletedTasks {
		cursor = cursor.Add(ct.Duration)
	}
	if cursor.After(tb.End) {
		cursor = tb.End
	}

	for _, t := range tasks {
		remaining := tb.End.Sub(cursor)
		if remaining <= 0 {
			break
		}

		if t.Duration > remaining {
			// Not enough time for this task — insert a break.
			scheduled = append(scheduled, ScheduledTask{
				Task: Task{
					Description: "Break",
					Duration:    remaining,
				},
				StartTime: cursor,
				IsBreak:   true,
			})
			break
		}

		scheduled = append(scheduled, ScheduledTask{
			Task:      t,
			StartTime: cursor,
		})
		cursor = cursor.Add(t.Duration)
	}

	return scheduled
}

// ScheduledMinutes returns the total minutes of actual tasks (not breaks)
// in the scheduled list.
func ScheduledMinutes(scheduled []ScheduledTask) int {
	total := 0
	for _, s := range scheduled {
		if !s.IsBreak {
			total += int(s.Task.Duration.Minutes())
		}
	}
	return total
}

// SequenceTasksAcrossTimeboxes sequences tasks chronologically across multiple
// timeboxes. The returned slice aligns with the input timeboxes by index.
//
// Tasks consumed by earlier timeboxes are not re-used by later ones.
// If a task does not fit in a timebox, that box gets a break and the task is
// carried forward to the next timebox.
func SequenceTasksAcrossTimeboxes(timeboxes []Timebox, tasks []Task) [][]ScheduledTask {
	result := make([][]ScheduledTask, len(timeboxes))
	taskIndex := 0

	for i, tb := range timeboxes {
		if taskIndex >= len(tasks) {
			result[i] = nil
			continue
		}

		scheduled := SequenceTasks(&tb, tasks[taskIndex:])
		result[i] = scheduled

		for _, st := range scheduled {
			if !st.IsBreak {
				taskIndex++
			}
		}
	}

	return result
}

// DailyTimeboxes groups timeboxes under a single date.
type DailyTimeboxes struct {
	Date      time.Time
	Timeboxes []Timebox
}

// SortByStart sorts the timeboxes in ascending order by start time.
func (dt *DailyTimeboxes) SortByStart() {
	sort.Slice(dt.Timeboxes, func(i, j int) bool {
		return dt.Timeboxes[i].Start.Before(dt.Timeboxes[j].Start)
	})
}
