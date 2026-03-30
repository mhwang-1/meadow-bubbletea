package model

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mhwang-1/meadow-bubbletea/internal/domain"
	"github.com/mhwang-1/meadow-bubbletea/internal/store"
	"github.com/mhwang-1/meadow-bubbletea/internal/ui"
)

// Mode represents the active interaction mode.
type Mode int

const (
	ModeExecute Mode = iota
	ModePlan
)

// View represents which calendar scope is displayed.
type View int

const (
	ViewDay View = iota
	ViewWeek
)

// MenuTab represents the active tab in the task list menu.
type MenuTab int

const (
	MenuTabActive MenuTab = iota
	MenuTabHistory
	MenuTabArchived
)

// SortMode represents the current sort order for task list menus.
type SortMode int

const (
	SortAlphabetical SortMode = iota
	SortDurationDesc
	SortDurationAsc
)

// RootModel is the top-level BubbleTea model for Meadow.
type RootModel struct {
	store           *store.Store
	mode            Mode
	view            View
	currentDate     time.Time
	width           int
	height          int
	ready           bool // set after the first WindowSizeMsg
	selectedTimebox int  // cursor index for timebox selection in day view
	selectedWeekday int  // 0=Sun..6=Sat cursor in week view

	// Input mode for timebox creation/editing.
	inputMode   InputMode
	inputBuffer string // text being typed for manual timebox creation or time editing
	inputPrompt string // prompt to show for current input

	// Confirmation dialog.
	showConfirm    bool
	confirmAction  func() // action to execute on confirm
	confirmMessage string // custom confirmation prompt (empty = default)

	// Task list menu overlay (slash menu).
	showTaskListMenu     bool
	taskListMenuTab      MenuTab
	taskListMenuFilter   string             // search text
	taskListMenuCursor   int                // selected index in the filtered list
	taskListMenuItems    []taskListMenuItem // cached list of task lists with stats
	taskListMenuViewport scrollViewport     // scrollable viewport for menu content

	// Archived tab state.
	archivedMenuYears        []int               // descending
	archivedMenuSelectedYear int                 // currently selected year pill
	archivedMenuFlatItems    []archivedMenuEntry // precomputed flat list for cursor
	archivedMenuCursor       int                 // flat cursor for grouped list

	// History tab state.
	historyYears        []int             // descending
	historySelectedYear int               // currently selected year pill
	historyWeeks        []int             // weeks for selected year, descending
	historySelectedWeek int               // currently selected week pill
	historyItems        []historyMenuItem // summary rows for selected week
	historyCursor       int               // cursor among list rows (0-indexed, excludes Total)

	// Sort state (per tab, reset on menu open).
	sortActive   SortMode
	sortHistory  SortMode
	sortArchived SortMode

	// Editor origin tracking for return-from-editor routing.
	editorOriginTab  MenuTab
	editorOriginYear int
	editorOriginWeek int

	// Task list editor overlay.
	showTaskListEditor  bool
	editingTaskListSlug string
	taskListEditor      TaskListEditorModel

	// Notes editor overlay.
	showNotesEditor bool
	notesEditor     EditorModel

	// Toast notification.
	toastMessage string
	toastExpiry  time.Time
}

// toastTickMsg is sent when a toast notification should be cleared.
type toastTickMsg struct{}

// NewRootModel creates a RootModel with sensible defaults.
func NewRootModel(s *store.Store) RootModel {
	return RootModel{
		store:           s,
		mode:            ModeExecute,
		view:            ViewDay,
		currentDate:     time.Now(),
		selectedWeekday: int(time.Now().Weekday()),
	}
}

