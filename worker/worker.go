package worker

import (
	"github.com/croz-ltd/dpcmder/events"
	"github.com/croz-ltd/dpcmder/model"
	"github.com/croz-ltd/dpcmder/repo/dp"
	"github.com/croz-ltd/dpcmder/repo/localfs"
	"github.com/croz-ltd/dpcmder/utils/logging"
	"github.com/croz-ltd/dpcmder/view/in/key"
)

// workingModel contains Model with all information on current DataPower and
// local filesystem we are showing in dpcmder.
var workingModel model.Model = model.Model{} //{currSide: model.Left}

// Init initializes DataPower and local filesystem access and load initial views.
func Init(keyPressedEventChan chan events.KeyPressedEvent, updateViewEventChan chan events.UpdateViewEvent) {
	logging.LogDebug("worker/Init()")
	go runWorkerInit(keyPressedEventChan, updateViewEventChan)
}

func runWorkerInit(keyPressedEventChan chan events.KeyPressedEvent, updateViewEventChan chan events.UpdateViewEvent) {
	logging.LogDebug("worker/runWorkerInit()")
	dp.InitNetworkSettings()
	initialLoad(updateViewEventChan)
	processInputEvent(keyPressedEventChan, updateViewEventChan)
}

func initialLoadDp() {
	logging.LogDebug("worker/initialLoadDp()")
	initialParent := dp.Repo.GetInitialParent()
	logging.LogDebug("worker/initialLoadDp(), initialParent: ", initialParent)
	workingModel.SetCurrPathForSide(model.Left, "")

	title := dp.Repo.GetTitle(initialParent)
	workingModel.SetTitle(model.Left, title)

	itemList := dp.Repo.GetList(initialParent)
	workingModel.SetItems(model.Left, itemList)
}

func initialLoadLocalfs() {
	logging.LogDebug("worker/initialLoadLocalfs()")
	initialParent := localfs.Repo.GetInitialParent()
	logging.LogDebug("worker/initialLoadLocalfs(), initialParent: ", initialParent)
	workingModel.SetCurrPathForSide(model.Right, initialParent.Path)

	title := localfs.Repo.GetTitle(initialParent)
	workingModel.SetTitle(model.Right, title)

	itemList := localfs.Repo.GetList(initialParent)
	workingModel.SetItems(model.Right, itemList)
}

func initialLoad(updateViewEventChan chan events.UpdateViewEvent) {
	logging.LogDebug("worker/initialLoad()")
	initialLoadDp()
	initialLoadLocalfs()

	updateViewEvent := events.UpdateViewEvent{Model: workingModel}
	updateViewEventChan <- updateViewEvent
}

func processInputEvent(keyPressedEventChan chan events.KeyPressedEvent, updateViewEventChan chan events.UpdateViewEvent) {
	logging.LogDebug("worker/processInputEvent()")
loop:
	for {
		logging.LogDebug("worker/processInputEvent(), waiting key pressed event.")
		keyPressedEvent := <-keyPressedEventChan
		logging.LogDebug("worker/processInputEvent(), keyPressedEvent:", keyPressedEvent)
		shouldUpdateView := false
		switch keyPressedEvent.KeyCode {
		case key.Chq:
			break loop
		case key.Tab:
			workingModel.ToggleSide()
			shouldUpdateView = true
			// case key.Return:
			// 	enterCurrentDirectory(m)

		}

		if shouldUpdateView {
			updateViewEvent := events.UpdateViewEvent{Model: workingModel}
			updateViewEventChan <- updateViewEvent
		}
	}
}
