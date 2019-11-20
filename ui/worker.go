package ui

import (
	"fmt"
	"github.com/croz-ltd/dpcmder/config"
	"github.com/croz-ltd/dpcmder/events"
	"github.com/croz-ltd/dpcmder/extprogs"
	"github.com/croz-ltd/dpcmder/help"
	"github.com/croz-ltd/dpcmder/model"
	"github.com/croz-ltd/dpcmder/repo"
	"github.com/croz-ltd/dpcmder/repo/dp"
	"github.com/croz-ltd/dpcmder/repo/localfs"
	"github.com/croz-ltd/dpcmder/ui/key"
	"github.com/croz-ltd/dpcmder/ui/out"
	"github.com/croz-ltd/dpcmder/utils/errs"
	"github.com/croz-ltd/dpcmder/utils/logging"
	"strings"
	"unicode/utf8"
)

// DpMissingPasswordError is constant error returned if DataPower password is
// not set and we want to connect to appliance.
const dpMissingPasswordError = errs.Error("DpMissingPasswordError")

// userDialogInputSessionInfo is structure containing all information neccessary
// for user entering information into input dialog.
type userDialogInputSessionInfo struct {
	inputQuestion        string
	inputAnswer          string
	inputAnswerCursorIdx int
	inputAnswerMasked    bool
	dialogCanceled       bool
	dialogSubmitted      bool
}

// userDialogResult is structure containing result of user input dialog.
type userDialogResult struct {
	inputAnswer     string
	dialogCanceled  bool
	dialogSubmitted bool
}

func (ud userDialogInputSessionInfo) String() string {
	return fmt.Sprintf("Session(q: '%s', a: '%s', cur: %d, masked: %T, c/s: %T/%T)",
		ud.inputQuestion, ud.inputAnswer,
		ud.inputAnswerCursorIdx, ud.inputAnswerMasked,
		ud.dialogCanceled, ud.dialogSubmitted)
}

// repos contains references to DataPower and local filesystem repositories.
var repos = []repo.Repo{model.Left: &dp.Repo, model.Right: &localfs.Repo}

// workingModel contains Model with all information on current DataPower and
// local filesystem we are showing in dpcmder.
var workingModel model.Model = model.Model{} //{currSide: model.Left}

// Dialog with user - question asked, user's answer and active state.
var dialogSession = userDialogInputSessionInfo{}

// InitialLoad initializes DataPower and local filesystem access and load initial views.
func InitialLoad() {
	logging.LogDebug("ui/InitialLoad()")
	dp.InitNetworkSettings()
	initialLoadDp()
	initialLoadLocalfs()

	setScreenSize()
	out.DrawEvent(events.UpdateViewEvent{Type: events.UpdateViewRefresh, Model: &workingModel})
}

