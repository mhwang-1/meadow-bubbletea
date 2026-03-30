package model

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mhwang-1/meadow-bubbletea/internal/domain"
	"github.com/mhwang-1/meadow-bubbletea/internal/ui"
)

// taskListMenuItem holds a task list's slug, display name, and weekly stats.
type taskListMenuItem struct {
	slug         string
	name         string
	totalMin     int
	scheduledMin int
	unschedMin   int
	doneMin      int
	nextTasks    []domain.Task // next 0–2 unscheduled, uncommented tasks
}

// archivedMenuEntry represents a single item in the archived tab's flat list.
// Week headers have isHeader=true and are not selectable.
type archivedMenuEntry struct {
	isHeader bool
	year     int
	week     int
	slug     string
	name     string
	label    string // rendered header text for week separators
}

// historyMenuItem holds a task list's summary for a single week.
type historyMenuItem struct {
	slug    string
	name    string
	doneMin int // total completed minutes for this week
}

// newScrollViewport creates a simple line-based scrolling viewport.
func newScrollViewport(width, height int) scrollViewport {
	return scrollViewport{Width: width, Height: height}
}

// taskListMenuViewportHeight returns a viewport height that fits the menu
// overlay within the available terminal height.
func (m RootModel) taskListMenuViewportHeight() int {
	// Keep enough room for the underlying app header and bottom bar while
	// accounting for menu chrome and optional confirmation/toast lines.
	vpHeight := m.height - 8
	if m.showConfirm {
		vpHeight -= 2
	}
	if m.toastMessage != "" && !m.toastExpiry.IsZero() && time.Now().Before(m.toastExpiry) {
		vpHeight -= 2
	}
	if vpHeight < 3 {
		vpHeight = 3
	}
	return vpHeight
}

// taskListMenuWidth returns the menu width constrained by terminal size.
func (m RootModel) taskListMenuWidth() int {
	menuWidth := 92
	if m.width-4 < menuWidth {
		menuWidth = m.width - 4
	}
	if menuWidth < 20 {
		menuWidth = 20
	}
	return menuWidth
}

// syncTaskListMenuViewportSize keeps the menu viewport dimensions in sync
// with the current terminal size and overlay chrome.
func (m *RootModel) syncTaskListMenuViewportSize() {
	m.taskListMenuViewport.Width = m.taskListMenuWidth() - 4
	m.taskListMenuViewport.Height = m.taskListMenuViewportHeight()
	m.taskListMenuViewport.SetYOffset(m.taskListMenuViewport.YOffset)
}

// openTaskListMenu loads all task lists with weekly stats and opens the overlay.
func (m *RootModel) openTaskListMenu() {
	lists, err := m.store.ListTaskLists()
	if err != nil {
		// Silently open an empty menu on error.
		m.taskListMenuItems = nil
		m.showTaskListMenu = true
		m.taskListMenuTab = MenuTabActive
		m.taskListMenuFilter = ""
		m.taskListMenuCursor = -1
		return
	}

	// Calculate weekly stats for each task list by scanning all 7 days.
	wi := domain.WeekForDate(m.currentDate)
	days := domain.DaysOfWeek(wi)

	type stats struct {
		scheduledMin   int
		scheduledCount int
		doneMin        int
		doneCount      int
	}
	statsMap := make(map[string]*stats)
	for _, tl := range lists {
		statsMap[tl.Slug] = &stats{}
	}

	completed, err := m.store.ReadCompleted(wi.Year, wi.Week)
	if err == nil {
		for _, ct := range completed {
			if s, ok := statsMap[ct.TaskListSlug]; ok {
				s.doneMin += int(ct.Task.Duration.Minutes())
				s.doneCount++
			}
		}
	}

	for _, day := range days {
		dt, err := m.store.ReadDailyTimeboxes(day)
		if err != nil {
			continue
		}

		listTasks := make(map[string][]domain.Task)
		for _, tb := range dt.Timeboxes {
			if tb.TaskListSlug == "" {
				continue
			}
			if _, ok := listTasks[tb.TaskListSlug]; ok {
				continue
			}
			tl, err := m.store.ReadTaskList(tb.TaskListSlug)
			if err != nil {
				listTasks[tb.TaskListSlug] = nil
				continue
			}
			listTasks[tb.TaskListSlug] = tl.ActiveTasks()
		}

		scheduledByIndex, scheduledMinByIndex := sequenceDailyTimeboxes(dt, listTasks)

		for i, tb := range dt.Timeboxes {
			if tb.TaskListSlug == "" {
				continue
			}
			s, ok := statsMap[tb.TaskListSlug]
			if !ok {
				continue
			}

			// Scheduled tasks (active timeboxes).
			if tb.Status == domain.StatusActive {
				s.scheduledMin += scheduledMinByIndex[i]
				for _, st := range scheduledByIndex[i] {
					if !st.IsBreak {
						s.scheduledCount++
					}
				}
			}
		}
	}

	var items []taskListMenuItem
	for _, tl := range lists {
		s := statsMap[tl.Slug]
		totalMin := int(tl.TotalDuration().Minutes())
		unschedMin := totalMin - s.doneMin - s.scheduledMin
		if unschedMin < 0 {
			unschedMin = 0
		}

		// Determine the next unscheduled tasks.
		active := tl.ActiveTasks()
		consumed := s.doneCount + s.scheduledCount
		var nextTasks []domain.Task
		if consumed < len(active) {
			remaining := active[consumed:]
			if len(remaining) > 2 {
				remaining = remaining[:2]
			}
			nextTasks = remaining
		}

		items = append(items, taskListMenuItem{
			slug:         tl.Slug,
			name:         tl.Name,
			totalMin:     totalMin,
			scheduledMin: s.scheduledMin,
			unschedMin:   unschedMin,
			doneMin:      s.doneMin,
			nextTasks:    nextTasks,
		})
	}

	m.taskListMenuItems = items
	m.showTaskListMenu = true
	m.taskListMenuTab = MenuTabActive
	m.sortActive = SortAlphabetical
	m.sortHistory = SortAlphabetical
	m.sortArchived = SortAlphabetical
	m.taskListMenuFilter = ""
	if len(items) > 0 {
		m.taskListMenuCursor = 0
	} else {
		m.taskListMenuCursor = -1
	}

	// Initialise the scrollable viewport for menu content.
	vpHeight := m.taskListMenuViewportHeight()
	menuWidth := m.taskListMenuWidth()
	m.taskListMenuViewport = newScrollViewport(menuWidth-4, vpHeight)
}

// closeTaskListMenu hides the task list picker overlay.
func (m *RootModel) closeTaskListMenu() {
	m.showTaskListMenu = false
}

// handleTaskListMenuKey processes key events while the menu overlay is open.
func (m *RootModel) handleTaskListMenuKey(msg tea.KeyMsg) tea.Cmd {
	m.syncTaskListMenuViewportSize()

	switch m.taskListMenuTab {
	case MenuTabHistory:
		return m.handleHistoryTabKey(msg)
	case MenuTabArchived:
		return m.handleArchivedTabKey(msg)
	default:
		return m.handleActiveTabKey(msg)
	}
}

