package telegram

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/mhwang-1/meadow-bubbletea/internal/domain"
)

// WizardState tracks the current wizard flow for a chat session.
type WizardState struct {
	Action    string
	Params    map[string]string
	MessageID int
	UpdatedAt time.Time
}

const sessionTTL = 5 * time.Minute

// sessionStore manages wizard sessions keyed by chatID.
type sessionStore struct {
	mu       sync.Mutex
	sessions map[int64]*WizardState
}

func newSessionStore() *sessionStore {
	return &sessionStore{
		sessions: make(map[int64]*WizardState),
	}
}

func (ss *sessionStore) get(chatID int64) *WizardState {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	ws, ok := ss.sessions[chatID]
	if !ok {
		return nil
	}
	if time.Since(ws.UpdatedAt) > sessionTTL {
		delete(ss.sessions, chatID)
		return nil
	}
	return ws
}

func (ss *sessionStore) set(chatID int64, ws *WizardState) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	ws.UpdatedAt = time.Now()
	ss.sessions[chatID] = ws
}

func (ss *sessionStore) clear(chatID int64) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	delete(ss.sessions, chatID)
}

// today returns today's date with time zeroed.
func today() time.Time {
	return domain.StripTimeForDate(time.Now())
}

// routeCallback parses callback data and dispatches to the appropriate handler.
func routeCallback(b *Bot, query *tgbotapi.CallbackQuery) {
	data := query.Data
	chatID := query.Message.Chat.ID
	msgID := query.Message.MessageID

	// Acknowledge the callback.
	callback := tgbotapi.NewCallback(query.ID, "")
	if _, err := b.api.Request(callback); err != nil {
		log.Printf("Error acknowledging callback: %v", err)
	}

	// Clear any pending text-input wizard on button press (unless we're
	// in a flow that specifically expects text).
	parts := strings.SplitN(data, ":", 2)
	action := parts[0]

	switch action {
	case "main":
		dateStr := ""
		if len(parts) > 1 {
			dateStr = parts[1]
		}
		date := parseDateOrToday(dateStr)
		handleNav(b, chatID, msgID, date)

	case "nav":
		dateStr := ""
		if len(parts) > 1 {
			dateStr = parts[1]
		}
		date := parseDateOrToday(dateStr)
		handleNav(b, chatID, msgID, date)

	case "done":
		if len(parts) < 2 {
			return
		}
		handleDoneCallback(b, chatID, msgID, parts[1])

	case "arch":
		if len(parts) < 2 {
			return
		}
		handleArchiveCallback(b, chatID, msgID, parts[1])

	case "lists":
		tab := "active"
		if len(parts) > 1 {
			tab = parts[1]
		}
		handleListsStart(b, chatID, msgID, tab)

	case "list":
		if len(parts) < 2 {
			return
		}
		handleListCallback(b, chatID, msgID, parts[1])

	case "tb":
		if len(parts) < 2 {
			return
		}
		handleTimeboxCallback(b, chatID, msgID, parts[1])

	case "notes":
		if len(parts) > 1 && parts[1] == "edit" {
			handleNotesEdit(b, chatID, msgID)
		} else {
			handleNotesStart(b, chatID, msgID)
		}

	case "undo":
		if len(parts) < 2 {
			return
		}
		handleUndoCallback(b, chatID, msgID, parts[1])

	case "unarc":
		if len(parts) < 2 {
			return
		}
		handleUnarchiveCallback(b, chatID, msgID, parts[1])

	case "hist":
		if len(parts) < 2 {
			return
		}
		handleHistoryCallback(b, chatID, msgID, parts[1])

	case "arcd":
		if len(parts) < 2 {
			return
		}
		handleArchivedCallback(b, chatID, msgID, parts[1])

	case "cancel":
		b.sessions.clear(chatID)
		dateStr := ""
		if len(parts) > 1 {
			dateStr = parts[1]
		}
		date := parseDateOrToday(dateStr)
		handleNav(b, chatID, msgID, date)
	}
}

// handleWizardTextInput handles text input when a wizard session is active.
func handleWizardTextInput(b *Bot, chatID int64, ws *WizardState, text string) {
	switch ws.Action {
	case "add_task":
		handleAddTaskText(b, chatID, ws, text)
	case "custom_time":
		handleCustomTimeText(b, chatID, ws, text)
	case "edit_notes":
		handleEditNotesText(b, chatID, ws, text)
	case "reserve_note":
		handleReserveNoteText(b, chatID, ws, text)
	default:
		b.sessions.clear(chatID)
		msg := tgbotapi.NewMessage(chatID, "Session expired. Use /start.")
		if _, err := b.api.Send(msg); err != nil {
			log.Printf("Error sending message: %v", err)
		}
	}
}

