package worker

import (
	"github.com/croz-ltd/dpcmder/events"
	"github.com/croz-ltd/dpcmder/model"
	"github.com/croz-ltd/dpcmder/repo"
	"github.com/croz-ltd/dpcmder/repo/dp"
	"github.com/croz-ltd/dpcmder/repo/localfs"
	"github.com/croz-ltd/dpcmder/utils/logging"
	"github.com/croz-ltd/dpcmder/view/in/key"
	"github.com/croz-ltd/dpcmder/view/out"
)

// repos contains references to DataPower and local filesystem repositories.
var repos = []repo.Repo{model.Left: &dp.Repo, model.Right: &localfs.Repo}

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
	initialView := dp.Repo.GetInitialView()
	workingModel.SetCurrentView(model.Left, initialView)
	logging.LogDebug("worker/initialLoadDp(), initialView: ", initialView)
	workingModel.SetCurrPathForSide(model.Left, "")

	title := dp.Repo.GetTitle(initialView)
	workingModel.SetTitle(model.Left, title)

	itemList := dp.Repo.GetList(initialView)
	workingModel.SetItems(model.Left, itemList)
}

func initialLoadLocalfs() {
	logging.LogDebug("worker/initialLoadLocalfs()")
	initialView := localfs.Repo.GetInitialView()
	logging.LogDebug("worker/initialLoadLocalfs(), initialView: ", initialView)
	workingModel.SetCurrentView(model.Right, initialView)
	workingModel.SetCurrPathForSide(model.Right, initialView.Path)

	title := localfs.Repo.GetTitle(initialView)
	workingModel.SetTitle(model.Right, title)

	itemList := localfs.Repo.GetList(initialView)
	workingModel.SetItems(model.Right, itemList)
}

func initialLoad(updateViewEventChan chan events.UpdateViewEvent) {
	logging.LogDebug("worker/initialLoad()")
	initialLoadDp()
	initialLoadLocalfs()

	setScreenSize()
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

		setScreenSize()

		shouldUpdateView := true
		switch keyPressedEvent.KeyCode {
		case key.Chq:
			break loop
		case key.Tab:
			workingModel.ToggleSide()
		case key.Return:
			enterCurrentDirectory()
		case key.Space:
			workingModel.ToggleCurrItem()
		// case key.Dot:
		// 	enterPath(m)
		// case key.ArrowLeft, key.Chj:
		// 	horizScroll -= 10
		// 	if horizScroll < 0 {
		// 		horizScroll = 0
		// 	}
		// case key.ArrowRight, key.Chl:
		// 	horizScroll += 10
		case key.ArrowUp, key.Chi:
			workingModel.NavUp()
		case key.ArrowDown, key.Chk:
			workingModel.NavDown()
		case key.ShiftArrowUp, key.ChI:
			workingModel.ToggleCurrItem()
			workingModel.NavUp()
		case key.ShiftArrowDown, key.ChK:
			workingModel.ToggleCurrItem()
			workingModel.NavDown()
		case key.PgUp, key.Chu:
			workingModel.NavPgUp()
		case key.PgDown, key.Cho:
			workingModel.NavPgDown()
		case key.ShiftPgUp, key.ChU:
			workingModel.SelPgUp()
			workingModel.NavPgUp()
		case key.ShiftPgDown, key.ChO:
			workingModel.SelPgDown()
			workingModel.NavPgDown()
		case key.Home, key.Cha:
			workingModel.NavTop()
		case key.End, key.Chz:
			workingModel.NavBottom()
		case key.ShiftHome, key.ChA:
			workingModel.SelToTop()
			workingModel.NavTop()
		case key.ShiftEnd, key.ChZ:
			workingModel.SelToBottom()
			workingModel.NavBottom()
		}

		if shouldUpdateView {
			updateViewEvent := events.UpdateViewEvent{Model: workingModel}
			updateViewEventChan <- updateViewEvent
		}
	}
}

func enterCurrentDirectory() {
	logging.LogDebug("worker/enterCurrentDirectory()")
	r := repos[workingModel.CurrSide()]
	item := workingModel.CurrItem()
	logging.LogDebug("worker/enterCurrentDirectory(), item: ", item)
	switch item.Type {
	case model.ItemDpConfiguration, model.ItemDpDomain, model.ItemDpFilestore, model.ItemDirectory, model.ItemNone:
		currentView := workingModel.CurrentView(workingModel.CurrSide())
		newView := r.NextView(currentView, *item)
		itemList := r.GetList(newView)
		title := r.GetTitle(newView)
		logging.LogDebug("worker/enterCurrentDirectory(), title: ", title)

		workingModel.SetCurrentView(workingModel.CurrSide(), newView)
		workingModel.SetItems(workingModel.CurrSide(), itemList)
		workingModel.SetTitle(workingModel.CurrSide(), title)
	default:
		logging.LogDebug("worker/enterCurrentDirectory(), type: ", item.Type)
	}
}

func setScreenSize() {
	width, height := out.GetScreenSize()
	workingModel.SetItemsMaxSize(height-3, width)
}