// ensureActiveMenuCursorVisible adjusts the menu viewport so the cursor is visible.
func (m *RootModel) ensureActiveMenuCursorVisible() {
	if m.taskListMenuCursor < 0 {
		m.taskListMenuViewport.SetYOffset(0)
		return
	}

	// Each item occupies: blank line + name line + stats line = 3 lines,
	// plus optionally a next line. The first item starts after the filter line (1 line).
	tbMinutes := m.selectedTimeboxMinutes()
	filtered := m.filteredTaskListItems()

	linePos := 1 // after filter line
	for i, item := range filtered {
		if i == m.taskListMenuCursor {
			break
		}
		linePos++ // blank line before item
		linePos++ // name line
		linePos++ // stats line
		if tbMinutes > 0 && len(item.nextTasks) > 0 {
			linePos++ // next line
		}
	}

	// The cursor item spans 2–3 lines (name + stats + optional next).
	cursorEnd := linePos + 2 // blank + name + stats
	if m.taskListMenuCursor < len(filtered) && tbMinutes > 0 && len(filtered[m.taskListMenuCursor].nextTasks) > 0 {
		cursorEnd++
	}

	// Handle "+ New Task List" which is at the end.
	if m.taskListMenuCursor == len(filtered) {
		linePos++ // new list label line
		cursorEnd = linePos
	}

	vpHeight := m.taskListMenuViewport.Height
	offset := m.taskListMenuViewport.YOffset
	totalLines := 3 // filter line + blank line + new list label
	for _, item := range filtered {
		totalLines += 3 // blank + name + stats
		if tbMinutes > 0 && len(item.nextTasks) > 0 {
			totalLines++
		}
	}
	maxOffset := totalLines - vpHeight
	if maxOffset < 0 {
		maxOffset = 0
	}

	// Near the end, pin to bottom so the last list item is always visible.
	if len(filtered) > 0 && m.taskListMenuCursor >= len(filtered)-1 {
		m.taskListMenuViewport.SetYOffset(maxOffset)
		return
	}

	// Scroll down if cursor is below visible area.
	if cursorEnd+1 > offset+vpHeight {
		m.taskListMenuViewport.SetYOffset(cursorEnd - vpHeight + 1)
	}
	// Scroll up if cursor is above visible area.
	if linePos < offset {
		m.taskListMenuViewport.SetYOffset(linePos)
	}

	if m.taskListMenuViewport.YOffset < 0 {
		m.taskListMenuViewport.SetYOffset(0)
	}
	if m.taskListMenuViewport.YOffset > maxOffset {
		m.taskListMenuViewport.SetYOffset(maxOffset)
	}
}

// handleActiveTabKey handles keys for the Active tab.
// Cursor -1 means no item is selected; 0..len(filtered)-1 are list items;
// len(filtered) is the "+ New Task List" option.
func (m *RootModel) handleActiveTabKey(msg tea.KeyMsg) tea.Cmd {
	filtered := m.filteredTaskListItems()
	key := msg.String()

	switch key {
	case "esc":
		m.closeTaskListMenu()
		return nil

	case "tab":
		m.taskListMenuTab = MenuTabHistory
		m.loadHistoryData()
		m.taskListMenuViewport.SetYOffset(0)
		return nil

	case "up":
		if m.taskListMenuCursor > -1 {
			m.taskListMenuCursor--
		}
		m.ensureActiveMenuCursorVisible()
		return nil

	case "down":
		// Allow cursor to reach the "+ New Task List" item (index == len(filtered)).
		if m.taskListMenuCursor < len(filtered) {
			m.taskListMenuCursor++
		}
		m.ensureActiveMenuCursorVisible()
		return nil

	case "enter", "ctrl+m", "ctrl+j":
		if m.taskListMenuCursor >= 0 && m.taskListMenuCursor == len(filtered) {
			// "+ New Task List" selected.
			m.closeTaskListMenu()
			m.inputMode = InputTaskListCreate
			m.inputPrompt = "New task list name: "
			m.inputBuffer = ""
			return nil
		}
		if m.taskListMenuCursor >= 0 && m.taskListMenuCursor < len(filtered) {
			return m.handleTaskListSelect(filtered[m.taskListMenuCursor])
		}
		return nil

	case "a":
		// Archive the selected task list.
		if m.taskListMenuCursor >= 0 && m.taskListMenuCursor < len(filtered) {
			item := filtered[m.taskListMenuCursor]
			assigned, _ := m.store.IsTaskListAssigned(item.slug)
			if assigned {
				return m.showToast("Cannot archive: list is assigned to active timeboxes", 3*time.Second)
			}
			m.showConfirm = true
			m.confirmAction = func() {
				wi := domain.WeekForDate(m.currentDate)
				if err := m.store.ArchiveTaskList(item.slug, wi.Year, wi.Week); err != nil {
					return
				}
				// Refresh the menu.
				m.closeTaskListMenu()
				m.openTaskListMenu()
			}
		}
		return nil

	case "d":
		// Delete the selected task list.
		if m.taskListMenuCursor >= 0 && m.taskListMenuCursor < len(filtered) {
			item := filtered[m.taskListMenuCursor]
			assigned, _ := m.store.IsTaskListAssigned(item.slug)
			if assigned {
				return m.showToast("Cannot delete: list is assigned to active timeboxes", 3*time.Second)
			}
			// Check if there are completed tasks — if so, block deletion.
			has, _ := m.store.HasCompletedTasks(item.slug)
			if has {
				return m.showToast("Cannot delete: has archived completed tasks", 3*time.Second)
			}
			m.showConfirm = true
			m.confirmAction = func() {
				if err := m.store.DeleteTaskList(item.slug); err != nil {
					return
				}
				// Refresh the menu.
				m.closeTaskListMenu()
				m.openTaskListMenu()
			}
		}
		return nil

	case "s":
		m.sortActive = (m.sortActive + 1) % 3
		m.applySortToActiveItems()
		m.taskListMenuViewport.SetYOffset(0)
		return nil

	case "pgup":
		// Move cursor up by roughly one page of items.
		pageItems := m.taskListMenuViewport.Height / 3
		if pageItems < 1 {
			pageItems = 1
		}
		m.taskListMenuCursor -= pageItems
		if m.taskListMenuCursor < 0 {
			m.taskListMenuCursor = 0
		}
		m.ensureActiveMenuCursorVisible()
		return nil

	case "pgdown":
		// Move cursor down by roughly one page of items.
		pageItems := m.taskListMenuViewport.Height / 3
		if pageItems < 1 {
			pageItems = 1
		}
		m.taskListMenuCursor += pageItems
		if m.taskListMenuCursor > len(filtered) {
			m.taskListMenuCursor = len(filtered)
		}
		m.ensureActiveMenuCursorVisible()
		return nil

	case "backspace":
		if len(m.taskListMenuFilter) > 0 {
			runes := []rune(m.taskListMenuFilter)
			m.taskListMenuFilter = string(runes[:len(runes)-1])
			m.resetMenuCursor()
			m.ensureActiveMenuCursorVisible()
		}
		return nil

	default:
		// Append printable characters to the filter (supports IME and paste).
		var added bool
		for _, r := range msg.Runes {
			if unicode.IsPrint(r) {
				m.taskListMenuFilter += string(r)
				added = true
			}
		}
		if added {
			m.resetMenuCursor()
			m.ensureActiveMenuCursorVisible()
		}
		return nil
	}
}

// ensureArchivedMenuCursorVisible adjusts the menu viewport so the archived cursor is visible.
func (m *RootModel) ensureArchivedMenuCursorVisible() {
	if len(m.archivedMenuFlatItems) == 0 {
		return
	}

	// Calculate the line position of the cursor in the rendered body.
	// Year pills take 1 line + blank line = 2 lines at the top.
	linePos := 2
	for i := 0; i < m.archivedMenuCursor && i < len(m.archivedMenuFlatItems); i++ {
		if m.archivedMenuFlatItems[i].isHeader {
			if i > 0 {
				linePos++ // blank line before header (except first)
			}
			linePos++ // header line
		} else {
			linePos++ // item line
		}
	}

	vpHeight := m.taskListMenuViewport.Height
	offset := m.taskListMenuViewport.YOffset

	if linePos+1 > offset+vpHeight {
		m.taskListMenuViewport.SetYOffset(linePos + 1 - vpHeight)
	}
	if linePos < offset {
		m.taskListMenuViewport.SetYOffset(linePos)
	}
}

