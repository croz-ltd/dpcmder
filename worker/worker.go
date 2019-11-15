package worker

import (
	"fmt"
	"github.com/croz-ltd/dpcmder/config"
	"github.com/croz-ltd/dpcmder/events"
	"github.com/croz-ltd/dpcmder/model"
	"github.com/croz-ltd/dpcmder/repo"
	"github.com/croz-ltd/dpcmder/repo/dp"
	"github.com/croz-ltd/dpcmder/repo/localfs"
	"github.com/croz-ltd/dpcmder/utils"
	"github.com/croz-ltd/dpcmder/utils/logging"
	"github.com/croz-ltd/dpcmder/view/in/key"
	"github.com/croz-ltd/dpcmder/view/out"
)

// DpMissingPasswordError is constant error returned if DataPower password is
// not set and we want to connect to appliance.
const DpMissingPasswordError = utils.Error("DpMissingPasswordError")

// userDialogInputSessionInfo is structure containing all information neccessary
// for user entering information into input dialog.
type userDialogInputSessionInfo struct {
	inputQuestion        string
	inputAnswer          string
	inputAnswerCursorIdx int
	inputAnswerMasked    bool
	userInputActive      bool
}

func (ud userDialogInputSessionInfo) String() string {
	return fmt.Sprintf("Session(%T, q: '%s', a: '%s')", ud.userInputActive, ud.inputQuestion, ud.inputAnswer)
}

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

	title := repo.GetTitle(initialItem)
	workingModel.SetCurrentView(side, initialItem.Config, title)

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
	updateViewEventChan <- events.UpdateViewEvent{Type: events.UpdateViewRefresh, Model: &workingModel}
}

func processInputEvent(keyPressedEventChan chan events.KeyPressedEvent, updateViewEventChan chan events.UpdateViewEvent) {
	logging.LogDebug("worker/processInputEvent() starting")

	// Dialog with user - question asked, user's answer and active state.
	var dialogSession = userDialogInputSessionInfo{}
loop:
	for {
		logging.LogDebug("worker/processInputEvent(), waiting key pressed event.")
		keyPressedEvent := <-keyPressedEventChan
		logging.LogDebug("worker/processInputEvent(), keyPressedEvent:", keyPressedEvent)

		setScreenSize()

		if dialogSession.userInputActive {
			updateViewEvent := processInputDialogInput(&dialogSession, keyPressedEvent.KeyCode)
			updateViewEventChan <- updateViewEvent
		} else {
			shouldUpdateView := true
			switch keyPressedEvent.KeyCode {
			case key.Chq:
				events.Quit = true
				break loop
			case key.Tab:
				workingModel.ToggleSide()
			case key.Return:
				err := enterCurrentDirectory()
				logging.LogDebug("worker/processInputEvent(), err: ", err)
				if err == DpMissingPasswordError {
					dialogSession.userInputActive = true
					dialogSession.inputQuestion = "Please enter DataPower password: "
					dialogSession.inputAnswer = ""
					shouldUpdateView = false
					updateViewEventChan <- events.UpdateViewEvent{
						Type:           events.UpdateViewShowDialog,
						DialogQuestion: dialogSession.inputQuestion,
						DialogAnswer:   dialogSession.inputAnswer}
				}
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
				updateViewEventChan <- events.UpdateViewEvent{Type: events.UpdateViewRefresh, Model: &workingModel}
			}
		}
	}
	logging.LogDebug("worker/processInputEvent() sending events.UpdateViewQuit")
	updateViewEventChan <- events.UpdateViewEvent{Type: events.UpdateViewQuit}
	logging.LogDebug("worker/processInputEvent() stopping")
}

