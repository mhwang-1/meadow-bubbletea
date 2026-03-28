package model

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hwang/meadow-bubbletea/internal/domain"
	"github.com/hwang/meadow-bubbletea/internal/store"
	"github.com/hwang/meadow-bubbletea/internal/ui"
)

// EditorMode represents the active sub-mode within the task list editor.
type EditorMode int

const (
	EditorModeEdit    EditorMode = iota // nano-like text editing
	EditorModeExecute                   // mark tasks as done
	EditorModeArchive                   // browse completed tasks by week
)

// TaskListEditorModel holds the state for the full-screen task list editor overlay.
type TaskListEditorModel struct {
	slug          string
	name          string
	editorMode    EditorMode
	readOnly      bool              // true for archived task lists (no editing)
	editor        EditorModel       // the nano-like text editor (Edit mode)
	executeCursor int               // cursor for Execute mode
	archiveYear   int               // selected year pill
	archiveWeek   int               // selected week pill
	archiveYears  []int             // available years (descending)
	archiveWeeks  map[int][]int     // available weeks per year (descending)
	archiveTasks  []archiveTaskItem // tasks for the selected year/week
	archiveCursor int               // cursor for Archive mode task selection
}

type archiveTaskItem struct {
	task          domain.Task
	completedDate time.Time
	dateStr       string // formatted completion date
	slug          string // task list slug
}

// openTaskListEditor loads the task list and initialises the editor model.
func (m *RootModel) openTaskListEditor(slug string) {
	tl, err := m.store.ReadTaskList(slug)
	if err != nil {
		// Cannot open — silently bail.
		return
	}

	// Build the raw text content from the task list.
	var lines []string
	for _, t := range tl.Tasks {
		lines = append(lines, domain.FormatTaskLine(t))
	}
	content := strings.Join(lines, "\n")

	// Reserve space: 3 lines for header/pills, rest for editor.
	editorHeight := m.height - 3
	if editorHeight < 5 {
		editorHeight = 5
	}

	editor := NewEditorModel(content, m.width, editorHeight)

	// Calculate stats line.
	statsLine := m.calculateTaskListStats(tl)
	editor.SetStatsLine(statsLine)

	m.taskListEditor = TaskListEditorModel{
		slug:         slug,
		name:         tl.Name,
		editorMode:   EditorModeEdit,
		editor:       editor,
		archiveWeeks: make(map[int][]int),
	}

	m.showTaskListEditor = true
	m.editingTaskListSlug = slug
}

// closeTaskListEditor hides the task list editor overlay.
func (m *RootModel) closeTaskListEditor() {
	m.showTaskListEditor = false
}

// openArchivedTaskListEditor opens an archived task list in read-only Archive mode.
func (m *RootModel) openArchivedTaskListEditor(year, week int, slug string) {
	tl, err := m.store.ReadArchivedTaskList(year, week, slug)
	if err != nil {
		return
	}

	m.taskListEditor = TaskListEditorModel{
		slug:         slug,
		name:         tl.Name,
		editorMode:   EditorModeArchive,
		readOnly:     true,
		archiveYear:  year,
		archiveWeek:  week,
		archiveWeeks: make(map[int][]int),
	}

	// Load archive years/weeks and data for this slug.
	m.loadArchiveYearsAndWeeks()
	m.taskListEditor.archiveYear = year
	m.taskListEditor.archiveWeek = week
	m.loadArchiveData()

	m.showTaskListEditor = true
	m.editingTaskListSlug = slug
}