// handleDoneCallback routes done wizard sub-actions.
// Formats: {date}, {date}:{tbIdx}, {date}:{tbIdx}:{taskIdx}
func handleDoneCallback(b *Bot, chatID int64, msgID int, params string) {
	parts := strings.Split(params, ":")
	switch len(parts) {
	case 1:
		// done:{date} — start done wizard
		date := parseDateOrToday(parts[0])
		handleDoneStart(b, chatID, msgID, date)
	case 2:
		// done:{date}:{tbIdx} — select timebox, show tasks
		date := parseDateOrToday(parts[0])
		tbIdx, _ := strconv.Atoi(parts[1])
		handleDoneSelectTimebox(b, chatID, msgID, date, tbIdx)
	case 3:
		// done:{date}:{tbIdx}:{taskIdx} — mark task done
		date := parseDateOrToday(parts[0])
		tbIdx, _ := strconv.Atoi(parts[1])
		taskIdx, _ := strconv.Atoi(parts[2])
		handleDoneSelectTask(b, chatID, msgID, date, tbIdx, taskIdx)
	}
}

// handleArchiveCallback routes archive wizard sub-actions.
// Formats: {date}, {date}:{tbIdx}, {date}:{tbIdx}:f
func handleArchiveCallback(b *Bot, chatID int64, msgID int, params string) {
	parts := strings.Split(params, ":")
	switch len(parts) {
	case 1:
		// arch:{date} — start archive wizard
		date := parseDateOrToday(parts[0])
		handleArchiveStart(b, chatID, msgID, date)
	case 2:
		// arch:{date}:{tbIdx} — archive specific timebox
		date := parseDateOrToday(parts[0])
		tbIdx, _ := strconv.Atoi(parts[1])
		handleArchiveSelect(b, chatID, msgID, date, tbIdx)
	case 3:
		// arch:{date}:{tbIdx}:f — force archive
		date := parseDateOrToday(parts[0])
		tbIdx, _ := strconv.Atoi(parts[1])
		handleArchiveConfirm(b, chatID, msgID, date, tbIdx, parts[2] == "f")
	}
}

// handleListCallback routes list wizard sub-actions.
// Formats: {slug}, {slug}:{action}, {slug}:{action}:{param}
func handleListCallback(b *Bot, chatID int64, msgID int, params string) {
	parts := strings.SplitN(params, ":", 3)
	switch len(parts) {
	case 1:
		// list:{slug} — view list
		handleListSelect(b, chatID, msgID, parts[0])
	case 2:
		// list:{slug}:{action}
		slug := parts[0]
		subAction := parts[1]
		switch subAction {
		case "assign":
			handleListAssign(b, chatID, msgID, slug)
		case "edit":
			handleListEdit(b, chatID, msgID, slug)
		case "arch":
			handleListArchive(b, chatID, msgID, slug)
		case "del":
			handleListDelete(b, chatID, msgID, slug)
		case "add":
			handleListAddTask(b, chatID, msgID, slug)
		}
	case 3:
		// list:{slug}:{action}:{param}
		slug := parts[0]
		subAction := parts[1]
		param := parts[2]
		switch subAction {
		case "rm":
			taskIdx, _ := strconv.Atoi(param)
			handleListRemoveTask(b, chatID, msgID, slug, taskIdx)
		case "at":
			// Assign to timebox: list:{slug}:at:{date}:{tbIdx}
			// param = {date}:{tbIdx} but we used SplitN(3) so it's joined
			subParts := strings.SplitN(param, ":", 2)
			if len(subParts) == 2 {
				date := parseDateOrToday(subParts[0])
				tbIdx, _ := strconv.Atoi(subParts[1])
				handleListAssignTimebox(b, chatID, msgID, slug, date, tbIdx)
			}
		}
	}
}