// Init implements tea.Model. No initial command is needed.
func (m RootModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m RootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.KeyMsg:
		key := msg.String()

		// When the notes editor is open, it captures all key events.
		if m.showNotesEditor && m.showConfirm {
			switch key {
			case "y":
				if m.confirmAction != nil {
					m.confirmAction()
				}
				m.clearConfirm()
			case "n", "esc":
				m.clearConfirm()
				m.showNotesEditor = false
			}
			return m, nil
		}
		if m.showNotesEditor {
			cmd := m.handleNotesEditorKey(msg)
			return m, cmd
		}

		// When the task list editor is open, it captures all key events.
		// Handle confirmation dialogs triggered by the editor (e.g. save on Esc).
		if m.showTaskListEditor && m.showConfirm {
			switch key {
			case "y":
				if m.confirmAction != nil {
					m.confirmAction()
				}
				m.clearConfirm()
			case "n", "esc":
				m.clearConfirm()
				m.closeTaskListEditor()
			}
			return m, nil
		}
		if m.showTaskListEditor {
			cmd := m.handleTaskListEditorKey(msg)
			return m, cmd
		}

		// When the task list menu is open, it captures all key events.
		if m.showTaskListMenu && m.showConfirm {
			switch key {
			case "y":
				if m.confirmAction != nil {
					m.confirmAction()
				}
				m.clearConfirm()
			case "n", "esc":
				m.clearConfirm()
			}
			return m, nil
		}
		if m.showTaskListMenu {
			cmd := m.handleTaskListMenuKey(msg)
			return m, cmd
		}

		// When a confirmation dialog is showing, capture y/n/escape.
		if m.showConfirm {
			switch key {
			case "y":
				if m.confirmAction != nil {
					m.confirmAction()
				}
				m.clearConfirm()
			case "n", "esc":
				m.clearConfirm()
			}
			return m, nil
		}

		// When in input mode, capture typed characters.
		if m.inputMode != InputNone {
			switch {
			case isConfirmKey(key):
				switch m.inputMode {
				case InputTimeboxCreate:
					m.handleTimeboxCreateConfirm()
				case InputTimeboxEdit:
					m.handleTimeboxEditConfirm()
				case InputTaskListCreate:
					return m, m.handleTaskListCreateConfirm()
				case InputReservedNote:
					m.handleReservedNoteConfirm()
				}
			case key == "esc":
				m.clearInput()
			case key == "backspace":
				m.handleInputBackspace()
			default:
				// Append printable characters (supports IME and paste).
				for _, r := range msg.Runes {
					m.handleInputChar(r)
				}
			}
			return m, nil
		}

		switch key {
		case "-":
			m.openNotesEditor()
			return m, nil
		case "/":
			m.openTaskListMenu()
			return m, nil
		case "n":
			if m.mode == ModePlan && m.view == ViewDay {
				m.handleTimeboxCreate()
			}
		case "enter":
			if m.view == ViewWeek {
				wi := domain.WeekForDate(m.currentDate)
				m.currentDate = wi.StartDate.AddDate(0, 0, m.selectedWeekday)
				m.view = ViewDay
				m.selectedTimebox = 0
				return m, nil
			}
			if m.mode == ModePlan && m.view == ViewDay && m.maxTimeboxCount() > 0 {
				m.handleTimeboxEdit()
			}
		case "r":
			if m.mode == ModePlan && m.view == ViewDay && m.maxTimeboxCount() > 0 {
				m.handleToggleReserved()
			}
		case "d":
			if m.mode == ModePlan && m.view == ViewDay && m.maxTimeboxCount() > 0 {
				m.handleTimeboxDelete()
			}
		case "a":
			if m.view == ViewDay && m.maxTimeboxCount() > 0 {
				m.handleTimeboxArchive()
			}
		case "U":
			if m.view == ViewDay && m.maxTimeboxCount() > 0 {
				m.handleTimeboxUnarchive()
			}
		case "x":
			if m.mode == ModeExecute && m.view == ViewDay {
				m.handleMarkDone()
			}
		case "u":
			if m.mode == ModeExecute && m.view == ViewDay {
				m.handleUnmarkDone()
			} else if m.mode == ModePlan && m.view == ViewDay && m.maxTimeboxCount() > 0 {
				return m, m.handleTimeboxUnassign()
			}
		case "tab":
			// Toggle mode: Execute <-> Plan
			if m.mode == ModeExecute {
				m.mode = ModePlan
			} else {
				m.mode = ModeExecute
			}
		case "shift+tab":
			// Toggle view: Day <-> Week
			if m.view == ViewDay {
				m.view = ViewWeek
				m.selectedWeekday = int(m.currentDate.Weekday())
			} else {
				m.view = ViewDay
				wi := domain.WeekForDate(m.currentDate)
				m.currentDate = wi.StartDate.AddDate(0, 0, m.selectedWeekday)
			}
		case "up":
			if m.selectedTimebox > 0 {
				m.selectedTimebox--
			}
		case "down":
			max := m.maxTimeboxCount()
			if max > 0 && m.selectedTimebox < max-1 {
				m.selectedTimebox++
			}
		case "left":
			if m.view == ViewWeek {
				if m.selectedWeekday > 0 {
					m.selectedWeekday--
				} else {
					m.currentDate = m.currentDate.AddDate(0, 0, -7)
					m.selectedWeekday = 6
				}
				wi := domain.WeekForDate(m.currentDate)
				m.currentDate = wi.StartDate.AddDate(0, 0, m.selectedWeekday)
			} else {
				m.currentDate = m.currentDate.AddDate(0, 0, -1)
				m.selectedTimebox = 0
			}
		case "right":
			if m.view == ViewWeek {
				if m.selectedWeekday < 6 {
					m.selectedWeekday++
				} else {
					m.currentDate = m.currentDate.AddDate(0, 0, 7)
					m.selectedWeekday = 0
				}
				wi := domain.WeekForDate(m.currentDate)
				m.currentDate = wi.StartDate.AddDate(0, 0, m.selectedWeekday)
			} else {
				m.currentDate = m.currentDate.AddDate(0, 0, 1)
				m.selectedTimebox = 0
			}
		case "t":
			// Jump to today.
			now := time.Now()
			m.currentDate = now
			m.selectedTimebox = 0
			m.selectedWeekday = int(now.Weekday())
		case "q", "ctrl+c":
			return m, tea.Quit
		}

	case toastTickMsg:
		if !m.toastExpiry.IsZero() && time.Now().After(m.toastExpiry) {
			m.toastMessage = ""
			m.toastExpiry = time.Time{}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		// Resize menu viewport if open.
		if m.showTaskListMenu {
			vpHeight := m.taskListMenuViewportHeight()
			menuWidth := m.taskListMenuWidth()
			m.taskListMenuViewport.Width = menuWidth - 4
			m.taskListMenuViewport.Height = vpHeight
		}
	}

	return m, nil
}

