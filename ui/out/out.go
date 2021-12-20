// Package out implements producing output to terminal user interface.
package out

import (
	"fmt"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/croz-ltd/dpcmder/events"
	"github.com/croz-ltd/dpcmder/model"
	"github.com/croz-ltd/dpcmder/utils/logging"
	"github.com/gdamore/tcell/v2"
)

// Colors used for coloring text and background.
const (
	fgNormal   = tcell.ColorDefault
	bgNormal   = tcell.ColorDefault
	fgSelected = tcell.ColorRed
	bgCurrent  = tcell.ColorGreen
)

// Styles used for console cell (text with background) coloring.
var (
	stNormal          = tcell.StyleDefault.Foreground(fgNormal).Background(bgNormal)
	stSelected        = tcell.StyleDefault.Foreground(fgSelected).Background(bgNormal)
	stCurrent         = tcell.StyleDefault.Foreground(fgNormal).Background(bgCurrent)
	stCurrentSelected = tcell.StyleDefault.Foreground(fgSelected).Background(bgCurrent)
	stCursor          = stNormal.Reverse(true)
)

// Screen is used to show text on console and poll input events (key press,
// console resize, could add mouse).
var Screen tcell.Screen

// screenActive - track state of Screen, when external programs are active
// don't draw on the screen (async sync events can cause draw events)
var screenActive = false

// Init initializes console screen.
func Init() {
	logging.LogDebug("ui/out/Init()")

	var err error
	Screen, err = tcell.NewScreen()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	err = Screen.Init()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	screenActive = true
}

// Stop terminates console screen.
func Stop() {
	logging.LogDebug("ui/out/Stop()")
	screenActive = false
	Screen.Fini()
	logging.LogDebug("ui/out/Stop() end")
}

// GetScreenSize returns size of console screen.
func GetScreenSize() (width, height int) {
	logging.LogDebugf("ui/out/GetScreenSize(), screenActive: %v", screenActive)
	if !screenActive {
		return
	}

	width, height = Screen.Size()
	logging.LogTracef("ui/out/GetScreenSize(), width: %d, height: %d", width, height)
	return width, height
}

// DrawEvent crates appropriate changes to screen for given event. Usually either
// refresh whole screen or just update status message.
func DrawEvent(updateViewEvent events.UpdateViewEvent) {
	logging.LogDebugf("ui/out/DrawEvent(%v), screenActive: %v", updateViewEvent, screenActive)

	if !screenActive {
		return
	}

	switch updateViewEvent.Type {
	case events.UpdateViewShowStatus:
		showStatus(updateViewEvent.Model, updateViewEvent.Status)
	case events.UpdateViewRefresh:
		refreshScreen(*updateViewEvent.Model)
	case events.UpdateViewShowQuestionDialog:
		showQuestionDialog(updateViewEvent.DialogQuestion, updateViewEvent.DialogAnswer, updateViewEvent.DialogAnswerCursorIdx)
	case events.UpdateViewShowListSelectionDialog:
		showListSelectionDialog(updateViewEvent.ListSelectionMessage, updateViewEvent.ListSelectionList, updateViewEvent.ListSelectionSelectedIdx)
	case events.UpdateViewShowProgress:
		showProgressDialog(updateViewEvent.Message, updateViewEvent.Progress)
	default:
		logging.LogDebugf("ui/out/DrawEvent() unknown event received: %v", updateViewEvent)
	}

	logging.LogDebug("ui/out/drawEvent() finished")
}

// refreshScreen refreshes the whole terminal screen.
func refreshScreen(m model.Model) {
	logging.LogDebugf("ui/out/refreshScreen('%v')", m)

	m.ResizeView()

	Screen.Clear()
	Screen.SetStyle(tcell.StyleDefault.Foreground(fgNormal).Background(bgNormal))

	width, _ := Screen.Size()
	if m.IsCurrentSide(model.Left) {
		writeLine(0, 0, m.Title(model.Left), m.HorizScroll, stCurrent)
		writeLine(width/2, 0, m.Title(model.Right), m.HorizScroll, stNormal)
	} else {
		writeLine(0, 0, m.Title(model.Left), m.HorizScroll, stNormal)
		writeLine(width/2, 0, m.Title(model.Right), m.HorizScroll, stCurrent)
	}

	for idx := 0; idx < m.GetVisibleItemCount(model.Left); idx++ {
		logging.LogTrace("ui/out/draw(), idx: ", idx)
		item := m.GetVisibleItem(model.Left, idx)
		var st = stNormal
		switch {
		case m.IsCurrentRow(model.Left, idx) && item.Selected:
			st = stCurrentSelected
		case m.IsCurrentRow(model.Left, idx):
			st = stCurrent
		case item.Selected:
			st = stSelected
		}
		writeLine(0, idx+2, item.DisplayString(), m.HorizScroll, st)
	}
	for idx := 0; idx < m.GetVisibleItemCount(model.Right); idx++ {
		logging.LogTrace("ui/out/draw(), idx: ", idx)
		item := m.GetVisibleItem(model.Right, idx)
		var st = stNormal
		switch {
		case m.IsCurrentRow(model.Right, idx) && item.Selected:
			st = stCurrentSelected
		case m.IsCurrentRow(model.Right, idx):
			st = stCurrent
		case item.Selected:
			st = stSelected
		}
		writeLine(width/2, idx+2, item.DisplayString(), m.HorizScroll, st)
	}

	showStatus(&m, m.LastStatus())
}

