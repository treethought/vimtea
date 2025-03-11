// Package vimtea provides a Vim-like text editor component for terminal applications
package vimtea

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Regular expression for matching ANSI escape sequences
// Used to correctly calculate visible text length with syntax highlighting
var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// View renders the editor and returns it as a string
// This is part of the bubbletea.Model interface
func (m *editorModel) View() string {
	// Build components from top to bottom
	components := []string{
		m.renderContent(), // Main editor content
	}
	if m.enableStatusBar {
		components = append(components, m.renderStatusLine()) // Status bar and command line
	}

	// Join all components vertically
	return lipgloss.JoinVertical(
		lipgloss.Top,
		components...,
	)
}

func (m *editorModel) renderContent() string {
	var sb strings.Builder

	var selStart, selEnd Cursor
	if m.mode == ModeVisual {
		selStart, selEnd = m.GetSelectionBoundary()
	}

	visibleContent := m.getVisibleContent()

	for i, line := range visibleContent {
		lineNum := i + m.viewport.YOffset + 1
		rowIdx := lineNum - 1

		sb.WriteString(m.renderLineNumber(lineNum, rowIdx))

		if rowIdx >= m.buffer.lineCount() {
			sb.WriteString("\n")
			continue
		}

		inVisualSelection := m.mode == ModeVisual && rowIdx >= selStart.Row && rowIdx <= selEnd.Row
		sb.WriteString(m.renderLine(line, rowIdx, inVisualSelection, selStart, selEnd))
		sb.WriteString("\n")
	}

	return sb.String()
}

func (m *editorModel) renderLine(line string, rowIdx int, inVisualSelection bool, selStart, selEnd Cursor) string {
	if m.mode == ModeVisual && m.isVisualLine && inVisualSelection {
		return m.selectedStyle.Render(line)
	}

	if m.mode != ModeVisual && m.yankHighlight.Active && m.isLineInYankHighlight(rowIdx) {
		return m.renderLineWithYankHighlight(line, rowIdx)
	}

	var displayedLine string
	if m.highlighter != nil && m.highlighter.enabled {
		displayedLine = m.highlighter.HighlightLine(line)
	} else {
		displayedLine = line
	}

	if rowIdx == m.cursor.Row {
		if len(line) == 0 {
			if m.cursor.Col == 0 {
				return m.renderCursor(" ")
			}
			return ""
		}

		if m.cursor.Col >= len(line) {
			return displayedLine + m.renderCursor(" ")
		}

		if m.mode == ModeVisual && !m.isVisualLine && inVisualSelection {
			return m.renderLineWithCursorInVisualSelection(line, rowIdx, selStart, selEnd)
		}

		if m.highlighter != nil && m.highlighter.enabled && line != displayedLine {
			return m.renderSyntaxHighlightedCursorLine(displayedLine, line)
		}

		return m.renderRegularCursorLine(line)
	}

	if m.mode == ModeVisual && !m.isVisualLine && inVisualSelection {
		return m.renderLineInVisualSelection(line, rowIdx, selStart, selEnd)
	}

	return displayedLine
}

func (m *editorModel) renderCursor(char string) string {
	if !m.cursorBlink {
		return char
	}

	switch m.mode {
	case ModeInsert:
		return lipgloss.NewStyle().Underline(true).Render(char)
	case ModeCommand:
		return char
	default:
		return m.cursorStyle.Render(char)
	}
}

