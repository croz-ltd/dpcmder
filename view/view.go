package view

import (
	"fmt"
	"github.com/croz-ltd/dpcmder/view/in"
	"github.com/croz-ltd/dpcmder/view/out"
	"github.com/nsf/termbox-go"
)

func Init(eventChan chan string) {
	fmt.Println("view/Init()")

	err := termbox.Init()
	if err != nil {
		panic(err)
	}
	defer termbox.Close()
	// setScreenSize(m)
	// draw(m)
	//
	// keyPressedLoop(m)
	out.Init(eventChan)
	in.Start(eventChan)
}
