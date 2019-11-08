package worker

import (
	"github.com/croz-ltd/dpcmder/events"
	"github.com/croz-ltd/dpcmder/model"
	"github.com/croz-ltd/dpcmder/repo/dp"
	"github.com/croz-ltd/dpcmder/repo/localfs"
	"github.com/croz-ltd/dpcmder/utils/logging"
)

// M contains Model with all information on current DataPower and filesystem
// we are showing in dpcmder.
var workingModel model.Model = model.Model{} //{currSide: model.Left}

func Init(keyPressedEventChan chan events.KeyPressedEvent, updateViewEventChan chan events.UpdateViewEvent) {
	logging.LogDebug("worker/Init()")
	go runWorkerInit(keyPressedEventChan, updateViewEventChan)
}

func runWorkerInit(keyPressedEventChan chan events.KeyPressedEvent, updateViewEventChan chan events.UpdateViewEvent) {
	logging.LogDebug("worker/runWorkerInit()")
	dp.InitNetworkSettings()
	dp.Repo.InitialLoad(&workingModel)
	localfs.Repo.InitialLoad(&workingModel)
	processInputEvent(keyPressedEventChan, updateViewEventChan)
}

func processInputEvent(keyPressedEventChan chan events.KeyPressedEvent, updateViewEventChan chan events.UpdateViewEvent) {
	logging.LogDebug("worker/processInputEvent()")
	for {
		logging.LogDebug("worker/processInputEvent(), waiting key pressed event.")
		keyPressedEvent := <-keyPressedEventChan
		logging.LogDebug("worker/processInputEvent(), keyPressedEvent:", keyPressedEvent)
		updateViewEvent := events.UpdateViewEvent{Txt: keyPressedEvent.HexBytes}
		updateViewEventChan <- updateViewEvent
	}

}
