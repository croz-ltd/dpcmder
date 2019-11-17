package worker

import (
	"fmt"
	"github.com/croz-ltd/dpcmder/config"
	"github.com/croz-ltd/dpcmder/events"
	"github.com/croz-ltd/dpcmder/extprogs"
	"github.com/croz-ltd/dpcmder/model"
	"github.com/croz-ltd/dpcmder/repo"
	"github.com/croz-ltd/dpcmder/repo/dp"
	"github.com/croz-ltd/dpcmder/repo/localfs"
	"github.com/croz-ltd/dpcmder/ui/in/key"
	"github.com/croz-ltd/dpcmder/ui/out"
	"github.com/croz-ltd/dpcmder/utils/errs"
	"github.com/croz-ltd/dpcmder/utils/logging"
	"strings"
	"unicode/utf8"
)

// DpMissingPasswordError is constant error returned if DataPower password is
// not set and we want to connect to appliance.
const dpMissingPasswordError = errs.Error("DpMissingPasswordError")

type userDialogType string

const askDpPassword = userDialogType("askDpPassword")
const askFilter = userDialogType("askFilter")
const askSearchNext = userDialogType("askSearchNext")
const askSearchPrev = userDialogType("askSearchPrev")

// userDialogInputSessionInfo is structure containing all information neccessary
// for user entering information into input dialog.
type userDialogInputSessionInfo struct {
	userInputActive      bool
	dialogType           userDialogType
	inputQuestion        string
	inputAnswer          string
	inputAnswerCursorIdx int
	inputAnswerMasked    bool
}

func (ud userDialogInputSessionInfo) String() string {
	return fmt.Sprintf("Session(%T (%s) q: '%s', a: '%s', cur: %d, masked: %T)",
		ud.userInputActive, ud.dialogType, ud.inputQuestion, ud.inputAnswer,
		ud.inputAnswerCursorIdx, ud.inputAnswerMasked)
}

// repos contains references to DataPower and local filesystem repositories.
var repos = []repo.Repo{model.Left: &dp.Repo, model.Right: &localfs.Repo}

// workingModel contains Model with all information on current DataPower and
// local filesystem we are showing in dpcmder.
var workingModel model.Model = model.Model{} //{currSide: model.Left}

var quitting = false

// IsQuitting checks if application is currently quitting.
func IsQuitting() bool { return quitting }

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

	itemList, err := repo.GetList(initialItem.Config)
	if err != nil {
		logging.LogDebug("worker/initialLoadRepo(): ", err)
		return
	}
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
			switch keyPressedEvent.KeyCode {
			case key.Chq:
				quitting = true
				break loop
			case key.Tab:
				workingModel.ToggleSide()
			case key.Return:
				err := enterCurrentDirectory()
				logging.LogDebug("worker/processInputEvent(), err: ", err)
				switch err {
				case dpMissingPasswordError:
					updateView := showInputDialog(&dialogSession, askDpPassword, "Please enter DataPower password: ", "", true)
					updateViewEventChan <- updateView
					continue
				case nil:
				default:
					updateView := events.UpdateViewEvent{
						Type: events.UpdateViewShowStatus, Status: err.Error(), Model: &workingModel}
					updateViewEventChan <- updateView
					switch err.(type) {
					case errs.UnexpectedHTTPResponse:
						setCurrentDpPlainPassword("")
					}
					continue
				}
			case key.Space:
				workingModel.ToggleCurrItem()
			// case key.Dot:
			// 	enterPath(m)
			case key.ArrowLeft, key.Chj:
				workingModel.HorizScroll -= 10
				if workingModel.HorizScroll < 0 {
					workingModel.HorizScroll = 0
				}
			case key.ArrowRight, key.Chl:
				workingModel.HorizScroll += 10
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
			case key.Chf:
				cf := workingModel.CurrentFilter()
				updateView := showInputDialog(&dialogSession, askFilter, "Filter by: ", cf, false)
				updateViewEventChan <- updateView
				continue
			case key.Slash:
				workingModel.SearchBy = ""
				updateView := showInputDialog(&dialogSession, askSearchNext, "Search by: ", workingModel.SearchBy, false)
				updateViewEventChan <- updateView
				continue
			case key.Chn, key.Chp:
				if workingModel.SearchBy == "" {
					var dialogType userDialogType
					switch keyPressedEvent.KeyCode {
					case key.Chn:
						dialogType = askSearchNext
					case key.Chp:
						dialogType = askSearchPrev
					}
					updateView := showInputDialog(&dialogSession, dialogType, "Search by: ", workingModel.SearchBy, false)
					updateViewEventChan <- updateView
					continue
				}

				var found bool
				switch keyPressedEvent.KeyCode {
				case key.Chn:
					found = workingModel.SearchNext(workingModel.SearchBy)
				case key.Chp:
					found = workingModel.SearchPrev(workingModel.SearchBy)
				}
				if !found {
					notFoundStatus := fmt.Sprintf("Item '%s' not found.", workingModel.SearchBy)
					updateView := events.UpdateViewEvent{
						Type: events.UpdateViewShowStatus, Status: notFoundStatus, Model: &workingModel}
					updateViewEventChan <- updateView
					continue
				}

			case key.F2, key.Ch2:
				repo := repos[workingModel.CurrSide()]
				repo.InvalidateCache()
				repo.GetList(workingModel.ViewConfig(workingModel.CurrSide()))
				refreshedStatus := "Current directory refreshed."
				updateView := events.UpdateViewEvent{
					Type: events.UpdateViewShowStatus, Status: refreshedStatus, Model: &workingModel}
				updateViewEventChan <- updateView

			case key.F3, key.Ch3:
				logging.LogDebug("-------- before")
				err := viewCurrent(&workingModel)
				logging.LogDebug("-------- after")
				if err != nil {
					updateView := events.UpdateViewEvent{
						Type: events.UpdateViewShowStatus, Status: err.Error(), Model: &workingModel}
					updateViewEventChan <- updateView
					continue
				}
			}

			updateViewEventChan <- events.UpdateViewEvent{Type: events.UpdateViewRefresh, Model: &workingModel}
		}
	}
	logging.LogDebug("worker/processInputEvent() sending events.UpdateViewQuit")
	updateViewEventChan <- events.UpdateViewEvent{Type: events.UpdateViewQuit}
	logging.LogDebug("worker/processInputEvent() stopping")
}

