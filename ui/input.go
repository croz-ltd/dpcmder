package ui

import (
	"github.com/croz-ltd/dpcmder/ui/out"
	"github.com/croz-ltd/dpcmder/utils/logging"
)

// StartReadingKeys starts (blocking) reading user's input.
func StartReadingKeys() {
	logging.LogDebug("ui/in/Start()")

	inputEventLoop()
}

// inputEventLoop is main loop reading user's input.
func inputEventLoop() {
	logging.LogDebug("ui/inputEventLoop() starting")

loop:
	for {
		logging.LogTrace("ui/inputEventLoop(), waiting to read")
		event := out.Screen.PollEvent()

		logging.LogTracef("ui/inputEventLoop(), event: %#v", event)
		err := ProcessInputEvent(event)
		if err != nil {
			break loop
		}
	}
	logging.LogDebug("ui/inputEventLoop() stopping")
}
