package telegram

import (
	"fmt"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/mhwang-1/meadow-bubbletea/internal/domain"
	"github.com/mhwang-1/meadow-bubbletea/internal/service"
)

// mainViewKeyboard builds the inline keyboard for the main day view.
func mainViewKeyboard(date time.Time) tgbotapi.InlineKeyboardMarkup {
	dateStr := fmtDate(date)
	prev := fmtDate(date.AddDate(0, 0, -1))
	next := fmtDate(date.AddDate(0, 0, 1))

	rows := [][]tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Done", "done:"+dateStr),
			tgbotapi.NewInlineKeyboardButtonData("Archive", "arch:"+dateStr),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("<< "+dayLabel(date.AddDate(0, 0, -1)), "nav:"+prev),
			tgbotapi.NewInlineKeyboardButtonData(dayLabel(date.AddDate(0, 0, 1))+" >>", "nav:"+next),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Lists", "lists"),
			tgbotapi.NewInlineKeyboardButtonData("New Timebox", "tb:new"),
			tgbotapi.NewInlineKeyboardButtonData("Notes", "notes"),
		),
	}
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

// timeboxSelectKeyboard lists timeboxes as buttons for selection.
func timeboxSelectKeyboard(timeboxes []service.DayViewTimebox, action string, dateStr string) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton
	for _, dvtb := range timeboxes {
		tb := dvtb.Timebox
		label := fmt.Sprintf("%s-%s", tb.Start.Format("15:04"), tb.End.Format("15:04"))
		if dvtb.TaskListName != "" {
			label += " " + dvtb.TaskListName
		}
		if len([]rune(label)) > 40 {
			label = string([]rune(label)[:39]) + "..."
		}
		data := fmt.Sprintf("%s:%s:%d", action, dateStr, dvtb.Index)
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(label, data),
		))
	}
	rows = append(rows, cancelRow(dateStr))
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

// taskSelectKeyboard lists tasks as buttons for selection.
func taskSelectKeyboard(tasks []domain.ScheduledTask, dateStr string, tbIdx int) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton
	pendingIdx := 0
	for _, st := range tasks {
		if st.IsBreak {
			continue
		}
		label := fmt.Sprintf("%s ~%s", st.Task.Description, domain.FormatDuration(st.Task.Duration))
		if len([]rune(label)) > 40 {
			label = string([]rune(label)[:39]) + "..."
		}
		data := fmt.Sprintf("done:%s:%d:%d", dateStr, tbIdx, pendingIdx)
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(label, data),
		))
		pendingIdx++
	}
	rows = append(rows, cancelRow(dateStr))
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

// taskListSelectKeyboard lists task lists as buttons.
func taskListSelectKeyboard(lists []*domain.TaskList, action string) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton
	for _, tl := range lists {
		label := fmt.Sprintf("%s (%d tasks)", tl.Name, len(tl.ActiveTasks()))
		if len([]rune(label)) > 40 {
			label = string([]rune(label)[:39]) + "..."
		}
		data := fmt.Sprintf("%s:%s", action, tl.Slug)
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(label, data),
		))
	}
	rows = append(rows, cancelRow(""))
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

// daySelectKeyboard shows day selection buttons.
func daySelectKeyboard(baseAction string) tgbotapi.InlineKeyboardMarkup {
	t := today()
	rows := [][]tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Today", baseAction+":"+fmtDate(t)),
			tgbotapi.NewInlineKeyboardButtonData("Tomorrow", baseAction+":"+fmtDate(t.AddDate(0, 0, 1))),
		),
	}

	// Next 5 days after tomorrow.
	var dayRow []tgbotapi.InlineKeyboardButton
	for i := 2; i <= 6; i++ {
		d := t.AddDate(0, 0, i)
		label := d.Format("Mon")
		dayRow = append(dayRow, tgbotapi.NewInlineKeyboardButtonData(label, baseAction+":"+fmtDate(d)))
		if len(dayRow) == 3 {
			rows = append(rows, dayRow)
			dayRow = nil
		}
	}
	if len(dayRow) > 0 {
		rows = append(rows, dayRow)
	}

	rows = append(rows, cancelRow(""))
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

