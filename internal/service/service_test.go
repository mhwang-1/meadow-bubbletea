package service

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/mhwang-1/meadow-bubbletea/internal/domain"
	"github.com/mhwang-1/meadow-bubbletea/internal/store"
)

// helper: create a store rooted at a temp directory.
func newTestService(t *testing.T) *Service {
	t.Helper()
	tmpDir := t.TempDir()
	s := store.NewStore(tmpDir)
	return New(s)
}

// helper: date on a known date (Friday 27 Mar 2026).
func testDate() time.Time {
	return time.Date(2026, 3, 27, 0, 0, 0, 0, time.UTC)
}

// helper: create a time on the test date.
func timeOn(hour, min int) time.Time {
	d := testDate()
	return time.Date(d.Year(), d.Month(), d.Day(), hour, min, 0, 0, d.Location())
}

// --- CreateTimebox tests ---

func TestCreateTimebox_Success(t *testing.T) {
	svc := newTestService(t)
	date := testDate()

	if err := svc.CreateTimebox(date, timeOn(9, 0), timeOn(11, 0)); err != nil {
		t.Fatalf("CreateTimebox: %v", err)
	}

	// Verify the timebox was created.
	view, err := svc.GetDayView(date)
	if err != nil {
		t.Fatalf("GetDayView: %v", err)
	}
	if len(view.Timeboxes) != 1 {
		t.Fatalf("expected 1 timebox, got %d", len(view.Timeboxes))
	}
	if view.Timeboxes[0].Timebox.Status != domain.StatusUnassigned {
		t.Errorf("status: got %q, want %q", view.Timeboxes[0].Timebox.Status, domain.StatusUnassigned)
	}
}

func TestCreateTimebox_Overlap(t *testing.T) {
	svc := newTestService(t)
	date := testDate()

	if err := svc.CreateTimebox(date, timeOn(9, 0), timeOn(11, 0)); err != nil {
		t.Fatalf("CreateTimebox first: %v", err)
	}

	// Overlapping timebox should fail.
	err := svc.CreateTimebox(date, timeOn(10, 0), timeOn(12, 0))
	if err == nil {
		t.Fatal("expected overlap error, got nil")
	}
}

func TestCreateTimebox_TooShort(t *testing.T) {
	svc := newTestService(t)
	date := testDate()

	err := svc.CreateTimebox(date, timeOn(9, 0), timeOn(9, 10))
	if err == nil {
		t.Fatal("expected too-short error, got nil")
	}
}

func TestCreateTimebox_StartAfterEnd(t *testing.T) {
	svc := newTestService(t)
	date := testDate()

	err := svc.CreateTimebox(date, timeOn(11, 0), timeOn(9, 0))
	if err == nil {
		t.Fatal("expected start-after-end error, got nil")
	}
}

// --- EditTimeboxTime tests ---

func TestEditTimeboxTime_Success(t *testing.T) {
	svc := newTestService(t)
	date := testDate()

	if err := svc.CreateTimebox(date, timeOn(9, 0), timeOn(11, 0)); err != nil {
		t.Fatalf("CreateTimebox: %v", err)
	}

	if err := svc.EditTimeboxTime(date, 0, timeOn(10, 0), timeOn(12, 0)); err != nil {
		t.Fatalf("EditTimeboxTime: %v", err)
	}

	view, err := svc.GetDayView(date)
	if err != nil {
		t.Fatalf("GetDayView: %v", err)
	}
	tb := view.Timeboxes[0].Timebox
	if tb.Start.Hour() != 10 || tb.End.Hour() != 12 {
		t.Errorf("times: got %s-%s, want 10:00-12:00",
			tb.Start.Format("15:04"), tb.End.Format("15:04"))
	}
}

// --- DeleteTimebox tests ---

