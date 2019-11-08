package worker

import (
	"github.com/croz-ltd/dpcmder/events"
	"github.com/croz-ltd/dpcmder/utils/logging"
)

func Init(keyPressedEventChan chan events.KeyPressedEvent, updateViewEventChan chan events.UpdateViewEvent) {
	logging.LogDebug("worker/Init()")
	go processInputEvent(keyPressedEventChan, updateViewEventChan)
}

func processInputEvent(keyPressedEventChan chan events.KeyPressedEvent, updateViewEventChan chan events.UpdateViewEvent) {
	logging.LogDebug("worker/processViewEvent()")
	for {
		logging.LogDebug("worker/processViewEvent(), waiting key pressed event.")
		keyPressedEvent := <-keyPressedEventChan
		logging.LogDebug("worker/processViewEvent(), keyPressedEvent:", keyPressedEvent)
		updateViewEvent := events.UpdateViewEvent{Txt: keyPressedEvent.HexBytes}
		updateViewEventChan <- updateViewEvent
	}

}
