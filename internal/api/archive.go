package api

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/mhwang-1/meadow-bubbletea/internal/domain"
	"github.com/mhwang-1/meadow-bubbletea/internal/service"
	"github.com/mhwang-1/meadow-bubbletea/internal/store"
)

// weekSummaryJSON is the API representation of a week summary.
type weekSummaryJSON struct {
	Week weekInfoJSON     `json:"week"`
	Days []daySummaryJSON `json:"days"`
}

// weekInfoJSON is the API representation of a WeekInfo.
type weekInfoJSON struct {
	Year      int    `json:"year"`
	Week      int    `json:"week"`
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
}

// daySummaryJSON is the API representation of a day within a week summary.
type daySummaryJSON struct {
	Date      string               `json:"date"`
	Timeboxes []dayViewTimeboxJSON `json:"timeboxes"`
}

func toWeekSummaryJSON(ws *service.WeekSummary) weekSummaryJSON {
	days := make([]daySummaryJSON, 7)
	for i, ds := range ws.Days {
		timeboxes := make([]dayViewTimeboxJSON, len(ds.Timeboxes))
		for j, dvt := range ds.Timeboxes {
			scheduled := make([]scheduledTaskJSON, len(dvt.ScheduledTasks))
			for k, st := range dvt.ScheduledTasks {
				scheduled[k] = scheduledTaskJSON{
					Description: st.Task.Description,
					Duration:    "~" + domain.FormatDuration(st.Task.Duration),
					StartTime:   st.StartTime.Format("15:04"),
					IsBreak:     st.IsBreak,
				}
			}

			completed := make([]completedTaskJSON, len(dvt.Timebox.CompletedTasks))
			for k, ct := range dvt.Timebox.CompletedTasks {
				completed[k] = completedTaskJSON{
					Description: ct.Description,
					Duration:    "~" + domain.FormatDuration(ct.Duration),
				}
			}

			timeboxes[j] = dayViewTimeboxJSON{
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

		days[i] = daySummaryJSON{
			Date:      ds.Date.Format("2006-01-02"),
			Timeboxes: timeboxes,
		}
	}

	return weekSummaryJSON{
		Week: weekInfoJSON{
			Year:      ws.Week.Year,
			Week:      ws.Week.Week,
			StartDate: ws.Week.StartDate.Format("2006-01-02"),
			EndDate:   ws.Week.EndDate.Format("2006-01-02"),
		},
		Days: days,
	}
}

// completedTaskResponseJSON is the API representation of a completed task from the archive.
type completedTaskResponseJSON struct {
	Description   string `json:"description"`
	Duration      string `json:"duration"`
	CompletedDate string `json:"completedDate"`
	TaskListSlug  string `json:"taskListSlug"`
}

func toCompletedTasksJSON(tasks []store.CompletedTask) []completedTaskResponseJSON {
	result := make([]completedTaskResponseJSON, len(tasks))
	for i, ct := range tasks {
		result[i] = completedTaskResponseJSON{
			Description:   ct.Task.Description,
			Duration:      "~" + domain.FormatDuration(ct.Task.Duration),
			CompletedDate: ct.CompletedDate.Format("2006-01-02"),
			TaskListSlug:  ct.TaskListSlug,
		}
	}
	return result
}

// archivedTimeboxJSON is the API representation of an archived timebox.
type archivedTimeboxJSON struct {
	Date           string              `json:"date"`
	Start          string              `json:"start"`
	End            string              `json:"end"`
	TaskListSlug   string              `json:"taskListSlug,omitempty"`
	Tag            string              `json:"tag,omitempty"`
	Note           string              `json:"note,omitempty"`
	CompletedTasks []completedTaskJSON `json:"completedTasks"`
}

func toArchivedTimeboxesJSON(timeboxes []store.ArchivedTimebox) []archivedTimeboxJSON {
	result := make([]archivedTimeboxJSON, len(timeboxes))
	for i, at := range timeboxes {
		completed := make([]completedTaskJSON, len(at.CompletedTasks))
		for j, ct := range at.CompletedTasks {
			completed[j] = completedTaskJSON{
				Description: ct.Description,
				Duration:    "~" + domain.FormatDuration(ct.Duration),
			}
		}

		result[i] = archivedTimeboxJSON{
			Date:           at.Date.Format("2006-01-02"),
			Start:          at.Start.Format("15:04"),
			End:            at.End.Format("15:04"),
			TaskListSlug:   at.TaskListSlug,
			Tag:            at.Tag,
			Note:           at.Note,
			CompletedTasks: completed,
		}
	}
	return result
}

// archivedTaskListGroupJSON represents archived task lists for a specific year/week.
type archivedTaskListGroupJSON struct {
	Year  int            `json:"year"`
	Week  int            `json:"week"`
	Lists []taskListJSON `json:"lists"`
}

func toArchivedTaskListsJSON(data map[int]map[int][]*domain.TaskList) []archivedTaskListGroupJSON {
	var result []archivedTaskListGroupJSON
	for year, weeks := range data {
		for week, lists := range weeks {
			group := archivedTaskListGroupJSON{
				Year:  year,
				Week:  week,
				Lists: make([]taskListJSON, len(lists)),
			}
			for i, tl := range lists {
				group.Lists[i] = toTaskListJSON(tl)
			}
			result = append(result, group)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Year != result[j].Year {
			return result[i].Year < result[j].Year
		}
		return result[i].Week < result[j].Week
	})
	return result
}

func (s *Server) handleGetWeekSummary(w http.ResponseWriter, r *http.Request) {
	dateStr := r.URL.Query().Get("date")
	var date time.Time
	var err error
	if dateStr == "" {
		date = time.Now()
	} else {
		date, err = parseDate(dateStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid date format: %s (expected YYYY-MM-DD)", dateStr))
			return
		}
	}

	ws, err := s.svc.GetWeekSummary(date)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, toWeekSummaryJSON(ws))
}

func (s *Server) handleGetHistory(w http.ResponseWriter, r *http.Request) {
	yearStr := r.PathValue("year")
	weekStr := r.PathValue("week")

	year, err := strconv.Atoi(yearStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid year: %s", yearStr))
		return
	}
	week, err := strconv.Atoi(weekStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid week: %s", weekStr))
		return
	}

	tasks, err := s.svc.GetHistory(year, week)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, toCompletedTasksJSON(tasks))
}

func (s *Server) handleGetArchivedTimeboxes(w http.ResponseWriter, r *http.Request) {
	yearStr := r.PathValue("year")
	weekStr := r.PathValue("week")

	year, err := strconv.Atoi(yearStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid year: %s", yearStr))
		return
	}
	week, err := strconv.Atoi(weekStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid week: %s", weekStr))
		return
	}

	timeboxes, err := s.svc.GetArchivedTimeboxes(year, week)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, toArchivedTimeboxesJSON(timeboxes))
}

func (s *Server) handleGetArchivedTaskLists(w http.ResponseWriter, r *http.Request) {
	data, err := s.svc.GetArchivedTaskLists()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, toArchivedTaskListsJSON(data))
}
