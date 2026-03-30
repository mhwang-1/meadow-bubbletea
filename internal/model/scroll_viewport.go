package model

import (
	"fmt"
	"strings"
)

// scrollViewport is a simple line-based viewport with vertical scrolling.
type scrollViewport struct {
	Width   int
	Height  int
	YOffset int
	lines   []string
}

// SetContent replaces the viewport content.
func (v *scrollViewport) SetContent(content string) {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	v.lines = strings.Split(normalized, "\n")
	v.clampYOffset()
}

// SetYOffset sets the vertical scroll offset.
func (v *scrollViewport) SetYOffset(offset int) {
	v.YOffset = offset
	v.clampYOffset()
}

// PageUp scrolls up by one viewport height.
func (v *scrollViewport) PageUp() {
	step := v.Height
	if step < 1 {
		step = 1
	}
	v.SetYOffset(v.YOffset - step)
}

// PageDown scrolls down by one viewport height.
func (v *scrollViewport) PageDown() {
	step := v.Height
	if step < 1 {
		step = 1
	}
	v.SetYOffset(v.YOffset + step)
}

// View returns the visible content slice.
func (v *scrollViewport) View() string {
	if v.Height <= 0 {
		return ""
	}
	v.clampYOffset()
	if len(v.lines) == 0 {
		return ""
	}
	if v.YOffset >= len(v.lines) {
		return ""
	}

	end := v.YOffset + v.Height
	if end > len(v.lines) {
		end = len(v.lines)
	}

	visible := v.lines[v.YOffset:end]
	return strings.Join(visible, "\n")
}

// Indicator returns a short scroll status label like "Lines 4-22/108".
func (v *scrollViewport) Indicator() string {
	_, _, start, end, total := v.window()
	if total == 0 {
		return "Lines 0/0"
	}
	return fmt.Sprintf("Lines %d-%d/%d", start, end, total)
}

func (v *scrollViewport) window() (startIdx, endIdx, startLine, endLine, total int) {
	total = len(v.lines)
	if v.Height <= 0 || total == 0 {
		return 0, 0, 0, 0, total
	}

	v.clampYOffset()

	startIdx = v.YOffset
	endIdx = v.YOffset + v.Height
	if endIdx > total {
		endIdx = total
	}

	startLine = startIdx + 1
	endLine = endIdx
	return startIdx, endIdx, startLine, endLine, total
}

func (v *scrollViewport) clampYOffset() {
	if v.YOffset < 0 {
		v.YOffset = 0
	}
	if len(v.lines) == 0 {
		return // content not set yet; trust caller's own clamp
	}

	maxOffset := len(v.lines) - v.Height
	if maxOffset < 0 {
		maxOffset = 0
	}

	if v.YOffset > maxOffset {
		v.YOffset = maxOffset
	}
}
