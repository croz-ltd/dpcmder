package ui

import (
	"bytes"
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
	"time"
	"unicode/utf8"
)

// dpMissingPasswordError is constant error returned if DataPower password is
// not set and we want to connect to appliance.
const dpMissingPasswordError = errs.Error("dpMissingPasswordError")

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
	dp.Repo.InitNetworkSettings(config.CurrentAppliance)
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
		updateStatus(err.Error())
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

	var err error

	switch keyCode {
	case key.Chq:
		return QuitError
	case key.Tab:
		workingModel.ToggleSide()
	case key.Return:
		err = enterCurrentDirectory()
		logging.LogDebug("ui/processInputEvent(), err: ", err)
		switch err {
		case dpMissingPasswordError:
			dialogResult := askUserInput("Please enter DataPower password: ", "", true)
			if dialogResult.inputAnswer != "" {
				setCurrentDpPlainPassword(dialogResult.inputAnswer)
			}
			err = nil
		case nil:
			// If no error occurs.
		default:
			switch err.(type) {
			case errs.UnexpectedHTTPResponse:
				setCurrentDpPlainPassword("")
			}
		}
	case key.Space:
		workingModel.ToggleCurrItem()
	case key.Dot:
		err = enterDirectoryPath(&workingModel)
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
		err = refreshCurrentView(&workingModel)
	case key.F3, key.Ch3:
		err = viewCurrent(&workingModel)
	case key.F4, key.Ch4:
		err = editCurrent(&workingModel)
	case key.F5, key.Ch5:
		err = copyCurrent(&workingModel)
	case key.Chd:
		err = diffCurrent(&workingModel)
	case key.F7, key.Ch7:
		err = createDirectory(&workingModel)
	case key.F8, key.Ch8:
		err = createEmptyFile(&workingModel)
	case key.Del, key.Chx:
		err = deleteCurrent(&workingModel)
	case key.Chs:
		err = syncModeToggle(&workingModel)
	case key.Chm:
		err = showStatusMessages(workingModel.Statuses())

	default:
		help.Show()
		updateStatusf("Key pressed hex value (before showing help): '%s'", keyCode)
	}

	if err != nil {
		updateStatus(err.Error())
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
	if item == nil {
		return errs.Error("Nothing found, can't enter current directory.")
	}
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

func refreshView(m *model.Model, side model.Side) error {
	repo := repos[side]
	repo.InvalidateCache()
	viewConfig := workingModel.ViewConfig(side)
	err := showItem(side, viewConfig, ".")
	if err != nil {
		return err
	}
	updateStatusf("Directory (%s) refreshed.", m.ViewConfig(side).Path)
	return nil
}

func refreshCurrentView(m *model.Model) error {
	return refreshView(m, m.CurrSide())
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

func diffCurrent(m *model.Model) error {
	logging.LogDebug("ui/diffCurrent()")
	dpItem := m.CurrItemForSide(model.Left)
	localItem := m.CurrItemForSide(model.Right)

	dpView := m.ViewConfig(model.Left)
	localView := m.ViewConfig(model.Right)

	if dpItem.Config.Type == localItem.Config.Type {
		dpCopyDir := extprogs.CreateTempDir("dp")
		localViewTmp := model.ItemConfig{Type: model.ItemDirectory, Path: dpCopyDir}
		// func copyItem(fromRepo, toRepo repo.Repo, fromViewConfig, toViewConfig *model.ItemConfig, item model.Item, confirmOverwrite string) (string, error) {
		copyItem(&dp.Repo, localfs.Repo, dpView, &localViewTmp, *dpItem, "y")
		dpCopyItemPath := dp.Repo.GetFilePath(dpCopyDir, dpItem.Name)
		localItemPath := localfs.Repo.GetFilePath(localView.Path, localItem.Name)
		err := extprogs.Diff(dpCopyItemPath, localItemPath)
		if err != nil {
			logging.LogDebugf("ui/diffCurrent(), err: %v", err)
			return err
		}
		extprogs.DeleteTempDir(dpCopyDir)
		logging.LogDebug("view.diffCurrent() after DeleteTempDir")
	}
	err := errs.Errorf("Can't compare different file types '%s' (%s) to '%s' (%s)",
		dpItem.Name, string(dpItem.Config.Type), localItem.Name, string(localItem.Config.Type))
	logging.LogDebug(err)
	return err
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

	fromViewConfigDir := model.ItemConfig{
		Parent:      fromViewConfig,
		Type:        model.ItemDirectory,
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
			logging.LogDebugf("ui/copyFile(): %v", copySuccess)
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
	logging.LogDebug("ui/createDirectory()")
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
	logging.LogDebug("ui/deleteCurrent()")
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

func enterDirectoryPath(m *model.Model) error {
	logging.LogDebug("ui/enterDirectoryPath()")
	side := m.CurrSide()
	viewConfig := m.ViewConfig(side)

	if side == model.Left && viewConfig.DpDomain == "" {
		return errs.Error("Can't enter path if DataPower domain is not selected first.")
	}

	dialogResult := askUserInput("Enter path: ", viewConfig.Path, false)
	if dialogResult.dialogCanceled {
		return errs.Error("Entering path canceled.")
	}

	path := dialogResult.inputAnswer
	newViewConfig, err := repos[side].GetViewConfigByPath(viewConfig, path)
	if err != nil {
		logging.LogDebugf("ui/enterDirectoryPath(), err: %v", err)
		return err
	}

	switch newViewConfig.Type {
	case model.ItemDirectory, model.ItemDpFilestore, model.ItemDpDomain:
		updateStatusf("Showing path '%s'.", path)
		return showItem(side, newViewConfig, ".")
	default:
		return errs.Errorf("Can't show path '%s', not directory nor filestore nor domain.", path)
	}
}

// showStatusMessages shows history of status messages in viewer program.
func showStatusMessages(statuses []string) error {
	statusesText := ""
	for _, status := range statuses {
		statusesText = statusesText + status + "\n"
	}
	return extprogs.View("Status_Messages", []byte(statusesText))
}

func syncModeToggle(m *model.Model) error {
	logging.LogDebug("ui/syncModeToggle()")
	var syncModeToggleConfirm userDialogResult

	dpViewConfig := m.ViewConfig(model.Left)
	dpApplianceName := dpViewConfig.DpAppliance
	dpDomain := dpViewConfig.DpDomain
	dpDir := dpViewConfig.Path

	if m.SyncModeOn {
		syncModeToggleConfirm = askUserInput("Are you sure you want to disable sync mode (y/n): ", "", false)
	} else {
		if dpDomain != "" && dpDir != "" {
			syncModeToggleConfirm = askUserInput("Are you sure you want to enable sync mode (y/n): ", "", false)
		} else {
			return errs.Errorf("Can't sync if DataPower domain (%s) or path (%s) are not selected.", dpDomain, dpDir)
		}
	}

	if syncModeToggleConfirm.dialogSubmitted && syncModeToggleConfirm.inputAnswer == "y" {
		m.SyncModeOn = !m.SyncModeOn
		if m.SyncModeOn {
			dp.SyncRepo.InitNetworkSettings(config.Conf.DataPowerAppliances[dpApplianceName])
			m.SyncDpDomain = dpDomain
			m.SyncDirDp = dpDir
			m.SyncDirLocal = m.ViewConfig(model.Right).Path
			m.SyncInitial = true
			go syncLocalToDp(m)
			updateStatusf("Synchronization mode enabled (%s/'%s' <- '%s').", m.SyncDpDomain, m.SyncDirDp, m.SyncDirLocal)
		} else {
			m.SyncDpDomain = ""
			m.SyncDirDp = ""
			m.SyncDirLocal = ""
			m.SyncInitial = false
			updateStatus("Synchronization mode disabled.")
		}
	} else {
		updateStatus("Synchronization mode change canceled.")
	}

	return nil
}

func syncLocalToDp(m *model.Model) {
	// 1. Fetch dp & local file tree
	// 2. Initial sync files from local to dp:
	// 2a. Copy non-existing from local to dp
	// 2b. Compare existing files and copy different files
	// 3. Save local file tree (file path + modify timestamp)
	// 4. Sync files from local to dp:
	// 4a. When local modify timestamp changes or new file appears copy to dp
	cnt := 0
	var treeOld localfs.Tree
	syncCheckTime := time.Duration(config.Conf.Sync.Seconds) * time.Second
	for m.SyncModeOn {
		var changesMade bool
		tree, err := localfs.LoadTree("", m.SyncDirLocal)
		if err != nil {
			updateStatusf("Sync err: %s.", err)
		}
		logging.LogDebug("syncLocalToDp(), tree: ", tree)

		if m.SyncInitial {
			changesMade = syncLocalToDpInitial(&tree)
			logging.LogDebug("syncLocalToDp(), after initial sync - changesMade: ", changesMade)
			m.SyncInitial = false
		} else {
			changesMade = syncLocalToDpLater(&tree, &treeOld)
			logging.LogDebug("syncLocalToDp(), after later sync - changesMade: ", changesMade)
		}

		treeOld = tree

		cnt++
		if changesMade {
			refreshView(m, model.Left)
		}
		time.Sleep(syncCheckTime)
	}
}

func syncLocalToDpInitial(tree *localfs.Tree) bool {
	changesMade := false
	logging.LogDebug(fmt.Sprintf("syncLocalToDpInitial(%v)", tree))
	// m := &model.Model{}
	m := &workingModel
	// logging.LogDebug("syncLocalToDpInitial(), syncDirDp: ", syncDirDp, ", tree.PathFromRoot: ", tree.PathFromRoot)

	if tree.Dir {
		dpPath := dp.SyncRepo.GetFilePath(m.SyncDirDp, tree.PathFromRoot)
		fileType, err := dp.SyncRepo.GetFileTypeByPath(m.SyncDpDomain, dpPath, ".")
		if err != nil {
			logging.LogDebug("worker/syncLocalToDpInitial(), err: ", err)
		}

		if fileType == model.ItemNone {
			dp.SyncRepo.CreateDirByPath(m.SyncDpDomain, dpPath, ".")
			changesMade = true
		} else if fileType == model.ItemFile {
			logging.LogDebugf("worker/syncLocalToDpInitial() - In place of dir there is a file on dp: '%s'", dpPath)
		}
		for _, child := range tree.Children {
			if syncLocalToDpInitial(&child) {
				changesMade = true
			}
		}
	} else {
		changesMade = updateDpFile(m, tree)
	}

	return changesMade
}

func syncLocalToDpLater(tree, treeOld *localfs.Tree) bool {
	changesMade := false
	logging.LogDebugf("worker/syncLocalToDpLater(%v, %v)", tree, treeOld)
	// m := &model.Model{}
	m := &workingModel

	if tree.Dir {
		if treeOld == nil {
			dpPath := dp.SyncRepo.GetFilePath(m.SyncDirDp, tree.PathFromRoot)
			fileType, err := dp.SyncRepo.GetFileTypeByPath(m.SyncDpDomain, dpPath, ".")
			if err != nil {
				logging.LogDebug("worker/syncLocalToDpLater(), err: ", err)
				return false
			}
			if fileType == model.ItemNone {
				dp.SyncRepo.CreateDirByPath(m.SyncDpDomain, dpPath, ".")
				changesMade = true
			} else if fileType == model.ItemFile {
				logging.LogDebugf("worker/syncLocalToDpLater() - In place of dir there is a file on dp: '%s'", dpPath)
				return false
			}
		}

		for _, child := range tree.Children {
			var childOld *localfs.Tree
			if treeOld != nil {
				childOld = treeOld.FindChild(&child)
			}
			if syncLocalToDpLater(&child, childOld) {
				changesMade = true
			}
		}
	} else {
		if tree.FileChanged(treeOld) {
			changesMade = updateDpFile(m, tree)
		}
	}

	return changesMade
}

func updateDpFile(m *model.Model, tree *localfs.Tree) bool {
	changesMade := false
	localBytes, err := localfs.GetFileByPath(tree.Path)
	if err != nil {
		logging.LogDebug("worker/updateDpFile(), couldn't get local file - err: ", err)
		return false
	}
	dpPath := dp.SyncRepo.GetFilePath(m.SyncDirDp, tree.PathFromRoot)
	dpBytes, err := dp.SyncRepo.GetFileByPath(m.SyncDpDomain, dpPath)
	if err != nil {
		logging.LogDebug("worker/updateDpFile(), couldn't get local file - err: ", err)
		return false
	}

	if bytes.Compare(localBytes, dpBytes) != 0 {
		changesMade = true
		res, err := dp.SyncRepo.UpdateFileByPath(m.SyncDpDomain, dpPath, localBytes)
		if err != nil {
			logging.LogDebug("worker/updateDpFile(), couldn't get local file - err: ", err)
		}
		logging.LogDebugf("worker/updateDpFile(), file '%s' updated: %T", dpPath, res)
		if res {
			updateStatusf("Dp file '%s' updated.", dpPath)
		} else {
			updateStatusf("Error updating file '%s'.", dpPath)
		}
	}

	return changesMade
}

func updateStatusf(format string, v ...interface{}) {
	status := fmt.Sprintf(format, v...)
	updateStatus(status)
}

func updateStatus(status string) {
	logging.LogDebugf("worker/updateStatus('%s')", status)
	updateView := events.UpdateViewEvent{
		Type: events.UpdateViewShowStatus, Status: status, Model: &workingModel}
	out.DrawEvent(updateView)
	workingModel.AddStatus(status)
}
