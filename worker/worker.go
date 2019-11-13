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
	defer out.Stop()
	dp.InitNetworkSettings()
	initialLoad(updateViewEventChan)
	processInputEvent(keyPressedEventChan, updateViewEventChan)
}

func initialLoadRepo(side model.Side, repo repo.Repo) {
	logging.LogDebugf("worker/initialLoadRepo(%v, %v)", side, repo)
	initialItem := repo.GetInitialItem()
	logging.LogDebugf("worker/initialLoadRepo(%v, %v), initialItem: %v", side, repo, initialItem)
	workingModel.SetCurrPathForSide(side, "")

	title := repo.GetTitle(initialItem)
	workingModel.SetTitle(side, *initialItem.Config, title)

	itemList := repo.GetList(initialItem)
	workingModel.SetItems(side, itemList)
}

func initialLoadDp() {
	initialLoadRepo(model.Left, &dp.Repo)
}

func initialLoadLocalfs() {
	initialLoadRepo(model.Right, &localfs.Repo)
}

func initialLoad(updateViewEventChan chan events.UpdateViewEvent) {
	logging.LogDebug("worker/initialLoad()")
	initialLoadDp()
	initialLoadLocalfs()

	setScreenSize()
	updateViewEventChan <- events.UpdateViewEvent{Type: events.UpdateViewRefresh, Model: workingModel}
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
			updateViewEventChan <- events.UpdateViewEvent{Type: events.UpdateViewRefresh, Model: workingModel}
		}
	}
}

func enterCurrentDirectory() {
	logging.LogDebug("worker/enterCurrentDirectory()")
	r := repos[workingModel.CurrSide()]
	item := workingModel.CurrItem()
	logging.LogDebug("worker/enterCurrentDirectory(), item: ", item)
	switch item.Config.Type {
	case model.ItemDpConfiguration, model.ItemDpDomain, model.ItemDpFilestore, model.ItemDirectory, model.ItemNone:
		itemList := r.GetList(*item)
		logging.LogDebug("worker/enterCurrentDirectory(), itemList: ", itemList)
		title := r.GetTitle(*item)
		logging.LogDebug("worker/enterCurrentDirectory(), title: ", title)

		oldViewConfig := workingModel.ViewConfig(workingModel.CurrSide())
		workingModel.SetItems(workingModel.CurrSide(), itemList)
		workingModel.SetTitle(workingModel.CurrSide(), *item.Config, title)
		if item.Name != ".." {
			workingModel.NavTop()
		} else {
			workingModel.SetCurrItemForSideAndConfig(workingModel.CurrSide(), &oldViewConfig)
		}
	default:
		logging.LogDebug("worker/enterCurrentDirectory(), unknown type: ", item.Config.Type)
	}
}

func setScreenSize() {
	width, height := out.GetScreenSize()
	workingModel.SetItemsMaxSize(height-3, width)
}
