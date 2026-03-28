package model

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hwang/meadow-bubbletea/internal/ui"
)

// EditorModel is a nano-like multi-line text editor component.
type EditorModel struct {
	lines      []string // each line of text
	cursorRow  int      // current cursor row
	cursorCol  int      // current cursor column
	scrollRow  int      // first visible row (for scrolling)
	viewHeight int      // number of visible rows
	viewWidth  int      // width of editor area
	clipboard  string   // cut/paste buffer (Ctrl+K/U)
	modified   bool     // whether content has been changed
	statsLine  string   // stats bar content (set externally)
}

// NewEditorModel creates an editor from a string (split into lines).
func NewEditorModel(content string, width, height int) EditorModel {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		lines = []string{""}
	}
	return EditorModel{
		lines:      lines,
		viewWidth:  width,
		viewHeight: height,
	}
}

// Content returns the full content as a string (join lines with newline).
func (e *EditorModel) Content() string {
	return strings.Join(e.lines, "\n")
}

// IsModified returns whether content has been changed.
func (e *EditorModel) IsModified() bool {
	return e.modified
}

// SetStatsLine sets the stats bar content.
func (e *EditorModel) SetStatsLine(s string) {
	e.statsLine = s
}

// Resize updates viewport dimensions.
func (e *EditorModel) Resize(width, height int) {
	e.viewWidth = width
	e.viewHeight = height
	e.ensureCursorVisible()
}

// HandleKey handles key events and returns whether the editor saved or closed.
func (e *EditorModel) HandleKey(msg tea.KeyMsg) (saved bool, closed bool) {
	key := msg.String()
	switch key {
	// Save / Close
	case "ctrl+o":
		return true, false
	case "ctrl+x":
		return false, true

	// Navigation
	case "up":
		if e.cursorRow > 0 {
			e.cursorRow--
			e.clampCursor()
			e.ensureCursorVisible()
		}
	case "down":
		if e.cursorRow < len(e.lines)-1 {
			e.cursorRow++
			e.clampCursor()
			e.ensureCursorVisible()
		}
	case "left":
		if e.cursorCol > 0 {
			e.cursorCol--
		} else if e.cursorRow > 0 {
			// Wrap to end of previous line.
			e.cursorRow--
			e.cursorCol = utf8.RuneCountInString(e.lines[e.cursorRow])
			e.ensureCursorVisible()
		}
	case "right":
		if e.cursorCol < utf8.RuneCountInString(e.lines[e.cursorRow]) {
			e.cursorCol++
		} else if e.cursorRow < len(e.lines)-1 {
			// Wrap to start of next line.
			e.cursorRow++
			e.cursorCol = 0
			e.ensureCursorVisible()
		}
	case "home", "ctrl+a":
		e.cursorCol = 0
	case "end", "ctrl+e":
		e.cursorCol = utf8.RuneCountInString(e.lines[e.cursorRow])

	// Editing
	case "enter":
		e.modified = true
		runes := []rune(e.lines[e.cursorRow])
		before := string(runes[:e.cursorCol])
		after := string(runes[e.cursorCol:])
		e.lines[e.cursorRow] = before
		// Insert new line after current row.
		newLines := make([]string, len(e.lines)+1)
		copy(newLines, e.lines[:e.cursorRow+1])
		newLines[e.cursorRow+1] = after
		copy(newLines[e.cursorRow+2:], e.lines[e.cursorRow+1:])
		e.lines = newLines
		e.cursorRow++
		e.cursorCol = 0
		e.ensureCursorVisible()

	case "backspace":
		if e.cursorCol > 0 {
			e.modified = true
			runes := []rune(e.lines[e.cursorRow])
			e.lines[e.cursorRow] = string(append(runes[:e.cursorCol-1], runes[e.cursorCol:]...))
			e.cursorCol--
		} else if e.cursorRow > 0 {
			// Merge with previous line.
			e.modified = true
			prevLen := utf8.RuneCountInString(e.lines[e.cursorRow-1])
			e.lines[e.cursorRow-1] += e.lines[e.cursorRow]
			e.lines = append(e.lines[:e.cursorRow], e.lines[e.cursorRow+1:]...)
			e.cursorRow--
			e.cursorCol = prevLen
			e.ensureCursorVisible()
		}

	case "delete", "ctrl+d":
		if e.cursorCol < utf8.RuneCountInString(e.lines[e.cursorRow]) {
			e.modified = true
			runes := []rune(e.lines[e.cursorRow])
			e.lines[e.cursorRow] = string(append(runes[:e.cursorCol], runes[e.cursorCol+1:]...))
		} else if e.cursorRow < len(e.lines)-1 {
			// Merge with next line.
			e.modified = true
			e.lines[e.cursorRow] += e.lines[e.cursorRow+1]
			e.lines = append(e.lines[:e.cursorRow+1], e.lines[e.cursorRow+2:]...)
		}

	// Cut / Paste
	case "ctrl+k":
		e.clipboard = e.lines[e.cursorRow]
		e.modified = true
		if len(e.lines) == 1 {
			// Keep at least one empty line.
			e.lines[0] = ""
			e.cursorCol = 0
		} else {
			e.lines = append(e.lines[:e.cursorRow], e.lines[e.cursorRow+1:]...)
			if e.cursorRow >= len(e.lines) {
				e.cursorRow = len(e.lines) - 1
			}
			e.clampCursor()
			e.ensureCursorVisible()
		}

	case "ctrl+u":
		if e.clipboard != "" {
			e.modified = true
			newLines := make([]string, len(e.lines)+1)
			copy(newLines, e.lines[:e.cursorRow])
			newLines[e.cursorRow] = e.clipboard
			copy(newLines[e.cursorRow+1:], e.lines[e.cursorRow:])
			e.lines = newLines
			e.cursorRow++
			e.clampCursor()
			e.ensureCursorVisible()
		}

	default:
		// Insert printable characters — handles single chars, IME multi-rune,
		// and bracketed paste (which may contain newlines).
		e.insertRunes(msg.Runes, msg.Paste)
	}

	return false, false
}