// showQuestionDialog shows question dialog on the terminal screen.
func showQuestionDialog(question, answer string, answerCursorIdx int) {
	logging.LogDebugf("ui/out/showQuestionDialog('%s', '%s', %d)", question, answer, answerCursorIdx)

	// termbox.Clear(fgNormal, bgNormal)
	width, height := Screen.Size()
	x := 10
	y := height/2 - 2
	dialogWidth := width - 20
	line := question + answer
	cursorIdx := utf8.RuneCountInString(question) + answerCursorIdx

	writeLine(x, y-2, buildLine("", "*", "", dialogWidth), 0, stNormal)
	writeLine(x, y-1, buildLine("*", " ", "*", dialogWidth), 0, stNormal)
	writeLine(x, y, buildLine("*", " ", "*", dialogWidth), 0, stNormal)
	writeLine(x, y+1, buildLine("*", " ", "*", dialogWidth), 0, stNormal)
	writeLine(x, y+2, buildLine("", "*", "", dialogWidth), 0, stNormal)
	writeLineWithCursor(x+2, y, line, 0, stNormal, x+2+cursorIdx, stCursor)

	Screen.Show()
}

// showListSelectionDialog shows list selection dialog on the terminal screen.
func showListSelectionDialog(message string, list []string, selectionIdx int) {
	logging.LogDebugf("ui/out/showListSelectionDialog('%s', '%v', %d)", message, list, selectionIdx)

	// termbox.Clear(fgNormal, bgNormal)
	width, height := Screen.Size()
	firstLine, lastLine := 2, height-3
	x := 10
	dialogWidth := width - 20

	// write top frame line
	writeLine(x, firstLine, buildLine("", "*", "", dialogWidth), 0, stNormal)

	// write message before list
	writeLine(x, firstLine+1, buildLine("*", " ", "*", dialogWidth), 0, stNormal)
	writeLine(x+2, firstLine+1, message, 0, stNormal)

	textLinesMaxNo := lastLine - firstLine - 2
	textLinesNo := len(list)
	textLineStart := firstLine + 2
	textLineEnd := lastLine - 1
	listScrollVert := 0
	if textLinesNo > textLinesMaxNo && selectionIdx >= textLinesMaxNo {
		logging.LogDebugf("ui/out/showListSelectionDialog() in if...")
		listScrollVert = selectionIdx - textLinesMaxNo + 1
	}
	logging.LogDebugf("ui/out/showListSelectionDialog() %d/%d/%d/%d", textLinesNo, textLinesMaxNo, selectionIdx, listScrollVert)

	// write select dialog frame and list items
	for lineNo := textLineStart; lineNo <= textLineEnd; lineNo++ {
		writeLine(x, lineNo, buildLine("*", " ", "*", dialogWidth), 0, stNormal)
		listIdx := lineNo - textLineStart + listScrollVert
		if listIdx < len(list) {
			text := list[listIdx]
			maxTextWidth := dialogWidth - 4
			if len(text) > maxTextWidth {
				text = text[len(text)-maxTextWidth:]
			}
			if listIdx == selectionIdx {
				writeLine(x+2, lineNo, text, 0, stCurrent)
			} else {
				writeLine(x+2, lineNo, text, 0, stNormal)
			}
		}
	}

	// write bottom frame line
	writeLine(x, lastLine, buildLine("", "*", "", dialogWidth), 0, stNormal)

	Screen.Show()
}

// writeLine writes given line on console screen at given position using given style.
func writeLine(x, y int, line string, horizScroll int, stNormal tcell.Style) int {
	return writeLineWithCursor(x, y, line, horizScroll, stNormal, -1, stNormal)
}