// handleTaskListEditorKey routes key events based on the current editor sub-mode.
func (m *RootModel) handleTaskListEditorKey(msg tea.KeyMsg) tea.Cmd {
	key := msg.String()

	// Read-only mode: only allow archive mode navigation.
	if m.taskListEditor.readOnly {
		return m.handleArchiveModeKey(key)
	}

	// Tab switches between sub-modes in all modes.
	if key == "tab" {
		switch m.taskListEditor.editorMode {
		case EditorModeEdit:
			m.taskListEditor.editorMode = EditorModeExecute
			m.taskListEditor.executeCursor = 0
			// Reload active tasks so Execute mode is up to date.
			m.reloadExecuteTasks()
		case EditorModeExecute:
			m.taskListEditor.editorMode = EditorModeArchive
			m.loadArchiveYearsAndWeeks()
			m.loadArchiveData()
		case EditorModeArchive:
			m.taskListEditor.editorMode = EditorModeEdit
		}
		return nil
	}

	switch m.taskListEditor.editorMode {
	case EditorModeEdit:
		return m.handleEditorModeKey(msg)
	case EditorModeExecute:
		return m.handleExecuteModeKey(key)
	case EditorModeArchive:
		return m.handleArchiveModeKey(key)
	}

	return nil
}

// handleEditorModeKey delegates to the nano-like EditorModel.
func (m *RootModel) handleEditorModeKey(msg tea.KeyMsg) tea.Cmd {
	key := msg.String()
	if key == "esc" {
		if m.taskListEditor.editor.IsModified() {
			// Prompt save before closing.
			m.showConfirm = true
			m.confirmAction = func() {
				if ok, _ := m.saveEditorContent(); ok {
					m.closeTaskListEditor()
				}
			}
			return nil
		}
		m.closeTaskListEditor()
		return nil
	}

	saved, closed := m.taskListEditor.editor.HandleKey(msg)

	if saved {
		ok, msg := m.saveEditorContent()
		if ok {
			return m.showToast("Saved", 2*time.Second)
		}
		return m.showToast(msg, 3*time.Second)
	}

	if closed {
		m.closeTaskListEditor()
	}

	return nil
}

// saveEditorContent parses editor text, validates task format, and writes it.
func (m *RootModel) saveEditorContent() (bool, string) {
	content := m.taskListEditor.editor.Content()
	lines := strings.Split(content, "\n")

	var tasks []domain.Task
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		task, err := domain.ParseTaskLine(trimmed)
		if err != nil {
			return false, fmt.Sprintf("Invalid task format on line %d", i+1)
		}
		tasks = append(tasks, task)
	}

	tl := &domain.TaskList{
		Name:  m.taskListEditor.name,
		Slug:  m.taskListEditor.slug,
		Tasks: tasks,
	}

	if err := m.store.WriteTaskList(tl); err != nil {
		return false, "Failed to save task list"
	}

	// Recalculate stats after save.
	statsLine := m.calculateTaskListStats(tl)
	m.taskListEditor.editor.SetStatsLine(statsLine)
	m.taskListEditor.editor.modified = false
	return true, ""
}

// reloadExecuteTasks refreshes the task list from the store for Execute mode.
func (m *RootModel) reloadExecuteTasks() {
	// The execute cursor will be clamped by the render function.
}

// handleExecuteModeKey handles keys in Execute mode: navigate and mark done.
func (m *RootModel) handleExecuteModeKey(key string) tea.Cmd {
	if key == "esc" {
		m.closeTaskListEditor()
		return nil
	}

	// Read the current task list to get active (non-commented) tasks.
	tl, err := m.store.ReadTaskList(m.taskListEditor.slug)
	if err != nil {
		return nil
	}
	activeTasks := tl.ActiveTasks()

	switch key {
	case "up":
		if m.taskListEditor.executeCursor > 0 {
			m.taskListEditor.executeCursor--
		}

	case "down":
		if m.taskListEditor.executeCursor < len(activeTasks)-1 {
			m.taskListEditor.executeCursor++
		}

	case "enter", "x":
		if len(activeTasks) == 0 || m.taskListEditor.executeCursor >= len(activeTasks) {
			return nil
		}

		selectedTask := activeTasks[m.taskListEditor.executeCursor]

		// Remove the task from the active task list.
		var updatedTasks []domain.Task
		removed := false
		for _, t := range tl.Tasks {
			if !removed && !t.Commented &&
				t.Description == selectedTask.Description &&
				t.Duration == selectedTask.Duration {
				removed = true
				continue
			}
			updatedTasks = append(updatedTasks, t)
		}
		tl.Tasks = updatedTasks

		if err := m.store.WriteTaskList(tl); err != nil {
			return nil
		}

		// Add to archive completed.md for the current week.
		wi := domain.WeekForDate(m.currentDate)
		completedTask := store.CompletedTask{
			Task:          selectedTask,
			CompletedDate: domain.StripTimeForDate(m.currentDate),
			TaskListSlug:  m.taskListEditor.slug,
		}
		_ = m.store.AddCompleted(wi.Year, wi.Week, completedTask)

		// Adjust cursor.
		newActive := tl.ActiveTasks()
		if m.taskListEditor.executeCursor >= len(newActive) && m.taskListEditor.executeCursor > 0 {
			m.taskListEditor.executeCursor--
		}

		// Update stats in the editor.
		statsLine := m.calculateTaskListStats(tl)
		m.taskListEditor.editor.SetStatsLine(statsLine)
	}

	return nil
}

