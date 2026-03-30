package api

// registerRoutes registers all API routes on the server's mux using Go 1.22+
// method routing patterns.
func (s *Server) registerRoutes() {
	// Task lists
	s.mux.HandleFunc("GET /api/tasklists", s.handleListTaskLists)
	s.mux.HandleFunc("POST /api/tasklists", s.handleCreateTaskList)
	s.mux.HandleFunc("GET /api/tasklists/{slug}", s.handleGetTaskList)
	s.mux.HandleFunc("PUT /api/tasklists/{slug}", s.handleUpdateTaskList)
	s.mux.HandleFunc("DELETE /api/tasklists/{slug}", s.handleDeleteTaskList)
	s.mux.HandleFunc("POST /api/tasklists/{slug}/archive", s.handleArchiveTaskList)

	// Timeboxes
	s.mux.HandleFunc("GET /api/dates/{date}/timeboxes", s.handleGetDayView)
	s.mux.HandleFunc("POST /api/dates/{date}/timeboxes", s.handleCreateTimebox)
	s.mux.HandleFunc("PUT /api/dates/{date}/timeboxes/{idx}", s.handleEditTimeboxTime)
	s.mux.HandleFunc("DELETE /api/dates/{date}/timeboxes/{idx}", s.handleDeleteTimebox)
	s.mux.HandleFunc("POST /api/dates/{date}/timeboxes/{idx}/assign", s.handleAssignTaskList)
	s.mux.HandleFunc("POST /api/dates/{date}/timeboxes/{idx}/reserve", s.handleSetReserved)
	s.mux.HandleFunc("DELETE /api/dates/{date}/timeboxes/{idx}/reserve", s.handleUnsetReserved)
	s.mux.HandleFunc("POST /api/dates/{date}/timeboxes/{idx}/done", s.handleMarkTaskDone)
	s.mux.HandleFunc("DELETE /api/dates/{date}/timeboxes/{idx}/done", s.handleUnmarkTaskDone)
	s.mux.HandleFunc("POST /api/dates/{date}/timeboxes/{idx}/archive", s.handleArchiveTimebox)
	s.mux.HandleFunc("DELETE /api/dates/{date}/timeboxes/{idx}/archive", s.handleUnarchiveTimebox)

	// Archive and queries
	s.mux.HandleFunc("GET /api/week", s.handleGetWeekSummary)
	s.mux.HandleFunc("GET /api/archive/{year}/{week}/completed", s.handleGetHistory)
	s.mux.HandleFunc("GET /api/archive/{year}/{week}/timeboxes", s.handleGetArchivedTimeboxes)
	s.mux.HandleFunc("GET /api/archive/tasklists", s.handleGetArchivedTaskLists)

	// Notes
	s.mux.HandleFunc("GET /api/notes", s.handleReadNotes)
	s.mux.HandleFunc("PUT /api/notes", s.handleWriteNotes)
}
