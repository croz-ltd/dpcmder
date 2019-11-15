package out

import (
	"github.com/croz-ltd/dpcmder/events"
	"github.com/croz-ltd/dpcmder/model"
	"github.com/croz-ltd/dpcmder/utils"
	"github.com/croz-ltd/dpcmder/utils/logging"
	"github.com/croz-ltd/dpcmder/utils/screen"
	"github.com/nsf/termbox-go"
	"unicode/utf8"
)

const (
	fgNormal   = termbox.ColorDefault
	fgSelected = termbox.ColorRed
	bgNormal   = termbox.ColorDefault
	bgCurrent  = termbox.ColorGreen
)

func Init(updateViewEventChan chan events.UpdateViewEvent) {
	logging.LogDebug("view/out/Init()")

	err := termbox.Init()
	if err != nil {
		panic(err)
	}

	go drawLoop(updateViewEventChan)
}

func Stop() {
	logging.LogDebug("view/out/Stop()")
	logging.LogDebug("view/out/Stop(), termbox.IsInit: ", termbox.IsInit)
	screen.TermboxClose()
	logging.LogDebug("view/out/Stop() end")
}

func GetScreenSize() (width, height int) {
	width, height = termbox.Size()
	logging.LogDebug("view/out/GetScreenSize(), width: ", width, ", height: ", height)
	return width, height
}

func drawLoop(updateViewEventChan chan events.UpdateViewEvent) {
	logging.LogDebug("view/out/drawLoop() starting")

	defer screen.TermboxClose()
loop:
	for {
		logging.LogDebug("view/out/drawLoop(), waiting update event.")
		updateViewEvent := <-updateViewEventChan
		logging.LogDebug("view/out/drawLoop(), updateViewEvent: ", updateViewEvent)
		if updateViewEvent.Type == events.UpdateViewQuit {
			logging.LogDebug("view/out/drawLoop() received events.UpdateViewQuit")
			break loop
		}
		draw(updateViewEvent)
	}
	logging.LogDebug("view/out/drawLoop() stopping")
}

func draw(updateViewEvent events.UpdateViewEvent) {
	logging.LogDebug("view/out/draw(", updateViewEvent, ")")
	switch updateViewEvent.Type {
	case events.UpdateViewRefresh:
		refreshScreen(*updateViewEvent.Model)
	case events.UpdateViewShowDialog:
		showQuestionDialog(updateViewEvent.DialogQuestion, updateViewEvent.DialogAnswer)
	}
}

func refreshScreen(m model.Model) {
	logging.LogDebugf("view/out/refreshScreen('%v')", m)

	termbox.Clear(fgNormal, bgNormal)

	width, _ := termbox.Size()
	if m.IsCurrentSide(model.Left) {
		writeLine(0, 0, m.Title(model.Left), fgNormal, bgCurrent)
		writeLine(width/2, 0, m.Title(model.Right), fgNormal, bgNormal)
	} else {
		writeLine(0, 0, m.Title(model.Left), fgNormal, bgNormal)
		writeLine(width/2, 0, m.Title(model.Right), fgNormal, bgCurrent)
	}

	for idx := 0; idx < m.GetVisibleItemCount(model.Left); idx++ {
		logging.LogTrace("view/out/draw(), idx: ", idx)
		item := m.GetVisibleItem(model.Left, idx)
		var fg = fgNormal
		var bg = bgNormal
		if m.IsCurrentRow(model.Left, idx) {
			bg = bgCurrent
		}
		if item.Selected {
			fg = fgSelected
		}
		writeLine(0, idx+2, item.DisplayString(), fg, bg)
	}
	for idx := 0; idx < m.GetVisibleItemCount(model.Right); idx++ {
		logging.LogTrace("view/out/draw(), idx: ", idx)
		item := m.GetVisibleItem(model.Right, idx)
		var fg = fgNormal
		var bg = bgNormal
		if m.IsCurrentRow(model.Right, idx) {
			bg = bgCurrent
		}
		if item.Selected {
			fg = fgSelected
		}
		writeLine(width/2, idx+2, item.DisplayString(), fg, bg)
	}

	// showStatus(m, currentStatus)

	termbox.Flush()
}

func showQuestionDialog(question, answer string) {
	logging.LogDebugf("view/out/showQuestionDialog('%s')", question)

	// termbox.Clear(fgNormal, bgNormal)
	width, height := termbox.Size()
	x := 10
	y := height/2 - 2
	dialogWidth := width - 20
	line := question + answer
	lineLen := utf8.RuneCountInString(line)
	writeLine(x, y-2, utils.BuildLine("", "*", "", dialogWidth), fgNormal, bgNormal)
	writeLine(x, y-1, utils.BuildLine("*", " ", "*", dialogWidth), fgNormal, bgNormal)
	writeLine(x, y, utils.BuildLine("*", " ", "*", dialogWidth), fgNormal, bgNormal)
	writeLine(x, y+1, utils.BuildLine("*", " ", "*", dialogWidth), fgNormal, bgNormal)
	writeLine(x, y+2, utils.BuildLine("", "*", "", dialogWidth), fgNormal, bgNormal)
	writeLineWithCursor(x+2, y, line, fgNormal, bgNormal, x+2+lineLen, termbox.AttrReverse, termbox.AttrReverse)

	termbox.Flush()
}

func writeLine(x, y int, line string, fg, bg termbox.Attribute) int {
	// logging.LogDebug("view/out/writeLine(", x, ",", y, ",", line, ",", fg, ",", bg, ")")
	return writeLineWithCursor(x, y, line, fg, bg, -1, fg, bg)
}

func writeLineWithCursor(x, y int, line string, fg, bg termbox.Attribute, cursorX int, cursorFg, cursorBg termbox.Attribute) int {
	// logging.LogDebug("view/out/writeLineWithCursor(", x, ",", y, ",", line, ",", fg, ",", bg, ",", cursorX, ",", cursorFg, ",", cursorBg, ")")
	// var scrollh = horizScroll
	scrollh := 0
	runeCount := utf8.RuneCountInString(line)
	if runeCount < scrollh {
		scrollh = runeCount
		if scrollh < 0 {
			scrollh = 0
		}
	}

	var xpos int
	runeIdx := 0
	for _, runeVal := range line {
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

// func showStatus(m *model.Model, status string) {
// 	var filterMsg string
// 	var syncMsg string
// 	if m.CurrentFilter() != "" {
// 		filterMsg = fmt.Sprintf("Filter: '%s' | ", m.CurrentFilter())
// 	}
// 	// if syncModeOn {
// 	// 	syncMsg = fmt.Sprintf("Sync: '%s' -> (%s) '%s' | ", syncDirLocal, syncDomainDp, syncDirDp)
// 	// }
//
// 	statusMsg := fmt.Sprintf("%s%s%s", syncMsg, filterMsg, status)
//
// 	_, h := termbox.Size()
// 	writeLine(0, h-1, statusMsg, termbox.ColorDefault, termbox.ColorDefault)
// 	termbox.Flush()
// }
