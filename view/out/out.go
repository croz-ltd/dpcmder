package out

import (
	"github.com/croz-ltd/dpcmder/events"
	"github.com/croz-ltd/dpcmder/model"
	"github.com/croz-ltd/dpcmder/utils/logging"
	"github.com/nsf/termbox-go"
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
	termbox.Close()
}

func GetScreenSize() (width, height int) {
	width, height = termbox.Size()
	logging.LogDebug("view/out/GetScreenSize(), width: ", width, ", height: ", height)
	return width, height
}

func drawLoop(updateViewEventChan chan events.UpdateViewEvent) {
	logging.LogDebug("view/out/drawLoop()")

	defer termbox.Close()

	for {
		logging.LogDebug("view/out/drawLoop(), waiting update event.")
		updateViewEvent := <-updateViewEventChan
		logging.LogDebug("view/out/drawLoop(), updateViewEvent: ", updateViewEvent)
		draw(updateViewEvent)
	}
}

func draw(updateViewEvent events.UpdateViewEvent) {
	logging.LogDebug("view/out/draw(", updateViewEvent, ")")
	m := updateViewEvent.Model

	termbox.Clear(fgNormal, bgNormal)
	width, _ := termbox.Size()
	// logging.LogDebug("view/out/draw(), width: ", width)
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

func writeLine(x, y int, line string, fg, bg termbox.Attribute) int {
	// logging.LogDebug("view/out/writeLine(", x, ",", y, ",", line, ",", fg, ",", bg, ")")
	return writeLineWithCursor(x, y, line, fg, bg, -1, fg, bg)
}

func writeLineWithCursor(x, y int, line string, fg, bg termbox.Attribute, cursorX int, cursorFg, cursorBg termbox.Attribute) int {
	// logging.LogDebug("view/out/writeLineWithCursor(", x, ",", y, ",", line, ",", fg, ",", bg, ",", cursorX, ",", cursorFg, ",", cursorBg, ")")
	var scrolledLine string
	// var scrollh = horizScroll
	var scrollh = 0
	if len(line) < scrollh {
		scrollh = len(line)
		if scrollh < 0 {
			scrollh = 0
		}
	}
	scrolledLine = line[scrollh:]
	var xpos int
	for i := 0; i < len(scrolledLine); i++ {
		xpos = x + i
		if cursorX == xpos {
			termbox.SetCell(xpos, y, rune(scrolledLine[i]), cursorFg, cursorBg)
		} else {
			termbox.SetCell(xpos, y, rune(scrolledLine[i]), fg, bg)
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
