package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/mhwang-1/meadow-bubbletea/internal/domain"
	"github.com/mhwang-1/meadow-bubbletea/internal/service"
)

// scheduledTaskJSON is the API representation of a scheduled task.
type scheduledTaskJSON struct {
	Description string `json:"description"`
	Duration    string `json:"duration"`
	StartTime   string `json:"startTime"`
	IsBreak     bool   `json:"isBreak,omitempty"`
}

// completedTaskJSON is the API representation of a completed task within a timebox.
type completedTaskJSON struct {
	Description string `json:"description"`
	Duration    string `json:"duration"`
}

// dayViewTimeboxJSON is the API representation of a timebox within a day view.
type dayViewTimeboxJSON struct {
	Index          int                 `json:"index"`
	Start          string              `json:"start"`
	End            string              `json:"end"`
	Status         string              `json:"status"`
	TaskListSlug   string              `json:"taskListSlug,omitempty"`
	TaskListName   string              `json:"taskListName,omitempty"`
	Tag            string              `json:"tag,omitempty"`
	Note           string              `json:"note,omitempty"`
	ScheduledTasks []scheduledTaskJSON `json:"scheduledTasks"`
	CompletedTasks []completedTaskJSON `json:"completedTasks"`
}

// dayViewJSON is the API representation of a day view.
type dayViewJSON struct {
	Date      string               `json:"date"`
	Timeboxes []dayViewTimeboxJSON `json:"timeboxes"`
}

func toDayViewJSON(dv *service.DayView) dayViewJSON {
	timeboxes := make([]dayViewTimeboxJSON, len(dv.Timeboxes))
	for i, dvt := range dv.Timeboxes {
		scheduled := make([]scheduledTaskJSON, len(dvt.ScheduledTasks))
		for j, st := range dvt.ScheduledTasks {
			scheduled[j] = scheduledTaskJSON{
				Description: st.Task.Description,
				Duration:    "~" + domain.FormatDuration(st.Task.Duration),
				StartTime:   st.StartTime.Format("15:04"),
				IsBreak:     st.IsBreak,
			}
		}

		completed := make([]completedTaskJSON, len(dvt.Timebox.CompletedTasks))
		for j, ct := range dvt.Timebox.CompletedTasks {
			completed[j] = completedTaskJSON{
				Description: ct.Description,
				Duration:    "~" + domain.FormatDuration(ct.Duration),
			}
		}

		timeboxes[i] = dayViewTimeboxJSON{
			Index:          dvt.Index,
			Start:          dvt.Timebox.Start.Format("15:04"),
			End:            dvt.Timebox.End.Format("15:04"),
			Status:         string(dvt.Timebox.Status),
			TaskListSlug:   dvt.Timebox.TaskListSlug,
			TaskListName:   dvt.TaskListName,
			Tag:            dvt.Timebox.Tag,
			Note:           dvt.Timebox.Note,
			ScheduledTasks: scheduled,
			CompletedTasks: completed,
		}
	}

	return dayViewJSON{
		Date:      dv.Date.Format("2006-01-02"),
		Timeboxes: timeboxes,
	}
}

func (s *Server) handleGetDayView(w http.ResponseWriter, r *http.Request) {
	dateStr := r.PathValue("date")
	date, err := parseDate(dateStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid date format: %s (expected YYYY-MM-DD)", dateStr))
		return
	}

	dv, err := s.svc.GetDayView(date)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, toDayViewJSON(dv))
}

// parseTimeOnDate parses an "HH:MM" string and places it on the given date.
func parseTimeOnDate(date time.Time, timeStr string) (time.Time, error) {
	t, err := time.Parse("15:04", timeStr)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid time format: %s (expected HH:MM)", timeStr)
	}
	return time.Date(
		date.Year(), date.Month(), date.Day(),
		t.Hour(), t.Minute(), 0, 0,
		date.Location(),
	), nil
}

func (s *Server) handleCreateTimebox(w http.ResponseWriter, r *http.Request) {
	dateStr := r.PathValue("date")
	date, err := parseDate(dateStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid date format: %s", dateStr))
		return
	}

	var body struct {
		Start string `json:"start"`
		End   string `json:"end"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	start, err := parseTimeOnDate(date, body.Start)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	end, err := parseTimeOnDate(date, body.End)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := s.svc.CreateTimebox(date, start, end); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Return the updated day view.
	dv, err := s.svc.GetDayView(date)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, toDayViewJSON(dv))
}

// parseTimeboxIdx extracts the timebox index from the URL path.
func parseTimeboxIdx(r *http.Request) (int, error) {
	idxStr := r.PathValue("idx")
	idx, err := strconv.Atoi(idxStr)
	if err != nil {
		return 0, fmt.Errorf("invalid timebox index: %s", idxStr)
	}
	return idx, nil
}

func (s *Server) handleEditTimeboxTime(w http.ResponseWriter, r *http.Request) {
	dateStr := r.PathValue("date")
	date, err := parseDate(dateStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid date format: %s", dateStr))
		return
	}

	idx, err := parseTimeboxIdx(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	var body struct {
		Start string `json:"start"`
		End   string `json:"end"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	start, err := parseTimeOnDate(date, body.Start)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	end, err := parseTimeOnDate(date, body.End)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := s.svc.EditTimeboxTime(date, idx, start, end); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	dv, err := s.svc.GetDayView(date)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, toDayViewJSON(dv))
}

