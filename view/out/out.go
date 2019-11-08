package out

import (
	"github.com/croz-ltd/dpcmder/events"
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
	// setScreenSize(m)
	// draw(m)
	//
	// keyPressedLoop(m)

	go drawLoop(updateViewEventChan)
}

func Stop() {
	termbox.Close()
}

func drawLoop(updateViewEventChan chan events.UpdateViewEvent) {
	logging.LogDebug("view/out/drawLoop()")
	for {
		logging.LogDebug("view/out/drawLoop(), waiting update event.")
		txt := <-updateViewEventChan
		logging.LogDebug("view/out/drawLoop(), txt: ", txt)
		draw(txt.Txt)
	}
}

func draw(txt string) {
	logging.LogDebug("view/out/draw(", txt, ")")
	termbox.Clear(termbox.ColorDefault, termbox.ColorDefault)
	termbox.SetCell(5, 5, rune(txt[0]), termbox.ColorDefault, termbox.ColorDefault)
	termbox.Flush()
}
