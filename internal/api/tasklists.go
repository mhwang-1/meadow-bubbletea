package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/mhwang-1/meadow-bubbletea/internal/domain"
)

// taskJSON is the API representation of a task.
type taskJSON struct {
	Description string `json:"description"`
	Duration    string `json:"duration"`
	Commented   bool   `json:"commented,omitempty"`
}

// taskListJSON is the API representation of a task list.
type taskListJSON struct {
	Name          string     `json:"name"`
	Slug          string     `json:"slug"`
	Tasks         []taskJSON `json:"tasks"`
	TotalDuration string     `json:"totalDuration"`
}

func toTaskJSON(t domain.Task) taskJSON {
	return taskJSON{
		Description: t.Description,
		Duration:    "~" + domain.FormatDuration(t.Duration),
		Commented:   t.Commented,
	}
}

func toTaskListJSON(tl *domain.TaskList) taskListJSON {
	tasks := make([]taskJSON, len(tl.Tasks))
	for i, t := range tl.Tasks {
		tasks[i] = toTaskJSON(t)
	}
	return taskListJSON{
		Name:          tl.Name,
		Slug:          tl.Slug,
		Tasks:         tasks,
		TotalDuration: "~" + domain.FormatDuration(tl.TotalDuration()),
	}
}

func parseTasks(raw []taskJSON) ([]domain.Task, error) {
	tasks := make([]domain.Task, len(raw))
	for i, rt := range raw {
		dur, err := domain.ParseDuration(rt.Duration)
		if err != nil {
			return nil, err
		}
		tasks[i] = domain.Task{
			Description: rt.Description,
			Duration:    dur,
			Commented:   rt.Commented,
		}
	}
	return tasks, nil
}

func (s *Server) handleListTaskLists(w http.ResponseWriter, r *http.Request) {
	lists, err := s.svc.ListTaskLists()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	result := make([]taskListJSON, 0, len(lists))
	for _, tl := range lists {
		result = append(result, toTaskListJSON(tl))
	}

	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleCreateTaskList(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if body.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	tl, err := s.svc.CreateTaskList(body.Name)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, toTaskListJSON(tl))
}

func (s *Server) handleGetTaskList(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")

	tl, err := s.svc.GetTaskList(slug)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, toTaskListJSON(tl))
}

func (s *Server) handleUpdateTaskList(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")

	var body struct {
		Tasks []taskJSON `json:"tasks"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	tasks, err := parseTasks(body.Tasks)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := s.svc.UpdateTaskList(slug, tasks); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Re-read to return the updated list.
	tl, err := s.svc.GetTaskList(slug)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, toTaskListJSON(tl))
}

func (s *Server) handleDeleteTaskList(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")

	if err := s.svc.DeleteTaskList(slug); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleArchiveTaskList(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")

	if err := s.svc.ArchiveTaskList(slug); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "archived"})
}

// parseDate parses a YYYY-MM-DD date string into a time.Time.
func parseDate(s string) (time.Time, error) {
	return time.Parse("2006-01-02", s)
}
