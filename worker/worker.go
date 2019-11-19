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

const (
	askDpPassword = userDialogType("askDpPassword")
	askFilter     = userDialogType("askFilter")
	askSearchNext = userDialogType("askSearchNext")
	askSearchPrev = userDialogType("askSearchPrev")
)

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

// updateViewEventChan is channel for sending view update events to refresh screen.
var updateViewEventChan chan events.UpdateViewEvent

// Dialog with user - question asked, user's answer and active state.
var dialogSession = userDialogInputSessionInfo{}

var quitting = false

// IsQuitting checks if application is currently quitting.
func IsQuitting() bool { return quitting }

// Init initializes DataPower and local filesystem access and load initial views.
func Init(uveChan chan events.UpdateViewEvent) {
	logging.LogDebug("worker/Init()")
	updateViewEventChan = uveChan
	runWorkerInit()
}

func runWorkerInit() {
	logging.LogDebug("worker/runWorkerInit()")
	dp.InitNetworkSettings()
	initialLoad(updateViewEventChan)
}

func initialLoadRepo(side model.Side, repo repo.Repo) {
	logging.LogDebugf("worker/initialLoadRepo(%v, %v)", side, repo)
	initialItem := repo.GetInitialItem()
	logging.LogDebugf("worker/initialLoadRepo(%v, %v), initialItem: %v", side, repo, initialItem)

	title := repo.GetTitle(initialItem.Config)
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

func ProcessInputEvent(keyCode key.KeyCode) {
	logging.LogDebugf("worker/ProcessInputEvent('%s')", keyCode)

	setScreenSize()

	if dialogSession.userInputActive {
		updateViewEvent := processInputDialogInput(&dialogSession, keyCode)
		updateViewEventChan <- updateViewEvent
	} else {
		switch keyCode {
		case key.Chq:
			quitting = true
			return
		case key.Tab:
			workingModel.ToggleSide()
		case key.Return:
			err := enterCurrentDirectory()
			logging.LogDebug("worker/processInputEvent(), err: ", err)
			switch err {
			case dpMissingPasswordError:
				updateView := showInputDialog(&dialogSession, askDpPassword, "Please enter DataPower password: ", "", true)
				updateViewEventChan <- updateView
				return
			case nil:
			default:
				updateStatus(err.Error())
				switch err.(type) {
				case errs.UnexpectedHTTPResponse:
					setCurrentDpPlainPassword("")
				}
				return
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
			return
		case key.Slash:
			workingModel.SearchBy = ""
			updateView := showInputDialog(&dialogSession, askSearchNext, "Search by: ", workingModel.SearchBy, false)
			updateViewEventChan <- updateView
			return
		case key.Chn, key.Chp:
			if workingModel.SearchBy == "" {
				var dialogType userDialogType
				switch keyCode {
				case key.Chn:
					dialogType = askSearchNext
				case key.Chp:
					dialogType = askSearchPrev
				}
				updateView := showInputDialog(&dialogSession, dialogType, "Search by: ", workingModel.SearchBy, false)
				updateViewEventChan <- updateView
				return
			}

			var found bool
			switch keyCode {
			case key.Chn:
				found = workingModel.SearchNext(workingModel.SearchBy)
			case key.Chp:
				found = workingModel.SearchPrev(workingModel.SearchBy)
			}
			if !found {
				notFoundStatus := fmt.Sprintf("Item '%s' not found.", workingModel.SearchBy)
				updateStatus(notFoundStatus)
				return
			}

		case key.F2, key.Ch2:
			repo := repos[workingModel.CurrSide()]
			repo.InvalidateCache()
			repo.GetList(workingModel.ViewConfig(workingModel.CurrSide()))
			refreshedStatus := "Current directory refreshed."
			updateStatus(refreshedStatus)

		case key.F3, key.Ch3:
			err := viewCurrent(&workingModel)
			if err != nil {
				updateStatus(err.Error())
				return
			}

		case key.F4, key.Ch4:
			err := editCurrent(&workingModel)
			if err != nil {
				updateStatus(err.Error())
				return
			}

		case key.F5, key.Ch5:
			err := copyCurrent(&workingModel)
			if err != nil {
				updateStatus(err.Error())
				return
			}
		}

		updateViewEventChan <- events.UpdateViewEvent{Type: events.UpdateViewRefresh, Model: &workingModel}
	}
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
	item := workingModel.CurrItem()
	return showItem(item.Config, item.Name)
}

func showItem(itemConfig *model.ItemConfig, itemName string) error {
	logging.LogDebug("worker/enterCurrentDirectory()")
	r := repos[workingModel.CurrSide()]
	logging.LogDebug("worker/enterCurrentDirectory(), itemConfig: ", itemConfig)
	if itemConfig.Type == model.ItemDpConfiguration {
		applianceName := itemName
		if applianceName != ".." {
			applicanceConfig := config.Conf.DataPowerAppliances[applianceName]
			logging.LogDebugf("worker/enterCurrentDirectory(), applicanceConfig: '%s'", applicanceConfig)
			if applicanceConfig.Password == "" {
				return dpMissingPasswordError
			}
		}
	}

	switch itemConfig.Type {
	case model.ItemDpConfiguration, model.ItemDpDomain, model.ItemDpFilestore, model.ItemDirectory, model.ItemNone:
		itemList, err := r.GetList(itemConfig)
		if err != nil {
			return err
		}
		logging.LogDebug("worker/enterCurrentDirectory(), itemList: ", itemList)
		title := r.GetTitle(itemConfig)
		logging.LogDebug("worker/enterCurrentDirectory(), title: ", title)

		oldViewConfig := workingModel.ViewConfig(workingModel.CurrSide())
		workingModel.SetItems(workingModel.CurrSide(), itemList)
		workingModel.SetCurrentView(workingModel.CurrSide(), itemConfig, title)
		if itemName != ".." {
			workingModel.NavTop()
		} else {
			workingModel.SetCurrItemForSideAndConfig(workingModel.CurrSide(), oldViewConfig)
		}
	default:
		logging.LogDebug("worker/enterCurrentDirectory(), unknown type: ", itemConfig.Type)
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
	case model.ItemDpConfiguration:
		fileContent, err := config.Conf.GetDpApplianceConfig(ci.Name)
		if err != nil {
			return err
		}
		err = extprogs.View(ci.Name, fileContent)
		if err != nil {
			return err
		}
	}

	return err
}

func editCurrent(m *model.Model) error {
	ci := m.CurrItem()
	logging.LogDebugf("worker/editCurrent(), item: %v", ci)
	var err error
	switch ci.Config.Type {
	case model.ItemFile:
		currView := workingModel.ViewConfig(workingModel.CurrSide())
		fileContent, err := repos[m.CurrSide()].GetFile(currView, ci.Name)
		if err != nil {
			return err
		}
		err, changed, newFileContent := extprogs.Edit(m.CurrItem().Name, fileContent)
		if err != nil {
			return err
		}
		if changed {
			_, err := repos[m.CurrSide()].UpdateFile(currView, ci.Name, newFileContent)
			if err != nil {
				return err
			}
			showItem(currView, ".")
		}

	case model.ItemDpConfiguration:
		fileContent, err := config.Conf.GetDpApplianceConfig(ci.Name)
		if err != nil {
			return err
		}
		err, changed, newFileContent := extprogs.Edit(m.CurrItem().Name, fileContent)
		if err != nil {
			return err
		}
		if changed {
			err := config.Conf.SetDpApplianceConfig(ci.Name, newFileContent)
			if err != nil {
				return err
			}
		}
	}

	return err
}

func copyCurrent(m *model.Model) error {
	fromSide := m.CurrSide()
	toSide := m.OtherSide()

	fromViewConfig := m.ViewConfig(fromSide)
	toViewConfig := m.ViewConfig(toSide)

	itemsToCopy := getSelectedOrCurrent(m)
	statusMsg := fmt.Sprintf("Copy from '%s' to '%s', items: %v", fromViewConfig, toViewConfig, itemsToCopy)
	updateStatus(statusMsg)

	confirmOverwrite := "n"
	for _, item := range itemsToCopy {
		confirmOverwrite, err := copyItem(repos[fromSide], repos[toSide], fromViewConfig, toViewConfig, item, confirmOverwrite)
		logging.LogDebugf("TODO: confirm overwrite: '%s'", confirmOverwrite)
		if err != nil {
			return err
		}
	}

	return showItem(m.ViewConfig(toSide), ".")
}

func getSelectedOrCurrent(m *model.Model) []model.Item {
	selectedItems := m.GetSelectedItems(m.CurrSide())
	if len(selectedItems) == 0 {
		selectedItems = make([]model.Item, 0)
		selectedItems = append(selectedItems, *m.CurrItem())
	}

	return selectedItems
}

func copyItem(fromRepo, toRepo repo.Repo, fromViewConfig, toViewConfig *model.ItemConfig, item model.Item, confirmOverwrite string) (string, error) {
	logging.LogDebugf("worker/copyItem(.., .., %v, %v, %v, %v)", fromViewConfig, toViewConfig, item, confirmOverwrite)
	switch item.Config.Type {
	case model.ItemDirectory:
		confirmOverwrite, err := copyDirs(fromRepo, toRepo, fromViewConfig, toViewConfig, item.Name, confirmOverwrite)
		if err != nil {
			return confirmOverwrite, err
		}
	case model.ItemFile:
		confirmOverwrite, err := copyFile(fromRepo, toRepo, fromViewConfig, toViewConfig, item.Name, confirmOverwrite)
		if err != nil {
			return confirmOverwrite, err
		}
	}

	return confirmOverwrite, nil
}

func copyDirs(fromRepo, toRepo repo.Repo, fromViewConfig, toViewConfig *model.ItemConfig, dirName, confirmOverwrite string) (string, error) {
	logging.LogDebugf("worker/copyDirs(.., .., %v, %v, '%s', %v)", fromViewConfig, toViewConfig, dirName, confirmOverwrite)
	toPath := toRepo.GetFilePath(toViewConfig.Path, dirName)
	createDirSuccess := true
	toFileType, err := toRepo.GetFileType(toViewConfig, toViewConfig.Path, dirName)
	if err != nil {
		return confirmOverwrite, err
	}
	switch toFileType {
	case model.ItemNone:
		createDirSuccess, err = toRepo.CreateDir(toViewConfig, toViewConfig.Path, dirName)
		if err != nil {
			return confirmOverwrite, err
		}
	case model.ItemDirectory:
	default:
		errMsg := fmt.Sprintf("Non dir '%s' exists (%v), can't create dir.", toPath, toFileType)
		return confirmOverwrite, errs.Error(errMsg)
	}

	var currentStatus string
	if createDirSuccess {
		currentStatus = fmt.Sprintf("Directory '%s' created.", toPath)
		updateStatus(currentStatus)
		items, err := fromRepo.GetList(fromViewConfig)
		if err != nil {
			return confirmOverwrite, err
		}
		for _, item := range items {
			confirmOverwrite, err = copyItem(fromRepo, toRepo, fromViewConfig, toViewConfig, item, confirmOverwrite)
		}
	} else {
		errMsg := fmt.Sprintf("ERROR: Directory '%s' not created.", toPath)
		updateStatus(errMsg)
		err = errs.Error(errMsg)
	}

	return confirmOverwrite, err
}

func copyFile(fromRepo, toRepo repo.Repo, fromViewConfig, toViewConfig *model.ItemConfig, fileName, confirmOverwrite string) (string, error) {
	logging.LogDebugf("worker/copyFile(.., .., .., %v, %v, %v, '%s')\n\n", fromViewConfig, toViewConfig, fileName, confirmOverwrite)
	targetFileType, err := toRepo.GetFileType(toViewConfig, toViewConfig.Path, fileName)
	if err != nil {
		return confirmOverwrite, err
	}
	logging.LogDebug(fmt.Sprintf("view targetFileType: %s\n", string(targetFileType)))

	switch targetFileType {
	case model.ItemDirectory:
		copyFileToDirStatus := fmt.Sprintf("ERROR: File '%s' could not be copied from '%s' to '%s' - directory with same name exists.", fileName, fromViewConfig.Path, toViewConfig.Path)
		updateStatus(copyFileToDirStatus)
	case model.ItemFile:
		if confirmOverwrite != "ya" && confirmOverwrite != "na" {
			logging.LogDebugf("TODO: confirm overwrite: '%s'", confirmOverwrite)
			// confirmOverwrite = userInput(m, fmt.Sprintf("Confirm overwrite of file '%s' at '%s' (y/ya/n/na): ", fileName, toParentPath), "")
		}
	}
	switch targetFileType {
	case model.ItemFile, model.ItemNone:
		fBytes, err := fromRepo.GetFile(fromViewConfig, fileName)
		if err != nil {
			return confirmOverwrite, err
		}
		copySuccess, err := toRepo.UpdateFile(toViewConfig, fileName, fBytes)
		if err != nil {
			return confirmOverwrite, err
		}
		logging.LogDebugf("view copySuccess: %v", copySuccess)
		if copySuccess {
			copySuccessStatus := fmt.Sprintf("File '%s' copied from '%s' to '%s'.", fileName, fromViewConfig.Path, toViewConfig.Path)
			updateStatus(copySuccessStatus)
		} else {
			copyErrStatus := fmt.Sprintf("ERROR: File '%s' not copied from '%s' to '%s'.", fileName, fromViewConfig.Path, toViewConfig.Path)
			updateStatus(copyErrStatus)
		}
	}
	return confirmOverwrite, nil
}

func updateStatus(status string) {
	updateView := events.UpdateViewEvent{
		Type: events.UpdateViewShowStatus, Status: status, Model: &workingModel}
	updateViewEventChan <- updateView
}