// View implements tea.Model.
func (m RootModel) View() string {
	if !m.ready {
		return "Loading..."
	}

	// Build mode and view labels for the header.
	modeStr := "Execute"
	if m.mode == ModePlan {
		modeStr = "Plan"
	}

	viewStr := "Day"
	if m.view == ViewWeek {
		viewStr = "Week"
	}

	var dateStr string
	if m.view == ViewWeek {
		wi := domain.WeekForDate(m.currentDate)
		dateStr = fmt.Sprintf("W%d · %s", wi.Week, domain.FormatWeekRange(wi))
	} else {
		wi := domain.WeekForDate(m.currentDate)
		dateStr = fmt.Sprintf("%s · W%d", domain.FormatDate(m.currentDate), wi.Week)

		// Show day difference from today if not viewing today.
		now := time.Now()
		todayDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		viewDate := time.Date(m.currentDate.Year(), m.currentDate.Month(), m.currentDate.Day(), 0, 0, 0, 0, now.Location())
		dayDiff := int(viewDate.Sub(todayDate).Hours() / 24)
		if dayDiff != 0 {
			sign := "+"
			if dayDiff < 0 {
				sign = ""
			}
			dateStr += fmt.Sprintf(" (%s%d)", sign, dayDiff)
		}
	}

	header := ui.RenderHeader(m.width, modeStr, viewStr, dateStr)

	// Calendar content rendered by calendar.go.
	var calendarContent string
	switch m.view {
	case ViewDay:
		calendarContent = m.renderDayView()
	case ViewWeek:
		calendarContent = m.renderWeekView()
	}

	shortcutBar := ui.RenderShortcutBar(m.width, m.shortcutBarText())

	// Build the bottom section: input line, confirm dialog, or shortcut bar.
	var bottomSection string
	if m.inputMode != InputNone {
		bottomSection = lipgloss.JoinVertical(lipgloss.Left,
			m.renderInputLine(),
			shortcutBar,
		)
	} else if m.showConfirm {
		bottomSection = lipgloss.JoinVertical(lipgloss.Left,
			m.renderConfirmLine(),
			shortcutBar,
		)
	} else {
		bottomSection = shortcutBar
	}

	base := lipgloss.JoinVertical(lipgloss.Left,
		header,
		calendarContent,
		bottomSection,
	)
	if m.toastMessage != "" && !m.toastExpiry.IsZero() && time.Now().Before(m.toastExpiry) {
		toastStyle := lipgloss.NewStyle().Bold(true).Foreground(ui.ColorAccent)
		base = lipgloss.JoinVertical(lipgloss.Left, base, toastStyle.Render(m.toastMessage))
	}

	// Overlay the notes editor if it is open.
	if m.showNotesEditor {
		return m.renderNotesEditor()
	}

	// Overlay the task list editor if it is open.
	if m.showTaskListEditor {
		return m.renderTaskListEditor()
	}

	// Overlay the task list menu if it is open.
	if m.showTaskListMenu {
		return m.renderTaskListMenu()
	}

	return base
}