// handleArchivedTabKey handles keys for the Archived tab.
func (m *RootModel) handleArchivedTabKey(msg tea.KeyMsg) tea.Cmd {
	key := msg.String()

	switch key {
	case "esc":
		m.closeTaskListMenu()
		return nil

	case "tab":
		m.taskListMenuTab = MenuTabActive
		m.taskListMenuFilter = ""
		m.resetMenuCursor()
		m.taskListMenuViewport.SetYOffset(0)
		return nil

	case "left":
		// Navigate year pills to older year.
		idx := m.archivedYearIndex()
		if idx < len(m.archivedMenuYears)-1 {
			m.archivedMenuSelectedYear = m.archivedMenuYears[idx+1]
			m.rebuildArchivedFlatItems()
			m.archivedMenuCursor = 0
			m.skipToNextSelectableItem(1)
			m.taskListMenuViewport.SetYOffset(0)
		}
		return nil

	case "right":
		// Navigate year pills to newer year.
		idx := m.archivedYearIndex()
		if idx > 0 {
			m.archivedMenuSelectedYear = m.archivedMenuYears[idx-1]
			m.rebuildArchivedFlatItems()
			m.archivedMenuCursor = 0
			m.skipToNextSelectableItem(1)
			m.taskListMenuViewport.SetYOffset(0)
		}
		return nil

	case "up":
		if m.archivedMenuCursor > 0 {
			m.archivedMenuCursor--
			// Skip headers.
			for m.archivedMenuCursor > 0 && m.archivedMenuFlatItems[m.archivedMenuCursor].isHeader {
				m.archivedMenuCursor--
			}
			// If we landed on a header at position 0, move forward.
			if m.archivedMenuFlatItems[m.archivedMenuCursor].isHeader {
				m.skipToNextSelectableItem(1)
			}
		}
		m.ensureArchivedMenuCursorVisible()
		return nil

	case "down":
		if m.archivedMenuCursor < len(m.archivedMenuFlatItems)-1 {
			m.archivedMenuCursor++
			m.skipToNextSelectableItem(1)
		}
		m.ensureArchivedMenuCursorVisible()
		return nil

	case "enter", "ctrl+m", "ctrl+j":
		if m.archivedMenuCursor >= 0 && m.archivedMenuCursor < len(m.archivedMenuFlatItems) {
			entry := m.archivedMenuFlatItems[m.archivedMenuCursor]
			if !entry.isHeader {
				m.editorOriginTab = MenuTabArchived
				if err := m.openArchivedTaskListEditor(entry.year, entry.week, entry.slug); err != nil {
					return m.showToast("Cannot open archived list. Debug source: "+err.Error(), 5*time.Second)
				}
				m.closeTaskListMenu()
			}
		}
		return nil

	case "s":
		m.sortArchived = (m.sortArchived + 1) % 3
		m.applySortToArchivedItems()
		m.taskListMenuViewport.SetYOffset(0)
		return nil

	case "pgup":
		// Move cursor up by roughly one page of items, skipping headers.
		pageItems := m.taskListMenuViewport.Height / 2
		if pageItems < 1 {
			pageItems = 1
		}
		m.archivedMenuCursor -= pageItems
		if m.archivedMenuCursor < 0 {
			m.archivedMenuCursor = 0
		}
		m.skipToNextSelectableItem(1)
		m.ensureArchivedMenuCursorVisible()
		return nil

	case "pgdown":
		// Move cursor down by roughly one page of items, skipping headers.
		pageItems := m.taskListMenuViewport.Height / 2
		if pageItems < 1 {
			pageItems = 1
		}
		m.archivedMenuCursor += pageItems
		if m.archivedMenuCursor >= len(m.archivedMenuFlatItems) {
			m.archivedMenuCursor = len(m.archivedMenuFlatItems) - 1
		}
		if m.archivedMenuCursor < 0 {
			m.archivedMenuCursor = 0
		}
		m.skipToNextSelectableItem(-1)
		m.ensureArchivedMenuCursorVisible()
		return nil
	}

	return nil
}

// handleHistoryTabKey handles keys for the History tab.
func (m *RootModel) handleHistoryTabKey(msg tea.KeyMsg) tea.Cmd {
	key := msg.String()

	switch key {
	case "esc":
		m.closeTaskListMenu()
		return nil

	case "tab":
		m.taskListMenuTab = MenuTabArchived
		m.loadArchivedMenuData()
		m.taskListMenuViewport.SetYOffset(0)
		return nil

	case "[":
		// Navigate year pills to older year.
		idx := m.historyYearIndex()
		if idx < len(m.historyYears)-1 {
			m.historySelectedYear = m.historyYears[idx+1]
			m.loadHistoryWeeksForYear()
			m.loadHistoryWeekData()
		}
		m.taskListMenuViewport.SetYOffset(0)
		return nil

	case "]":
		// Navigate year pills to newer year.
		idx := m.historyYearIndex()
		if idx > 0 {
			m.historySelectedYear = m.historyYears[idx-1]
			m.loadHistoryWeeksForYear()
			m.loadHistoryWeekData()
		}
		m.taskListMenuViewport.SetYOffset(0)
		return nil

	case "left", "h":
		// Navigate week pills to older week.
		idx := m.historyWeekIndex()
		if idx > 0 {
			m.historySelectedWeek = m.historyWeeks[idx-1]
			m.loadHistoryWeekData()
		} else {
			// Move to older year when already at oldest week in current year.
			yIdx := m.historyYearIndex()
			if yIdx < len(m.historyYears)-1 {
				m.historySelectedYear = m.historyYears[yIdx+1]
				m.loadHistoryWeeksForYear()
				if len(m.historyWeeks) > 0 {
					m.historySelectedWeek = m.historyWeeks[len(m.historyWeeks)-1]
					m.loadHistoryWeekData()
				}
			}
		}
		m.taskListMenuViewport.SetYOffset(0)
		return nil

	case "right", "l":
		// Navigate week pills to newer week.
		idx := m.historyWeekIndex()
		if idx < len(m.historyWeeks)-1 {
			m.historySelectedWeek = m.historyWeeks[idx+1]
			m.loadHistoryWeekData()
		} else {
			// Move to newer year when already at newest week in current year.
			yIdx := m.historyYearIndex()
			if yIdx > 0 {
				m.historySelectedYear = m.historyYears[yIdx-1]
				m.loadHistoryWeeksForYear()
				if len(m.historyWeeks) > 0 {
					m.historySelectedWeek = m.historyWeeks[0]
					m.loadHistoryWeekData()
				}
			}
		}
		m.taskListMenuViewport.SetYOffset(0)
		return nil

	case "up":
		if m.historyCursor > 0 {
			m.historyCursor--
		}
		m.ensureHistoryMenuCursorVisible()
		return nil

	case "down":
		if m.historyCursor < len(m.historyItems)-1 {
			m.historyCursor++
		}
		m.ensureHistoryMenuCursorVisible()
		return nil

	case "enter", "ctrl+m", "ctrl+j":
		if len(m.historyItems) > 0 && m.historyCursor >= 0 && m.historyCursor < len(m.historyItems) {
			item := m.historyItems[m.historyCursor]
			m.editorOriginTab = MenuTabHistory
			m.editorOriginYear = m.historySelectedYear
			m.editorOriginWeek = m.historySelectedWeek
			if err := m.openArchivedTaskListEditor(m.historySelectedYear, m.historySelectedWeek, item.slug); err != nil {
				return m.showToast("Cannot open history snapshot. Debug source: "+err.Error(), 5*time.Second)
			}
			m.closeTaskListMenu()
		}
		return nil

	case "s":
		m.sortHistory = (m.sortHistory + 1) % 3
		m.applySortToHistoryItems()
		m.taskListMenuViewport.SetYOffset(0)
		return nil

	case "pgup":
		// Move cursor up by roughly one page of items.
		pageItems := m.taskListMenuViewport.Height / 2
		if pageItems < 1 {
			pageItems = 1
		}
		m.historyCursor -= pageItems
		if m.historyCursor < 0 {
			m.historyCursor = 0
		}
		m.ensureHistoryMenuCursorVisible()
		return nil

	case "pgdown":
		// Move cursor down by roughly one page of items.
		pageItems := m.taskListMenuViewport.Height / 2
		if pageItems < 1 {
			pageItems = 1
		}
		m.historyCursor += pageItems
		if m.historyCursor >= len(m.historyItems) {
			m.historyCursor = len(m.historyItems) - 1
		}
		if m.historyCursor < 0 {
			m.historyCursor = 0
		}
		m.ensureHistoryMenuCursorVisible()
		return nil
	}

	return nil
}

