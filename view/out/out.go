package out

import (
	"fmt"
	"github.com/nsf/termbox-go"
)

func Init(eventChan chan string) {
	fmt.Println("view/out/Init()")
	go drawLoop(eventChan)
}

func drawLoop(eventChan chan string) {
	for {
		txt := <-eventChan
		draw(txt)
	}
}

func draw(txt string) {
	termbox.Clear(termbox.ColorDefault, termbox.ColorDefault)
	termbox.SetCell(5, 5, rune(txt[0]), termbox.ColorDefault, termbox.ColorDefault)
	termbox.Flush()
}