// handleArchiveModeKey handles keys in Archive mode: navigate pills and unmark.
func (m *RootModel) handleArchiveModeKey(key string) tea.Cmd {
	if key == "esc" {
		m.closeTaskListEditor()
		return nil
	}

	switch key {
	case "up":
		// Navigate year pills (move to previous = earlier year in the sorted descending list).
		yearIdx := m.archiveYearIndex()
		if yearIdx < len(m.taskListEditor.archiveYears)-1 {
			m.taskListEditor.archiveYear = m.taskListEditor.archiveYears[yearIdx+1]
			// Reset to the first available week for the new year.
			weeks := m.taskListEditor.archiveWeeks[m.taskListEditor.archiveYear]
			if len(weeks) > 0 {
				m.taskListEditor.archiveWeek = weeks[0]
			}
			m.taskListEditor.archiveCursor = 0
			m.loadArchiveData()
		}

	case "down":
		// Navigate year pills (move to next = more recent year in the sorted descending list).
		yearIdx := m.archiveYearIndex()
		if yearIdx > 0 {
			m.taskListEditor.archiveYear = m.taskListEditor.archiveYears[yearIdx-1]
			weeks := m.taskListEditor.archiveWeeks[m.taskListEditor.archiveYear]
			if len(weeks) > 0 {
				m.taskListEditor.archiveWeek = weeks[0]
			}
			m.taskListEditor.archiveCursor = 0
			m.loadArchiveData()
		}

	case "left":
		// Navigate week pills (move to older week).
		weekIdx := m.archiveWeekIndex()
		weeks := m.taskListEditor.archiveWeeks[m.taskListEditor.archiveYear]
		if weekIdx < len(weeks)-1 {
			m.taskListEditor.archiveWeek = weeks[weekIdx+1]
			m.taskListEditor.archiveCursor = 0
			m.loadArchiveData()
		}

	case "right":
		// Navigate week pills (move to newer week).
		weekIdx := m.archiveWeekIndex()
		if weekIdx > 0 {
			weeks := m.taskListEditor.archiveWeeks[m.taskListEditor.archiveYear]
			m.taskListEditor.archiveWeek = weeks[weekIdx-1]
			m.taskListEditor.archiveCursor = 0
			m.loadArchiveData()
		}

	case "j":
		// Move archive cursor down.
		if m.taskListEditor.archiveCursor < len(m.taskListEditor.archiveTasks)-1 {
			m.taskListEditor.archiveCursor++
		}

	case "k":
		// Move archive cursor up.
		if m.taskListEditor.archiveCursor > 0 {
			m.taskListEditor.archiveCursor--
		}

	case "u":
		// Unmark is not available in read-only mode.
		if m.taskListEditor.readOnly {
			return nil
		}
		// Unmark: move the selected archived task back to the active task list.
		if len(m.taskListEditor.archiveTasks) == 0 ||
			m.taskListEditor.archiveCursor >= len(m.taskListEditor.archiveTasks) {
			return nil
		}

		item := m.taskListEditor.archiveTasks[m.taskListEditor.archiveCursor]

		// Add back to the active task list.
		tl, err := m.store.ReadTaskList(m.taskListEditor.slug)
		if err != nil {
			return nil
		}
		tl.Tasks = append(tl.Tasks, item.task)
		if err := m.store.WriteTaskList(tl); err != nil {
			return nil
		}

		// Remove from daily completed tasks and archive completed.md.
		_, _ = m.removeTaskFromDailyCompleted(item.completedDate, item.slug, item.task)
		_, _ = m.removeCompletedArchiveEntry(m.taskListEditor.archiveYear, m.taskListEditor.archiveWeek, store.CompletedTask{
			Task:          item.task,
			CompletedDate: item.completedDate,
			TaskListSlug:  item.slug,
		})

		// Adjust cursor.
		if m.taskListEditor.archiveCursor >= len(m.taskListEditor.archiveTasks) && m.taskListEditor.archiveCursor > 0 {
			m.taskListEditor.archiveCursor--
		}

		// Reload archive data and stats.
		m.loadArchiveData()
		tl, _ = m.store.ReadTaskList(m.taskListEditor.slug)
		if tl != nil {
			statsLine := m.calculateTaskListStats(tl)
			m.taskListEditor.editor.SetStatsLine(statsLine)
		}
	}

	return nil
}