// ensureHistoryMenuCursorVisible adjusts the menu viewport so the history cursor is visible.
func (m *RootModel) ensureHistoryMenuCursorVisible() {
	if len(m.historyItems) == 0 {
		return
	}

	// Layout: year pills (1) + week pills (1) + blank (1) + total row (1) + items (1 each).
	// Cursor i maps to line: 4 + i (0-indexed).
	linePos := 4 + m.historyCursor

	vpHeight := m.taskListMenuViewport.Height
	offset := m.taskListMenuViewport.YOffset

	if linePos+1 > offset+vpHeight {
		m.taskListMenuViewport.SetYOffset(linePos + 1 - vpHeight)
	}
	if linePos < offset {
		m.taskListMenuViewport.SetYOffset(linePos)
	}
}

// historyYearIndex returns the index of the currently selected year in historyYears.
func (m *RootModel) historyYearIndex() int {
	for i, y := range m.historyYears {
		if y == m.historySelectedYear {
			return i
		}
	}
	return 0
}

// historyWeekIndex returns the index of the currently selected week in historyWeeks.
func (m *RootModel) historyWeekIndex() int {
	for i, w := range m.historyWeeks {
		if w == m.historySelectedWeek {
			return i
		}
	}
	return 0
}

// applySortToHistoryItems re-sorts historyItems based on sortHistory mode.
func (m *RootModel) applySortToHistoryItems() {
	switch m.sortHistory {
	case SortAlphabetical:
		sort.Slice(m.historyItems, func(i, j int) bool {
			return m.historyItems[i].name < m.historyItems[j].name
		})
	case SortDurationDesc:
		sort.Slice(m.historyItems, func(i, j int) bool {
			if m.historyItems[i].doneMin != m.historyItems[j].doneMin {
				return m.historyItems[i].doneMin > m.historyItems[j].doneMin
			}
			return m.historyItems[i].name < m.historyItems[j].name
		})
	case SortDurationAsc:
		sort.Slice(m.historyItems, func(i, j int) bool {
			if m.historyItems[i].doneMin != m.historyItems[j].doneMin {
				return m.historyItems[i].doneMin < m.historyItems[j].doneMin
			}
			return m.historyItems[i].name < m.historyItems[j].name
		})
	}
	// Clamp cursor.
	if m.historyCursor >= len(m.historyItems) {
		m.historyCursor = len(m.historyItems) - 1
	}
	if m.historyCursor < 0 {
		m.historyCursor = 0
	}
}

// applySortToActiveItems re-sorts taskListMenuItems based on sortActive mode.
func (m *RootModel) applySortToActiveItems() {
	switch m.sortActive {
	case SortAlphabetical:
		sort.Slice(m.taskListMenuItems, func(i, j int) bool {
			return m.taskListMenuItems[i].name < m.taskListMenuItems[j].name
		})
	case SortDurationDesc:
		sort.Slice(m.taskListMenuItems, func(i, j int) bool {
			if m.taskListMenuItems[i].totalMin != m.taskListMenuItems[j].totalMin {
				return m.taskListMenuItems[i].totalMin > m.taskListMenuItems[j].totalMin
			}
			return m.taskListMenuItems[i].name < m.taskListMenuItems[j].name
		})
	case SortDurationAsc:
		sort.Slice(m.taskListMenuItems, func(i, j int) bool {
			if m.taskListMenuItems[i].totalMin != m.taskListMenuItems[j].totalMin {
				return m.taskListMenuItems[i].totalMin < m.taskListMenuItems[j].totalMin
			}
			return m.taskListMenuItems[i].name < m.taskListMenuItems[j].name
		})
	}
	m.resetMenuCursor()
}

// applySortToArchivedItems re-sorts archived items within each week group
// based on sortArchived mode.
func (m *RootModel) applySortToArchivedItems() {
	// Identify week group boundaries.
	type group struct {
		headerIdx int
		items     []archivedMenuEntry
	}

	var groups []group
	for i, entry := range m.archivedMenuFlatItems {
		if entry.isHeader {
			groups = append(groups, group{headerIdx: i})
		} else if len(groups) > 0 {
			groups[len(groups)-1].items = append(groups[len(groups)-1].items, entry)
		}
	}

	// Sort items within each group.
	for gi := range groups {
		g := &groups[gi]
		switch m.sortArchived {
		case SortAlphabetical:
			sort.Slice(g.items, func(i, j int) bool {
				return g.items[i].name < g.items[j].name
			})
		case SortDurationDesc:
			durations := m.archivedDurationsForWeek(g.items)
			sort.Slice(g.items, func(i, j int) bool {
				di := durations[g.items[i].slug]
				dj := durations[g.items[j].slug]
				if di != dj {
					return di > dj
				}
				return g.items[i].name < g.items[j].name
			})
		case SortDurationAsc:
			durations := m.archivedDurationsForWeek(g.items)
			sort.Slice(g.items, func(i, j int) bool {
				di := durations[g.items[i].slug]
				dj := durations[g.items[j].slug]
				if di != dj {
					return di < dj
				}
				return g.items[i].name < g.items[j].name
			})
		}
	}

	// Rebuild the flat list.
	var rebuilt []archivedMenuEntry
	for _, g := range groups {
		rebuilt = append(rebuilt, m.archivedMenuFlatItems[g.headerIdx])
		rebuilt = append(rebuilt, g.items...)
	}
	m.archivedMenuFlatItems = rebuilt

	// Reset cursor to first selectable item.
	m.archivedMenuCursor = 0
	if len(m.archivedMenuFlatItems) > 0 {
		m.skipToNextSelectableItem(1)
	}
}

// archivedDurationsForWeek returns completed durations for archived task lists
// within a single week group.
func (m *RootModel) archivedDurationsForWeek(items []archivedMenuEntry) map[string]int {
	if len(items) == 0 {
		return nil
	}
	year := items[0].year
	week := items[0].week

	durations, err := m.store.CompletedDurationsBySlug(year, week)
	if err != nil {
		return nil
	}

	result := make(map[string]int)
	for slug, dur := range durations {
		result[slug] = int(dur.Minutes())
	}
	return result
}

// skipToNextSelectableItem moves the archived cursor forward past headers.
func (m *RootModel) skipToNextSelectableItem(dir int) {
	for m.archivedMenuCursor >= 0 &&
		m.archivedMenuCursor < len(m.archivedMenuFlatItems) &&
		m.archivedMenuFlatItems[m.archivedMenuCursor].isHeader {
		m.archivedMenuCursor += dir
	}
	if m.archivedMenuCursor >= len(m.archivedMenuFlatItems) {
		m.archivedMenuCursor = len(m.archivedMenuFlatItems) - 1
	}
	if m.archivedMenuCursor < 0 {
		m.archivedMenuCursor = 0
	}
}