func processInputDialogInput(dialogSession *userDialogInputSessionInfo, keyCode key.KeyCode) events.UpdateViewEvent {
	dialogSession.inputAnswer = dialogSession.inputAnswer + utils.ConvertKeyCodeStringToString(keyCode)
	logging.LogDebug("worker/processInputEvent(): '%s'", dialogSession)
	switch keyCode {
	// case key.Backspace, key.BackspaceWin:
	// 	if dialogSession.inputAnswerCursorIdx > 0:
	// 	dialogSession.inputAnswer = strings.
	case key.Esc:
		logging.LogDebug("worker/processInputEvent() canceling user input: '%s'", dialogSession)
		dialogSession.userInputActive = false
		dialogSession.inputAnswer = ""
		return events.UpdateViewEvent{Type: events.UpdateViewRefresh, Model: &workingModel}
	case key.Return:
		logging.LogDebug("worker/processInputEvent() accepting user input: '%s'", dialogSession)
		if dialogSession.inputAnswer != "" {
			item := workingModel.CurrItem()
			applianceName := item.Name
			applicanceConfig := config.Conf.DataPowerAppliances[applianceName]
			logging.LogDebug("worker/processInputEvent() applicanceConfig before: '%s'", applicanceConfig)
			applicanceConfig.SetDpPlaintextPassword(dialogSession.inputAnswer)
			config.Conf.DataPowerAppliances[applianceName] = applicanceConfig
			logging.LogDebug("worker/processInputEvent() applicanceConfig after : '%s'", applicanceConfig)
			// config.SetDpPassword(dialogSession.inputAnswer)
		}
		dialogSession.userInputActive = false
		return events.UpdateViewEvent{Type: events.UpdateViewRefresh, Model: &workingModel}
	default:
		return events.UpdateViewEvent{
			Type:           events.UpdateViewShowDialog,
			DialogQuestion: dialogSession.inputQuestion,
			DialogAnswer:   dialogSession.inputAnswer}
	}

	return events.UpdateViewEvent{
		Type:           events.UpdateViewShowDialog,
		DialogQuestion: dialogSession.inputQuestion,
		DialogAnswer:   dialogSession.inputAnswer}

	// switch hexBytesRead {
	// case key.Return:
	// 	break loop
	// case key.Backspace, key.BackspaceWin:
	// 	// Remove character before cursor
	// 	if rbIdx > 0 {
	// 		rbSuffix := make([]byte, rbLen-rbIdx)
	// 		copy(rbSuffix[:], rb[rbIdx:rbLen])
	// 		copy(rb[rbIdx-1:], rbSuffix[:])
	// 		rb[rbLen-1] = 0
	// 		rbIdx--
	// 		rbLen--
	// 		if rbIdx < 0 {
	// 			rbIdx = 0
	// 		}
	// 		if rbLen < 0 {
	// 			rbLen = 0
	// 		}
	// 	}
	// case key.Del:
	// 	// Remove character at cursor
	// 	if rbIdx < rbLen {
	// 		rbSuffix := make([]byte, rbLen-rbIdx-1)
	// 		copy(rbSuffix[:], rb[rbIdx+1:rbLen])
	// 		copy(rb[rbIdx:], rbSuffix[:])
	// 		rb[rbLen-1] = 0
	// 		rbLen--
	// 		if rbLen < 0 {
	// 			rbLen = 0
	// 		}
	// 	}
	// case key.ArrowLeft:
	// 	rbIdx--
	// 	if rbIdx < 0 {
	// 		rbIdx = 0
	// 	}
	// case key.ArrowRight:
	// 	rbIdx++
	// 	if rbIdx > rbLen {
	// 		rbIdx = rbLen
	// 	}
	// case key.Esc:
	// 	return ""
	// default:
	// 	// Insert string in middle of current string
	// 	rbSuffix := make([]byte, rbLen-rbIdx)
	// 	copy(rbSuffix[:], rb[rbIdx:rbLen])
	// 	copy(rb[rbIdx:rbIdx+bytesReadCount], bytesRead)
	// 	copy(rb[rbIdx+bytesReadCount:], rbSuffix[:])
	// 	rbIdx += bytesReadCount
	// 	rbLen += bytesReadCount
	// 	if rbLen > len(rb) {
	// 		rbLen = len(rb)
	// 	}
	// 	if rbIdx > rbLen {
	// 		rbIdx = rbLen
	// 	}
	// }

}

func enterCurrentDirectory() error {
	logging.LogDebug("worker/enterCurrentDirectory()")
	r := repos[workingModel.CurrSide()]
	item := workingModel.CurrItem()
	logging.LogDebug("worker/enterCurrentDirectory(), item: ", item)
	if item.Config.Type == model.ItemDpConfiguration {
		applianceName := item.Name
		if applianceName != ".." {
			applicanceConfig := config.Conf.DataPowerAppliances[applianceName]
			logging.LogDebugf("worker/enterCurrentDirectory(), applicanceConfig: '%s'", applicanceConfig)
			if applicanceConfig.Password == "" {
				return DpMissingPasswordError
			}
		}
	}

	switch item.Config.Type {
	case model.ItemDpConfiguration, model.ItemDpDomain, model.ItemDpFilestore, model.ItemDirectory, model.ItemNone:
		itemList := r.GetList(*item)
		logging.LogDebug("worker/enterCurrentDirectory(), itemList: ", itemList)
		title := r.GetTitle(*item)
		logging.LogDebug("worker/enterCurrentDirectory(), title: ", title)

		oldViewConfig := workingModel.ViewConfig(workingModel.CurrSide())
		workingModel.SetItems(workingModel.CurrSide(), itemList)
		workingModel.SetCurrentView(workingModel.CurrSide(), item.Config, title)
		if item.Name != ".." {
			workingModel.NavTop()
		} else {
			workingModel.SetCurrItemForSideAndConfig(workingModel.CurrSide(), oldViewConfig)
		}
	default:
		logging.LogDebug("worker/enterCurrentDirectory(), unknown type: ", item.Config.Type)
	}

	return nil
}

func setScreenSize() {
	width, height := out.GetScreenSize()
	workingModel.SetItemsMaxSize(height-3, width)
}