// insertRunes inserts one or more runes at the cursor. For paste with
// newlines, it splits the content into multiple lines.
func (e *EditorModel) insertRunes(runes []rune, paste bool) {
	// Filter to printable runes (keep newlines for paste).
	var filtered []rune
	for _, r := range runes {
		if unicode.IsPrint(r) || (paste && r == '\n') {
			filtered = append(filtered, r)
		}
	}
	if len(filtered) == 0 {
		return
	}
	e.modified = true

	if paste && containsRune(filtered, '\n') {
		// Split pasted text on newlines.
		segments := splitRunes(filtered, '\n')

		// Insert first segment at cursor on current line.
		line := []rune(e.lines[e.cursorRow])
		before := line[:e.cursorCol]
		after := line[e.cursorCol:]

		firstLine := make([]rune, 0, len(before)+len(segments[0]))
		firstLine = append(firstLine, before...)
		firstLine = append(firstLine, segments[0]...)

		if len(segments) == 1 {
			e.lines[e.cursorRow] = string(firstLine)
			e.cursorCol += len(segments[0])
		} else {
			// Last segment gets the remainder of the original line.
			lastSeg := segments[len(segments)-1]
			lastLine := make([]rune, 0, len(lastSeg)+len(after))
			lastLine = append(lastLine, lastSeg...)
			lastLine = append(lastLine, after...)

			// Build new lines slice.
			newCount := len(e.lines) + len(segments) - 1
			newLines := make([]string, 0, newCount)
			newLines = append(newLines, e.lines[:e.cursorRow]...)
			newLines = append(newLines, string(firstLine))
			for _, seg := range segments[1 : len(segments)-1] {
				newLines = append(newLines, string(seg))
			}
			newLines = append(newLines, string(lastLine))
			newLines = append(newLines, e.lines[e.cursorRow+1:]...)
			e.lines = newLines

			e.cursorRow += len(segments) - 1
			e.cursorCol = len(lastSeg)
		}
	} else {
		// Insert all runes at cursor position on current line.
		line := []rune(e.lines[e.cursorRow])
		newLine := make([]rune, 0, len(line)+len(filtered))
		newLine = append(newLine, line[:e.cursorCol]...)
		newLine = append(newLine, filtered...)
		newLine = append(newLine, line[e.cursorCol:]...)
		e.lines[e.cursorRow] = string(newLine)
		e.cursorCol += len(filtered)
	}
	e.ensureCursorVisible()
}

func containsRune(runes []rune, r rune) bool {
	for _, c := range runes {
		if c == r {
			return true
		}
	}
	return false
}

func splitRunes(runes []rune, sep rune) [][]rune {
	var result [][]rune
	var current []rune
	for _, r := range runes {
		if r == sep {
			result = append(result, current)
			current = nil
		} else {
			current = append(current, r)
		}
	}
	result = append(result, current)
	return result
}