// archivedYearIndex returns the index of the currently selected year.
func (m *RootModel) archivedYearIndex() int {
	for i, y := range m.archivedMenuYears {
		if y == m.archivedMenuSelectedYear {
			return i
		}
	}
	return 0
}

// loadArchivedMenuData loads archived task lists and builds the flat item list.
func (m *RootModel) loadArchivedMenuData() {
	archivedMap, err := m.store.ListArchivedTaskLists()
	if err != nil {
		m.archivedMenuYears = nil
		m.archivedMenuFlatItems = nil
		m.archivedMenuCursor = 0
		return
	}

	// Extract sorted years (descending).
	var years []int
	for y := range archivedMap {
		years = append(years, y)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(years)))

	m.archivedMenuYears = years
	if len(years) > 0 {
		m.archivedMenuSelectedYear = years[0]
	}

	m.rebuildArchivedFlatItemsFromMap(archivedMap)
	m.archivedMenuCursor = 0
	if len(m.archivedMenuFlatItems) > 0 {
		m.skipToNextSelectableItem(1)
	}
}

// rebuildArchivedFlatItems rebuilds the flat item list by re-scanning.
func (m *RootModel) rebuildArchivedFlatItems() {
	archivedMap, _ := m.store.ListArchivedTaskLists()
	m.rebuildArchivedFlatItemsFromMap(archivedMap)
}

// rebuildArchivedFlatItemsFromMap builds the flat list from the given map for the selected year.
func (m *RootModel) rebuildArchivedFlatItemsFromMap(archivedMap map[int]map[int][]*domain.TaskList) {
	m.archivedMenuFlatItems = nil

	yearData, ok := archivedMap[m.archivedMenuSelectedYear]
	if !ok {
		return
	}

	// Sort weeks descending.
	var weeks []int
	for w := range yearData {
		weeks = append(weeks, w)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(weeks)))

	for _, week := range weeks {
		// Build week header.
		wi := m.weekInfoForYearWeek(m.archivedMenuSelectedYear, week)
		weekRange := domain.FormatWeekRange(wi)
		label := fmt.Sprintf("W%d \u00b7 %s", week, weekRange)

		m.archivedMenuFlatItems = append(m.archivedMenuFlatItems, archivedMenuEntry{
			isHeader: true,
			year:     m.archivedMenuSelectedYear,
			week:     week,
			label:    label,
		})

		for _, tl := range yearData[week] {
			m.archivedMenuFlatItems = append(m.archivedMenuFlatItems, archivedMenuEntry{
				year: m.archivedMenuSelectedYear,
				week: week,
				slug: tl.Slug,
				name: tl.Name,
			})
		}
	}
}

// weekInfoForYearWeek reconstructs a WeekInfo for a given year/week.
func (m *RootModel) weekInfoForYearWeek(year, week int) domain.WeekInfo {
	jan1 := time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC)
	offset := int(jan1.Weekday())
	w1Sunday := jan1.AddDate(0, 0, -offset)
	weekSunday := w1Sunday.AddDate(0, 0, (week-1)*7)
	weekSaturday := weekSunday.AddDate(0, 0, 6)

	return domain.WeekInfo{
		Year:      year,
		Week:      week,
		StartDate: weekSunday,
		EndDate:   weekSaturday,
	}
}

// resetMenuCursor sets the cursor to 0 if there are filtered items, or -1
// (no selection) if there are none, so the user must arrow-down to reach
// "+ New Task List".
func (m *RootModel) resetMenuCursor() {
	if len(m.filteredTaskListItems()) > 0 {
		m.taskListMenuCursor = 0
	} else {
		m.taskListMenuCursor = -1
	}
}

// filteredTaskListItems returns menu items whose name matches the current
// filter (case-insensitive substring match).
func (m *RootModel) filteredTaskListItems() []taskListMenuItem {
	if m.taskListMenuFilter == "" {
		return m.taskListMenuItems
	}

	lower := strings.ToLower(m.taskListMenuFilter)
	var result []taskListMenuItem
	for _, item := range m.taskListMenuItems {
		if strings.Contains(strings.ToLower(item.name), lower) {
			result = append(result, item)
		}
	}
	return result
}

// handleTaskListSelect acts on the chosen menu item. In Plan mode Day view
// it assigns/reassigns the selected timebox; otherwise it opens the task list
// editor.
func (m *RootModel) handleTaskListSelect(item taskListMenuItem) tea.Cmd {
	// Plan mode Day view: assign/reassign the currently selected timebox.
	if m.mode == ModePlan && m.view == ViewDay {
		dt, err := m.store.ReadDailyTimeboxes(m.currentDate)
		if err == nil {
			dt.SortByStart()
			if m.selectedTimebox >= 0 && m.selectedTimebox < len(dt.Timeboxes) {
				tb := &dt.Timeboxes[m.selectedTimebox]
				if tb.Status != domain.StatusArchived && !tb.IsReserved() {
					if len(tb.CompletedTasks) > 0 {
						return m.showToast("Cannot change assignment: unmark done tasks first", 3*time.Second)
					}
					tbMinutes := tb.DurationMinutes()

					// Check if the first next task exceeds the timebox.
					if len(item.nextTasks) > 0 {
						firstTask := item.nextTasks[0]
						firstMin := int(firstTask.Duration.Minutes())
						if firstMin > tbMinutes {
							// Show confirmation prompt — do NOT close menu yet.
							m.showConfirm = true
							m.confirmMessage = fmt.Sprintf(
								"Next task \"%s ~%s\" does not fit in this %dm timebox. Assign anyway?",
								firstTask.Description,
								domain.FormatDuration(firstTask.Duration),
								tbMinutes,
							)
							slug := item.slug
							m.confirmAction = func() {
								dt2, err2 := m.store.ReadDailyTimeboxes(m.currentDate)
								if err2 != nil {
									return
								}
								dt2.SortByStart()
								if m.selectedTimebox >= 0 && m.selectedTimebox < len(dt2.Timeboxes) {
									tb2 := &dt2.Timeboxes[m.selectedTimebox]
									if tb2.Status != domain.StatusArchived && !tb2.IsReserved() && len(tb2.CompletedTasks) == 0 {
										tb2.TaskListSlug = slug
										tb2.Status = domain.StatusActive
										_ = m.store.WriteDailyTimeboxes(dt2)
									}
								}
								m.closeTaskListMenu()
							}
							return nil
						}
					}

					tb.TaskListSlug = item.slug
					tb.Status = domain.StatusActive
					_ = m.store.WriteDailyTimeboxes(dt)
					m.closeTaskListMenu()
					return nil
				}
			}
		}
	}

	// Default: open the task list editor.
	m.closeTaskListMenu()
	m.editorOriginTab = MenuTabActive
	m.openTaskListEditor(item.slug)
	return nil
}