func TestDeleteTimebox_Success(t *testing.T) {
	svc := newTestService(t)
	date := testDate()

	if err := svc.CreateTimebox(date, timeOn(9, 0), timeOn(11, 0)); err != nil {
		t.Fatalf("CreateTimebox: %v", err)
	}

	if err := svc.DeleteTimebox(date, 0); err != nil {
		t.Fatalf("DeleteTimebox: %v", err)
	}

	view, err := svc.GetDayView(date)
	if err != nil {
		t.Fatalf("GetDayView: %v", err)
	}
	if len(view.Timeboxes) != 0 {
		t.Errorf("expected 0 timeboxes after delete, got %d", len(view.Timeboxes))
	}
}

// --- AssignTaskList tests ---

func TestAssignTaskList_Success(t *testing.T) {
	svc := newTestService(t)
	date := testDate()

	// Create a task list and a timebox.
	if _, err := svc.CreateTaskList("Work"); err != nil {
		t.Fatalf("CreateTaskList: %v", err)
	}
	if err := svc.CreateTimebox(date, timeOn(9, 0), timeOn(11, 0)); err != nil {
		t.Fatalf("CreateTimebox: %v", err)
	}

	if err := svc.AssignTaskList(date, 0, "work"); err != nil {
		t.Fatalf("AssignTaskList: %v", err)
	}

	view, err := svc.GetDayView(date)
	if err != nil {
		t.Fatalf("GetDayView: %v", err)
	}
	tb := view.Timeboxes[0].Timebox
	if tb.TaskListSlug != "work" {
		t.Errorf("slug: got %q, want %q", tb.TaskListSlug, "work")
	}
	if tb.Status != domain.StatusActive {
		t.Errorf("status: got %q, want %q", tb.Status, domain.StatusActive)
	}
}

// --- SetReserved / UnsetReserved tests ---

func TestSetReservedAndUnset(t *testing.T) {
	svc := newTestService(t)
	date := testDate()

	if err := svc.CreateTimebox(date, timeOn(9, 0), timeOn(11, 0)); err != nil {
		t.Fatalf("CreateTimebox: %v", err)
	}

	if err := svc.SetReserved(date, 0, "Doctor visit"); err != nil {
		t.Fatalf("SetReserved: %v", err)
	}

	view, err := svc.GetDayView(date)
	if err != nil {
		t.Fatalf("GetDayView: %v", err)
	}
	tb := view.Timeboxes[0].Timebox
	if !tb.IsReserved() {
		t.Error("expected timebox to be reserved")
	}
	if tb.Note != "Doctor visit" {
		t.Errorf("note: got %q, want %q", tb.Note, "Doctor visit")
	}

	if err := svc.UnsetReserved(date, 0); err != nil {
		t.Fatalf("UnsetReserved: %v", err)
	}

	view, err = svc.GetDayView(date)
	if err != nil {
		t.Fatalf("GetDayView: %v", err)
	}
	tb = view.Timeboxes[0].Timebox
	if tb.IsReserved() {
		t.Error("expected timebox to not be reserved after unset")
	}
	if tb.Status != domain.StatusUnassigned {
		t.Errorf("status: got %q, want %q", tb.Status, domain.StatusUnassigned)
	}
}

// --- MarkTaskDone / UnmarkTaskDone tests ---