// archiveYearIndex returns the index of the current archiveYear in the sorted archiveYears slice.
func (m *RootModel) archiveYearIndex() int {
	for i, y := range m.taskListEditor.archiveYears {
		if y == m.taskListEditor.archiveYear {
			return i
		}
	}
	return 0
}

// archiveWeekIndex returns the index of the current archiveWeek in the weeks for the current year.
func (m *RootModel) archiveWeekIndex() int {
	weeks := m.taskListEditor.archiveWeeks[m.taskListEditor.archiveYear]
	for i, w := range weeks {
		if w == m.taskListEditor.archiveWeek {
			return i
		}
	}
	return 0
}

// loadArchiveYearsAndWeeks scans the archive directory structure to discover
// available years and weeks.
func (m *RootModel) loadArchiveYearsAndWeeks() {
	archiveDir := filepath.Join(m.store.DataDir, "archive")

	m.taskListEditor.archiveYears = nil
	m.taskListEditor.archiveWeeks = make(map[int][]int)

	yearEntries, err := os.ReadDir(archiveDir)
	if err != nil {
		// No archive directory yet — set defaults to current week.
		wi := domain.WeekForDate(m.currentDate)
		m.taskListEditor.archiveYears = []int{wi.Year}
		m.taskListEditor.archiveWeeks[wi.Year] = []int{wi.Week}
		m.taskListEditor.archiveYear = wi.Year
		m.taskListEditor.archiveWeek = wi.Week
		return
	}

	yearsMap := make(map[int][]int)

	for _, ye := range yearEntries {
		if !ye.IsDir() {
			continue
		}
		year, err := strconv.Atoi(ye.Name())
		if err != nil {
			continue
		}

		weekDir := filepath.Join(archiveDir, ye.Name())
		weekEntries, err := os.ReadDir(weekDir)
		if err != nil {
			continue
		}

		var weeks []int
		for _, we := range weekEntries {
			if !we.IsDir() {
				continue
			}
			week, err := strconv.Atoi(we.Name())
			if err != nil {
				continue
			}
			weeks = append(weeks, week)
		}

		if len(weeks) > 0 {
			// Sort weeks descending (most recent first).
			sort.Sort(sort.Reverse(sort.IntSlice(weeks)))
			yearsMap[year] = weeks
		}
	}

	// Collect years sorted descending.
	var years []int
	for y := range yearsMap {
		years = append(years, y)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(years)))

	if len(years) == 0 {
		// No archives found — default to current week.
		wi := domain.WeekForDate(m.currentDate)
		years = []int{wi.Year}
		yearsMap[wi.Year] = []int{wi.Week}
	}

	m.taskListEditor.archiveYears = years
	m.taskListEditor.archiveWeeks = yearsMap
	m.taskListEditor.archiveYear = years[0]
	m.taskListEditor.archiveWeek = yearsMap[years[0]][0]
}