// renderTaskListMenu renders the task list picker overlay centred on screen.
func (m RootModel) renderTaskListMenu() string {
	// Determine overlay width.
	menuWidth := m.taskListMenuWidth()

	innerWidth := menuWidth - 4 // account for border padding
	m.taskListMenuViewport.Height = m.taskListMenuViewportHeight()

	var b strings.Builder

	// Tab pills.
	b.WriteString(ui.ModePill("Active", m.taskListMenuTab == MenuTabActive))
	b.WriteString(" ")
	b.WriteString(ui.ModePill("History", m.taskListMenuTab == MenuTabHistory))
	b.WriteString(" ")
	b.WriteString(ui.ModePill("Archived", m.taskListMenuTab == MenuTabArchived))
	b.WriteString("\n\n")

	switch m.taskListMenuTab {
	case MenuTabHistory:
		b.WriteString(m.renderHistoryTabContent(innerWidth))
	case MenuTabArchived:
		b.WriteString(m.renderArchivedTabContent())
	default:
		b.WriteString(m.renderActiveTabContent(innerWidth))
	}

	// Show confirmation dialog if active.
	if m.showConfirm {
		b.WriteString("\n")
		confirmStyle := lipgloss.NewStyle().Bold(true).Foreground(ui.ColorAccent)
		b.WriteString(confirmStyle.Render(m.renderConfirmLine()))
		b.WriteString("\n")
	}

	// Show toast if active.
	if m.toastMessage != "" && !m.toastExpiry.IsZero() && time.Now().Before(m.toastExpiry) {
		b.WriteString("\n")
		toastStyle := lipgloss.NewStyle().Bold(true).Foreground(ui.ColorAccent)
		b.WriteString(toastStyle.Render(m.toastMessage))
		b.WriteString("\n")
	}

	content := b.String()

	// Build bordered box.
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ui.ColorDimmed).
		Padding(0, 1).
		Width(menuWidth)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ui.ColorText)

	box := borderStyle.Render(content)

	// Inject the title into the top border.
	title := titleStyle.Render(" Task Lists ")
	boxLines := strings.Split(box, "\n")
	if len(boxLines) > 0 {
		topBorder := boxLines[0]
		runes := []rune(topBorder)
		titleRunes := []rune(title)
		if len(runes) > len(titleRunes)+4 {
			copy(runes[2:2+len(titleRunes)], titleRunes)
			boxLines[0] = string(runes)
		}
		box = strings.Join(boxLines, "\n")
	}

	// Centre the box on screen.
	placed := lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Top, box)
	if m.height > 2 {
		return "\n" + placed
	}
	return placed
}

// renderActiveTabBody renders the scrollable body of the Active tab (filter + items + new list).
func (m RootModel) renderActiveTabBody(innerWidth int) string {
	var b strings.Builder

	// Filter input line.
	filterDisplay := m.taskListMenuFilter
	if filterDisplay == "" {
		filterDisplay = ""
	}
	filterLine := padOrTruncate(fmt.Sprintf("> %s  (type to filter)", filterDisplay), innerWidth)
	b.WriteString(filterLine)
	b.WriteString("\n")

	// Filtered items.
	filtered := m.filteredTaskListItems()

	highlightStyle := lipgloss.NewStyle().Bold(true)

	if len(filtered) == 0 && m.taskListMenuFilter != "" {
		b.WriteString("\n")
		b.WriteString(ui.ShortcutBarStyle.Render("  No matches"))
		b.WriteString("\n")
	}

	// Pre-compute max column widths for right-aligned stats.
	maxTW, maxSW, maxUW, maxDW := 0, 0, 0, 0
	for _, item := range filtered {
		if w := lipgloss.Width(formatMinutesAsHours(item.totalMin)); w > maxTW {
			maxTW = w
		}
		if w := lipgloss.Width(formatMinutesAsHours(item.scheduledMin)); w > maxSW {
			maxSW = w
		}
		if w := lipgloss.Width(formatMinutesAsHours(item.unschedMin)); w > maxUW {
			maxUW = w
		}
		if w := lipgloss.Width(formatMinutesAsHours(item.doneMin)); w > maxDW {
			maxDW = w
		}
	}

	tbMinutes := m.selectedTimeboxMinutes()

	for i, item := range filtered {
		b.WriteString("\n")

		prefix := "  "
		if i == m.taskListMenuCursor {
			prefix = "> "
		}

		nameStr := item.name
		if innerWidth > 2 {
			nameStr = truncate(nameStr, innerWidth-2)
		}
		if i == m.taskListMenuCursor {
			nameStr = highlightStyle.Render(nameStr)
		}
		b.WriteString(fmt.Sprintf("%s%s\n", prefix, nameStr))

		// Right-align each value within its column.
		tVal := formatMinutesAsHours(item.totalMin)
		sVal := formatMinutesAsHours(item.scheduledMin)
		uVal := formatMinutesAsHours(item.unschedMin)
		dVal := formatMinutesAsHours(item.doneMin)

		statsLine := fmt.Sprintf("    Total: %*s | Scheduled: %*s | Unscheduled: %*s | Done: %*s",
			maxTW, tVal, maxSW, sVal, maxUW, uVal, maxDW, dVal)
		statsLine = truncate(statsLine, innerWidth)
		if i == m.taskListMenuCursor {
			statsLine = highlightStyle.Render(statsLine)
		}
		b.WriteString(statsLine)
		b.WriteString("\n")

		// Next unscheduled tasks line (only when assigning to a timebox).
		if tbMinutes > 0 && len(item.nextTasks) > 0 {
			b.WriteString(m.renderNextTasksLine(item.nextTasks, tbMinutes, innerWidth, i == m.taskListMenuCursor))
			b.WriteString("\n")
		}
	}

	// New task list option (selectable via arrow keys).
	b.WriteString("\n")
	prefix := "  "
	if m.taskListMenuCursor == len(filtered) {
		prefix = "> "
	}
	newListLabel := prefix + "[+ New Task List]"
	if m.taskListMenuCursor == len(filtered) {
		newListLabel = highlightStyle.Render(newListLabel)
	} else {
		newListLabel = ui.TaskPendingStyle.Render(newListLabel)
	}
	b.WriteString(newListLabel)

	return b.String()
}

// renderActiveTabContent renders the Active tab content through the viewport.
func (m RootModel) renderActiveTabContent(innerWidth int) string {
	body := m.renderActiveTabBody(innerWidth)
	m.taskListMenuViewport.Width = innerWidth
	m.taskListMenuViewport.SetContent(body)
	scrollLabel := m.taskListMenuViewport.Indicator()

	var b strings.Builder
	b.WriteString(m.taskListMenuViewport.View())
	b.WriteString("\n")

	// Shortcut hints (always visible, outside viewport).
	hint := scrollLabel + " \u00b7 \u2191\u2193 navigate \u00b7 Enter select \u00b7 a archive \u00b7 d delete \u00b7 s sort: " +
		sortModeLabel(m.sortActive, "Total") +
		" \u00b7 Tab next tab \u00b7 Esc close"
	b.WriteString(ui.ShortcutBarStyle.Render(truncate(hint, innerWidth)))
	b.WriteString("\n")

	return b.String()
}

// sortModeLabel returns a display label for the current sort mode.
func sortModeLabel(mode SortMode, durationLabel string) string {
	switch mode {
	case SortDurationDesc:
		return durationLabel + " \u2193"
	case SortDurationAsc:
		return durationLabel + " \u2191"
	default:
		return "A-Z"
	}
}