func (s *Server) handleDeleteTimebox(w http.ResponseWriter, r *http.Request) {
	dateStr := r.PathValue("date")
	date, err := parseDate(dateStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid date format: %s", dateStr))
		return
	}

	idx, err := parseTimeboxIdx(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := s.svc.DeleteTimebox(date, idx); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAssignTaskList(w http.ResponseWriter, r *http.Request) {
	dateStr := r.PathValue("date")
	date, err := parseDate(dateStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid date format: %s", dateStr))
		return
	}

	idx, err := parseTimeboxIdx(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	var body struct {
		Slug string `json:"slug"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if body.Slug == "" {
		writeError(w, http.StatusBadRequest, "slug is required")
		return
	}

	if err := s.svc.AssignTaskList(date, idx, body.Slug); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	dv, err := s.svc.GetDayView(date)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, toDayViewJSON(dv))
}

func (s *Server) handleSetReserved(w http.ResponseWriter, r *http.Request) {
	dateStr := r.PathValue("date")
	date, err := parseDate(dateStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid date format: %s", dateStr))
		return
	}

	idx, err := parseTimeboxIdx(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	var body struct {
		Note string `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if err := s.svc.SetReserved(date, idx, body.Note); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	dv, err := s.svc.GetDayView(date)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, toDayViewJSON(dv))
}

func (s *Server) handleUnsetReserved(w http.ResponseWriter, r *http.Request) {
	dateStr := r.PathValue("date")
	date, err := parseDate(dateStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid date format: %s", dateStr))
		return
	}

	idx, err := parseTimeboxIdx(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := s.svc.UnsetReserved(date, idx); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	dv, err := s.svc.GetDayView(date)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, toDayViewJSON(dv))
}

func (s *Server) handleMarkTaskDone(w http.ResponseWriter, r *http.Request) {
	dateStr := r.PathValue("date")
	date, err := parseDate(dateStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid date format: %s", dateStr))
		return
	}

	idx, err := parseTimeboxIdx(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	var body struct {
		TaskIdx int `json:"taskIdx"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if err := s.svc.MarkTaskDone(date, idx, body.TaskIdx); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	dv, err := s.svc.GetDayView(date)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, toDayViewJSON(dv))
}

func (s *Server) handleUnmarkTaskDone(w http.ResponseWriter, r *http.Request) {
	dateStr := r.PathValue("date")
	date, err := parseDate(dateStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid date format: %s", dateStr))
		return
	}

	idx, err := parseTimeboxIdx(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	var body struct {
		TaskIdx int `json:"taskIdx"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if err := s.svc.UnmarkTaskDone(date, idx, body.TaskIdx); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	dv, err := s.svc.GetDayView(date)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, toDayViewJSON(dv))
}

func (s *Server) handleArchiveTimebox(w http.ResponseWriter, r *http.Request) {
	dateStr := r.PathValue("date")
	date, err := parseDate(dateStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid date format: %s", dateStr))
		return
	}

	idx, err := parseTimeboxIdx(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	var body struct {
		Force bool `json:"force"`
	}
	// Body is optional; default force=false.
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err != io.EOF {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if err := s.svc.ArchiveTimebox(date, idx, body.Force); err != nil {
		var pendingErr *service.PendingTasksError
		if errors.As(err, &pendingErr) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(map[string]any{
				"error":        fmt.Sprintf("%d pending task(s)", pendingErr.Count),
				"pendingCount": pendingErr.Count,
			})
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	dv, err := s.svc.GetDayView(date)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, toDayViewJSON(dv))
}

func (s *Server) handleUnarchiveTimebox(w http.ResponseWriter, r *http.Request) {
	dateStr := r.PathValue("date")
	date, err := parseDate(dateStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid date format: %s", dateStr))
		return
	}

	idx, err := parseTimeboxIdx(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := s.svc.UnarchiveTimebox(date, idx); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	dv, err := s.svc.GetDayView(date)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, toDayViewJSON(dv))
}
