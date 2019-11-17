package out

import (
	"fmt"
	"github.com/croz-ltd/dpcmder/events"
	"github.com/croz-ltd/dpcmder/model"
	"github.com/croz-ltd/dpcmder/utils/logging"
	"github.com/croz-ltd/dpcmder/utils/screen"
	"github.com/nsf/termbox-go"
	"strings"
	"unicode/utf8"
)

const (
	fgNormal   = termbox.ColorDefault
	fgSelected = termbox.ColorRed
	bgNormal   = termbox.ColorDefault
	bgCurrent  = termbox.ColorGreen
)

func Init(updateViewEventChan chan events.UpdateViewEvent) {
	logging.LogDebug("ui/out/Init()")

	err := termbox.Init()
	if err != nil {
		panic(err)
	}

	go drawLoop(updateViewEventChan)
}

func Stop() {
	logging.LogDebug("ui/out/Stop()")
	logging.LogDebug("ui/out/Stop(), termbox.IsInit: ", termbox.IsInit)
	screen.TermboxClose()
	logging.LogDebug("ui/out/Stop() end")
}

func GetScreenSize() (width, height int) {
	width, height = termbox.Size()
	logging.LogDebug("ui/out/GetScreenSize(), width: ", width, ", height: ", height)
	return width, height
}

func drawLoop(updateViewEventChan chan events.UpdateViewEvent) {
	logging.LogDebug("ui/out/drawLoop() starting")

	defer screen.TermboxClose()
loop:
	for {
		logging.LogDebug("ui/out/drawLoop(), waiting update event.")
		updateViewEvent := <-updateViewEventChan
		logging.LogDebug("ui/out/drawLoop(), updateViewEvent: ", updateViewEvent)
		switch updateViewEvent.Type {
		case events.UpdateViewQuit:
			logging.LogDebug("ui/out/drawLoop() received events.UpdateViewQuit")
			break loop
		case events.UpdateViewShowStatus:
			showStatus(updateViewEvent.Model, updateViewEvent.Status)
		default:
			draw(updateViewEvent)
		}
	}
	logging.LogDebug("ui/out/drawLoop() stopping")
}

func draw(updateViewEvent events.UpdateViewEvent) {
	logging.LogDebug("ui/out/draw(", updateViewEvent, ")")
	switch updateViewEvent.Type {
	case events.UpdateViewRefresh:
		refreshScreen(*updateViewEvent.Model)
	case events.UpdateViewShowDialog:
		showQuestionDialog(updateViewEvent.DialogQuestion, updateViewEvent.DialogAnswer, updateViewEvent.DialogAnswerCursorIdx)
	}
}

func refreshScreen(m model.Model) {
	logging.LogDebugf("ui/out/refreshScreen('%v')", m)

	termbox.Clear(fgNormal, bgNormal)

	width, _ := termbox.Size()
	if m.IsCurrentSide(model.Left) {
		writeLine(0, 0, m.Title(model.Left), m.HorizScroll, fgNormal, bgCurrent)
		writeLine(width/2, 0, m.Title(model.Right), m.HorizScroll, fgNormal, bgNormal)
	} else {
		writeLine(0, 0, m.Title(model.Left), m.HorizScroll, fgNormal, bgNormal)
		writeLine(width/2, 0, m.Title(model.Right), m.HorizScroll, fgNormal, bgCurrent)
	}

	for idx := 0; idx < m.GetVisibleItemCount(model.Left); idx++ {
		logging.LogTrace("ui/out/draw(), idx: ", idx)
		item := m.GetVisibleItem(model.Left, idx)
		var fg = fgNormal
		var bg = bgNormal
		if m.IsCurrentRow(model.Left, idx) {
			bg = bgCurrent
		}
		if item.Selected {
			fg = fgSelected
		}
		writeLine(0, idx+2, item.DisplayString(), m.HorizScroll, fg, bg)
	}
	for idx := 0; idx < m.GetVisibleItemCount(model.Right); idx++ {
		logging.LogTrace("ui/out/draw(), idx: ", idx)
		item := m.GetVisibleItem(model.Right, idx)
		var fg = fgNormal
		var bg = bgNormal
		if m.IsCurrentRow(model.Right, idx) {
			bg = bgCurrent
		}
		if item.Selected {
			fg = fgSelected
		}
		writeLine(width/2, idx+2, item.DisplayString(), m.HorizScroll, fg, bg)
	}

	// showStatus(m, currentStatus)

	termbox.Flush()
}