// renderHistoryTabBody renders the scrollable body of the History tab.
func (m RootModel) renderHistoryTabBody(innerWidth int) string {
	var b strings.Builder

	if len(m.historyYears) == 0 {
		b.WriteString("\n")
		b.WriteString(ui.ShortcutBarStyle.Render("  No completed tasks yet"))
		return b.String()
	}

	// Year pills.
	for i, year := range m.historyYears {
		if i > 0 {
			b.WriteString("  ")
		}
		label := fmt.Sprintf("%d", year)
		if year == m.historySelectedYear {
			b.WriteString(ui.PillActiveStyle.Render(label))
		} else {
			b.WriteString(ui.PillInactiveStyle.Render(label))
		}
	}
	b.WriteString("\n")

	// Week pills.
	for i, week := range m.historyWeeks {
		if i > 0 {
			b.WriteString(" ")
		}
		label := fmt.Sprintf("W%d", week)
		if week == m.historySelectedWeek {
			b.WriteString(ui.PillActiveStyle.Render(label))
		} else {
			b.WriteString(ui.PillInactiveStyle.Render(label))
		}
	}
	b.WriteString("\n\n")

	if len(m.historyItems) == 0 {
		b.WriteString(ui.ShortcutBarStyle.Render("  No completed tasks for this week"))
		return b.String()
	}

	highlightStyle := lipgloss.NewStyle().Bold(true)
	boldStyle := lipgloss.NewStyle().Bold(true).Foreground(ui.ColorAccent)

	// Calculate total done minutes.
	totalDoneMin := 0
	for _, item := range m.historyItems {
		totalDoneMin += item.doneMin
	}

	// Calculate column widths.
	maxNameLen := lipgloss.Width("Total")
	for _, item := range m.historyItems {
		if w := lipgloss.Width(item.name); w > maxNameLen {
			maxNameLen = w
		}
	}

	maxDurLen := lipgloss.Width(formatMinutesAsHours(totalDoneMin))
	for _, item := range m.historyItems {
		if w := lipgloss.Width(formatMinutesAsHours(item.doneMin)); w > maxDurLen {
			maxDurLen = w
		}
	}

	doneColWidth := 5 + 1 + maxDurLen
	availNameWidth := innerWidth - 2 - 2 - doneColWidth
	if availNameWidth < 10 {
		availNameWidth = 10
	}
	if maxNameLen > availNameWidth {
		maxNameLen = availNameWidth
	}

	// Total row (not selectable, bold).
	totalLine := fmt.Sprintf("  %-*s  Done: %*s",
		maxNameLen, "Total",
		maxDurLen, formatMinutesAsHours(totalDoneMin))
	b.WriteString(boldStyle.Render(totalLine))
	b.WriteString("\n")

	// List rows.
	for i, item := range m.historyItems {
		prefix := "  "
		if i == m.historyCursor {
			prefix = "> "
		}

		displayName := item.name
		if lipgloss.Width(displayName) > maxNameLen {
			runes := []rune(displayName)
			for lipgloss.Width(string(runes)) > maxNameLen-1 && len(runes) > 0 {
				runes = runes[:len(runes)-1]
			}
			displayName = string(runes) + "\u2026"
		}

		line := fmt.Sprintf("%s%-*s  Done: %*s",
			prefix,
			maxNameLen, displayName,
			maxDurLen, formatMinutesAsHours(item.doneMin))

		if i == m.historyCursor {
			b.WriteString(highlightStyle.Render(line))
		} else {
			b.WriteString(line)
		}
		b.WriteString("\n")
	}

	return b.String()
}

// renderHistoryTabContent renders the History tab content through the viewport.
func (m RootModel) renderHistoryTabContent(innerWidth int) string {
	body := m.renderHistoryTabBody(innerWidth)
	m.taskListMenuViewport.Width = innerWidth
	m.taskListMenuViewport.SetContent(body)
	scrollLabel := m.taskListMenuViewport.Indicator()

	var b strings.Builder
	b.WriteString(m.taskListMenuViewport.View())
	b.WriteString("\n")

	// Shortcut hints (always visible, outside viewport).
	if len(m.historyYears) == 0 {
		hint := scrollLabel + " \u00b7 s sort: " + sortModeLabel(m.sortHistory, "Done") + " \u00b7 Tab next tab \u00b7 Esc close"
		b.WriteString(ui.ShortcutBarStyle.Render(truncate(hint, innerWidth)))
	} else {
		hint := scrollLabel + " \u00b7 \u2190\u2192 week \u00b7 [/] year \u00b7 \u2191\u2193 select \u00b7 Enter open \u00b7 s sort: " +
			sortModeLabel(m.sortHistory, "Done") +
			" \u00b7 Tab next tab \u00b7 Esc close"
		b.WriteString(ui.ShortcutBarStyle.Render(truncate(hint, innerWidth)))
	}
	b.WriteString("\n")

	return b.String()
}

// renderArchivedTabBody renders the scrollable body of the Archived tab.
func (m RootModel) renderArchivedTabBody() string {
	var b strings.Builder

	if len(m.archivedMenuYears) == 0 {
		b.WriteString(ui.ShortcutBarStyle.Render("  No archived task lists"))
		return b.String()
	}

	// Year pills.
	for i, year := range m.archivedMenuYears {
		if i > 0 {
			b.WriteString("  ")
		}
		label := fmt.Sprintf("%d", year)
		if year == m.archivedMenuSelectedYear {
			b.WriteString(ui.PillActiveStyle.Render(label))
		} else {
			b.WriteString(ui.PillInactiveStyle.Render(label))
		}
	}
	b.WriteString("\n\n")

	highlightStyle := lipgloss.NewStyle().Bold(true)
	headerStyle := ui.DateStyle

	if len(m.archivedMenuFlatItems) == 0 {
		b.WriteString(ui.ShortcutBarStyle.Render("  No archived task lists for this year"))
		return b.String()
	}

	for i, entry := range m.archivedMenuFlatItems {
		if entry.isHeader {
			if i > 0 {
				b.WriteString("\n")
			}
			b.WriteString(headerStyle.Render(entry.label))
			b.WriteString("\n")
		} else {
			prefix := "  "
			if i == m.archivedMenuCursor {
				prefix = "> "
			}
			nameStr := entry.name
			if i == m.archivedMenuCursor {
				nameStr = highlightStyle.Render(nameStr)
			}
			b.WriteString(fmt.Sprintf("%s%s\n", prefix, nameStr))
		}
	}

	return b.String()
}

// renderArchivedTabContent renders the Archived tab content through the viewport.
func (m RootModel) renderArchivedTabContent() string {
	body := m.renderArchivedTabBody()
	m.taskListMenuViewport.SetContent(body)
	innerWidth := m.taskListMenuViewport.Width
	scrollLabel := m.taskListMenuViewport.Indicator()

	var b strings.Builder
	b.WriteString(m.taskListMenuViewport.View())
	b.WriteString("\n")

	// Shortcut hints (always visible, outside viewport).
	hint := scrollLabel + " \u00b7 \u2190\u2192 year \u00b7 \u2191\u2193 navigate \u00b7 Enter view \u00b7 s sort: " +
		sortModeLabel(m.sortArchived, "Done") +
		" \u00b7 Tab next tab \u00b7 Esc close"
	b.WriteString(ui.ShortcutBarStyle.Render(truncate(hint, innerWidth)))
	b.WriteString("\n")

	return b.String()
}

// handleTaskListCreateConfirm creates a new task list from the input buffer
// and opens the editor for it.
func (m *RootModel) handleTaskListCreateConfirm() tea.Cmd {
	name := strings.TrimSpace(m.inputBuffer)

	if name == "" {
		return m.showToast("Task list name cannot be empty", 3*time.Second)
	}

	slug := domain.Slugify(name)
	if slug == "" {
		return m.showToast("Task list name is invalid", 3*time.Second)
	}

	path := filepath.Join(m.store.TaskListDir(), slug+".md")
	if _, err := os.Stat(path); err == nil {
		m.inputMode = InputTaskListCreate
		m.inputPrompt = "Task list already exists. Choose another name: "
		m.inputBuffer = name
		return m.showToast("Task list already exists", 3*time.Second)
	} else if !os.IsNotExist(err) {
		return m.showToast("Cannot check existing task lists. Debug source: "+err.Error(), 5*time.Second)
	}

	tl := &domain.TaskList{
		Name: name,
		Slug: slug,
	}
	if err := m.store.WriteTaskList(tl); err != nil {
		return m.showToast("Failed to create task list. Debug source: "+err.Error(), 5*time.Second)
	}

	m.clearInput()
	m.openTaskListEditor(slug)
	return nil
}