// loadArchiveData reads completed tasks from the archive for the selected
// year/week, filtering by the current task list slug.
func (m *RootModel) loadArchiveData() {
	year := m.taskListEditor.archiveYear
	week := m.taskListEditor.archiveWeek

	completed, err := m.store.ReadCompleted(year, week)
	if err != nil {
		m.taskListEditor.archiveTasks = nil
		return
	}

	var items []archiveTaskItem
	for _, ct := range completed {
		if ct.TaskListSlug != m.taskListEditor.slug {
			continue
		}
		items = append(items, archiveTaskItem{
			task:          ct.Task,
			completedDate: ct.CompletedDate,
			dateStr:       ct.CompletedDate.Format("Mon, 02 Jan"),
			slug:          ct.TaskListSlug,
		})
	}

	m.taskListEditor.archiveTasks = items

	// Clamp the cursor.
	if m.taskListEditor.archiveCursor >= len(items) {
		if len(items) > 0 {
			m.taskListEditor.archiveCursor = len(items) - 1
		} else {
			m.taskListEditor.archiveCursor = 0
		}
	}
}

// renderTaskListEditor renders the full-screen editor overlay.
func (m *RootModel) renderTaskListEditor() string {
	var b strings.Builder

	// Header: task list name and mode pills.
	nameStyle := lipgloss.NewStyle().Bold(true).Foreground(ui.ColorText)
	b.WriteString(nameStyle.Render(m.taskListEditor.name))
	b.WriteString("  ")

	if m.taskListEditor.readOnly {
		// Read-only: show only Archive pill and read-only label.
		b.WriteString(ui.PillActiveStyle.Render("Archive"))
		b.WriteString("  ")
		b.WriteString(ui.ShortcutBarStyle.Render("(read-only)"))
	} else {
		// Mode pills.
		modes := []struct {
			label string
			mode  EditorMode
		}{
			{"Edit", EditorModeEdit},
			{"Execute", EditorModeExecute},
			{"Archive", EditorModeArchive},
		}

		for i, mp := range modes {
			if i > 0 {
				b.WriteString(" ")
			}
			if mp.mode == m.taskListEditor.editorMode {
				b.WriteString(ui.PillActiveStyle.Render(mp.label))
			} else {
				b.WriteString(ui.PillInactiveStyle.Render(mp.label))
			}
		}

		b.WriteString("  ")
		b.WriteString(ui.ShortcutBarStyle.Render("Tab to switch"))
	}
	b.WriteString("\n")

	// Separator.
	sepStyle := lipgloss.NewStyle().Foreground(ui.ColorBorder)
	b.WriteString(sepStyle.Render(strings.Repeat("\u2500", m.width)))
	b.WriteString("\n")

	// Content based on mode.
	contentHeight := m.height - 3 // header + separator + bottom margin
	if contentHeight < 3 {
		contentHeight = 3
	}

	switch m.taskListEditor.editorMode {
	case EditorModeEdit:
		m.taskListEditor.editor.Resize(m.width, contentHeight)
		b.WriteString(m.taskListEditor.editor.View())

	case EditorModeExecute:
		b.WriteString(m.renderExecuteMode(contentHeight))

	case EditorModeArchive:
		b.WriteString(m.renderArchiveMode(contentHeight))
	}

	// Show confirmation dialog if active (e.g. "Save before closing?").
	if m.showConfirm {
		b.WriteString("\n")
		confirmStyle := lipgloss.NewStyle().Bold(true).Foreground(ui.ColorAccent)
		b.WriteString(confirmStyle.Render("Save changes before closing? (y/n)"))
	}

	// Show toast notification if active.
	if m.toastMessage != "" && !m.toastExpiry.IsZero() && time.Now().Before(m.toastExpiry) {
		b.WriteString("\n")
		toastStyle := lipgloss.NewStyle().Bold(true).Foreground(ui.ColorAccent)
		b.WriteString(toastStyle.Render(m.toastMessage))
	}

	content := b.String()
	return lipgloss.Place(m.width, m.height, lipgloss.Left, lipgloss.Top, content)
}