// timeSelectKeyboard shows common time buttons.
func timeSelectKeyboard(dateStr string, baseAction string, startHour int) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton

	// Generate time slots from startHour to startHour+10h in 30min increments.
	var row []tgbotapi.InlineKeyboardButton
	for h := startHour; h < startHour+10 && h < 24; h++ {
		for _, m := range []int{0, 30} {
			if h == startHour+10-1 && m == 30 {
				break
			}
			timeStr := fmt.Sprintf("%02d%02d", h, m)
			label := fmt.Sprintf("%02d:%02d", h, m)
			data := fmt.Sprintf("%s:%s:%s", baseAction, dateStr, timeStr)
			row = append(row, tgbotapi.NewInlineKeyboardButtonData(label, data))
			if len(row) == 4 {
				rows = append(rows, row)
				row = nil
			}
		}
	}
	if len(row) > 0 {
		rows = append(rows, row)
	}

	// Custom time button.
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("Custom...", fmt.Sprintf("%s:%s:custom", baseAction, dateStr)),
	))

	rows = append(rows, cancelRow(""))
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

// endTimeSelectKeyboard shows end time buttons relative to start time.
func endTimeSelectKeyboard(dateStr, startTime, baseAction string) tgbotapi.InlineKeyboardMarkup {
	sh, sm := parseHHMM(startTime)
	startMins := sh*60 + sm

	offsets := []int{30, 60, 90, 120, 150, 180}
	var row []tgbotapi.InlineKeyboardButton
	var rows [][]tgbotapi.InlineKeyboardButton

	for _, off := range offsets {
		endMins := startMins + off
		if endMins >= 24*60 {
			break
		}
		eh := endMins / 60
		em := endMins % 60
		timeStr := fmt.Sprintf("%02d%02d", eh, em)
		label := fmt.Sprintf("%02d:%02d", eh, em)
		data := fmt.Sprintf("%s:%s:%s:%s", baseAction, dateStr, startTime, timeStr)
		row = append(row, tgbotapi.NewInlineKeyboardButtonData(label, data))
		if len(row) == 3 {
			rows = append(rows, row)
			row = nil
		}
	}
	if len(row) > 0 {
		rows = append(rows, row)
	}

	// Custom end time.
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("Custom...", fmt.Sprintf("%s:%s:%s:custom", baseAction, dateStr, startTime)),
	))

	rows = append(rows, cancelRow(""))
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

// confirmKeyboard shows Yes/No confirmation buttons.
func confirmKeyboard(yesData, noData string) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Yes", yesData),
			tgbotapi.NewInlineKeyboardButtonData("No", noData),
		),
	)
}

// cancelRow returns a cancel button row that returns to the main view.
func cancelRow(dateStr string) []tgbotapi.InlineKeyboardButton {
	data := "cancel"
	if dateStr != "" {
		data = "cancel:" + dateStr
	}
	return tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("Cancel", data),
	)
}

// listViewKeyboard builds the keyboard for viewing a single task list.
func listViewKeyboard(slug, dateStr string) tgbotapi.InlineKeyboardMarkup {
	rows := [][]tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Assign", "list:"+slug+":assign"),
			tgbotapi.NewInlineKeyboardButtonData("Edit", "list:"+slug+":edit"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Archive List", "list:"+slug+":arch"),
			tgbotapi.NewInlineKeyboardButtonData("Delete List", "list:"+slug+":del"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Back to Lists", "lists"),
		),
	}
	if dateStr != "" {
		rows = append(rows, cancelRow(dateStr))
	} else {
		rows = append(rows, cancelRow(""))
	}
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

// listEditKeyboard builds the keyboard for editing a task list.
func listEditKeyboard(slug string) tgbotapi.InlineKeyboardMarkup {
	rows := [][]tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Add Task", "list:"+slug+":add"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Back to List", "list:"+slug),
		),
	}
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

// listsTabKeyboard builds tab buttons for the lists view.
func listsTabKeyboard(activeTab string) []tgbotapi.InlineKeyboardButton {
	tabs := []struct {
		label string
		data  string
	}{
		{"Active", "lists:active"},
		{"History", "lists:history"},
		{"Archived", "lists:archived"},
	}

	var buttons []tgbotapi.InlineKeyboardButton
	for _, tab := range tabs {
		label := tab.label
		if tab.data == "lists:"+activeTab {
			label = "[" + label + "]"
		}
		buttons = append(buttons, tgbotapi.NewInlineKeyboardButtonData(label, tab.data))
	}
	return buttons
}

// parseHHMM parses an HHMM string into hours and minutes.
func parseHHMM(s string) (int, int) {
	if len(s) != 4 {
		return 0, 0
	}
	h := int(s[0]-'0')*10 + int(s[1]-'0')
	m := int(s[2]-'0')*10 + int(s[3]-'0')
	return h, m
}