func showQuestionDialog(question, answer string, answerCursorIdx int) {
	logging.LogDebugf("ui/out/showQuestionDialog('%s')", question)

	// termbox.Clear(fgNormal, bgNormal)
	width, height := termbox.Size()
	x := 10
	y := height/2 - 2
	dialogWidth := width - 20
	line := question + answer
	cursorIdx := utf8.RuneCountInString(question) + answerCursorIdx
	writeLine(x, y-2, buildLine("", "*", "", dialogWidth), 0, fgNormal, bgNormal)
	writeLine(x, y-1, buildLine("*", " ", "*", dialogWidth), 0, fgNormal, bgNormal)
	writeLine(x, y, buildLine("*", " ", "*", dialogWidth), 0, fgNormal, bgNormal)
	writeLine(x, y+1, buildLine("*", " ", "*", dialogWidth), 0, fgNormal, bgNormal)
	writeLine(x, y+2, buildLine("", "*", "", dialogWidth), 0, fgNormal, bgNormal)
	writeLineWithCursor(x+2, y, line, 0, fgNormal, bgNormal, x+2+cursorIdx, termbox.AttrReverse, termbox.AttrReverse)

	termbox.Flush()
}

func writeLine(x, y int, line string, horizScroll int, fg, bg termbox.Attribute) int {
	return writeLineWithCursor(x, y, line, horizScroll, fg, bg, -1, fg, bg)
}

func writeLineWithCursor(x, y int, line string, horizScroll int, fg, bg termbox.Attribute, cursorX int, cursorFg, cursorBg termbox.Attribute) int {
	scrolledLine := scrollLineHoriz(line, horizScroll)

	var xpos int
	runeIdx := 0
	for _, runeVal := range scrolledLine {
		xpos = x + runeIdx
		runeIdx = runeIdx + 1
		if cursorX == xpos {
			termbox.SetCell(xpos, y, runeVal, cursorFg, cursorBg)
		} else {
			termbox.SetCell(xpos, y, runeVal, fg, bg)
		}
	}
	// logging.LogDebug("x: ", x, ", scrolledLine: ", scrolledLine, ", len(scrolledLine): ", len(scrolledLine))
	// logging.LogDebug("cursorX: ", cursorX, "xpos: ", xpos)
	if cursorX > xpos {
		termbox.SetCell(cursorX, y, rune(' '), cursorFg, cursorBg)
	} else if cursorX == -2 {
		// logging.LogDebug("------ ", "xpos+1: ", (xpos + 1))
		termbox.SetCell(xpos+1, y, rune(' '), cursorFg, cursorBg)
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

func scrollLineHoriz(line string, horizScroll int) string {
	var scrollh = horizScroll
	runeCount := utf8.RuneCountInString(line)
	if runeCount < scrollh {
		scrollh = runeCount
	}

	runeIdx := 0
	if scrollh != 0 {
		scrolledLine := ""
		for byteIdx, _ := range line {
			if runeIdx == scrollh {
				scrolledLine = line[byteIdx:]
				break
			}
			runeIdx = runeIdx + 1
		}
		return scrolledLine
	}

	return line
}

func showStatus(m *model.Model, status string) {
	var filterMsg string
	var syncMsg string
	if m.CurrentFilter() != "" {
		filterMsg = fmt.Sprintf("Filter: '%s' | ", m.CurrentFilter())
	}
	if m.SyncModeOn {
		syncMsg = fmt.Sprintf("Sync: '%s' -> (%s) '%s' | ", m.SyncDirLocal, m.SyncDomainDp, m.SyncDirDp)
	}

	statusMsg := fmt.Sprintf("%s%s%s", syncMsg, filterMsg, status)

	_, h := termbox.Size()
	writeLine(0, h-1, statusMsg, m.HorizScroll, termbox.ColorDefault, termbox.ColorDefault)
	termbox.Flush()
}