// renderExecuteMode renders the Execute mode view.
func (m *RootModel) renderExecuteMode(height int) string {
	var b strings.Builder

	// Shortcut hints.
	hintStyle := ui.ShortcutBarStyle
	b.WriteString(hintStyle.Render("\u2191\u2193 navigate \u00b7 Enter/x mark done \u00b7 Esc close"))
	b.WriteString("\n\n")

	// Read the current task list.
	tl, err := m.store.ReadTaskList(m.taskListEditor.slug)
	if err != nil {
		b.WriteString("  Error loading tasks.")
		return b.String()
	}

	activeTasks := tl.ActiveTasks()

	if len(activeTasks) == 0 {
		b.WriteString("  No active tasks.")
		return b.String()
	}

	// Clamp cursor.
	if m.taskListEditor.executeCursor >= len(activeTasks) {
		m.taskListEditor.executeCursor = len(activeTasks) - 1
	}
	if m.taskListEditor.executeCursor < 0 {
		m.taskListEditor.executeCursor = 0
	}

	highlightStyle := lipgloss.NewStyle().Bold(true).Foreground(ui.ColorAccent)
	normalStyle := lipgloss.NewStyle().Foreground(ui.ColorText)

	for i, task := range activeTasks {
		line := fmt.Sprintf("  \u25cb %s ~%s",
			task.Description,
			domain.FormatDuration(task.Duration))

		if i == m.taskListEditor.executeCursor {
			b.WriteString(highlightStyle.Render(line))
		} else {
			b.WriteString(normalStyle.Render(line))
		}
		b.WriteString("\n")
	}

	// Stats line at bottom.
	b.WriteString("\n")
	statsLine := m.calculateTaskListStats(tl)
	b.WriteString(ui.ShortcutBarStyle.Render(statsLine))

	return b.String()
}

// renderArchiveMode renders the Archive mode view.
func (m *RootModel) renderArchiveMode(height int) string {
	var b strings.Builder

	// Shortcut hints.
	hintStyle := ui.ShortcutBarStyle
	if m.taskListEditor.readOnly {
		b.WriteString(hintStyle.Render("\u2191\u2193 year \u00b7 \u2190\u2192 week \u00b7 j/k select \u00b7 Esc close"))
	} else {
		b.WriteString(hintStyle.Render("\u2191\u2193 year \u00b7 \u2190\u2192 week \u00b7 j/k select \u00b7 u unmark \u00b7 Esc close"))
	}
	b.WriteString("\n\n")

	// Year pills.
	for i, year := range m.taskListEditor.archiveYears {
		if i > 0 {
			b.WriteString("  ")
		}
		label := fmt.Sprintf("%d", year)
		if year == m.taskListEditor.archiveYear {
			b.WriteString(ui.PillActiveStyle.Render(label))
		} else {
			b.WriteString(ui.PillInactiveStyle.Render(label))
		}
	}
	b.WriteString("\n")

	// Week pills for the selected year.
	weeks := m.taskListEditor.archiveWeeks[m.taskListEditor.archiveYear]
	for i, week := range weeks {
		if i > 0 {
			b.WriteString(" ")
		}
		label := fmt.Sprintf("W%d", week)
		if week == m.taskListEditor.archiveWeek {
			b.WriteString(ui.PillActiveStyle.Render(label))
		} else {
			b.WriteString(ui.PillInactiveStyle.Render(label))
		}
	}
	b.WriteString("\n\n")

	// Week header with date range.
	wi := m.weekInfoForArchive()
	weekRange := domain.FormatWeekRange(wi)
	headerLine := fmt.Sprintf("\u2500\u2500 %d \u00b7 Week %d (%s) \u2500\u2500",
		m.taskListEditor.archiveYear, m.taskListEditor.archiveWeek, weekRange)
	b.WriteString(ui.DateStyle.Render(headerLine))
	b.WriteString("\n")

	if len(m.taskListEditor.archiveTasks) == 0 {
		b.WriteString("\n  No completed tasks for this week.")
		return b.String()
	}

	// Clamp cursor.
	if m.taskListEditor.archiveCursor >= len(m.taskListEditor.archiveTasks) {
		m.taskListEditor.archiveCursor = len(m.taskListEditor.archiveTasks) - 1
	}
	if m.taskListEditor.archiveCursor < 0 {
		m.taskListEditor.archiveCursor = 0
	}

	doneStyle := ui.TaskDoneStyle
	highlightStyle := lipgloss.NewStyle().Bold(true).Foreground(ui.ColorAccent)
	dateColStyle := ui.DateStyle

	for i, item := range m.taskListEditor.archiveTasks {
		taskStr := fmt.Sprintf("  \u2713 %s ~%s",
			item.task.Description,
			domain.FormatDuration(item.task.Duration))

		// Pad task string to align the date column.
		dateCol := dateColStyle.Render(item.dateStr)

		if i == m.taskListEditor.archiveCursor {
			b.WriteString(highlightStyle.Render(taskStr))
			b.WriteString("    ")
			b.WriteString(dateCol)
		} else {
			b.WriteString(doneStyle.Render(taskStr))
			b.WriteString("    ")
			b.WriteString(dateCol)
		}
		b.WriteString("\n")
	}

	return b.String()
}