func showInputDialog(dialogSession *userDialogInputSessionInfo, dialogType userDialogType, question, answer string, answerMasked bool) events.UpdateViewEvent {
	dialogSession.dialogType = dialogType
	dialogSession.userInputActive = true
	dialogSession.inputQuestion = question
	dialogSession.inputAnswer = answer
	dialogSession.inputAnswerCursorIdx = utf8.RuneCountInString(answer)
	dialogSession.inputAnswerMasked = answerMasked

	return events.UpdateViewEvent{
		Type:                  events.UpdateViewShowDialog,
		DialogQuestion:        dialogSession.inputQuestion,
		DialogAnswer:          dialogSession.inputAnswer,
		DialogAnswerCursorIdx: dialogSession.inputAnswerCursorIdx}
}

func processInputDialogInput(dialogSession *userDialogInputSessionInfo, keyCode key.KeyCode) events.UpdateViewEvent {
	logging.LogDebugf("worker/processInputDialogInput(): '%s'", dialogSession)
	switch keyCode {
	case key.Backspace, key.BackspaceWin:
		if dialogSession.inputAnswerCursorIdx > 0 {
			changedAnswer := ""
			runeIdx := 0
			for _, runeVal := range dialogSession.inputAnswer {
				if runeIdx+1 != dialogSession.inputAnswerCursorIdx {
					changedAnswer = changedAnswer + string(runeVal)
				}
				runeIdx = runeIdx + 1
			}
			dialogSession.inputAnswer = changedAnswer
			dialogSession.inputAnswerCursorIdx = dialogSession.inputAnswerCursorIdx - 1
		}
	case key.Del:
		if dialogSession.inputAnswerCursorIdx < utf8.RuneCountInString(dialogSession.inputAnswer) {
			changedAnswer := ""
			runeIdx := 0
			for _, runeVal := range dialogSession.inputAnswer {
				if runeIdx != dialogSession.inputAnswerCursorIdx {
					changedAnswer = changedAnswer + string(runeVal)
				}
				runeIdx = runeIdx + 1
			}
			dialogSession.inputAnswer = changedAnswer
		}
	case key.ArrowLeft:
		if dialogSession.inputAnswerCursorIdx > 0 {
			dialogSession.inputAnswerCursorIdx = dialogSession.inputAnswerCursorIdx - 1
		}
	case key.ArrowRight:
		if dialogSession.inputAnswerCursorIdx < utf8.RuneCountInString(dialogSession.inputAnswer) {
			dialogSession.inputAnswerCursorIdx = dialogSession.inputAnswerCursorIdx + 1
		}
	case key.Home:
		dialogSession.inputAnswerCursorIdx = 0
	case key.End:
		dialogSession.inputAnswerCursorIdx = utf8.RuneCountInString(dialogSession.inputAnswer)
	case key.Esc:
		logging.LogDebug("worker/processInputDialogInput() canceling user input: '%s'", dialogSession)
		dialogSession.userInputActive = false
		dialogSession.inputAnswer = ""
		dialogSession.inputAnswerCursorIdx = 0
		return events.UpdateViewEvent{Type: events.UpdateViewRefresh, Model: &workingModel}
	case key.Return:
		logging.LogDebugf("worker/processInputDialogInput() accepting user input: '%s'", dialogSession)
		switch dialogSession.dialogType {
		case askDpPassword:
			if dialogSession.inputAnswer != "" {
				setCurrentDpPlainPassword(dialogSession.inputAnswer)
			}
		case askFilter:
			workingModel.SetCurrentFilter(dialogSession.inputAnswer)
		case askSearchNext, askSearchPrev:
			workingModel.SearchBy = dialogSession.inputAnswer
			if workingModel.SearchBy != "" {
				found := false
				switch dialogSession.dialogType {
				case askSearchNext:
					found = workingModel.SearchNext(workingModel.SearchBy)
				case askSearchPrev:
					found = workingModel.SearchPrev(workingModel.SearchBy)
				}
				if !found {
					notFoundStatus := fmt.Sprintf("Item '%s' not found.", workingModel.SearchBy)
					return events.UpdateViewEvent{
						Type: events.UpdateViewShowStatus, Status: notFoundStatus, Model: &workingModel}
				}
			}

		default:
			logging.LogDebugf("worker/processInputDialogInput() unknown input dialog type: '%s'", dialogSession.dialogType)
		}
		dialogSession.userInputActive = false
		dialogSession.inputAnswer = ""
		dialogSession.inputAnswerCursorIdx = 0
		return events.UpdateViewEvent{Type: events.UpdateViewRefresh, Model: &workingModel}
	default:
		changedAnswer := ""
		answerLen := utf8.RuneCountInString(dialogSession.inputAnswer)
		runeIdx := 0
		for _, runeVal := range dialogSession.inputAnswer {
			if runeIdx == dialogSession.inputAnswerCursorIdx {
				changedAnswer = changedAnswer + key.ConvertKeyCodeStringToString(keyCode)
			}
			changedAnswer = changedAnswer + string(runeVal)
			runeIdx = runeIdx + 1
		}
		if answerLen == runeIdx && runeIdx == dialogSession.inputAnswerCursorIdx {
			changedAnswer = changedAnswer + key.ConvertKeyCodeStringToString(keyCode)
		}
		dialogSession.inputAnswer = changedAnswer
		dialogSession.inputAnswerCursorIdx = dialogSession.inputAnswerCursorIdx + 1
	}

	answer := dialogSession.inputAnswer
	if dialogSession.inputAnswerMasked {
		answerLen := utf8.RuneCountInString(dialogSession.inputAnswer)
		answer = strings.Repeat("*", answerLen)
	}
	return events.UpdateViewEvent{
		Type:                  events.UpdateViewShowDialog,
		DialogQuestion:        dialogSession.inputQuestion,
		DialogAnswer:          answer,
		DialogAnswerCursorIdx: dialogSession.inputAnswerCursorIdx}
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
				return dpMissingPasswordError
			}
		}
	}

	switch item.Config.Type {
	case model.ItemDpConfiguration, model.ItemDpDomain, model.ItemDpFilestore, model.ItemDirectory, model.ItemNone:
		itemList, err := r.GetList(item.Config)
		if err != nil {
			return err
		}
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

func setCurrentDpPlainPassword(password string) {
	item := workingModel.CurrItem()
	applianceName := item.Config.DpAppliance
	logging.LogDebugf("worker/setDpPlainPassword() applicanceName: '%s'", applianceName)
	applicanceConfig := config.Conf.DataPowerAppliances[applianceName]
	logging.LogDebugf("worker/setDpPlainPassword() applicanceConfig before: '%s'", applicanceConfig)
	applicanceConfig.SetDpPlaintextPassword(password)
	config.Conf.DataPowerAppliances[applianceName] = applicanceConfig
	logging.LogDebugf("worker/setDpPlainPassword() applicanceConfig after : '%s'", applicanceConfig)
}

func setScreenSize() {
	_, height := out.GetScreenSize()
	workingModel.ItemMaxRows = height - 3
}

func viewCurrent(m *model.Model) error {
	ci := m.CurrItem()
	logging.LogDebugf("worker/viewCurrent(), item: %v", ci)
	var err error
	switch ci.Config.Type {
	case model.ItemFile:

		currView := workingModel.ViewConfig(workingModel.CurrSide())
		fileContent, err := repos[m.CurrSide()].GetFile(currView, ci.Name)
		if err != nil {
			return err
		}
		if fileContent != nil {
			err = extprogs.View(ci.Name, fileContent)
			if err != nil {
				return err
			}
		}

		return errs.Error(fmt.Sprintf("Can't fetch file '%s' from path '%s'.", ci.Name, m.CurrPath()))
	case model.ItemDpConfiguration:
		err = extprogs.View(ci.Name, config.Conf.GetDpApplianceConfig(ci.Name))
	}

	return err
}
