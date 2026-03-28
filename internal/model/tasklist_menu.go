package model

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hwang/meadow-bubbletea/internal/domain"
	"github.com/hwang/meadow-bubbletea/internal/ui"
)

// taskListMenuItem holds a task list's slug, display name, and weekly stats.
type taskListMenuItem struct {
	slug         string
	name         string
	totalMin     int
	scheduledMin int
	unschedMin   int
	doneMin      int
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
		scheduledMin int
		doneMin      int
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

		_, scheduledMinByIndex := sequenceDailyTimeboxes(dt, listTasks)

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
		items = append(items, taskListMenuItem{
			slug:         tl.Slug,
			name:         tl.Name,
			totalMin:     totalMin,
			scheduledMin: s.scheduledMin,
			unschedMin:   unschedMin,
			doneMin:      s.doneMin,
		})
	}

	m.taskListMenuItems = items
	m.showTaskListMenu = true
	m.taskListMenuTab = MenuTabActive
	m.taskListMenuFilter = ""
	if len(items) > 0 {
		m.taskListMenuCursor = 0
	} else {
		m.taskListMenuCursor = -1
	}
}

// closeTaskListMenu hides the task list picker overlay.
func (m *RootModel) closeTaskListMenu() {
	m.showTaskListMenu = false
}