// weekInfoForArchive reconstructs a WeekInfo for the currently selected archive year/week.
func (m *RootModel) weekInfoForArchive() domain.WeekInfo {
	// Reconstruct the WeekInfo by finding the Sunday of the selected week.
	// Week 1 starts on the Sunday on or before Jan 1.
	jan1 := time.Date(m.taskListEditor.archiveYear, time.January, 1, 0, 0, 0, 0, time.UTC)
	// Find Sunday on or before Jan 1.
	offset := int(jan1.Weekday()) // Sunday=0
	w1Sunday := jan1.AddDate(0, 0, -offset)
	weekSunday := w1Sunday.AddDate(0, 0, (m.taskListEditor.archiveWeek-1)*7)
	weekSaturday := weekSunday.AddDate(0, 0, 6)

	return domain.WeekInfo{
		Year:      m.taskListEditor.archiveYear,
		Week:      m.taskListEditor.archiveWeek,
		StartDate: weekSunday,
		EndDate:   weekSaturday,
	}
}

// calculateTaskListStats returns a stats string like "Total: 8h | Scheduled: 2h48m | Unscheduled: 5h12m | Completed: 0h".
func (m *RootModel) calculateTaskListStats(tl *domain.TaskList) string {
	totalMin := int(tl.TotalDuration().Minutes())

	// Calculate scheduled minutes from the current week's active timeboxes.
	wi := domain.WeekForDate(m.currentDate)
	days := domain.DaysOfWeek(wi)

	scheduledMin := 0
	completedMin := 0

	completed, err := m.store.ReadCompleted(wi.Year, wi.Week)
	if err == nil {
		for _, ct := range completed {
			if ct.TaskListSlug == tl.Slug {
				completedMin += int(ct.Task.Duration.Minutes())
			}
		}
	}

	for _, day := range days {
		dt, err := m.store.ReadDailyTimeboxes(day)
		if err != nil {
			continue
		}

		listTasks := map[string][]domain.Task{tl.Slug: tl.ActiveTasks()}
		_, scheduledMinByIndex := sequenceDailyTimeboxes(dt, listTasks)

		for i, tb := range dt.Timeboxes {
			if tb.TaskListSlug != tl.Slug {
				continue
			}
			// Scheduled (active timeboxes).
			if tb.Status == domain.StatusActive {
				scheduledMin += scheduledMinByIndex[i]
			}
		}
	}

	unscheduledMin := totalMin - completedMin - scheduledMin
	if unscheduledMin < 0 {
		unscheduledMin = 0
	}

	return fmt.Sprintf("Total: %s | Scheduled: %s | Unscheduled: %s | Completed: %s",
		formatMinutesAsHours(totalMin),
		formatMinutesAsHours(scheduledMin),
		formatMinutesAsHours(unscheduledMin),
		formatMinutesAsHours(completedMin))
}