// loadHistoryData scans the archive directory for years/weeks with completed tasks
// and loads the summary for the selected year/week.
func (m *RootModel) loadHistoryData() {
	archiveDir := filepath.Join(m.store.DataDir, "archive")

	m.historyYears = nil
	m.historyWeeks = nil
	m.historyItems = nil
	m.historyCursor = 0

	yearEntries, err := os.ReadDir(archiveDir)
	if err != nil {
		return
	}

	// Discover years and weeks that have completed tasks.
	type yearWeeks struct {
		year  int
		weeks []int
	}
	var allYears []yearWeeks

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
			// Only include weeks that have completed tasks.
			durations, err := m.store.CompletedDurationsBySlug(year, week)
			if err != nil || len(durations) == 0 {
				continue
			}
			weeks = append(weeks, week)
		}

		if len(weeks) > 0 {
			sort.Ints(weeks)
			allYears = append(allYears, yearWeeks{year: year, weeks: weeks})
		}
	}

	// Sort years descending.
	sort.Slice(allYears, func(i, j int) bool {
		return allYears[i].year > allYears[j].year
	})

	if len(allYears) == 0 {
		return
	}

	for _, yw := range allYears {
		m.historyYears = append(m.historyYears, yw.year)
	}

	m.historySelectedYear = allYears[0].year
	m.historyWeeks = allYears[0].weeks
	m.historySelectedWeek = allYears[0].weeks[len(allYears[0].weeks)-1]

	m.loadHistoryWeekData()
}

// loadHistoryWeeksForYear updates historyWeeks for the currently selected year.
func (m *RootModel) loadHistoryWeeksForYear() {
	archiveDir := filepath.Join(m.store.DataDir, "archive")
	yearDir := filepath.Join(archiveDir, strconv.Itoa(m.historySelectedYear))

	m.historyWeeks = nil

	weekEntries, err := os.ReadDir(yearDir)
	if err != nil {
		return
	}

	for _, we := range weekEntries {
		if !we.IsDir() {
			continue
		}
		week, err := strconv.Atoi(we.Name())
		if err != nil {
			continue
		}
		durations, err := m.store.CompletedDurationsBySlug(m.historySelectedYear, week)
		if err != nil || len(durations) == 0 {
			continue
		}
		m.historyWeeks = append(m.historyWeeks, week)
	}

	sort.Ints(m.historyWeeks)

	if len(m.historyWeeks) > 0 {
		m.historySelectedWeek = m.historyWeeks[len(m.historyWeeks)-1]
	}
}

// loadHistoryWeekData loads the summary items for the selected year/week.
func (m *RootModel) loadHistoryWeekData() {
	durations, err := m.store.CompletedDurationsBySlug(m.historySelectedYear, m.historySelectedWeek)
	if err != nil {
		m.historyItems = nil
		return
	}

	// Build items with display names from completed.md slugs.
	// Try to resolve names from active or archived task lists.
	var items []historyMenuItem
	for slug, dur := range durations {
		name := m.resolveTaskListName(slug)
		items = append(items, historyMenuItem{
			slug:    slug,
			name:    name,
			doneMin: int(dur.Minutes()),
		})
	}

	// Default sort: alphabetical.
	sort.Slice(items, func(i, j int) bool {
		return items[i].name < items[j].name
	})

	m.historyItems = items
	m.historyCursor = 0
}

// resolveTaskListName tries to find a display name for a slug by checking
// active lists, then archived lists. Falls back to the slug itself.
func (m *RootModel) resolveTaskListName(slug string) string {
	// Check active task lists.
	tl, err := m.store.ReadTaskList(slug)
	if err == nil {
		return tl.Name
	}

	// Check archived task lists (scan all weeks).
	archivedMap, err := m.store.ListArchivedTaskLists()
	if err == nil {
		for _, weekMap := range archivedMap {
			for _, lists := range weekMap {
				for _, tl := range lists {
					if tl.Slug == slug {
						return tl.Name
					}
				}
			}
		}
	}

	// As a last resort, return the slug as-is.
	return slug
}

// selectedTimeboxMinutes returns the duration in minutes of the currently
// selected timebox, or 0 if no timebox is selected/not assignable for list
// assignment in Plan mode.
func (m RootModel) selectedTimeboxMinutes() int {
	if m.mode != ModePlan {
		return 0
	}
	dt, err := m.store.ReadDailyTimeboxes(m.currentDate)
	if err != nil {
		return 0
	}
	dt.SortByStart()
	if m.selectedTimebox < 0 || m.selectedTimebox >= len(dt.Timeboxes) {
		return 0
	}
	tb := dt.Timeboxes[m.selectedTimebox]
	if tb.IsReserved() || tb.Status == domain.StatusArchived {
		return 0
	}
	return tb.DurationMinutes()
}

// renderNextTasksLine renders the "next:" line showing upcoming unscheduled tasks.
// tbMinutes is the selected timebox duration (0 = no dimming). innerWidth is the
// available width. highlighted indicates whether this item is under the cursor.
func (m RootModel) renderNextTasksLine(tasks []domain.Task, tbMinutes, innerWidth int, highlighted bool) string {
	dimStyle := lipgloss.NewStyle().Faint(true)
	highlightStyle := lipgloss.NewStyle().Bold(true)

	prefix := "    next: "
	prefixWidth := lipgloss.Width(prefix)

	// Build task segments: "Description ~Duration"
	type segment struct {
		desc string
		dur  string
		dim  bool
	}

	var segments []segment
	cumulative := 0
	for _, t := range tasks {
		taskMin := int(t.Duration.Minutes())
		cumulative += taskMin
		dim := tbMinutes > 0 && cumulative > tbMinutes
		segments = append(segments, segment{
			desc: t.Description,
			dur:  "~" + domain.FormatDuration(t.Duration),
			dim:  dim,
		})
	}

	// Calculate available width for task descriptions.
	// Format: "    next: {desc1} {dur1} · {desc2} {dur2}"
	separatorWidth := 0
	if len(segments) > 1 {
		separatorWidth = 3 // " · "
	}
	fixedWidth := prefixWidth + separatorWidth
	for _, seg := range segments {
		fixedWidth += 1 + lipgloss.Width(seg.dur) // space + duration
	}
	descBudget := innerWidth - fixedWidth
	if descBudget < 0 {
		descBudget = 0
	}

	// Distribute description budget evenly.
	perDesc := 0
	if len(segments) > 0 && descBudget > 0 {
		perDesc = descBudget / len(segments)
	}

	// Pre-truncate descriptions to fit within budget.
	for i := range segments {
		if perDesc <= 0 {
			segments[i].desc = ""
			continue
		}
		if lipgloss.Width(segments[i].desc) > perDesc {
			runes := []rune(segments[i].desc)
			for lipgloss.Width(string(runes)) > perDesc-3 && len(runes) > 0 {
				runes = runes[:len(runes)-1]
			}
			if perDesc <= 3 {
				segments[i].desc = string(runes)
			} else {
				segments[i].desc = string(runes) + "..."
			}
		}
	}

	// Build the rendered line.
	var parts []string
	for _, seg := range segments {
		taskStr := seg.desc + " " + seg.dur
		if seg.dim {
			taskStr = seg.desc + " " + dimStyle.Render(seg.dur)
		}
		parts = append(parts, taskStr)
	}

	line := prefix + strings.Join(parts, " \u00b7 ")
	if highlighted {
		// Re-render fully highlighted (bold), preserving dim on dimmed durations.
		var hParts []string
		for _, seg := range segments {
			if seg.dim {
				hParts = append(hParts, highlightStyle.Render(seg.desc+" ")+dimStyle.Render(seg.dur))
			} else {
				hParts = append(hParts, highlightStyle.Render(seg.desc+" "+seg.dur))
			}
		}
		line = highlightStyle.Render(prefix) + strings.Join(hParts, highlightStyle.Render(" \u00b7 "))
	}

	return line
}
