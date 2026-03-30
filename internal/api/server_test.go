package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mhwang-1/meadow-bubbletea/internal/service"
	"github.com/mhwang-1/meadow-bubbletea/internal/store"
)

const testToken = "test-secret-token"

// newTestServer creates a test API server backed by a temp data directory.
func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	tmpDir := t.TempDir()
	st := store.NewStore(tmpDir)
	svc := service.New(st)
	srv := New(svc, testToken, 0)
	return httptest.NewServer(srv.Handler())
}

// authGet sends an authenticated GET request.
func authGet(ts *httptest.Server, path string) (*http.Response, error) {
	req, err := http.NewRequest("GET", ts.URL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+testToken)
	return http.DefaultClient.Do(req)
}

// authReq sends an authenticated request with the given method and JSON body.
func authReq(ts *httptest.Server, method, path string, body any) (*http.Response, error) {
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			return nil, err
		}
	}
	req, err := http.NewRequest(method, ts.URL+path, &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+testToken)
	req.Header.Set("Content-Type", "application/json")
	return http.DefaultClient.Do(req)
}

// decodeJSON decodes a JSON response into the target.
func decodeJSON(t *testing.T, resp *http.Response, target any) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		t.Fatalf("decoding JSON response: %v", err)
	}
}

// --- Auth middleware tests ---