func TestMarkAndUnmarkTaskDone(t *testing.T) {
	svc := newTestService(t)
	date := testDate()

	// Create a task list with tasks.
	tl, err := svc.CreateTaskList("Work")
	if err != nil {
		t.Fatalf("CreateTaskList: %v", err)
	}

	tasks := []domain.Task{
		{Description: "Review PRs", Duration: 30 * time.Minute},
		{Description: "Write docs", Duration: 1 * time.Hour},
	}
	if err := svc.UpdateTaskList(tl.Slug, tasks); err != nil {
		t.Fatalf("UpdateTaskList: %v", err)
	}

	// Create and assign a timebox.
	if err := svc.CreateTimebox(date, timeOn(9, 0), timeOn(12, 0)); err != nil {
		t.Fatalf("CreateTimebox: %v", err)
	}
	if err := svc.AssignTaskList(date, 0, "work"); err != nil {
		t.Fatalf("AssignTaskList: %v", err)
	}

	// Mark first task as done (taskIdx=0 for first pending task).
	if err := svc.MarkTaskDone(date, 0, 0); err != nil {
		t.Fatalf("MarkTaskDone: %v", err)
	}

	// Verify: timebox should have 1 completed task, task list should have 1 task.
	view, err := svc.GetDayView(date)
	if err != nil {
		t.Fatalf("GetDayView: %v", err)
	}
	if len(view.Timeboxes[0].Timebox.CompletedTasks) != 1 {
		t.Fatalf("completed tasks: got %d, want 1", len(view.Timeboxes[0].Timebox.CompletedTasks))
	}
	if view.Timeboxes[0].Timebox.CompletedTasks[0].Description != "Review PRs" {
		t.Errorf("completed task: got %q, want %q",
			view.Timeboxes[0].Timebox.CompletedTasks[0].Description, "Review PRs")
	}

	gotTL, err := svc.GetTaskList("work")
	if err != nil {
		t.Fatalf("GetTaskList: %v", err)
	}
	if len(gotTL.Tasks) != 1 {
		t.Fatalf("task list tasks: got %d, want 1", len(gotTL.Tasks))
	}
	if gotTL.Tasks[0].Description != "Write docs" {
		t.Errorf("remaining task: got %q, want %q", gotTL.Tasks[0].Description, "Write docs")
	}

	// Verify archive has the completed task.
	wi := domain.WeekForDate(date)
	history, err := svc.GetHistory(wi.Year, wi.Week)
	if err != nil {
		t.Fatalf("GetHistory: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("history: got %d entries, want 1", len(history))
	}

	// Unmark the task (most recently completed = index 0, which is the last one).
	if err := svc.UnmarkTaskDone(date, 0, 0); err != nil {
		t.Fatalf("UnmarkTaskDone: %v", err)
	}

	// Verify: timebox should have 0 completed tasks, task list should have 2 tasks.
	view, err = svc.GetDayView(date)
	if err != nil {
		t.Fatalf("GetDayView after unmark: %v", err)
	}
	if len(view.Timeboxes[0].Timebox.CompletedTasks) != 0 {
		t.Errorf("completed tasks after unmark: got %d, want 0",
			len(view.Timeboxes[0].Timebox.CompletedTasks))
	}

	gotTL, err = svc.GetTaskList("work")
	if err != nil {
		t.Fatalf("GetTaskList after unmark: %v", err)
	}
	if len(gotTL.Tasks) != 2 {
		t.Errorf("task list tasks after unmark: got %d, want 2", len(gotTL.Tasks))
	}

	// Verify archive entry was removed.
	history, err = svc.GetHistory(wi.Year, wi.Week)
	if err != nil {
		t.Fatalf("GetHistory after unmark: %v", err)
	}
	if len(history) != 0 {
		t.Errorf("history after unmark: got %d entries, want 0", len(history))
	}
}

// --- ArchiveTimebox tests ---

func TestArchiveTimebox_Success(t *testing.T) {
	svc := newTestService(t)
	date := testDate()

	// Create a task list and timebox, assign, mark all tasks done.
	if _, err := svc.CreateTaskList("Work"); err != nil {
		t.Fatalf("CreateTaskList: %v", err)
	}
	tasks := []domain.Task{
		{Description: "Task A", Duration: 30 * time.Minute},
	}
	if err := svc.UpdateTaskList("work", tasks); err != nil {
		t.Fatalf("UpdateTaskList: %v", err)
	}
	if err := svc.CreateTimebox(date, timeOn(9, 0), timeOn(11, 0)); err != nil {
		t.Fatalf("CreateTimebox: %v", err)
	}
	if err := svc.AssignTaskList(date, 0, "work"); err != nil {
		t.Fatalf("AssignTaskList: %v", err)
	}
	if err := svc.MarkTaskDone(date, 0, 0); err != nil {
		t.Fatalf("MarkTaskDone: %v", err)
	}

	// Archive (no pending tasks, force=false should work).
	if err := svc.ArchiveTimebox(date, 0, false); err != nil {
		t.Fatalf("ArchiveTimebox: %v", err)
	}

	// Verify the timebox is archived.
	view, err := svc.GetDayView(date)
	if err != nil {
		t.Fatalf("GetDayView: %v", err)
	}
	if view.Timeboxes[0].Timebox.Status != domain.StatusArchived {
		t.Errorf("status: got %q, want %q",
			view.Timeboxes[0].Timebox.Status, domain.StatusArchived)
	}

	// Verify archived timeboxes record was created.
	wi := domain.WeekForDate(date)
	archived, err := svc.GetArchivedTimeboxes(wi.Year, wi.Week)
	if err != nil {
		t.Fatalf("GetArchivedTimeboxes: %v", err)
	}
	if len(archived) != 1 {
		t.Fatalf("archived timeboxes: got %d, want 1", len(archived))
	}
}

func TestArchiveTimebox_PendingTasksNoForce(t *testing.T) {
	svc := newTestService(t)
	date := testDate()

	if _, err := svc.CreateTaskList("Work"); err != nil {
		t.Fatalf("CreateTaskList: %v", err)
	}
	tasks := []domain.Task{
		{Description: "Task A", Duration: 30 * time.Minute},
		{Description: "Task B", Duration: 30 * time.Minute},
	}
	if err := svc.UpdateTaskList("work", tasks); err != nil {
		t.Fatalf("UpdateTaskList: %v", err)
	}
	if err := svc.CreateTimebox(date, timeOn(9, 0), timeOn(11, 0)); err != nil {
		t.Fatalf("CreateTimebox: %v", err)
	}
	if err := svc.AssignTaskList(date, 0, "work"); err != nil {
		t.Fatalf("AssignTaskList: %v", err)
	}

	// Archive without force — should fail with PendingTasksError.
	err := svc.ArchiveTimebox(date, 0, false)
	if err == nil {
		t.Fatal("expected PendingTasksError, got nil")
	}
	var pendingErr *PendingTasksError
	if !errors.As(err, &pendingErr) {
		t.Fatalf("expected PendingTasksError, got %T: %v", err, err)
	}
	if pendingErr.Count != 2 {
		t.Errorf("pending count: got %d, want 2", pendingErr.Count)
	}
}

func TestArchiveTimebox_PendingTasksWithForce(t *testing.T) {
	svc := newTestService(t)
	date := testDate()

	if _, err := svc.CreateTaskList("Work"); err != nil {
		t.Fatalf("CreateTaskList: %v", err)
	}
	tasks := []domain.Task{
		{Description: "Task A", Duration: 30 * time.Minute},
	}
	if err := svc.UpdateTaskList("work", tasks); err != nil {
		t.Fatalf("UpdateTaskList: %v", err)
	}
	if err := svc.CreateTimebox(date, timeOn(9, 0), timeOn(11, 0)); err != nil {
		t.Fatalf("CreateTimebox: %v", err)
	}
	if err := svc.AssignTaskList(date, 0, "work"); err != nil {
		t.Fatalf("AssignTaskList: %v", err)
	}

	// Archive with force — should succeed despite pending tasks.
	if err := svc.ArchiveTimebox(date, 0, true); err != nil {
		t.Fatalf("ArchiveTimebox with force: %v", err)
	}

	view, err := svc.GetDayView(date)
	if err != nil {
		t.Fatalf("GetDayView: %v", err)
	}
	if view.Timeboxes[0].Timebox.Status != domain.StatusArchived {
		t.Errorf("status: got %q, want %q",
			view.Timeboxes[0].Timebox.Status, domain.StatusArchived)
	}
}

func TestArchiveTimebox_NotArchivable(t *testing.T) {
	svc := newTestService(t)
	date := testDate()

	// Unassigned timebox cannot be archived.
	if err := svc.CreateTimebox(date, timeOn(9, 0), timeOn(11, 0)); err != nil {
		t.Fatalf("CreateTimebox: %v", err)
	}
	err := svc.ArchiveTimebox(date, 0, false)
	if err == nil {
		t.Fatal("expected error archiving unassigned timebox")
	}

	// Reserved timebox cannot be archived.
	if err := svc.SetReserved(date, 0, "Meeting"); err != nil {
		t.Fatalf("SetReserved: %v", err)
	}
	err = svc.ArchiveTimebox(date, 0, false)
	if err == nil {
		t.Fatal("expected error archiving reserved timebox")
	}
}

// --- UnarchiveTimebox tests ---

func TestUnarchiveTimebox(t *testing.T) {
	svc := newTestService(t)
	date := testDate()

	if _, err := svc.CreateTaskList("Work"); err != nil {
		t.Fatalf("CreateTaskList: %v", err)
	}
	if err := svc.CreateTimebox(date, timeOn(9, 0), timeOn(11, 0)); err != nil {
		t.Fatalf("CreateTimebox: %v", err)
	}
	if err := svc.AssignTaskList(date, 0, "work"); err != nil {
		t.Fatalf("AssignTaskList: %v", err)
	}
	if err := svc.ArchiveTimebox(date, 0, true); err != nil {
		t.Fatalf("ArchiveTimebox: %v", err)
	}

	if err := svc.UnarchiveTimebox(date, 0); err != nil {
		t.Fatalf("UnarchiveTimebox: %v", err)
	}

	view, err := svc.GetDayView(date)
	if err != nil {
		t.Fatalf("GetDayView: %v", err)
	}
	if view.Timeboxes[0].Timebox.Status != domain.StatusActive {
		t.Errorf("status: got %q, want %q",
			view.Timeboxes[0].Timebox.Status, domain.StatusActive)
	}

	// Verify archived record was removed.
	wi := domain.WeekForDate(date)
	archived, err := svc.GetArchivedTimeboxes(wi.Year, wi.Week)
	if err != nil {
		t.Fatalf("GetArchivedTimeboxes: %v", err)
	}
	if len(archived) != 0 {
		t.Errorf("archived timeboxes after unarchive: got %d, want 0", len(archived))
	}
}

// --- CreateTaskList tests ---

func TestCreateTaskList_Success(t *testing.T) {
	svc := newTestService(t)

	tl, err := svc.CreateTaskList("Work 03/2026")
	if err != nil {
		t.Fatalf("CreateTaskList: %v", err)
	}
	if tl.Name != "Work 03/2026" {
		t.Errorf("name: got %q, want %q", tl.Name, "Work 03/2026")
	}
	if tl.Slug != "work-03-2026" {
		t.Errorf("slug: got %q, want %q", tl.Slug, "work-03-2026")
	}

	// Verify it can be read back.
	got, err := svc.GetTaskList("work-03-2026")
	if err != nil {
		t.Fatalf("GetTaskList: %v", err)
	}
	if got.Name != "Work 03/2026" {
		t.Errorf("read back name: got %q, want %q", got.Name, "Work 03/2026")
	}
}

func TestCreateTaskList_Duplicate(t *testing.T) {
	svc := newTestService(t)

	if _, err := svc.CreateTaskList("Work"); err != nil {
		t.Fatalf("CreateTaskList first: %v", err)
	}

	_, err := svc.CreateTaskList("Work")
	if err == nil {
		t.Fatal("expected error for duplicate task list")
	}
}

// --- UpdateTaskList tests ---

func TestUpdateTaskList_Success(t *testing.T) {
	svc := newTestService(t)

	if _, err := svc.CreateTaskList("Work"); err != nil {
		t.Fatalf("CreateTaskList: %v", err)
	}

	tasks := []domain.Task{
		{Description: "Review PRs", Duration: 30 * time.Minute},
		{Description: "Write docs", Duration: 1 * time.Hour},
	}
	if err := svc.UpdateTaskList("work", tasks); err != nil {
		t.Fatalf("UpdateTaskList: %v", err)
	}

	got, err := svc.GetTaskList("work")
	if err != nil {
		t.Fatalf("GetTaskList: %v", err)
	}
	if !reflect.DeepEqual(got.Tasks, tasks) {
		t.Errorf("tasks mismatch:\n  got:  %+v\n  want: %+v", got.Tasks, tasks)
	}
	// Verify name is preserved.
	if got.Name != "Work" {
		t.Errorf("name: got %q, want %q", got.Name, "Work")
	}
}

// --- DeleteTaskList tests ---

func TestDeleteTaskList_Success(t *testing.T) {
	svc := newTestService(t)

	if _, err := svc.CreateTaskList("Temp"); err != nil {
		t.Fatalf("CreateTaskList: %v", err)
	}

	if err := svc.DeleteTaskList("temp"); err != nil {
		t.Fatalf("DeleteTaskList: %v", err)
	}

	// Verify it's gone.
	_, err := svc.GetTaskList("temp")
	if err == nil {
		t.Fatal("expected error reading deleted task list")
	}
}

func TestDeleteTaskList_WithCompletedTasks(t *testing.T) {
	svc := newTestService(t)
	date := testDate()

	if _, err := svc.CreateTaskList("Work"); err != nil {
		t.Fatalf("CreateTaskList: %v", err)
	}
	tasks := []domain.Task{
		{Description: "Task A", Duration: 30 * time.Minute},
	}
	if err := svc.UpdateTaskList("work", tasks); err != nil {
		t.Fatalf("UpdateTaskList: %v", err)
	}
	if err := svc.CreateTimebox(date, timeOn(9, 0), timeOn(11, 0)); err != nil {
		t.Fatalf("CreateTimebox: %v", err)
	}
	if err := svc.AssignTaskList(date, 0, "work"); err != nil {
		t.Fatalf("AssignTaskList: %v", err)
	}
	if err := svc.MarkTaskDone(date, 0, 0); err != nil {
		t.Fatalf("MarkTaskDone: %v", err)
	}

	// Should fail because of archived completed tasks.
	err := svc.DeleteTaskList("work")
	if err == nil {
		t.Fatal("expected error deleting task list with completed tasks")
	}
}

// --- ListTaskLists tests ---

func TestListTaskLists(t *testing.T) {
	svc := newTestService(t)

	if _, err := svc.CreateTaskList("Bravo"); err != nil {
		t.Fatalf("CreateTaskList: %v", err)
	}
	if _, err := svc.CreateTaskList("Alpha"); err != nil {
		t.Fatalf("CreateTaskList: %v", err)
	}

	lists, err := svc.ListTaskLists()
	if err != nil {
		t.Fatalf("ListTaskLists: %v", err)
	}
	if len(lists) != 2 {
		t.Fatalf("count: got %d, want 2", len(lists))
	}
	if lists[0].Name != "Alpha" || lists[1].Name != "Bravo" {
		t.Errorf("order: got [%q, %q], want [Alpha, Bravo]", lists[0].Name, lists[1].Name)
	}
}

// --- GetDayView tests ---

func TestGetDayView_SequencedTasks(t *testing.T) {
	svc := newTestService(t)
	date := testDate()

	// Create a task list with tasks.
	if _, err := svc.CreateTaskList("Work"); err != nil {
		t.Fatalf("CreateTaskList: %v", err)
	}
	tasks := []domain.Task{
		{Description: "Task A", Duration: 30 * time.Minute},
		{Description: "Task B", Duration: 45 * time.Minute},
		{Description: "Task C", Duration: 1 * time.Hour},
	}
	if err := svc.UpdateTaskList("work", tasks); err != nil {
		t.Fatalf("UpdateTaskList: %v", err)
	}

	// Create and assign a timebox.
	if err := svc.CreateTimebox(date, timeOn(9, 0), timeOn(11, 0)); err != nil {
		t.Fatalf("CreateTimebox: %v", err)
	}
	if err := svc.AssignTaskList(date, 0, "work"); err != nil {
		t.Fatalf("AssignTaskList: %v", err)
	}

	view, err := svc.GetDayView(date)
	if err != nil {
		t.Fatalf("GetDayView: %v", err)
	}

	if len(view.Timeboxes) != 1 {
		t.Fatalf("timebox count: got %d, want 1", len(view.Timeboxes))
	}

	dvt := view.Timeboxes[0]
	if dvt.TaskListName != "Work" {
		t.Errorf("task list name: got %q, want %q", dvt.TaskListName, "Work")
	}

	// 2h timebox: Task A (30m) + Task B (45m) + break (45m) since Task C (1h) doesn't fit
	// remaining after A+B = 45m, but Task C = 1h > 45m, so break.
	if len(dvt.ScheduledTasks) != 3 {
		t.Fatalf("scheduled tasks: got %d, want 3", len(dvt.ScheduledTasks))
	}

	if dvt.ScheduledTasks[0].Task.Description != "Task A" {
		t.Errorf("[0] description: got %q, want %q", dvt.ScheduledTasks[0].Task.Description, "Task A")
	}
	if !dvt.ScheduledTasks[0].StartTime.Equal(timeOn(9, 0)) {
		t.Errorf("[0] start: got %v, want 09:00", dvt.ScheduledTasks[0].StartTime.Format("15:04"))
	}

	if dvt.ScheduledTasks[1].Task.Description != "Task B" {
		t.Errorf("[1] description: got %q, want %q", dvt.ScheduledTasks[1].Task.Description, "Task B")
	}
	if !dvt.ScheduledTasks[1].StartTime.Equal(timeOn(9, 30)) {
		t.Errorf("[1] start: got %v, want 09:30", dvt.ScheduledTasks[1].StartTime.Format("15:04"))
	}

	if !dvt.ScheduledTasks[2].IsBreak {
		t.Error("[2] expected break")
	}
	if dvt.ScheduledTasks[2].Task.Duration != 45*time.Minute {
		t.Errorf("[2] break duration: got %v, want 45m", dvt.ScheduledTasks[2].Task.Duration)
	}
}

func TestGetDayView_MultipleTimeboxesSameList(t *testing.T) {
	svc := newTestService(t)
	date := testDate()

	if _, err := svc.CreateTaskList("Work"); err != nil {
		t.Fatalf("CreateTaskList: %v", err)
	}
	tasks := []domain.Task{
		{Description: "Task A", Duration: 30 * time.Minute},
		{Description: "Task B", Duration: 30 * time.Minute},
		{Description: "Task C", Duration: 30 * time.Minute},
	}
	if err := svc.UpdateTaskList("work", tasks); err != nil {
		t.Fatalf("UpdateTaskList: %v", err)
	}

	// Create two timeboxes for the same list.
	if err := svc.CreateTimebox(date, timeOn(9, 0), timeOn(10, 0)); err != nil {
		t.Fatalf("CreateTimebox 1: %v", err)
	}
	if err := svc.CreateTimebox(date, timeOn(14, 0), timeOn(15, 0)); err != nil {
		t.Fatalf("CreateTimebox 2: %v", err)
	}
	if err := svc.AssignTaskList(date, 0, "work"); err != nil {
		t.Fatalf("AssignTaskList 1: %v", err)
	}
	if err := svc.AssignTaskList(date, 1, "work"); err != nil {
		t.Fatalf("AssignTaskList 2: %v", err)
	}

	view, err := svc.GetDayView(date)
	if err != nil {
		t.Fatalf("GetDayView: %v", err)
	}

	// First timebox (1h): Task A (30m) + Task B (30m)
	tb0 := view.Timeboxes[0]
	if len(tb0.ScheduledTasks) != 2 {
		t.Fatalf("tb0 scheduled: got %d, want 2", len(tb0.ScheduledTasks))
	}
	if tb0.ScheduledTasks[0].Task.Description != "Task A" {
		t.Errorf("tb0[0]: got %q, want %q", tb0.ScheduledTasks[0].Task.Description, "Task A")
	}
	if tb0.ScheduledTasks[1].Task.Description != "Task B" {
		t.Errorf("tb0[1]: got %q, want %q", tb0.ScheduledTasks[1].Task.Description, "Task B")
	}

	// Second timebox (1h): Task C (30m) only + break possibly
	tb1 := view.Timeboxes[1]
	if len(tb1.ScheduledTasks) < 1 {
		t.Fatalf("tb1 scheduled: got %d, want at least 1", len(tb1.ScheduledTasks))
	}
	if tb1.ScheduledTasks[0].Task.Description != "Task C" {
		t.Errorf("tb1[0]: got %q, want %q", tb1.ScheduledTasks[0].Task.Description, "Task C")
	}
}

// --- Notes tests ---

func TestNotes(t *testing.T) {
	svc := newTestService(t)

	// Read empty notes.
	content, err := svc.ReadNotes()
	if err != nil {
		t.Fatalf("ReadNotes (empty): %v", err)
	}
	if content != "" {
		t.Errorf("expected empty notes, got %q", content)
	}

	// Write and read back.
	if err := svc.WriteNotes("Hello, world!"); err != nil {
		t.Fatalf("WriteNotes: %v", err)
	}

	content, err = svc.ReadNotes()
	if err != nil {
		t.Fatalf("ReadNotes: %v", err)
	}
	if content != "Hello, world!" {
		t.Errorf("notes: got %q, want %q", content, "Hello, world!")
	}
}

// --- GetWeekSummary tests ---

func TestGetWeekSummary(t *testing.T) {
	svc := newTestService(t)
	date := testDate() // Friday 27 Mar 2026

	// Create a timebox on that day.
	if err := svc.CreateTimebox(date, timeOn(9, 0), timeOn(11, 0)); err != nil {
		t.Fatalf("CreateTimebox: %v", err)
	}

	summary, err := svc.GetWeekSummary(date)
	if err != nil {
		t.Fatalf("GetWeekSummary: %v", err)
	}

	// Friday is index 5 (Sunday=0).
	if len(summary.Days[5].Timeboxes) != 1 {
		t.Errorf("Friday timeboxes: got %d, want 1", len(summary.Days[5].Timeboxes))
	}

	// Other days should be empty.
	for i, day := range summary.Days {
		if i == 5 {
			continue
		}
		if len(day.Timeboxes) != 0 {
			t.Errorf("day %d timeboxes: got %d, want 0", i, len(day.Timeboxes))
		}
	}
}

// --- ArchiveTaskList tests ---

func TestArchiveTaskList(t *testing.T) {
	svc := newTestService(t)

	if _, err := svc.CreateTaskList("Old Work"); err != nil {
		t.Fatalf("CreateTaskList: %v", err)
	}

	if err := svc.ArchiveTaskList("old-work"); err != nil {
		t.Fatalf("ArchiveTaskList: %v", err)
	}

	// Should no longer be in active list.
	_, err := svc.GetTaskList("old-work")
	if err == nil {
		t.Fatal("expected error reading archived task list from active")
	}

	// Should appear in archived lists.
	archived, err := svc.GetArchivedTaskLists()
	if err != nil {
		t.Fatalf("GetArchivedTaskLists: %v", err)
	}
	found := false
	for _, weeks := range archived {
		for _, lists := range weeks {
			for _, tl := range lists {
				if tl.Slug == "old-work" {
					found = true
				}
			}
		}
	}
	if !found {
		t.Error("archived task list not found in GetArchivedTaskLists")
	}
}