// writeLineWithCursor writes given line with cursor shown at given position.
func writeLineWithCursor(x, y int, line string, horizScroll int, stNormal tcell.Style, cursorX int, stCursor tcell.Style) int {
	scrolledLine := scrollLineHoriz(line, horizScroll)

	var xpos int
	runeIdx := 0
	for _, runeVal := range scrolledLine {
		xpos = x + runeIdx
		runeIdx = runeIdx + 1
		if cursorX == xpos {
			Screen.SetCell(xpos, y, stCursor, runeVal)
		} else {
			Screen.SetCell(xpos, y, stNormal, runeVal)
		}
	}
	// logging.LogDebug("x: ", x, ", scrolledLine: ", scrolledLine, ", len(scrolledLine): ", len(scrolledLine))
	// logging.LogDebug("cursorX: ", cursorX, "xpos: ", xpos)
	if cursorX > xpos {
		Screen.SetCell(cursorX, y, stCursor, rune(' '))
	} else if cursorX == -2 {
		// logging.LogDebug("------ ", "xpos+1: ", (xpos + 1))
		Screen.SetCell(xpos+1, y, stCursor, rune(' '))
	}
	return xpos + 1
}

// buildLine creates line with given length and given start of line string,
// middle string and end of line string. For example:
// buildLine("<", "-", ">", 10) -> "<-------->".
func buildLine(first, middle, last string, length int) string {
	middleLen := (length - utf8.RuneCountInString(first) - utf8.RuneCountInString(last)) / utf8.RuneCountInString(middle)
	return first + strings.Repeat(middle, middleLen) + last
}

// scrollLineHoriz returns horizontaly scrolled line (prefix of line is cut if
// we need to show scrolled line).
func scrollLineHoriz(line string, horizScroll int) string {
	var scrollh = horizScroll
	runeCount := utf8.RuneCountInString(line)
	if runeCount < scrollh {
		scrollh = runeCount
	}

	runeIdx := 0
	if scrollh != 0 {
		scrolledLine := ""
		for byteIdx := range line {
			if runeIdx == scrollh {
				scrolledLine = line[byteIdx:]
				break
			}
			runeIdx++
		}
		return scrolledLine
	}

	return line
}

var syncStatusBlink bool

// showStatus shows status string at bottom of dpcmder console screen.
func showStatus(m *model.Model, status string) {
	syncStatusBlink = !syncStatusBlink
	var filterMsg string
	var syncMsg string
	if m.CurrentFilter() != "" {
		filterMsg = fmt.Sprintf("Filter: '%s' | ", m.CurrentFilter())
	}
	if m.SyncModeOn {
		var syncStatusSymbol string
		if syncStatusBlink {
			syncStatusSymbol = "*"
		} else {
			syncStatusSymbol = " "
		}
		syncMsg = fmt.Sprintf("%s Sync (%s/'%s' <- '%s') | ", syncStatusSymbol, m.SyncDpDomain, m.SyncDirDp, m.SyncDirLocal)
	}

	statusMsg := fmt.Sprintf("%s%s%s", syncMsg, filterMsg, status)

	w, h := Screen.Size()
	writeLine(0, h-1, strings.Repeat(" ", w), m.HorizScroll, stNormal)
	writeLine(0, h-1, statusMsg, m.HorizScroll, stNormal)
	Screen.Show()
}

// showProgressDialog shows progress dialog with current progress (0-99).
func showProgressDialog(msg string, progress int) {
	logging.LogDebugf("ui/out/showProgressDialog('%s', %d)", msg, progress)

	width, height := Screen.Size()
	x := 10
	y := height/2 - 2
	dialogWidth := width - 20

	var progressX int
	var progressY int
	if progress < 50 {
		progressX = x + 2 + (dialogWidth-4)*progress/50
		progressY = y - 1
	} else {
		progressX = x + 2 + (dialogWidth-4)*(99-progress)/50
		progressY = y + 1
	}
	writeLine(x, y-2, buildLine("", "*", "", dialogWidth), 0, stNormal)
	writeLine(x, y-1, buildLine("*", " ", "*", dialogWidth), 0, stNormal)
	writeLine(x, y, buildLine("*", " ", "*", dialogWidth), 0, stNormal)
	writeLine(x, y+1, buildLine("*", " ", "*", dialogWidth), 0, stNormal)
	writeLine(x, y+2, buildLine("", "*", "", dialogWidth), 0, stNormal)
	writeLine(progressX, progressY, "****", 0, stNormal)
	writeLine(x+4, y, msg, 0, stNormal)

	Screen.Show()
}