// handleTaskListMenuKey processes key events while the menu overlay is open.
func (m *RootModel) handleTaskListMenuKey(msg tea.KeyMsg) tea.Cmd {
	if m.taskListMenuTab == MenuTabArchived {
		return m.handleArchivedTabKey(msg)
	}
	return m.handleActiveTabKey(msg)
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
		m.taskListMenuTab = MenuTabArchived
		m.loadArchivedMenuData()
		return nil

	case "up":
		if m.taskListMenuCursor > -1 {
			m.taskListMenuCursor--
		}
		return nil

	case "down":
		// Allow cursor to reach the "+ New Task List" item (index == len(filtered)).
		if m.taskListMenuCursor < len(filtered) {
			m.taskListMenuCursor++
		}
		return nil

	case "enter":
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

	case "backspace":
		if len(m.taskListMenuFilter) > 0 {
			runes := []rune(m.taskListMenuFilter)
			m.taskListMenuFilter = string(runes[:len(runes)-1])
			m.resetMenuCursor()
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
		}
		return nil
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
		return nil

	case "left":
		// Navigate year pills to older year.
		idx := m.archivedYearIndex()
		if idx < len(m.archivedMenuYears)-1 {
			m.archivedMenuSelectedYear = m.archivedMenuYears[idx+1]
			m.rebuildArchivedFlatItems()
			m.archivedMenuCursor = 0
			m.skipToNextSelectableItem(1)
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
		return nil

	case "down":
		if m.archivedMenuCursor < len(m.archivedMenuFlatItems)-1 {
			m.archivedMenuCursor++
			m.skipToNextSelectableItem(1)
		}
		return nil

	case "enter":
		if m.archivedMenuCursor >= 0 && m.archivedMenuCursor < len(m.archivedMenuFlatItems) {
			entry := m.archivedMenuFlatItems[m.archivedMenuCursor]
			if !entry.isHeader {
				m.closeTaskListMenu()
				m.openArchivedTaskListEditor(entry.year, entry.week, entry.slug)
			}
		}
		return nil
	}

	return nil
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

// handleTaskListSelect acts on the chosen menu item. In Plan mode with an
// unassigned timebox it assigns the task list; otherwise it opens the task
// list editor.
func (m *RootModel) handleTaskListSelect(item taskListMenuItem) tea.Cmd {
	defer m.closeTaskListMenu()

	// Plan mode: assign to the currently selected timebox if it is unassigned.
	if m.mode == ModePlan {
		dt, err := m.store.ReadDailyTimeboxes(m.currentDate)
		if err == nil {
			dt.SortByStart()
			if m.selectedTimebox >= 0 && m.selectedTimebox < len(dt.Timeboxes) {
				tb := &dt.Timeboxes[m.selectedTimebox]
				if (tb.Status == domain.StatusUnassigned || tb.TaskListSlug == "") && !tb.IsReserved() {
					tb.TaskListSlug = item.slug
					tb.Status = domain.StatusActive
					_ = m.store.WriteDailyTimeboxes(dt)
					return nil
				}
			}
		}
	}

	// Default: open the task list editor.
	m.openTaskListEditor(item.slug)
	return nil
}

// renderTaskListMenu renders the task list picker overlay centred on screen.
func (m RootModel) renderTaskListMenu() string {
	// Determine overlay width.
	menuWidth := 80
	if m.width-4 < menuWidth {
		menuWidth = m.width - 4
	}
	if menuWidth < 20 {
		menuWidth = 20
	}

	innerWidth := menuWidth - 4 // account for border padding

	var b strings.Builder

	// Tab pills.
	b.WriteString(ui.ModePill("Active", m.taskListMenuTab == MenuTabActive))
	b.WriteString(" ")
	b.WriteString(ui.ModePill("Archived", m.taskListMenuTab == MenuTabArchived))
	b.WriteString("\n\n")

	if m.taskListMenuTab == MenuTabArchived {
		b.WriteString(m.renderArchivedTabContent())
	} else {
		b.WriteString(m.renderActiveTabContent(innerWidth))
	}

	// Show confirmation dialog if active.
	if m.showConfirm {
		b.WriteString("\n")
		confirmStyle := lipgloss.NewStyle().Bold(true).Foreground(ui.ColorAccent)
		b.WriteString(confirmStyle.Render("Are you sure? (y/n)"))
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
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

// renderActiveTabContent renders the Active tab content.
func (m RootModel) renderActiveTabContent(innerWidth int) string {
	var b strings.Builder

	// Filter input line.
	filterDisplay := m.taskListMenuFilter
	if filterDisplay == "" {
		filterDisplay = ""
	}
	filterLine := fmt.Sprintf("> %s", filterDisplay)
	if lipgloss.Width(filterLine) < innerWidth {
		filterLine += strings.Repeat("_", innerWidth-lipgloss.Width(filterLine))
	}
	b.WriteString(filterLine)
	b.WriteString("  (type to filter)")
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

	for i, item := range filtered {
		b.WriteString("\n")

		prefix := "  "
		if i == m.taskListMenuCursor {
			prefix = "> "
		}

		nameStr := item.name
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
		if i == m.taskListMenuCursor {
			statsLine = highlightStyle.Render(statsLine)
		}
		b.WriteString(statsLine)
		b.WriteString("\n")
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
	b.WriteString("\n")

	// Shortcut hints.
	b.WriteString("\n")
	b.WriteString(ui.ShortcutBarStyle.Render("\u2191\u2193 navigate \u00b7 Enter select \u00b7 a archive \u00b7 d delete \u00b7 Tab archived \u00b7 Esc close"))
	b.WriteString("\n")

	return b.String()
}

// renderArchivedTabContent renders the Archived tab content.
func (m RootModel) renderArchivedTabContent() string {
	var b strings.Builder

	if len(m.archivedMenuYears) == 0 {
		b.WriteString(ui.ShortcutBarStyle.Render("  No archived task lists"))
		b.WriteString("\n\n")
		b.WriteString(ui.ShortcutBarStyle.Render("Tab active \u00b7 Esc close"))
		b.WriteString("\n")
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
		b.WriteString("\n")
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

	// Shortcut hints.
	b.WriteString("\n")
	b.WriteString(ui.ShortcutBarStyle.Render("\u2190\u2192 year \u00b7 \u2191\u2193 navigate \u00b7 Enter view \u00b7 Tab active \u00b7 Esc close"))
	b.WriteString("\n")

	return b.String()
}

// handleTaskListCreateConfirm creates a new task list from the input buffer
// and opens the editor for it.
func (m *RootModel) handleTaskListCreateConfirm() {
	name := strings.TrimSpace(m.inputBuffer)
	m.clearInput()

	if name == "" {
		return
	}

	slug := domain.Slugify(name)
	if slug == "" {
		return
	}

	if _, err := m.store.ReadTaskList(slug); err == nil {
		m.inputMode = InputTaskListCreate
		m.inputPrompt = "Task list already exists. Choose another name: "
		m.inputBuffer = name
		return
	} else if err != nil && !os.IsNotExist(err) {
		return
	}

	tl := &domain.TaskList{
		Name: name,
		Slug: slug,
	}
	if err := m.store.WriteTaskList(tl); err != nil {
		return
	}

	m.openTaskListEditor(slug)
}