func TestAuth_MissingToken(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/tasklists")
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status: got %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestAuth_WrongToken(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL+"/api/tasklists", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status: got %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestAuth_CorrectToken(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	resp, err := authGet(ts, "/api/tasklists")
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: got %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

// --- Task list tests ---

func TestListTaskLists_Empty(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	resp, err := authGet(ts, "/api/tasklists")
	if err != nil {
		t.Fatalf("request: %v", err)
	}

	var lists []taskListJSON
	decodeJSON(t, resp, &lists)

	if len(lists) != 0 {
		t.Errorf("expected empty list, got %d items", len(lists))
	}
}

func TestCreateAndGetTaskList(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	// Create a task list.
	resp, err := authReq(ts, "POST", "/api/tasklists", map[string]string{"name": "Work 03/2026"})
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create status: got %d, want %d", resp.StatusCode, http.StatusCreated)
	}

	var created taskListJSON
	decodeJSON(t, resp, &created)

	if created.Name != "Work 03/2026" {
		t.Errorf("name: got %q, want %q", created.Name, "Work 03/2026")
	}
	if created.Slug != "work-03-2026" {
		t.Errorf("slug: got %q, want %q", created.Slug, "work-03-2026")
	}

	// GET the created list.
	resp, err = authGet(ts, "/api/tasklists/work-03-2026")
	if err != nil {
		t.Fatalf("get request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get status: got %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var got taskListJSON
	decodeJSON(t, resp, &got)
	if got.Name != "Work 03/2026" {
		t.Errorf("name: got %q, want %q", got.Name, "Work 03/2026")
	}

	// List all task lists — should have 1.
	resp, err = authGet(ts, "/api/tasklists")
	if err != nil {
		t.Fatalf("list request: %v", err)
	}
	var lists []taskListJSON
	decodeJSON(t, resp, &lists)
	if len(lists) != 1 {
		t.Errorf("list count: got %d, want 1", len(lists))
	}
}

func TestUpdateTaskList(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	// Create, then update.
	authReq(ts, "POST", "/api/tasklists", map[string]string{"name": "Work"})

	body := map[string]any{
		"tasks": []map[string]any{
			{"description": "Review PRs", "duration": "~30m"},
			{"description": "Write docs", "duration": "~1h"},
		},
	}
	resp, err := authReq(ts, "PUT", "/api/tasklists/work", body)
	if err != nil {
		t.Fatalf("update request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("update status: got %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var updated taskListJSON
	decodeJSON(t, resp, &updated)
	if len(updated.Tasks) != 2 {
		t.Fatalf("task count: got %d, want 2", len(updated.Tasks))
	}
	if updated.Tasks[0].Description != "Review PRs" {
		t.Errorf("task[0] description: got %q, want %q", updated.Tasks[0].Description, "Review PRs")
	}
	if updated.Tasks[0].Duration != "~30m" {
		t.Errorf("task[0] duration: got %q, want %q", updated.Tasks[0].Duration, "~30m")
	}
	if updated.Tasks[1].Description != "Write docs" {
		t.Errorf("task[1] description: got %q, want %q", updated.Tasks[1].Description, "Write docs")
	}
	if updated.TotalDuration != "~1h30m" {
		t.Errorf("totalDuration: got %q, want %q", updated.TotalDuration, "~1h30m")
	}
}

func TestDeleteTaskList(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	authReq(ts, "POST", "/api/tasklists", map[string]string{"name": "Temp"})

	resp, err := authReq(ts, "DELETE", "/api/tasklists/temp", nil)
	if err != nil {
		t.Fatalf("delete request: %v", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("delete status: got %d, want %d", resp.StatusCode, http.StatusNoContent)
	}
	resp.Body.Close()

	// Verify it's gone.
	resp, err = authGet(ts, "/api/tasklists/temp")
	if err != nil {
		t.Fatalf("get request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("get after delete status: got %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

// --- Timebox tests ---

func TestCreateAndGetTimeboxes(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	// Create a timebox.
	body := map[string]string{"start": "09:00", "end": "11:00"}
	resp, err := authReq(ts, "POST", "/api/dates/2026-03-27/timeboxes", body)
	if err != nil {
		t.Fatalf("create timebox request: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create timebox status: got %d, want %d", resp.StatusCode, http.StatusCreated)
	}

	var dv dayViewJSON
	decodeJSON(t, resp, &dv)

	if dv.Date != "2026-03-27" {
		t.Errorf("date: got %q, want %q", dv.Date, "2026-03-27")
	}
	if len(dv.Timeboxes) != 1 {
		t.Fatalf("timebox count: got %d, want 1", len(dv.Timeboxes))
	}
	if dv.Timeboxes[0].Start != "09:00" {
		t.Errorf("start: got %q, want %q", dv.Timeboxes[0].Start, "09:00")
	}
	if dv.Timeboxes[0].End != "11:00" {
		t.Errorf("end: got %q, want %q", dv.Timeboxes[0].End, "11:00")
	}
	if dv.Timeboxes[0].Status != "unassigned" {
		t.Errorf("status: got %q, want %q", dv.Timeboxes[0].Status, "unassigned")
	}

	// GET the day view.
	resp, err = authGet(ts, "/api/dates/2026-03-27/timeboxes")
	if err != nil {
		t.Fatalf("get day view request: %v", err)
	}
	var gotDv dayViewJSON
	decodeJSON(t, resp, &gotDv)
	if len(gotDv.Timeboxes) != 1 {
		t.Errorf("get day view timebox count: got %d, want 1", len(gotDv.Timeboxes))
	}
}

func TestTimeboxAssignAndDone(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	// Create a task list with tasks.
	authReq(ts, "POST", "/api/tasklists", map[string]string{"name": "Work"})
	authReq(ts, "PUT", "/api/tasklists/work", map[string]any{
		"tasks": []map[string]any{
			{"description": "Task A", "duration": "~30m"},
			{"description": "Task B", "duration": "~45m"},
		},
	})

	// Create and assign a timebox.
	authReq(ts, "POST", "/api/dates/2026-03-27/timeboxes", map[string]string{"start": "09:00", "end": "11:00"})
	resp, err := authReq(ts, "POST", "/api/dates/2026-03-27/timeboxes/0/assign", map[string]string{"slug": "work"})
	if err != nil {
		t.Fatalf("assign request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("assign status: got %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var dv dayViewJSON
	decodeJSON(t, resp, &dv)
	if dv.Timeboxes[0].Status != "active" {
		t.Errorf("status after assign: got %q, want %q", dv.Timeboxes[0].Status, "active")
	}
	if dv.Timeboxes[0].TaskListSlug != "work" {
		t.Errorf("slug after assign: got %q, want %q", dv.Timeboxes[0].TaskListSlug, "work")
	}
	if dv.Timeboxes[0].TaskListName != "Work" {
		t.Errorf("task list name: got %q, want %q", dv.Timeboxes[0].TaskListName, "Work")
	}

	// Should have scheduled tasks.
	if len(dv.Timeboxes[0].ScheduledTasks) < 2 {
		t.Fatalf("scheduled tasks: got %d, want at least 2", len(dv.Timeboxes[0].ScheduledTasks))
	}

	// Mark task done.
	resp, err = authReq(ts, "POST", "/api/dates/2026-03-27/timeboxes/0/done", map[string]int{"taskIdx": 0})
	if err != nil {
		t.Fatalf("mark done request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("mark done status: got %d, want %d", resp.StatusCode, http.StatusOK)
	}

	decodeJSON(t, resp, &dv)
	if len(dv.Timeboxes[0].CompletedTasks) != 1 {
		t.Errorf("completed tasks: got %d, want 1", len(dv.Timeboxes[0].CompletedTasks))
	}
}

func TestArchiveTimebox_PendingTasks(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	// Set up: create list, timebox, assign.
	authReq(ts, "POST", "/api/tasklists", map[string]string{"name": "Work"})
	authReq(ts, "PUT", "/api/tasklists/work", map[string]any{
		"tasks": []map[string]any{
			{"description": "Task A", "duration": "~30m"},
		},
	})
	authReq(ts, "POST", "/api/dates/2026-03-27/timeboxes", map[string]string{"start": "09:00", "end": "11:00"})
	authReq(ts, "POST", "/api/dates/2026-03-27/timeboxes/0/assign", map[string]string{"slug": "work"})

	// Archive without force — should get 409 with pending count.
	resp, err := authReq(ts, "POST", "/api/dates/2026-03-27/timeboxes/0/archive", map[string]bool{"force": false})
	if err != nil {
		t.Fatalf("archive request: %v", err)
	}
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("archive status: got %d, want %d", resp.StatusCode, http.StatusConflict)
	}

	var errResp map[string]any
	decodeJSON(t, resp, &errResp)
	if _, ok := errResp["pendingCount"]; !ok {
		t.Error("response missing pendingCount field")
	}

	// Archive with force — should succeed.
	resp, err = authReq(ts, "POST", "/api/dates/2026-03-27/timeboxes/0/archive", map[string]bool{"force": true})
	if err != nil {
		t.Fatalf("force archive request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("force archive status: got %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var dv dayViewJSON
	decodeJSON(t, resp, &dv)
	if dv.Timeboxes[0].Status != "archived" {
		t.Errorf("status after archive: got %q, want %q", dv.Timeboxes[0].Status, "archived")
	}
}

// --- Notes tests ---

func TestNotesRoundTrip(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	// Read empty notes.
	resp, err := authGet(ts, "/api/notes")
	if err != nil {
		t.Fatalf("read notes request: %v", err)
	}
	var notes map[string]string
	decodeJSON(t, resp, &notes)
	if notes["content"] != "" {
		t.Errorf("initial notes: got %q, want empty", notes["content"])
	}

	// Write notes.
	resp, err = authReq(ts, "PUT", "/api/notes", map[string]string{"content": "Hello, world!"})
	if err != nil {
		t.Fatalf("write notes request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("write notes status: got %d, want %d", resp.StatusCode, http.StatusOK)
	}
	resp.Body.Close()

	// Read back.
	resp, err = authGet(ts, "/api/notes")
	if err != nil {
		t.Fatalf("read notes request: %v", err)
	}
	decodeJSON(t, resp, &notes)
	if notes["content"] != "Hello, world!" {
		t.Errorf("notes after write: got %q, want %q", notes["content"], "Hello, world!")
	}
}

// --- Week summary test ---

func TestGetWeekSummary(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	// Create a timebox on a known date.
	authReq(ts, "POST", "/api/dates/2026-03-27/timeboxes", map[string]string{"start": "09:00", "end": "11:00"})

	resp, err := authGet(ts, "/api/week?date=2026-03-27")
	if err != nil {
		t.Fatalf("week summary request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("week summary status: got %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var ws weekSummaryJSON
	decodeJSON(t, resp, &ws)
	if ws.Week.Year != 2026 {
		t.Errorf("year: got %d, want 2026", ws.Week.Year)
	}
	if len(ws.Days) != 7 {
		t.Fatalf("days count: got %d, want 7", len(ws.Days))
	}

	// Friday = index 5 (Sunday=0). Verify it has the timebox.
	if len(ws.Days[5].Timeboxes) != 1 {
		t.Errorf("Friday timeboxes: got %d, want 1", len(ws.Days[5].Timeboxes))
	}
}

// --- Reserved timebox test ---

func TestSetAndUnsetReserved(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	// Create a timebox.
	authReq(ts, "POST", "/api/dates/2026-03-27/timeboxes", map[string]string{"start": "09:00", "end": "11:00"})

	// Set reserved.
	resp, err := authReq(ts, "POST", "/api/dates/2026-03-27/timeboxes/0/reserve", map[string]string{"note": "Doctor visit"})
	if err != nil {
		t.Fatalf("set reserved request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("set reserved status: got %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var dv dayViewJSON
	decodeJSON(t, resp, &dv)
	if dv.Timeboxes[0].Tag != "reserved" {
		t.Errorf("tag: got %q, want %q", dv.Timeboxes[0].Tag, "reserved")
	}
	if dv.Timeboxes[0].Note != "Doctor visit" {
		t.Errorf("note: got %q, want %q", dv.Timeboxes[0].Note, "Doctor visit")
	}

	// Unset reserved. Use a fresh variable to avoid stale fields from the previous decode.
	resp, err = authReq(ts, "DELETE", "/api/dates/2026-03-27/timeboxes/0/reserve", nil)
	if err != nil {
		t.Fatalf("unset reserved request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unset reserved status: got %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var dv2 dayViewJSON
	decodeJSON(t, resp, &dv2)
	if dv2.Timeboxes[0].Tag != "" {
		t.Errorf("tag after unset: got %q, want empty", dv2.Timeboxes[0].Tag)
	}
	if dv2.Timeboxes[0].Status != "unassigned" {
		t.Errorf("status after unset: got %q, want %q", dv2.Timeboxes[0].Status, "unassigned")
	}
}