func (m *editorModel) renderLineNumber(lineNum int, rowIdx int) string {
	if rowIdx >= m.buffer.lineCount() {
		return m.lineNumberStyle.Render("    ")
	}

	if rowIdx == m.cursor.Row {
		return m.currentLineNumberStyle.Render(fmt.Sprintf("%4d", lineNum))
	}

	if m.relativeNumbers {
		distance := abs(rowIdx - m.cursor.Row)
		return m.lineNumberStyle.Render(fmt.Sprintf("%4d", distance))
	}

	return m.lineNumberStyle.Render(fmt.Sprintf("%4d", lineNum))
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func (m *editorModel) renderRegularCursorLine(line string) string {
	var sb strings.Builder

	sb.WriteString(line[:m.cursor.Col])

	cursorChar := string(line[m.cursor.Col])
	sb.WriteString(m.renderCursor(cursorChar))

	if m.cursor.Col < len(line)-1 {
		sb.WriteString(line[m.cursor.Col+1:])
	}

	return sb.String()
}

func (m *editorModel) renderSyntaxHighlightedCursorLine(highlightedLine, plainLine string) string {
	plainRunes := []rune(plainLine)
	cursorIdx := m.cursor.Col

	if cursorIdx >= len(plainRunes) {
		return highlightedLine + m.renderCursor(" ")
	}

	ansiMatches := ansiRegex.FindAllStringIndex(highlightedLine, -1)

	plainToHighlighted := make(map[int]int)
	plainIdx := 0

	for i := 0; i < len(highlightedLine); {
		isAnsi := false
		for _, match := range ansiMatches {
			if match[0] == i {
				i = match[1]
				isAnsi = true
				break
			}
		}

		if isAnsi {
			continue
		}

		plainToHighlighted[plainIdx] = i
		plainIdx++
		i++
	}

	highlightedCursorPos, exists := plainToHighlighted[cursorIdx]
	if !exists {
		return m.renderRegularCursorLine(plainLine)
	}

	var ansiBeforeCursor string
	for _, match := range ansiMatches {
		if match[0] < highlightedCursorPos {
			ansiBeforeCursor += highlightedLine[match[0]:match[1]]
		}
	}

	var sb strings.Builder
	sb.WriteString(highlightedLine[:highlightedCursorPos])
	sb.WriteString("\x1b[0m")

	cursorChar := string(plainRunes[cursorIdx])
	sb.WriteString(m.renderCursor(cursorChar))

	sb.WriteString(ansiBeforeCursor)

	if highlightedCursorPos+1 < len(highlightedLine) {
		afterCursorStart := highlightedCursorPos + 1

		for _, match := range ansiMatches {
			if afterCursorStart >= match[0] && afterCursorStart < match[1] {
				afterCursorStart = match[1]
				break
			}
		}

		sb.WriteString(highlightedLine[afterCursorStart:])
	}

	return sb.String()
}

func (m *editorModel) renderLineWithCursorInVisualSelection(line string, rowIdx int, selStart, selEnd Cursor) string {
	var sb strings.Builder

	selBegin := 0
	if rowIdx == selStart.Row {
		selBegin = selStart.Col
	}

	selEndCol := len(line)
	if rowIdx == selEnd.Row {
		selEndCol = selEnd.Col + 1
	}

	if rowIdx == selStart.Row && selStart.Col > 0 {
		sb.WriteString(line[:selStart.Col])
	}

	if m.cursor.Col >= selBegin && m.cursor.Col < selEndCol {

		if m.cursor.Col > selBegin {
			sb.WriteString(m.selectedStyle.Render(line[selBegin:m.cursor.Col]))
		}

		cursorChar := string(line[m.cursor.Col])
		if m.cursorBlink {
			sb.WriteString(m.cursorStyle.Render(cursorChar))
		} else {
			sb.WriteString(m.selectedStyle.Render(cursorChar))
		}

		if m.cursor.Col+1 < selEndCol {
			sb.WriteString(m.selectedStyle.Render(line[m.cursor.Col+1 : selEndCol]))
		}
	} else {
		sb.WriteString(m.selectedStyle.Render(line[selBegin:selEndCol]))
	}

	if selEndCol < len(line) {
		sb.WriteString(line[selEndCol:])
	}

	return sb.String()
}

func (m *editorModel) renderLineInVisualSelection(line string, rowIdx int, selStart, selEnd Cursor) string {
	var sb strings.Builder

	selBegin := 0
	if rowIdx == selStart.Row {
		selBegin = selStart.Col
	}

	selEndCol := len(line)
	if rowIdx == selEnd.Row {
		selEndCol = selEnd.Col + 1
	}

	if selBegin > 0 {
		sb.WriteString(line[:selBegin])
	}

	sb.WriteString(m.selectedStyle.Render(line[selBegin:min(selEndCol, len(line))]))

	if selEndCol < len(line) {
		sb.WriteString(line[selEndCol:])
	}

	return sb.String()
}

func (m editorModel) getVisibleContent() []string {
	startLine := m.viewport.YOffset
	endLine := startLine + m.height

	if startLine < 0 {
		startLine = 0
	}

	contentLines := []string{}

	for i := startLine; i < min(endLine, m.buffer.lineCount()); i++ {
		contentLines = append(contentLines, m.buffer.Line(i))
	}

	emptyLinesNeeded := m.height - len(contentLines)
	for range emptyLinesNeeded {
		contentLines = append(contentLines, "")
	}

	return contentLines
}

func (m *editorModel) renderStatusLine() string {
	status := m.getStatusText()
	cursorPos := fmt.Sprintf(" %d:%d ", m.cursor.Row+1, m.cursor.Col+1)

	padding := max(m.width-lipgloss.Width(status)-lipgloss.Width(cursorPos), 0)

	return m.statusStyle.Render(status + strings.Repeat(" ", padding) + cursorPos)
}

func (m *editorModel) getStatusText() string {
	if m.mode == ModeCommand {
		return ":" + m.commandBuffer
	}

	status := fmt.Sprintf(" %s", m.mode)
	if len(m.keySequence) > 0 {
		status += fmt.Sprintf(" | %s", strings.Join(m.keySequence, ""))
	}

	if m.statusMessage != "" {
		status += fmt.Sprintf(" | %s", m.statusMessage)
	}

	return status
}

func (m *editorModel) isLineInYankHighlight(rowIdx int) bool {
	return m.yankHighlight.Active &&
		rowIdx >= m.yankHighlight.Start.Row && rowIdx <= m.yankHighlight.End.Row
}

func (m *editorModel) getYankHighlightBounds(rowIdx int) (int, int) {
	if !m.yankHighlight.Active || !m.isLineInYankHighlight(rowIdx) {
		return -1, -1
	}

	start := 0
	end := m.buffer.lineLength(rowIdx)

	if !m.yankHighlight.IsLinewise {
		if rowIdx == m.yankHighlight.Start.Row {
			start = m.yankHighlight.Start.Col
		}

		if rowIdx == m.yankHighlight.End.Row {
			end = m.yankHighlight.End.Col + 1
		}
	}

	return start, end
}

func (m *editorModel) renderLineWithYankHighlight(line string, rowIdx int) string {
	var sb strings.Builder

	start, end := m.getYankHighlightBounds(rowIdx)
	if start < 0 || end < 0 {
		return line
	}

	start = max(0, min(start, len(line)))
	end = max(0, min(end, len(line)))

	if start > 0 {
		sb.WriteString(line[:start])
	}

	if rowIdx == m.cursor.Row && m.cursor.Col >= start && m.cursor.Col < end {

		if m.cursor.Col > start {
			sb.WriteString(lipgloss.NewStyle().Background(lipgloss.Color("7")).Render(line[start:m.cursor.Col]))
		}

		cursorChar := string(line[m.cursor.Col])
		if m.cursorBlink {
			sb.WriteString(m.cursorStyle.Render(cursorChar))
		} else {
			sb.WriteString(lipgloss.NewStyle().Background(lipgloss.Color("7")).Render(cursorChar))
		}

		if m.cursor.Col+1 < end {
			sb.WriteString(lipgloss.NewStyle().Background(lipgloss.Color("7")).Render(line[m.cursor.Col+1 : end]))
		}
	} else {
		sb.WriteString(lipgloss.NewStyle().Background(lipgloss.Color("7")).Render(line[start:end]))
	}

	if end < len(line) {
		sb.WriteString(line[end:])
	}

	return sb.String()
}