// View renders the editor.
func (e *EditorModel) View() string {
	var b strings.Builder

	greyStyle := ui.TaskPendingStyle
	lineNumStyle := lipgloss.NewStyle().Foreground(ui.ColorDimmed)
	cursorStyle := lipgloss.NewStyle().Background(ui.ColorAccent).Foreground(ui.ColorBg)
	modifiedStyle := lipgloss.NewStyle().Foreground(ui.ColorAccent).Bold(true)

	// Stats line at the top (if set).
	if e.statsLine != "" {
		stats := e.statsLine
		if e.modified {
			stats += "  " + modifiedStyle.Render("[Modified]")
		}
		b.WriteString(stats)
		b.WriteString("\n")
	} else if e.modified {
		b.WriteString(modifiedStyle.Render("[Modified]"))
		b.WriteString("\n")
	}

	// Calculate how many rows we can display.
	// Reserve lines for: stats (if present), shortcut bar.
	displayHeight := e.viewHeight
	if e.statsLine != "" || e.modified {
		displayHeight--
	}
	displayHeight-- // shortcut bar

	if displayHeight < 1 {
		displayHeight = 1
	}

	// Render visible lines.
	end := e.scrollRow + displayHeight
	if end > len(e.lines) {
		end = len(e.lines)
	}

	for row := e.scrollRow; row < e.scrollRow+displayHeight; row++ {
		if row < len(e.lines) {
			lineNum := lineNumStyle.Render(fmt.Sprintf("%3d", row+1))
			line := e.lines[row]

			var renderedLine string
			if row == e.cursorRow {
				// Render line with block cursor at cursor position.
				renderedLine = e.renderLineWithCursor(line, e.cursorCol, greyStyle)
			} else if strings.HasPrefix(line, "#") {
				renderedLine = greyStyle.Render(line)
			} else {
				renderedLine = line
			}

			b.WriteString(lineNum + " " + renderedLine)
		} else {
			// Empty row beyond content — show tilde like vim.
			lineNum := lineNumStyle.Render("  ~")
			b.WriteString(lineNum)
		}

		if row < e.scrollRow+displayHeight-1 {
			b.WriteString("\n")
		}
	}

	// Shortcut bar at the bottom.
	shortcutBar := ui.ShortcutBarStyle.
		Render("Ctrl+O Save | Ctrl+X Close | Ctrl+K Cut | Ctrl+U Paste")
	b.WriteString("\n")
	b.WriteString(shortcutBar)

	_ = cursorStyle // used indirectly in renderLineWithCursor
	return b.String()
}

// renderLineWithCursor renders a single line with a block cursor at the given column.
// col is a rune index, not a byte index.
func (e *EditorModel) renderLineWithCursor(line string, col int, commentStyle lipgloss.Style) string {
	isComment := strings.HasPrefix(line, "#")
	cursorStyle := lipgloss.NewStyle().Background(ui.ColorAccent).Foreground(ui.ColorBg)

	runes := []rune(line)
	if col >= len(runes) {
		// Cursor is at the end of the line (or beyond); show block cursor after text.
		var text string
		if isComment {
			text = commentStyle.Render(line)
		} else {
			text = line
		}
		return text + cursorStyle.Render(" ")
	}

	// Split into before-cursor, cursor char, after-cursor.
	before := string(runes[:col])
	cursorChar := string(runes[col])
	after := string(runes[col+1:])

	if isComment {
		return commentStyle.Render(before) + cursorStyle.Render(cursorChar) + commentStyle.Render(after)
	}
	return before + cursorStyle.Render(cursorChar) + after
}

// ensureCursorVisible adjusts scrollRow so the cursor row is visible.
func (e *EditorModel) ensureCursorVisible() {
	// Calculate available display height (same logic as View).
	displayHeight := e.viewHeight
	if e.statsLine != "" || e.modified {
		displayHeight--
	}
	displayHeight-- // shortcut bar
	if displayHeight < 1 {
		displayHeight = 1
	}

	if e.cursorRow < e.scrollRow {
		e.scrollRow = e.cursorRow
	}
	if e.cursorRow >= e.scrollRow+displayHeight {
		e.scrollRow = e.cursorRow - displayHeight + 1
	}
	if e.scrollRow < 0 {
		e.scrollRow = 0
	}
}

// clampCursor ensures cursor is within valid bounds.
func (e *EditorModel) clampCursor() {
	if e.cursorRow < 0 {
		e.cursorRow = 0
	}
	if e.cursorRow >= len(e.lines) {
		e.cursorRow = len(e.lines) - 1
	}
	if e.cursorCol < 0 {
		e.cursorCol = 0
	}
	lineRuneCount := utf8.RuneCountInString(e.lines[e.cursorRow])
	if e.cursorCol > lineRuneCount {
		e.cursorCol = lineRuneCount
	}
}