func isConfirmKey(key string) bool {
	return key == "enter" || key == "ctrl+m" || key == "ctrl+j"
}

// showToast sets a toast message that auto-clears after the given duration.
func (m *RootModel) showToast(msg string, d time.Duration) tea.Cmd {
	m.toastMessage = msg
	m.toastExpiry = time.Now().Add(d)
	return tea.Tick(d, func(time.Time) tea.Msg {
		return toastTickMsg{}
	})
}

// openNotesEditor loads notes.md and opens the full-screen editor.
func (m *RootModel) openNotesEditor() {
	content, err := m.store.ReadNotes()
	if err != nil {
		content = ""
	}

	editorHeight := m.height - 3
	if editorHeight < 5 {
		editorHeight = 5
	}

	m.notesEditor = NewEditorModel(content, m.width, editorHeight)
	m.showNotesEditor = true
}

// handleNotesEditorKey routes key events to the notes editor.
func (m *RootModel) handleNotesEditorKey(msg tea.KeyMsg) tea.Cmd {
	key := msg.String()
	if key == "esc" {
		if m.notesEditor.IsModified() {
			m.showConfirm = true
			m.confirmAction = func() {
				_ = m.store.WriteNotes(m.notesEditor.Content())
				m.showNotesEditor = false
			}
			return nil
		}
		m.showNotesEditor = false
		return nil
	}

	saved, closed := m.notesEditor.HandleKey(msg)

	if saved {
		if err := m.store.WriteNotes(m.notesEditor.Content()); err != nil {
			return m.showToast("Failed to save notes", 3*time.Second)
		}
		m.notesEditor.modified = false
		return m.showToast("Saved", 2*time.Second)
	}

	if closed {
		m.showNotesEditor = false
	}

	return nil
}

// renderNotesEditor renders the full-screen notes editor overlay.
func (m *RootModel) renderNotesEditor() string {
	var b strings.Builder

	nameStyle := lipgloss.NewStyle().Bold(true).Foreground(ui.ColorText)
	b.WriteString(nameStyle.Render("Notes"))
	b.WriteString("\n")

	sepStyle := lipgloss.NewStyle().Foreground(ui.ColorBorder)
	b.WriteString(sepStyle.Render(strings.Repeat("\u2500", m.width)))
	b.WriteString("\n")

	contentHeight := m.height - 3
	if contentHeight < 3 {
		contentHeight = 3
	}

	m.notesEditor.Resize(m.width, contentHeight)
	b.WriteString(m.notesEditor.View())

	if m.showConfirm {
		b.WriteString("\n")
		confirmStyle := lipgloss.NewStyle().Bold(true).Foreground(ui.ColorAccent)
		b.WriteString(confirmStyle.Render("Save changes before closing? (y/n)"))
	}

	if m.toastMessage != "" && !m.toastExpiry.IsZero() && time.Now().Before(m.toastExpiry) {
		b.WriteString("\n")
		toastStyle := lipgloss.NewStyle().Bold(true).Foreground(ui.ColorAccent)
		b.WriteString(toastStyle.Render(m.toastMessage))
	}

	content := b.String()
	return lipgloss.Place(m.width, m.height, lipgloss.Left, lipgloss.Top, content)
}

// shortcutBarText returns context-sensitive keybinding hints for the bottom bar.
func (m RootModel) shortcutBarText() string {
	switch {
	case m.view == ViewDay && m.mode == ModeExecute:
		return "↑↓ navigate · x done · u undo · a archive · U unarchive · / lists · - notes · t today · Tab plan · Shift+Tab week · q quit"
	case m.view == ViewDay && m.mode == ModePlan:
		return "↑↓ navigate · n new · Enter edit · / assign/change · u unassign · r reserve · d delete · a archive · U unarchive · - notes · t today · Tab execute · Shift+Tab week · q quit"
	case m.view == ViewWeek:
		return "←→ navigate · Enter open day · / task lists · - notes · t today · Tab mode · Shift+Tab day · q quit"
	default:
		return "Tab mode · Shift+Tab view · q quit"
	}
}