func initialLoadRepo(side model.Side, repo repo.Repo) {
	logging.LogDebugf("ui/initialLoadRepo(%v, %v)", side, repo)
	initialItem := repo.GetInitialItem()
	logging.LogDebugf("ui/initialLoadRepo(%v, %v), initialItem: %v", side, repo, initialItem)

	title := repo.GetTitle(initialItem.Config)
	workingModel.SetCurrentView(side, initialItem.Config, title)

	itemList, err := repo.GetList(initialItem.Config)
	if err != nil {
		logging.LogDebug("ui/initialLoadRepo(): ", err)
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

const QuitError = errs.Error("QuitError")

func ProcessInputEvent(keyCode key.KeyCode) error {
	logging.LogDebugf("ui/ProcessInputEvent('%s')", keyCode)

	setScreenSize()

	switch keyCode {
	case key.Chq:
		return QuitError
	case key.Tab:
		workingModel.ToggleSide()
	case key.Return:
		err := enterCurrentDirectory()
		logging.LogDebug("ui/processInputEvent(), err: ", err)
		switch err {
		case dpMissingPasswordError:
			dialogResult := askUserInput("Please enter DataPower password: ", "", true)
			if dialogResult.inputAnswer != "" {
				setCurrentDpPlainPassword(dialogResult.inputAnswer)
			}
		case nil:
			// If no error occurs.
		default:
			updateStatus(err.Error())
			switch err.(type) {
			case errs.UnexpectedHTTPResponse:
				setCurrentDpPlainPassword("")
			}
			return nil
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
		dialogResult := askUserInput("Filter by: ", cf, false)
		workingModel.SetCurrentFilter(dialogResult.inputAnswer)

	case key.Slash:
		workingModel.SearchBy = ""
		dialogResult := askUserInput("Search by: ", workingModel.SearchBy, false)
		workingModel.SearchBy = dialogResult.inputAnswer
		if workingModel.SearchBy != "" {
			found := workingModel.SearchNext(workingModel.SearchBy)
			if !found {
				notFoundStatus := fmt.Sprintf("Item '%s' not found.", workingModel.SearchBy)
				updateStatus(notFoundStatus)
			}
		}

	case key.Chn, key.Chp:
		if workingModel.SearchBy == "" {
			dialogResult := askUserInput("Search by: ", workingModel.SearchBy, false)
			workingModel.SearchBy = dialogResult.inputAnswer
		}
		if workingModel.SearchBy != "" {
			found := false
			switch keyCode {
			case key.Chn:
				found = workingModel.SearchNext(workingModel.SearchBy)
			case key.Chp:
				found = workingModel.SearchPrev(workingModel.SearchBy)
			}
			if !found {
				notFoundStatus := fmt.Sprintf("Item '%s' not found.", workingModel.SearchBy)
				updateStatus(notFoundStatus)
			}
		}

	case key.F2, key.Ch2:
		repo := repos[workingModel.CurrSide()]
		repo.InvalidateCache()
		viewConfig := workingModel.ViewConfig(workingModel.CurrSide())
		showItem(workingModel.CurrSide(), viewConfig, ".")
		updateStatusf("Current directory (%s) refreshed.", viewConfig.Path)

	case key.F3, key.Ch3:
		err := viewCurrent(&workingModel)
		if err != nil {
			updateStatus(err.Error())
		}

	case key.F4, key.Ch4:
		err := editCurrent(&workingModel)
		if err != nil {
			updateStatus(err.Error())
		}

	case key.F5, key.Ch5:
		err := copyCurrent(&workingModel)
		if err != nil {
			updateStatus(err.Error())
		}

	// case key.Chd:
	// 	diffCurrent(m)
	case key.F7, key.Ch7:
		err := createDirectory(&workingModel)
		if err != nil {
			updateStatus(err.Error())
		}
	case key.F8, key.Ch8:
		err := createEmptyFile(&workingModel)
		if err != nil {
			updateStatus(err.Error())
		}

	case key.Del, key.Chx:
		err := deleteCurrent(&workingModel)
		if err != nil {
			updateStatus(err.Error())
		}

		// case key.Chs:
		// 	syncModeToggle(m)
	case key.Chm:
		err := showStatusMessages(workingModel.Statuses())
		if err != nil {
			updateStatus(err.Error())
		}

	default:
		help.Show()
		updateStatusf("Key pressed hex value (before showing help): '%s'", keyCode)
	}

	out.DrawEvent(events.UpdateViewEvent{Type: events.UpdateViewRefresh, Model: &workingModel})

	return nil
}

func showInputDialog(dialogSession *userDialogInputSessionInfo) events.UpdateViewEvent {
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

func askUserInput(question, answer string, answerMasked bool) userDialogResult {
	dialogSession := userDialogInputSessionInfo{inputQuestion: question,
		inputAnswer:          answer,
		inputAnswerCursorIdx: utf8.RuneCountInString(answer),
		inputAnswerMasked:    answerMasked}
loop:
	for {
		updateViewEvent := showInputDialog(&dialogSession)
		out.DrawEvent(updateViewEvent)
		keyCode, err := kcr.readNext()
		if err != nil {
			updateStatus(err.Error())
			break loop
		}
		processInputDialogInput(&dialogSession, keyCode)
		if dialogSession.dialogCanceled || dialogSession.dialogSubmitted {
			break loop
		}
	}
	return userDialogResult{inputAnswer: dialogSession.inputAnswer,
		dialogCanceled:  dialogSession.dialogCanceled,
		dialogSubmitted: dialogSession.dialogSubmitted}
}
func processInputDialogInput(dialogSession *userDialogInputSessionInfo, keyCode key.KeyCode) {
	logging.LogDebugf("ui/processInputDialogInput(): '%s'", dialogSession)
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
		logging.LogDebug("ui/processInputDialogInput() canceling user input: '%s'", dialogSession)
		dialogSession.dialogCanceled = true
	case key.Return:
		logging.LogDebugf("ui/processInputDialogInput() accepting user input: '%s'", dialogSession)
		dialogSession.dialogSubmitted = true
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
}

func enterCurrentDirectory() error {
	item := workingModel.CurrItem()
	return showItem(workingModel.CurrSide(), item.Config, item.Name)
}

func showItem(side model.Side, itemConfig *model.ItemConfig, itemName string) error {
	logging.LogDebugf("ui/showItem(%d, .., '%s')", side, itemName)
	r := repos[side]
	logging.LogDebug("ui/showItem(), itemConfig: ", itemConfig)
	if itemConfig.Type == model.ItemDpConfiguration {
		applianceName := itemName
		if applianceName != ".." {
			applicanceConfig := config.Conf.DataPowerAppliances[applianceName]
			logging.LogDebugf("ui/showItem(), applicanceConfig: '%s'", applicanceConfig)
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
		logging.LogDebug("ui/showItem(), itemList: ", itemList)
		title := r.GetTitle(itemConfig)
		logging.LogDebug("ui/showItem(), title: ", title)

		oldViewConfig := workingModel.ViewConfig(side)
		workingModel.SetItems(side, itemList)
		workingModel.SetCurrentView(side, itemConfig, title)
		if itemName != ".." {
			workingModel.NavTop()
		} else {
			workingModel.SetCurrItemForSideAndConfig(side, oldViewConfig)
		}
	default:
		logging.LogDebug("ui/showItem(), unknown type: ", itemConfig.Type)
	}

	return nil
}

func setCurrentDpPlainPassword(password string) {
	item := workingModel.CurrItem()
	applianceName := item.Config.DpAppliance
	logging.LogDebugf("ui/setDpPlainPassword() applicanceName: '%s'", applianceName)
	applicanceConfig := config.Conf.DataPowerAppliances[applianceName]
	logging.LogDebugf("ui/setDpPlainPassword() applicanceConfig before: '%s'", applicanceConfig)
	applicanceConfig.SetDpPlaintextPassword(password)
	config.Conf.DataPowerAppliances[applianceName] = applicanceConfig
	logging.LogDebugf("ui/setDpPlainPassword() applicanceConfig after : '%s'", applicanceConfig)
}

func setScreenSize() {
	_, height := out.GetScreenSize()
	workingModel.ItemMaxRows = height - 3
}

func viewCurrent(m *model.Model) error {
	ci := m.CurrItem()
	logging.LogDebugf("ui/viewCurrent(), item: %v", ci)
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
	logging.LogDebugf("ui/editCurrent(), item: %v", ci)
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
			showItem(workingModel.CurrSide(), currView, ".")
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
	itemsDisplayToCopy := make([]string, len(itemsToCopy))
	for idx, item := range itemsToCopy {
		itemsDisplayToCopy[idx] = item.DisplayString()
	}
	updateStatusf("Copy from '%s' to '%s', items: %v", fromViewConfig.Path, toViewConfig.Path, itemsDisplayToCopy)

	var confirmOverwrite = "n"
	var err error
	for _, item := range itemsToCopy {
		confirmOverwrite, err = copyItem(repos[fromSide], repos[toSide], fromViewConfig, toViewConfig, item, confirmOverwrite)
		if err != nil {
			return err
		}
		if confirmOverwrite == "na" {
			break
		}
	}

	return showItem(toSide, m.ViewConfig(toSide), ".")
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
	logging.LogDebugf("ui/copyItem(.., .., %v, %v, %v, '%s')", fromViewConfig, toViewConfig, item, confirmOverwrite)
	res := confirmOverwrite
	var err error
	switch item.Config.Type {
	case model.ItemDirectory:
		res, err = copyDirs(fromRepo, toRepo, fromViewConfig, toViewConfig, item.Name, confirmOverwrite)
		if err != nil {
			return res, err
		}
	case model.ItemFile:
		res, err = copyFile(fromRepo, toRepo, fromViewConfig, toViewConfig, item.Name, confirmOverwrite)
		logging.LogDebugf("ui/copyItem(), res from copyFile(): '%s'", res)
		if err != nil {
			return res, err
		}
	default:
		updateStatusf("Only files and directories can be copied.")
	}

	logging.LogDebugf("ui/copyItem(), res: '%s'", res)
	return res, nil
}

func copyDirs(fromRepo, toRepo repo.Repo, fromViewConfig, toViewConfig *model.ItemConfig, dirName, confirmOverwrite string) (string, error) {
	logging.LogDebugf("ui/copyDirs(.., .., %v, %v, '%s', '%s')", fromViewConfig, toViewConfig, dirName, confirmOverwrite)
	toParentPath := toViewConfig.Path
	toFileType, err := toRepo.GetFileType(toViewConfig, toParentPath, dirName)
	if err != nil {
		return confirmOverwrite, err
	}
	toPath := toRepo.GetFilePath(toParentPath, dirName)
	switch toFileType {
	case model.ItemNone:
		_, err = toRepo.CreateDir(toViewConfig, toParentPath, dirName)
		if err != nil {
			logging.LogDebugf("ui/copyDirs() - err: %v", err)
			return confirmOverwrite, err
		}
		updateStatusf("Directory '%s' created.", toPath)
	case model.ItemDirectory:
		updateStatusf("Directory '%s' already exists.", toPath)
	default:
		errMsg := fmt.Sprintf("Non dir '%s' exists (%v), can't create dir.", toPath, toFileType)
		logging.LogDebugf("ui/copyDirs() - %s", errMsg)
		return confirmOverwrite, errs.Error(errMsg)
	}

	fromViewConfigDir := model.ItemConfig{Parent: fromViewConfig,
		Path:        fromRepo.GetFilePath(fromViewConfig.Path, dirName),
		DpAppliance: fromViewConfig.DpAppliance,
		DpDomain:    fromViewConfig.DpDomain,
		DpFilestore: fromViewConfig.DpFilestore}
	items, err := fromRepo.GetList(&fromViewConfigDir)
	if err != nil {
		return confirmOverwrite, err
	}
	for _, item := range items {
		if item.Name != ".." {
			toViewConfigDir := model.ItemConfig{Parent: toViewConfig,
				Path:        toRepo.GetFilePath(toViewConfig.Path, dirName),
				DpAppliance: toViewConfig.DpAppliance,
				DpDomain:    toViewConfig.DpDomain,
				DpFilestore: toViewConfig.DpFilestore}
			confirmOverwrite, err = copyItem(fromRepo, toRepo, &fromViewConfigDir, &toViewConfigDir, item, confirmOverwrite)
			if err != nil {
				return confirmOverwrite, err
			}
		}
	}

	return confirmOverwrite, err
}

func copyFile(fromRepo, toRepo repo.Repo, fromViewConfig, toViewConfig *model.ItemConfig, fileName, confirmOverwrite string) (string, error) {
	logging.LogDebugf("ui/copyFile(.., .., .., %v, %v, %v, '%s')", fromViewConfig, toViewConfig, fileName, confirmOverwrite)
	res := confirmOverwrite
	targetFileType, err := toRepo.GetFileType(toViewConfig, toViewConfig.Path, fileName)
	if err != nil {
		return res, err
	}

	switch targetFileType {
	case model.ItemDirectory:
		copyFileToDirStatus := fmt.Sprintf("ERROR: File '%s' could not be copied from '%s' to '%s' - directory with same name exists.", fileName, fromViewConfig.Path, toViewConfig.Path)
		updateStatus(copyFileToDirStatus)
	case model.ItemFile:
		if res != "ya" && res != "na" {
			logging.LogDebugf("TODO: confirm overwrite: '%s'", res)
			dialogResult := askUserInput(
				fmt.Sprintf("Confirm overwrite of file '%s' at '%s' (y/ya/n/na): ",
					fileName, toViewConfig.Path), "", false)
			if dialogResult.dialogSubmitted {
				res = dialogResult.inputAnswer
			}
			// confirmOverwrite = userInput(m, fmt.Sprintf("Confirm overwrite of file '%s' at '%s' (y/ya/n/na): ", fileName, toParentPath), "")
		}
	case model.ItemNone:
		res = "y"
	}
	if res == "y" || res == "ya" {
		switch targetFileType {
		case model.ItemFile, model.ItemNone:
			fBytes, err := fromRepo.GetFile(fromViewConfig, fileName)
			if err != nil {
				return res, err
			}
			copySuccess, err := toRepo.UpdateFile(toViewConfig, fileName, fBytes)
			if err != nil {
				return res, err
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
	} else {
		updateStatusf("Canceled overwrite of '%s'", fileName)
	}

	logging.LogDebugf("ui/copyFile(), res: '%s'", res)
	return res, nil
}

func createEmptyFile(m *model.Model) error {
	logging.LogDebugf("ui/createEmptyFile()")
	dialogResult := askUserInput("Enter file name for file to create: ", "", false)
	if dialogResult.dialogSubmitted {
		fileName := dialogResult.inputAnswer
		side := m.CurrSide()
		viewConfig := m.ViewConfig(side)
		r := repos[side]
		targetFileType, err := r.GetFileType(viewConfig, viewConfig.Path, fileName)
		if err != nil {
			return err
		}
		if targetFileType != model.ItemNone {
			return errs.Errorf("File with name '%s' already exists at '%s'.", fileName, viewConfig.Path)
		}
		_, err = r.UpdateFile(viewConfig, fileName, make([]byte, 0))
		if err != nil {
			return errs.Errorf("Can't create file with name '%s' - '%v'.", fileName, err)
		}

		updateStatusf("File '%s' created.", fileName)
		return showItem(side, viewConfig, ".")
	}
	updateStatus("Creation of new file canceled.")
	return nil
}

func createDirectory(m *model.Model) error {
	logging.LogDebugf("ui/createDirectory()")
	dialogResult := askUserInput("Enter directory name for file to create: ", "", false)
	if dialogResult.dialogSubmitted {
		dirName := dialogResult.inputAnswer
		side := m.CurrSide()
		viewConfig := m.ViewConfig(side)
		r := repos[side]
		targetFileType, err := r.GetFileType(viewConfig, viewConfig.Path, dirName)
		if err != nil {
			return err
		}
		if targetFileType != model.ItemNone {
			return errs.Errorf("Directory with name '%s' already exists at '%s'.", dirName, viewConfig.Path)
		}
		_, err = r.CreateDir(viewConfig, viewConfig.Path, dirName)
		if err != nil {
			return errs.Errorf("Can't create directory with name '%s' - '%v'.", dirName, err)
		}

		updateStatusf("Directory '%s' created.", dirName)
		return showItem(side, viewConfig, ".")
	}
	updateStatus("Creation of new directory canceled.")
	return nil
}

func deleteCurrent(m *model.Model) error {
	selectedItems := getSelectedOrCurrent(m)

	side := m.CurrSide()
	viewConfig := m.ViewConfig(side)

	confirmResponse := "n"
	for _, item := range selectedItems {
		if confirmResponse != "ya" && confirmResponse != "na" {
			confirmResponse = "n"
			dialogResult := askUserInput(
				fmt.Sprintf("Confirm deletion of file '%s' at '%s' (y/ya/n/na): ",
					item.Name, viewConfig.Path), "", false)
			if dialogResult.dialogSubmitted {
				confirmResponse = dialogResult.inputAnswer
			}
		}
		if confirmResponse == "y" || confirmResponse == "ya" {
			res, err := repos[m.CurrSide()].Delete(viewConfig, viewConfig.Path, item.Name)
			if err != nil {
				updateStatus(err.Error())
			}
			if res {
				updateStatusf("Successfully deleted file '%s'.", item.Name)
			} else {
				updateStatusf("ERROR: couldn't delete file '%s'.", item.Name)
			}
			showItem(side, viewConfig, item.Name)
		} else {
			updateStatusf("Cancelete deleting of file '%s'.", item.Name)
		}
	}

	return nil
}

func showStatusMessages(statuses []string) error {
	statusesText := ""
	for _, status := range statuses {
		statusesText = statusesText + status + "\n"
	}
	return extprogs.View("Status_Messages", []byte(statusesText))
}

func updateStatusf(format string, v ...interface{}) {
	status := fmt.Sprintf(format, v...)
	updateStatus(status)
}

func updateStatus(status string) {
	updateView := events.UpdateViewEvent{
		Type: events.UpdateViewShowStatus, Status: status, Model: &workingModel}
	out.DrawEvent(updateView)
	workingModel.AddStatus(status)
}