// handleTimeboxCallback routes new timebox wizard sub-actions.
func handleTimeboxCallback(b *Bot, chatID int64, msgID int, params string) {
	parts := strings.Split(params, ":")
	switch {
	case parts[0] == "new" && len(parts) == 1:
		// tb:new — start new timebox wizard
		handleNewTimeboxStart(b, chatID, msgID)
	case parts[0] == "new" && len(parts) == 2:
		// tb:new:{date} — select day
		handleNewTimeboxDay(b, chatID, msgID, parts[1])
	case parts[0] == "new" && len(parts) == 3:
		// tb:new:{date}:{startTime}
		handleNewTimeboxStartTime(b, chatID, msgID, parts[1], parts[2])
	case parts[0] == "new" && len(parts) == 4:
		// tb:new:{date}:{startTime}:{endTime}
		handleNewTimeboxEnd(b, chatID, msgID, parts[1], parts[2], parts[3])
	case parts[0] == "post" && len(parts) >= 2:
		// tb:post:{action}:{params...}
		handleTimeboxPostAction(b, chatID, msgID, parts[1:])
	}
}

// handleUndoCallback routes undo (unmark done) actions.
// Format: undo:{date}:{tbIdx}:{taskIdx}
func handleUndoCallback(b *Bot, chatID int64, msgID int, params string) {
	parts := strings.Split(params, ":")
	if len(parts) < 3 {
		return
	}
	date := parseDateOrToday(parts[0])
	tbIdx, _ := strconv.Atoi(parts[1])
	taskIdx, _ := strconv.Atoi(parts[2])
	handleUndoDone(b, chatID, msgID, date, tbIdx, taskIdx)
}

// handleUnarchiveCallback routes unarchive actions.
// Format: unarc:{date}:{tbIdx}
func handleUnarchiveCallback(b *Bot, chatID int64, msgID int, params string) {
	parts := strings.Split(params, ":")
	if len(parts) < 2 {
		return
	}
	date := parseDateOrToday(parts[0])
	tbIdx, _ := strconv.Atoi(parts[1])
	handleUnarchive(b, chatID, msgID, date, tbIdx)
}

// handleHistoryCallback routes history browsing.
// Format: hist:{year}:{week}
func handleHistoryCallback(b *Bot, chatID int64, msgID int, params string) {
	parts := strings.Split(params, ":")
	if len(parts) < 2 {
		return
	}
	year, _ := strconv.Atoi(parts[0])
	week, _ := strconv.Atoi(parts[1])
	handleHistoryBrowse(b, chatID, msgID, year, week)
}

// handleArchivedCallback routes archived lists browsing.
// Format: arcd:{year}:{week}
func handleArchivedCallback(b *Bot, chatID int64, msgID int, params string) {
	parts := strings.Split(params, ":")
	if len(parts) < 2 {
		return
	}
	year, _ := strconv.Atoi(parts[0])
	week, _ := strconv.Atoi(parts[1])
	handleArchivedBrowse(b, chatID, msgID, year, week)
}

// parseDateOrToday parses a date string or returns today.
func parseDateOrToday(s string) time.Time {
	if s == "" {
		return today()
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return today()
	}
	return t
}

// editMessage edits an existing message's text and keyboard.
func editMessage(b *Bot, chatID int64, msgID int, text string, keyboard *tgbotapi.InlineKeyboardMarkup) {
	edit := tgbotapi.NewEditMessageText(chatID, msgID, text)
	edit.ParseMode = "HTML"
	if keyboard != nil {
		edit.ReplyMarkup = keyboard
	}
	if _, err := b.api.Send(edit); err != nil {
		log.Printf("Error editing message: %v", err)
	}
}

// sendMessage sends a new message and returns the message ID.
func sendMessage(b *Bot, chatID int64, text string, keyboard *tgbotapi.InlineKeyboardMarkup) int {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "HTML"
	if keyboard != nil {
		msg.ReplyMarkup = keyboard
	}
	sent, err := b.api.Send(msg)
	if err != nil {
		log.Printf("Error sending message: %v", err)
		return 0
	}
	return sent.MessageID
}

// sendOrEdit sends a new message if messageID is 0, otherwise edits in place.
func sendOrEdit(b *Bot, chatID int64, msgID int, text string, keyboard *tgbotapi.InlineKeyboardMarkup) {
	if msgID == 0 {
		sendMessage(b, chatID, text, keyboard)
	} else {
		editMessage(b, chatID, msgID, text, keyboard)
	}
}

// fmtDate formats a time for callback data.
func fmtDate(t time.Time) string {
	return t.Format("2006-01-02")
}

// dayLabel returns a friendly label for a date.
func dayLabel(date time.Time) string {
	t := today()
	switch {
	case date.Equal(t):
		return "Today"
	case date.Equal(t.AddDate(0, 0, 1)):
		return "Tomorrow"
	case date.Equal(t.AddDate(0, 0, -1)):
		return "Yesterday"
	default:
		return fmt.Sprintf("%s %d %s", date.Format("Mon"), date.Day(), date.Format("Jan"))
	}
}
