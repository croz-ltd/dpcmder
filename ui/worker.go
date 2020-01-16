package ui

import (
	"bytes"
	"fmt"
	"github.com/croz-ltd/dpcmder/config"
	"github.com/croz-ltd/dpcmder/events"
	"github.com/croz-ltd/dpcmder/extprogs"
	"github.com/croz-ltd/dpcmder/model"
	"github.com/croz-ltd/dpcmder/repo"
	"github.com/croz-ltd/dpcmder/repo/dp"
	"github.com/croz-ltd/dpcmder/repo/localfs"
	"github.com/croz-ltd/dpcmder/ui/out"
	"github.com/croz-ltd/dpcmder/utils/errs"
	"github.com/croz-ltd/dpcmder/utils/logging"
	"github.com/gdamore/tcell"
	"strings"
	"time"
	"unicode/utf8"
)

// dpMissingPasswordError is constant error returned if DataPower password is
// not set and we want to connect to appliance.
const dpMissingPasswordError = errs.Error("dpMissingPasswordError")

// QuitError is constant used to recognize when user wants to quit dpcmder.
const QuitError = errs.Error("QuitError")

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

// listSelectionDialogSessionInfo is structure containing all information neccessary
// for user selection item from list selection dialog.
type listSelectionDialogSessionInfo struct {
	message         string
	list            []string
	selectionIdx    int
	dialogCanceled  bool
	dialogSubmitted bool
}

func (lsd listSelectionDialogSessionInfo) String() string {
	return fmt.Sprintf("Session(m: '%s', len(l): %d, idx: %d, c/s: %T/%T)",
		lsd.message, len(lsd.list),
		lsd.selectionIdx,
		lsd.dialogCanceled, lsd.dialogSubmitted)
}

// repos contains references to DataPower and local filesystem repositories.
var repos = []repo.Repo{model.Left: &dp.Repo, model.Right: &localfs.Repo}

// workingModel contains Model with all information on current DataPower and
// local filesystem we are showing in dpcmder.
var workingModel model.Model = model.Model{} //{currSide: model.Left}

// progressDialogInfo is structure containing all information needed to show
// progress dialog for long running actions.
type progressDialogInfo struct {
	visible       bool
	waitUserInput bool
	value         int
	msg           string
}

// progressDialogSession contains progress dialog info for long running actions.
var progressDialogSession = progressDialogInfo{}

// InitialLoad initializes DataPower and local filesystem access and load initial views.
func InitialLoad() {
	logging.LogDebug("ui/InitialLoad()")
	err := dp.Repo.InitNetworkSettings(config.CurrentAppliance)
	if err != nil {
		logging.LogDebug("ui/initialLoadRepo(): ", err)
		updateStatus(err.Error())
	}
	initialLoadDp()
	initialLoadLocalfs()

	setScreenSize()
	out.DrawEvent(events.UpdateViewEvent{Type: events.UpdateViewRefresh, Model: &workingModel})
	updateStatusf("Press 'h' key to show help.")
}

// initialLoadRepo loads initial view for given repo on given side.
func initialLoadRepo(side model.Side, repo repo.Repo) {
	logging.LogDebugf("ui/initialLoadRepo(%v, %v)", side, repo)
	initialItem, err := repo.GetInitialItem()
	if err != nil {
		logging.LogDebug("ui/initialLoadRepo(): ", err)
		updateStatus(err.Error())
		return
	}
	logging.LogDebugf("ui/initialLoadRepo(%v, %v), initialItem: %v", side, repo, initialItem)

	title := repo.GetTitle(initialItem.Config)
	workingModel.AddNextView(side, initialItem.Config, title)

	itemList, err := repo.GetList(initialItem.Config)
	if err != nil {
		logging.LogDebug("ui/initialLoadRepo(): ", err)
		updateStatus(err.Error())
		return
	}
	workingModel.SetItems(side, itemList)
}

// initialLoadDp loads initial DataPower view on the left side.
func initialLoadDp() {
	initialLoadRepo(model.Left, &dp.Repo)
}

// initialLoadLocalfs loads initial local filesystem view on the right side.
func initialLoadLocalfs() {
	initialLoadRepo(model.Right, &localfs.Repo)
}

// ProcessInputEvent processes given input event and do appropriate action.
// This is function with main logic of dpcmder.
func ProcessInputEvent(event tcell.Event) error {
	logging.LogDebugf("ui/ProcessInputEvent(%#v)", event)

	setScreenSize()

	var err error

	switch event := event.(type) {
	case *tcell.EventKey:
		c := event.Rune()
		k := event.Key()
		m := event.Modifiers()
		// updateStatusf("Key pressed value: '%#v'", k)
		switch {
		case c == 'q', (k == tcell.KeyCtrlC && m == tcell.ModCtrl):
			return QuitError
		case k == tcell.KeyTab:
			workingModel.ToggleSide()
		case k == tcell.KeyEnter:
			err = enterCurrentDirectory()
		case (k == tcell.KeyLeft && m == tcell.ModShift), c == 'J':
			err = showPrevView()
		case (k == tcell.KeyRight && m == tcell.ModShift), c == 'L':
			err = showNextView()
		case c == 'H':
			err = showViewHistory()
		case c == ' ':
			workingModel.ToggleCurrItem()
		case c == '.':
			err = enterDirectoryPath(&workingModel)
		case k == tcell.KeyLeft, c == 'j':
			workingModel.HorizScroll -= 10
			if workingModel.HorizScroll < 0 {
				workingModel.HorizScroll = 0
			}
		case k == tcell.KeyRight, c == 'l':
			workingModel.HorizScroll += 10
		case (k == tcell.KeyUp && m == tcell.ModNone), c == 'i':
			workingModel.NavUp()
		case (k == tcell.KeyDown && m == tcell.ModNone), c == 'k':
			workingModel.NavDown()
		case (k == tcell.KeyUp && m == tcell.ModShift), c == 'I':
			workingModel.ToggleCurrItem()
			workingModel.NavUp()
		case (k == tcell.KeyDown && m == tcell.ModShift), c == 'K':
			workingModel.ToggleCurrItem()
			workingModel.NavDown()
		case (k == tcell.KeyPgUp && m == tcell.ModNone), c == 'u':
			workingModel.NavPgUp()
		case (k == tcell.KeyPgDn && m == tcell.ModNone), c == 'o':
			workingModel.NavPgDown()
		case (k == tcell.KeyPgUp && m == tcell.ModShift), c == 'U':
			workingModel.SelPgUp()
			workingModel.NavPgUp()
		case (k == tcell.KeyPgDn && m == tcell.ModShift), c == 'O':
			workingModel.SelPgDown()
			workingModel.NavPgDown()
		case (k == tcell.KeyHome && m == tcell.ModNone), c == 'a':
			workingModel.NavTop()
		case (k == tcell.KeyEnd && m == tcell.ModNone), c == 'z':
			workingModel.NavBottom()
		case (k == tcell.KeyHome && m == tcell.ModShift), c == 'A':
			workingModel.SelToTop()
			workingModel.NavTop()
		case (k == tcell.KeyEnd && m == tcell.ModShift), c == 'Z':
			workingModel.SelToBottom()
			workingModel.NavBottom()
		case c == 'f':
			filterItems(&workingModel)
		case c == '/':
			searchItem(&workingModel)
		case c == 'n', c == 'p':
			searchNextItem(&workingModel, c == 'p')
		case k == tcell.KeyF2, c == '2':
			err = refreshCurrentView(&workingModel)
		case k == tcell.KeyF3, c == '3':
			err = viewCurrent(&workingModel)
		case k == tcell.KeyF4, c == '4':
			err = editCurrent(&workingModel)
		case k == tcell.KeyF5, c == '5':
			err = copyCurrent(&workingModel)
		case c == 'd':
			err = diffCurrent(&workingModel)
		case k == tcell.KeyF7, c == '7':
			err = createDirectoryOrDomain(&workingModel)
		case k == tcell.KeyF8, c == '8':
			err = createEmptyFile(&workingModel)
		case k == tcell.KeyF9, c == '9':
			err = cloneCurrent(&workingModel)
		case k == tcell.KeyDelete, c == 'x':
			err = deleteCurrent(&workingModel)
		case c == 's':
			err = syncModeToggle(&workingModel)
		case c == 'S':
			err = saveDataPowerConfig(&workingModel)
		case c == 'm':
			err = showStatusMessages(workingModel.Statuses())
		case c == '0':
			err = toggleObjectMode(&workingModel)
		case c == '?':
			err = showItemInfo(&workingModel)
		case c == 'h':
			err = extprogs.ShowHelp()

		default:
			err = extprogs.ShowHelp()
			updateStatusf("Key event value (before showing help): '%#v'", event)
		}
	case *tcell.EventResize:
	}

	if err != nil {
		updateStatus(err.Error())
	}

	out.DrawEvent(events.UpdateViewEvent{Type: events.UpdateViewRefresh, Model: &workingModel})

	return nil
}

// prepareInputDialog prepares information for input dialog showing on console screen.
func prepareInputDialog(dialogSession *userDialogInputSessionInfo) events.UpdateViewEvent {
	answer := dialogSession.inputAnswer
	if dialogSession.inputAnswerMasked {
		answerLen := utf8.RuneCountInString(dialogSession.inputAnswer)
		answer = strings.Repeat("*", answerLen)
	}

	return events.UpdateViewEvent{
		Type:                  events.UpdateViewShowQuestionDialog,
		DialogQuestion:        dialogSession.inputQuestion,
		DialogAnswer:          answer,
		DialogAnswerCursorIdx: dialogSession.inputAnswerCursorIdx}
}

// askUserInput asks user for input like confirmation of some action or name of
// new file/directory.
func askUserInput(question, answer string, answerMasked bool) userDialogResult {
	// When progress dialog is shown we don't won't it to hid our input dialog.
	progressDialogSession.waitUserInput = true

	dialogSession := userDialogInputSessionInfo{inputQuestion: question,
		inputAnswer:          answer,
		inputAnswerCursorIdx: utf8.RuneCountInString(answer),
		inputAnswerMasked:    answerMasked}
loop:
	for {
		updateViewEvent := prepareInputDialog(&dialogSession)
		out.DrawEvent(updateViewEvent)
		event := out.Screen.PollEvent()
		switch event := event.(type) {
		case *tcell.EventKey:
			processInputDialogInput(&dialogSession, event)
		}

		if dialogSession.dialogCanceled || dialogSession.dialogSubmitted {
			break loop
		}
	}
	// When progress dialog was shown we show it after we don't need input dialog
	// any more.
	progressDialogSession.waitUserInput = false

	return userDialogResult{inputAnswer: dialogSession.inputAnswer,
		dialogCanceled:  dialogSession.dialogCanceled,
		dialogSubmitted: dialogSession.dialogSubmitted}
}

// processInputDialogInput processes user's input to input dialog.
func processInputDialogInput(dialogSession *userDialogInputSessionInfo, keyEvent *tcell.EventKey) {
	logging.LogDebugf("ui/processInputDialogInput(): '%s'", dialogSession)
	c := keyEvent.Rune()
	k := keyEvent.Key()
	switch {
	case k == tcell.KeyRune:
		changedAnswer := ""
		answerLen := utf8.RuneCountInString(dialogSession.inputAnswer)
		runeIdx := 0
		for _, runeVal := range dialogSession.inputAnswer {
			if runeIdx == dialogSession.inputAnswerCursorIdx {
				changedAnswer = changedAnswer + string(c)
			}
			changedAnswer = changedAnswer + string(runeVal)
			runeIdx = runeIdx + 1
		}
		if answerLen == runeIdx && runeIdx == dialogSession.inputAnswerCursorIdx {
			changedAnswer = changedAnswer + string(c)
		}
		dialogSession.inputAnswer = changedAnswer
		dialogSession.inputAnswerCursorIdx = dialogSession.inputAnswerCursorIdx + 1
	case k == tcell.KeyEsc:
		logging.LogDebug("ui/processInputDialogInput() canceling user input: '%s'", dialogSession)
		dialogSession.dialogCanceled = true
	case k == tcell.KeyEnter:
		logging.LogDebugf("ui/processInputDialogInput() accepting user input: '%s'", dialogSession)
		dialogSession.dialogSubmitted = true
	case k == tcell.KeyDEL:
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
	case k == tcell.KeyDelete:
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
	case k == tcell.KeyLeft:
		if dialogSession.inputAnswerCursorIdx > 0 {
			dialogSession.inputAnswerCursorIdx = dialogSession.inputAnswerCursorIdx - 1
		}
	case k == tcell.KeyRight:
		if dialogSession.inputAnswerCursorIdx < utf8.RuneCountInString(dialogSession.inputAnswer) {
			dialogSession.inputAnswerCursorIdx = dialogSession.inputAnswerCursorIdx + 1
		}
	case k == tcell.KeyHome:
		dialogSession.inputAnswerCursorIdx = 0
	case k == tcell.KeyEnd:
		dialogSession.inputAnswerCursorIdx = utf8.RuneCountInString(dialogSession.inputAnswer)
	}
}

func enterCurrentDirectory() error {
	logging.LogDebugf("ui/enterCurrentDirectory()")
	item := workingModel.CurrItem()
	if item == nil {
		return errs.Error("Nothing found, can't enter current directory.")
	}
	err := showItem(workingModel.CurrSide(), item.Config, item.Name)

	switch err {
	case dpMissingPasswordError:
		dialogResult := askUserInput("Please enter DataPower password: ", "", true)
		if dialogResult.inputAnswer != "" {
			setCurrentDpPlainPassword(dialogResult.inputAnswer)
		}
		return nil
	case nil:
		// If no error occurs.
	default:
		switch err.(type) {
		case errs.UnexpectedHTTPResponse:
			setCurrentDpPlainPassword("")
		}
	}

	return err
}

func showItem(side model.Side, itemConfig *model.ItemConfig, itemName string) error {
	logging.LogDebugf("ui/showItem(%d, %v, '%s')", side, itemConfig, itemName)
	if itemConfig.Type == model.ItemDpConfiguration {
		applianceName := itemName
		if applianceName != ".." && applianceName != "." {
			applicanceConfig := config.Conf.DataPowerAppliances[applianceName]
			dpTransientPassword := config.DpTransientPasswordMap[applianceName]
			logging.LogDebugf("ui/showItem(), applicanceConfig: '%s'", applicanceConfig)
			if applicanceConfig.Password == "" && dpTransientPassword == "" {
				return dpMissingPasswordError
			}
		}
	}

	var itemList model.ItemList
	var err error

	r := repos[side]
	switch itemConfig.Type {
	case model.ItemDpConfiguration, model.ItemDpDomain, model.ItemDpFilestore,
		model.ItemDirectory,
		model.ItemDpObjectClassList, model.ItemDpObjectClass,
		model.ItemNone:
		itemList, err = r.GetList(itemConfig)
	default:
		return errs.Errorf("ui/showItem(), unknown type: %s", itemConfig.Type)
	}

	if err != nil {
		return err
	}
	logging.LogDebug("ui/showItem(), itemList: ", itemList)
	title := r.GetTitle(itemConfig)
	logging.LogDebug("ui/showItem(), title: ", title)

	oldViewConfig := workingModel.ViewConfig(side)
	oldCurrItem := workingModel.CurrItemForSide(side)
	workingModel.SetItems(side, itemList)
	// If we are refreshing current view or showing a view from history - don't
	// change view history.
	// If we are entering new directory/appliance/... add new view to history.
	switch itemName {
	case ".":
		workingModel.SetCurrentView(side, itemConfig, title)
	default:
		workingModel.AddNextView(side, itemConfig, title)
	}
	switch itemName {
	case "..":
		workingModel.SetCurrItemForSideAndConfig(side, oldViewConfig)
	case ".":
		workingModel.SetCurrItemForSide(side, oldCurrItem.Name)
	default:
		workingModel.NavTopForSide(side)
	}

	return nil
}

// showPrevView navigate to the previous view from the view history.
func showPrevView() error {
	logging.LogDebugf("ui/showPrevView()")
	side := workingModel.CurrSide()
	oldView := workingModel.ViewConfig(side)

	newView := workingModel.NavCurrentViewBack(side)
	logging.LogDebugf("ui/showPrevView(), side: %v, oldView: %v, newView: %s",
		side, oldView, newView)

	// If previous view in history requires filestore mode, switch to filestore mode.
	if side == model.Left &&
		newView.Type != model.ItemDpObjectClassList &&
		newView.Type != model.ItemDpObjectClass &&
		dp.Repo.ObjectConfigMode {
		dp.Repo.ObjectConfigMode = false
	}

	if newView == oldView {
		updateStatusf("Can't move back from the first view in the history.")
	}

	return showItem(side, newView, ".")
}

// showNextView navigate to the next view from the view history.
func showNextView() error {
	logging.LogDebugf("ui/showNextView()")
	side := workingModel.CurrSide()
	oldView := workingModel.ViewConfig(side)

	newView := workingModel.NavCurrentViewForward(side)
	logging.LogDebugf("ui/showNextView(), side: %v, oldView: %v, newView: %s",
		side, oldView, newView)

	// If next view in history requires object mode, switch to object config mode.
	if side == model.Left && newView.Type == model.ItemDpObjectClassList && !dp.Repo.ObjectConfigMode {
		dp.Repo.ObjectConfigMode = true
	}

	if newView == oldView {
		updateStatusf("Can't move forward from the last view in the history.")
	}

	return showItem(side, newView, ".")
}

// showViewHistory shows dialog with history of views where we can select any
// view and jump straight to it.
func showViewHistory() error {
	logging.LogDebug("ui/showViewHistory()")
	// When progress dialog is shown we don't won't it to hid our selection dialog.
	progressDialogSession.waitUserInput = true

	side := workingModel.CurrSide()
	viewHistory := workingModel.ViewConfigHistoryList(side)
	logging.LogDebugf("ui/showViewHistory(), viewHistory: %v", viewHistory)
	pathHistory := make([]string, len(viewHistory))
	r := repos[side]
	for idx, view := range viewHistory {
		pathHistory[idx] = r.GetTitle(view)
	}
	logging.LogDebugf("ui/showViewHistory(), pathHistory: %v", pathHistory)
	dialogSession := listSelectionDialogSessionInfo{message: "Select a view:",
		list: pathHistory}

loop:
	for {
		updateViewEvent := events.UpdateViewEvent{
			Type:                     events.UpdateViewShowListSelectionDialog,
			ListSelectionMessage:     dialogSession.message,
			ListSelectionList:        dialogSession.list,
			ListSelectionSelectedIdx: dialogSession.selectionIdx}

		out.DrawEvent(updateViewEvent)
		event := out.Screen.PollEvent()
		switch event := event.(type) {
		case *tcell.EventKey:
			processSelectListDialogInput(&dialogSession, event)
		}

		if dialogSession.dialogCanceled || dialogSession.dialogSubmitted {
			break loop
		}
	}

	if dialogSession.dialogSubmitted {
		newView := workingModel.NavCurrentViewIdx(side, dialogSession.selectionIdx)
		showItem(side, newView, ".")
	}

	// When progress dialog was shown we show it after we don't need selection
	// dialog any more.
	progressDialogSession.waitUserInput = false

	return nil
}

// processSelectListDialogInput processes user's input to list selection dialog.
func processSelectListDialogInput(dialogSession *listSelectionDialogSessionInfo, keyEvent *tcell.EventKey) {
	logging.LogDebugf("ui/processSelectListDialogInput(): '%s'", dialogSession)
	c := keyEvent.Rune()
	k := keyEvent.Key()
	switch {
	case k == tcell.KeyEsc:
		logging.LogDebug("ui/processSelectListDialogInput() canceling selection: '%s'", dialogSession)
		dialogSession.dialogCanceled = true
	case k == tcell.KeyEnter:
		logging.LogDebugf("ui/processSelectListDialogInput() accepting selection: '%s'", dialogSession)
		dialogSession.dialogSubmitted = true
	case k == tcell.KeyUp, c == 'i':
		if dialogSession.selectionIdx > 0 {
			dialogSession.selectionIdx = dialogSession.selectionIdx - 1
		}
	case k == tcell.KeyDown, c == 'k':
		if dialogSession.selectionIdx < len(dialogSession.list)-1 {
			dialogSession.selectionIdx = dialogSession.selectionIdx + 1
		}
	case k == tcell.KeyHome, c == 'a':
		dialogSession.selectionIdx = 0
	case k == tcell.KeyEnd, c == 'z':
		dialogSession.selectionIdx = len(dialogSession.list) - 1
	}
}

func setCurrentDpPlainPassword(password string) {
	item := workingModel.CurrItem()
	applianceName := item.Config.DpAppliance
	logging.LogDebugf("ui/setDpPlainPassword() applicanceName: '%s'", applianceName)
	config.DpTransientPasswordMap[applianceName] = password
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
	out.DrawEvent(events.UpdateViewEvent{Type: events.UpdateViewRefresh, Model: m})
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
		if m.CurrSide() == model.Left {
			currView := workingModel.ViewConfig(workingModel.CurrSide())
			showProgressDialogf("Fetching '%s' file from DataPower...", ci.Name)
			fileContent, err := repos[m.CurrSide()].GetFile(currView, ci.Name)
			hideProgressDialog()
			if err != nil {
				return err
			}
			if fileContent != nil {
				err = extprogs.View("*."+ci.Name, fileContent)
				if err != nil {
					return err
				}
			}
		} else {
			err = extprogs.ViewFile(ci.Config.Path)
			if err != nil {
				return err
			}
		}
	case model.ItemDpConfiguration:
		fileContent, err := config.Conf.GetDpApplianceConfig(ci.Name)
		if err != nil {
			return err
		}
		err = extprogs.View("*."+ci.Name+".json", fileContent)
		if err != nil {
			return err
		}
	case model.ItemDpObject:
		objectContent, err := dp.Repo.GetObject(ci.Config.DpDomain, ci.Config.Path, ci.Name, false)
		if err != nil {
			return err
		}
		err = extprogs.View(getObjectTmpName(ci.Name), objectContent)
		if err != nil {
			return err
		}
	default:
		return errs.Errorf("Can't view item '%s' (%s)", ci.Name, ci.Config.Type.UserFriendlyString())
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
		if m.CurrSide() == model.Left {
			showProgressDialogf("Fetching '%s' file from DataPower...", ci.Name)
			fileContent, err := repos[m.CurrSide()].GetFile(currView, ci.Name)
			hideProgressDialog()
			if err != nil {
				return err
			}
			changed, newFileContent, err := extprogs.Edit("*."+m.CurrItem().Name, fileContent)
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
		} else {
			err := extprogs.EditFile(ci.Config.Path)
			if err != nil {
				return err
			}
			showItem(workingModel.CurrSide(), currView, ".")
		}
		updateStatusf("File '%s' on path '%s' updated.", ci.Name, ci.Config.Path)

	case model.ItemDpConfiguration:
		fileContent, err := config.Conf.GetDpApplianceConfig(ci.Name)
		if err != nil {
			return err
		}
		changed, newFileContent, err := extprogs.Edit(m.CurrItem().Name+"*.json", fileContent)
		if err != nil {
			return err
		}
		if changed {
			err := config.Conf.SetDpApplianceConfig(ci.Name, newFileContent)
			if err != nil {
				return err
			}
		}
		updateStatusf("DataPower configuration '%s' updated.", ci.Name)

	case model.ItemDpObject:
		objectContent, err := dp.Repo.GetObject(ci.Config.DpDomain, ci.Config.Path, ci.Name, false)
		if err != nil {
			return err
		}
		changed, newObjectContent, err := extprogs.Edit(getObjectTmpName(ci.Name), objectContent)
		if err != nil {
			return err
		}
		if changed {
			err := dp.Repo.SetObject(ci.Config.DpDomain, ci.Config.Path, ci.Name, newObjectContent, true)
			if err != nil {
				return err
			}
			updateStatusf("DataPower object '%s' of class '%s' updated.", ci.Name, ci.Config.Path)
			currView := workingModel.ViewConfig(workingModel.CurrSide())
			showItem(workingModel.CurrSide(), currView, ".")
		} else {
			updateStatusf("DataPower object '%s' of class '%s' not changed.", ci.Name, ci.Config.Path)
		}

	default:
		return errs.Errorf("Can't edit item '%s' (%s)", ci.Name, ci.Config.Type.UserFriendlyString())
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

func diffFilesWithCleanup(tmpDir, oldPath, newPath string) error {
	logging.LogDebug("ui/diffFiles()")
	err := extprogs.Diff(oldPath, newPath)
	if err != nil {
		errdel := extprogs.DeleteTempDir(tmpDir)
		if errdel == nil {
			updateStatusf("Deleted tmp dir on localfs '%s'", tmpDir)
		} else {
			logging.LogDebugf("ui/diffFiles() - Error deleting tmp dir on localfs '%s': %v", tmpDir, errdel)
		}
		logging.LogDebugf("ui/diffFiles() - diff err: %v", err)
		return err
	}
	err = extprogs.DeleteTempDir(tmpDir)
	if err != nil {
		logging.LogDebugf("ui/diffFiles() - delete tmp dir err: %v", err)
		return err
	}
	updateStatusf("Deleted tmp dir on localfs '%s'", tmpDir)
	logging.LogDebug("ui/diffFiles() end")
	return nil
}

func diffCurrent(m *model.Model) error {
	logging.LogDebug("ui/diffCurrent()")
	dpItem := m.CurrItemForSide(model.Left)
	localItem := m.CurrItemForSide(model.Right)

	dpView := m.ViewConfig(model.Left)
	localView := m.ViewConfig(model.Right)

	if dpItem.Config.Type == localItem.Config.Type ||
		(dpItem.Config.Type == model.ItemDpFilestore && localItem.Config.Type == model.ItemDirectory) ||
		(dpItem.Config.Type == model.ItemDpObject && dpItem.Modified == "modified") {

	}
	switch {
	case dpItem.Config.Type == localItem.Config.Type ||
		(dpItem.Config.Type == model.ItemDpFilestore && localItem.Config.Type == model.ItemDirectory):
		dpCopyDir := extprogs.CreateTempDir("dp")
		updateStatusf("Created tmp dir on localfs '%s'", dpCopyDir)
		localViewTmp := model.ItemConfig{Type: model.ItemDirectory, Path: dpCopyDir}
		// func copyItem(fromRepo, toRepo repo.Repo, fromViewConfig, toViewConfig *model.ItemConfig, item model.Item, confirmOverwrite string) (string, error) {
		copyItem(&dp.Repo, localfs.Repo, dpView, &localViewTmp, *dpItem, "y")
		var dpDirName string
		switch dpItem.Config.Type {
		case model.ItemDpFilestore:
			dpDirName = dpItem.Name[0 : len(dpItem.Name)-1]
		default:
			dpDirName = dpItem.Name
		}
		dpCopyItemPath := localfs.Repo.GetFilePath(dpCopyDir, dpDirName)
		localItemPath := localfs.Repo.GetFilePath(localView.Path, localItem.Name)

		err := diffFilesWithCleanup(dpCopyDir, dpCopyItemPath, localItemPath)

		return err
	case dpItem.Config.Type == model.ItemDpObject && dpItem.Modified == "modified":
		dpCopyDir := extprogs.CreateTempDir("dp")
		updateStatusf("Created tmp dir on localfs '%s'", dpCopyDir)

		localViewTmp := model.ItemConfig{Type: model.ItemDirectory, Path: dpCopyDir}

		objectContentMemory, err := dp.Repo.GetObject(
			dpItem.Config.DpDomain, dpItem.Config.Path, dpItem.Name, false)
		if err != nil {
			return err
		}
		objectContentSaved, err := dp.Repo.GetObject(
			dpItem.Config.DpDomain, dpItem.Config.Path, dpItem.Name, true)
		if err != nil {
			return err
		}

		objectNameMemory := dpItem.Name + "_memory.xml"
		objectNameSaved := dpItem.Name + "_saved.xml"

		_, err = localfs.Repo.UpdateFile(&localViewTmp, objectNameMemory, objectContentMemory)
		if err != nil {
			return err
		}
		_, err = localfs.Repo.UpdateFile(&localViewTmp, objectNameSaved, objectContentSaved)
		if err != nil {
			return err
		}

		dpObjectMemoryPath := localfs.Repo.GetFilePath(dpCopyDir, objectNameMemory)
		dpObjectSavedPath := localfs.Repo.GetFilePath(dpCopyDir, objectNameSaved)

		err = diffFilesWithCleanup(dpCopyDir, dpObjectSavedPath, dpObjectMemoryPath)

		return err
	case dpItem.Config.Type == model.ItemDpObject:
		err := errs.Errorf("Can't view changes on DataPower object '%s' if not modified (%s)",
			dpItem.Name, dpItem.Modified)
		logging.LogDebug(err)
		return err
	default:
		err := errs.Errorf("Can't compare different file types '%s' (%s) to '%s' (%s)",
			dpItem.Name, string(dpItem.Config.Type), localItem.Name, string(localItem.Config.Type))
		logging.LogDebug(err)
		return err
	}
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
	case model.ItemDpFilestore:
		res, err = copyFilestore(fromRepo, toRepo, fromViewConfig, toViewConfig, item.Name, confirmOverwrite)
		if err != nil {
			return res, err
		}
	case model.ItemDirectory:
		res, err = copyDirs(fromRepo, toRepo, fromViewConfig, toViewConfig, item.Name, confirmOverwrite)
		if err != nil {
			return res, err
		}
	case model.ItemFile:
		// If we copy to DataPower and we are in ObjectConfigMode we copy file to object.
		if toRepo.String() == dp.Repo.String() && dp.Repo.ObjectConfigMode {
			res, err = copyFileToObject(item.Config, item.Name, fromRepo, toRepo, fromViewConfig, toViewConfig, confirmOverwrite)
		} else {
			res, err = copyFile(fromRepo, toRepo, fromViewConfig, toViewConfig, item.Name, confirmOverwrite)
		}

		if err != nil {
			return res, err
		}
	case model.ItemDpDomain:
		err = exportDomain(fromViewConfig, toViewConfig, item.Name)
		if err != nil {
			return res, err
		}
	case model.ItemDpConfiguration:
		err = exportAppliance(item.Config, toViewConfig, item.Name)
		if err != nil {
			return res, err
		}
	case model.ItemDpObject:
		res, err = copyObjectToFile(item.Config, item.Name, fromRepo, toRepo, fromViewConfig, toViewConfig, confirmOverwrite)
		if err != nil {
			return res, err
		}
	default:
		updateStatusf("Item of type '%s' can't be copied/exported.", item.Config.Type.UserFriendlyString())
	}

	logging.LogDebugf("ui/copyItem(), res: '%s'", res)
	return res, nil
}

func copyFilestore(fromRepo, toRepo repo.Repo, fromViewConfig, toViewConfig *model.ItemConfig, dirName, confirmOverwrite string) (string, error) {
	dirToName := dirName[0 : len(dirName)-1]
	return copyDirsOrFilestores(fromRepo, toRepo, fromViewConfig, toViewConfig, dirName, dirToName, confirmOverwrite)
}

func copyDirs(fromRepo, toRepo repo.Repo, fromViewConfig, toViewConfig *model.ItemConfig, dirName, confirmOverwrite string) (string, error) {
	return copyDirsOrFilestores(fromRepo, toRepo, fromViewConfig, toViewConfig, dirName, dirName, confirmOverwrite)
}

func copyDirsOrFilestores(fromRepo, toRepo repo.Repo, fromViewConfig, toViewConfig *model.ItemConfig, dirFromName, dirToName, confirmOverwrite string) (string, error) {
	logging.LogDebugf("ui/copyDirsOrFilestores(.., .., %v, %v, '%s', '%s', '%s')", fromViewConfig, toViewConfig, dirFromName, dirToName, confirmOverwrite)
	toParentPath := toViewConfig.Path
	toFileType, err := toRepo.GetFileType(toViewConfig, toParentPath, dirToName)
	if err != nil {
		return confirmOverwrite, err
	}
	toPath := toRepo.GetFilePath(toParentPath, dirToName)
	switch toFileType {
	case model.ItemNone:
		_, err = toRepo.CreateDir(toViewConfig, toParentPath, dirToName)
		if err != nil {
			logging.LogDebugf("ui/copyDirsOrFilestores() - err: %v", err)
			return confirmOverwrite, err
		}
		updateStatusf("Directory '%s' created.", toPath)
	case model.ItemDirectory:
		updateStatusf("Directory '%s' already exists.", toPath)
	case model.ItemDpFilestore:
		updateStatusf("DataPower filestore '%s' already exists.", toPath)
	default:
		errMsg := fmt.Sprintf("Non dir '%s' exists (%v), can't create dir.", toPath, toFileType)
		logging.LogDebugf("ui/copyDirsOrFilestores() - %s", errMsg)
		return confirmOverwrite, errs.Error(errMsg)
	}

	fromViewConfigDir := model.ItemConfig{
		Parent:      fromViewConfig,
		Type:        model.ItemDirectory,
		Path:        fromRepo.GetFilePath(fromViewConfig.Path, dirFromName),
		DpAppliance: fromViewConfig.DpAppliance,
		DpDomain:    fromViewConfig.DpDomain,
		DpFilestore: fromViewConfig.DpFilestore}
	items, err := fromRepo.GetList(&fromViewConfigDir)
	if err != nil {
		return confirmOverwrite, err
	}

	showProgressDialog("Copying files from DataPower...")
	defer hideProgressDialog()
	for _, item := range items {
		if item.Name != ".." {
			toViewConfigDir := model.ItemConfig{Parent: toViewConfig,
				Path:        toRepo.GetFilePath(toViewConfig.Path, dirToName),
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
	logging.LogDebugf("ui/copyFile(.., .., %v, %v, '%s', '%s')",
		fromViewConfig, toViewConfig, fileName, confirmOverwrite)
	showProgressDialog("Copying file(s) from DataPower...")
	defer hideProgressDialog()
	updateProgressDialogMessagef("Preparing to copy file '%s' from %s to %s...",
		fileName, fromRepo, toRepo)
	res := confirmOverwrite
	targetFileType, err := toRepo.GetFileType(toViewConfig, toViewConfig.Path, fileName)
	if err != nil {
		return res, err
	}

	switch targetFileType {
	case model.ItemDirectory:
		copyFileToDirStatus :=
			fmt.Sprintf("ERROR: File '%s' could not be copied from '%s' to '%s' - directory with same name exists.",
				fileName, fromViewConfig.Path, toViewConfig.Path)
		updateStatus(copyFileToDirStatus)
	case model.ItemFile:
		if res != "ya" && res != "na" {
			logging.LogDebugf("ui/copyFile(), confirm overwrite: '%s'", res)
			dialogResult := askUserInput(
				fmt.Sprintf("Confirm overwrite of file '%s' at '%s' (y/ya/n/na): ",
					fileName, toViewConfig.Path), "", false)
			if dialogResult.dialogSubmitted {
				res = dialogResult.inputAnswer
			}
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
				copySuccessStatus := fmt.Sprintf("File '%s' copied from '%s' to '%s'.",
					fileName, fromViewConfig.Path, toViewConfig.Path)
				updateStatus(copySuccessStatus)
			} else {
				copyErrStatus := fmt.Sprintf("ERROR: File '%s' not copied from '%s' to '%s'.",
					fileName, fromViewConfig.Path, toViewConfig.Path)
				updateStatus(copyErrStatus)
			}
		}
	} else {
		updateStatusf("Canceled overwrite of '%s'", fileName)
	}

	logging.LogDebugf("ui/copyFile(), res: '%s'", res)
	return res, nil
}

func copyObjectToFile(itemConfig *model.ItemConfig, itemName string,
	fromRepo, toRepo repo.Repo, fromViewConfig, toViewConfig *model.ItemConfig,
	confirmOverwrite string) (string, error) {
	logging.LogDebugf("ui/copyObjectToFile(%v, '%s', .., .., %v, %v, '%s')",
		itemConfig, itemName, fromViewConfig, toViewConfig, confirmOverwrite)
	res := confirmOverwrite

	objectName := itemName
	var objectFileSuffix string

	switch dp.Repo.GetManagementInterface() {
	case config.DpInterfaceRest:
		objectFileSuffix = ".json"
	case config.DpInterfaceSoma:
		objectFileSuffix = ".xml"
	default:
		logging.LogDebug("ui/copyObjectToFile(), using neither REST neither SOMA.")
		return "", errs.Error("DataPower management interface not set.")
	}
	objectFileName := itemName + objectFileSuffix
	logging.LogDebugf("ui/copyObjectToFile(), objectName: '%s', objectFileName: '%s'.",
		objectName, objectFileName)

	targetFileType, err := toRepo.GetFileType(toViewConfig, toViewConfig.Path, objectFileName)
	if err != nil {
		return res, err
	}

	switch targetFileType {
	case model.ItemDirectory:
		copyFileToDirStatus :=
			fmt.Sprintf("ERROR: Object '%s' could not be copied from '%s' to '%s' - directory with same name exists.",
				objectFileName, fromViewConfig.Path, toViewConfig.Path)
		updateStatus(copyFileToDirStatus)
	case model.ItemFile:
		if res != "ya" && res != "na" {
			logging.LogDebugf("ui/copyObjectToFile(), confirm overwrite: '%s'", res)
			dialogResult := askUserInput(
				fmt.Sprintf("Confirm overwrite of file '%s' at '%s' (y/ya/n/na): ",
					objectFileName, toViewConfig.Path), "", false)
			if dialogResult.dialogSubmitted {
				res = dialogResult.inputAnswer
			}
		}
	case model.ItemNone:
		res = "y"
	}
	if res == "y" || res == "ya" {
		switch targetFileType {
		case model.ItemFile, model.ItemNone:
			fBytes, err := dp.Repo.GetObject(itemConfig.DpDomain, itemConfig.Path, objectName, false)
			if err != nil {
				return res, err
			}
			copySuccess, err := toRepo.UpdateFile(toViewConfig, objectFileName, fBytes)
			if err != nil {
				return res, err
			}
			logging.LogDebugf("ui/copyObject(): %v", copySuccess)
			if copySuccess {
				copySuccessStatus := fmt.Sprintf("File '%s' copied from '%s' to '%s'.",
					objectFileName, fromViewConfig.Path, toViewConfig.Path)
				updateStatus(copySuccessStatus)
			} else {
				copyErrStatus := fmt.Sprintf("ERROR: File '%s' not copied from '%s' to '%s'.",
					objectFileName, fromViewConfig.Path, toViewConfig.Path)
				updateStatus(copyErrStatus)
			}
		}
	} else {
		updateStatusf("Canceled overwrite of '%s'", objectFileName)
	}

	logging.LogDebugf("ui/copyObjectToFile(), res: '%s'", res)
	return res, nil
}

func copyFileToObject(itemConfig *model.ItemConfig, itemName string,
	fromRepo, toRepo repo.Repo, fromViewConfig, toViewConfig *model.ItemConfig,
	confirmOverwrite string) (string, error) {
	logging.LogDebugf("ui/copyFileToObject(%v, '%s', .., .., %v, %v, '%s')",
		itemConfig, itemName, fromViewConfig, toViewConfig, confirmOverwrite)
	res := confirmOverwrite

	var objectFileSuffix string

	switch dp.Repo.GetManagementInterface() {
	case config.DpInterfaceRest:
		objectFileSuffix = ".json"
	case config.DpInterfaceSoma:
		objectFileSuffix = ".xml"
	default:
		logging.LogDebug("ui/copyFileToObject(), using neither REST neither SOMA.")
		return "", errs.Error("DataPower management interface not set.")
	}

	if !strings.HasSuffix(itemName, objectFileSuffix) {
		return "", errs.Errorf("Copy from file '%s' to object - wrong suffix, '%s' expected.",
			itemName, objectFileSuffix)
	}
	objectFileName := itemName

	objectBytesLocal, err := localfs.Repo.GetFile(fromViewConfig, objectFileName)
	if err != nil {
		return "", err
	}
	objectClassName, objectName, err := dp.Repo.ParseObjectClassAndName(objectBytesLocal)
	if err != nil {
		return "", err
	}
	objectBytesDp, err := dp.Repo.GetObject(
		toViewConfig.DpDomain, objectClassName, objectName, false)
	if err != nil {
		return "", err
	}

	targetItemType := model.ItemDpObject
	if objectBytesDp == nil {
		targetItemType = model.ItemNone
	}

	existingObject := false
	switch targetItemType {
	case model.ItemDpObject:
		existingObject = true
		if res != "ya" && res != "na" {
			logging.LogDebugf("ui/copyFileToObject(), confirm overwrite: '%s'", res)
			dialogResult := askUserInput(
				fmt.Sprintf("Confirm overwrite of object '%s' of class '%s' from  file '%s' (y/ya/n/na): ",
					objectName, objectClassName, objectFileName), "", false)
			if dialogResult.dialogSubmitted {
				res = dialogResult.inputAnswer
			}
		}
	case model.ItemNone:
		res = "y"
	default:
		return "", errs.Errorf("Unknown target item type (%s).", targetItemType)
	}

	logging.LogDebugf("ui/copyFileToObject(), targetItemType: '%s', existingObject: %t, res: '%s'.",
		targetItemType, existingObject, res)

	if res == "y" || res == "ya" {
		err = dp.Repo.SetObject(
			toViewConfig.DpDomain, objectClassName, objectName, objectBytesLocal, existingObject)
		if err != nil {
			return res, err
		}
		logging.LogDebugf("ui/copyFileToObject() Object '%s' of class '%s' copied from file '%s' to the appliance.",
			objectName, objectClassName, objectFileName)
		updateStatusf("Object '%s' of class '%s' copied from file '%s' to the appliance.",
			objectName, objectClassName, objectFileName)
	} else {
		updateStatusf("Canceled overwrite of '%s'", objectName)
	}

	logging.LogDebugf("ui/copyFileToObject(), res: '%s'", res)
	return res, nil
}

func exportDomain(fromViewConfig, toViewConfig *model.ItemConfig, domainName string) error {
	logging.LogDebugf("ui/exportDomain(%v, %v, '%s')", fromViewConfig, toViewConfig, domainName)
	exportFileName := fromViewConfig.DpAppliance + "_" + domainName + "_" + time.Now().Format("20060102150405") + ".zip"
	logging.LogDebugf("ui/exportDomain() exportFileName: '%s'", exportFileName)
	showProgressDialogf("Exporting domain '%s'...", domainName)
	exportFileBytes, err := dp.Repo.ExportDomain(domainName, exportFileName)
	hideProgressDialog()
	if err != nil {
		return err
	}
	_, err = localfs.Repo.UpdateFile(toViewConfig, exportFileName, exportFileBytes)
	if err == nil {
		updateStatusf("Domain '%s' exported to file '%s' on path '%s'.",
			domainName, exportFileName, toViewConfig.Path)
	}
	return err
}

func exportAppliance(dpApplianceConfig, toViewConfig *model.ItemConfig, applianceConfigName string) error {
	logging.LogDebugf("ui/exportAppliance(%v, %v)", dpApplianceConfig, toViewConfig)
	applianceName := dpApplianceConfig.DpAppliance
	exportFileName := applianceName + "_" + time.Now().Format("20060102150405") + ".zip"
	logging.LogDebugf("ui/exportAppliance() exportFileName: '%s'", exportFileName)

	applicanceConfig := config.Conf.DataPowerAppliances[applianceName]
	dpTransientPassword := config.DpTransientPasswordMap[applianceName]
	logging.LogDebugf("ui/exportAppliance(), applicanceConfig: '%s'", applicanceConfig)
	if applicanceConfig.Password == "" && dpTransientPassword == "" {
		logging.LogDebugf("ui/exportAppliance(), before asking password.")
		dialogResult := askUserInput("Please enter DataPower password: ", "", true)
		logging.LogDebugf("ui/exportAppliance(), after asking password (%s).", dialogResult)
		if dialogResult.dialogCanceled || dialogResult.inputAnswer == "" {
			return nil
		}
		setCurrentDpPlainPassword(dialogResult.inputAnswer)
	}

	showProgressDialogf("Exporting DataPower appliance '%s'...", applianceName)
	exportFileBytes, err := dp.Repo.ExportAppliance(applianceConfigName, exportFileName)
	hideProgressDialog()
	if err != nil {
		return err
	}
	_, err = localfs.Repo.UpdateFile(toViewConfig, exportFileName, exportFileBytes)
	if err == nil {
		updateStatusf("Appliance '%s' exported to file '%s' on path '%s'.",
			applianceName, exportFileName, toViewConfig.Path)
	}
	return err
}

func createEmptyFile(m *model.Model) error {
	logging.LogDebugf("ui/createEmptyFile()")
	side := m.CurrSide()
	viewConfig := m.ViewConfig(side)
	switch viewConfig.Type {
	case model.ItemDirectory, model.ItemDpFilestore:
		dialogResult := askUserInput("Enter file name for file to create: ", "", false)
		if dialogResult.dialogSubmitted {
			fileName := dialogResult.inputAnswer
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
	case model.ItemNone:
		if side == model.Left {
			dialogResult := askUserInput("Enter DataPower configuration name to create: ", "", false)
			if dialogResult.dialogSubmitted {
				confName := dialogResult.inputAnswer

				fileContent, err := config.Conf.CreateDpApplianceConfig()
				if err != nil {
					return err
				}
				changed, newFileContent, err := extprogs.Edit(confName, fileContent)
				if err != nil {
					return err
				}
				if changed {
					err := config.Conf.SetDpApplianceConfig(confName, newFileContent)
					if err != nil {
						return err
					}
				}

				updateStatusf("DataPower configuration '%s' created.", confName)
				return showItem(side, viewConfig, ".")
			}
			updateStatus("Creation of new DataPower configuration canceled.")
		}
	}
	return nil
}

func cloneCurrent(m *model.Model) error {
	logging.LogDebug("ui/cloneCurrent()")
	currentItem := m.CurrItem()

	var newItemName string
	renameDialogResult := askUserInput(
		fmt.Sprintf("Enter name of cloned %s: ",
			currentItem.Config.Type.UserFriendlyString()), currentItem.Name, false)
	if renameDialogResult.dialogSubmitted {
		newItemName = renameDialogResult.inputAnswer

		side := m.CurrSide()
		viewConfig := m.ViewConfig(side)

		var err error
		switch currentItem.Config.Type {
		case model.ItemDpConfiguration:
			var clonedConfigContent []byte
			_, valueInMap := config.Conf.DataPowerAppliances[newItemName]
			if valueInMap {
				return errs.Errorf("DataPower configuration '%s' already exists.", newItemName)
			}
			clonedConfigContent, err = config.Conf.GetDpApplianceConfig(currentItem.Name)
			if err != nil {
				return err
			}
			err = config.Conf.SetDpApplianceConfig(newItemName, clonedConfigContent)
		case model.ItemDpObject:
			// func (r *dpRepo) GetObject(dpDomain, objectClass, objectName string, persisted bool) ([]byte, error) {
			dpDomain := currentItem.Config.DpDomain
			objectClass := currentItem.Config.Path
			objectNameOld := currentItem.Name
			objectNameNew := newItemName
			objectConfigToOverwrite, err := dp.Repo.GetObject(dpDomain, objectClass, objectNameNew, false)
			logging.LogDebugf("ui/cloneCurrent(), err: %v, objectConfigToOverwrite: '%v'", err, objectConfigToOverwrite)
			if err != nil {
				return err
			}
			existingObject := false
			if objectConfigToOverwrite != nil {
				existingObject = true
				confirmDialogResult := askUserInput(
					fmt.Sprintf("Object of class %s with name '%s' exists, overwrite (y,n): ",
						currentItem.Config.Type.UserFriendlyString(), objectNameNew), "", false)
				if confirmDialogResult.dialogCanceled || confirmDialogResult.inputAnswer != "y" {
					updateStatusf("Clonning of '%s' (%s) canceled.",
						currentItem.Name, currentItem.Config.Type.UserFriendlyString())
					return nil
				}
			}
			objectConfigOld, err := dp.Repo.GetObject(dpDomain, objectClass, objectNameOld, false)
			logging.LogDebugf("ui/cloneCurrent(), err: %v, objectConfigOld: '%v'", err, objectConfigOld)
			if err != nil {
				return err
			}
			objectConfigNew, err := dp.Repo.RenameObject(objectConfigOld, objectNameNew)
			logging.LogDebugf("ui/cloneCurrent(), err: %v, objectConfigNew: '%v'", err, objectConfigNew)
			if err != nil {
				return err
			}
			err = dp.Repo.SetObject(dpDomain, objectClass, objectNameNew, objectConfigNew, existingObject)
			if err != nil {
				return err
			}

			updateStatusf("DataPower object '%s' of class '%s' cloned to '%s'.", objectClass, objectNameOld, objectNameNew)
			return showItem(side, viewConfig, ".")
		default:
			return errs.Errorf("Can't clone item '%s' (%s).",
				currentItem.Name, currentItem.Config.Type.UserFriendlyString())
		}

		if err != nil {
			return err
		}

		updateStatusf("'%s' (%s) cloned to '%s'.",
			currentItem.Name, currentItem.Config.Type.UserFriendlyString(), newItemName)
		return showItem(side, viewConfig, ".")
	} else {
		updateStatusf("Clonning of '%s' (%s) canceled.",
			currentItem.Name, currentItem.Config.Type.UserFriendlyString())
	}

	return nil
}

func createDirectoryOrDomain(m *model.Model) error {
	logging.LogDebug("ui/createDirectoryOrDomain()")

	side := m.CurrSide()
	viewConfig := m.ViewConfig(side)
	switch viewConfig.Type {
	case model.ItemDirectory, model.ItemDpFilestore:
		return createDirectory(m)
	case model.ItemDpConfiguration:
		return createDomain(m)
	default:
		return errs.Errorf("Can't create directory, parent type '%s' doesn't support directory creation.",
			viewConfig.Type.UserFriendlyString())
	}
}

func createDirectory(m *model.Model) error {
	logging.LogDebug("ui/createDirectory()")
	dialogResult := askUserInput("Enter directory name to create: ", "", false)
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

func createDomain(m *model.Model) error {
	logging.LogDebug("ui/createDomain()")
	dialogResult := askUserInput("Enter domain name to create: ", "", false)
	if dialogResult.dialogSubmitted {
		domainName := dialogResult.inputAnswer
		side := m.CurrSide()
		viewConfig := m.ViewConfig(side)
		err := dp.Repo.CreateDomain(domainName)
		if err != nil {
			return err
		}

		updateStatusf("Domain '%s' created.", domainName)
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
				fmt.Sprintf("Confirm deletion of '%s' (%s) at '%s' (y/ya/n/na): ",
					item.Name, item.Config.Type.UserFriendlyString(), viewConfig.Path), "", false)
			if dialogResult.dialogSubmitted {
				confirmResponse = dialogResult.inputAnswer
			}
		}
		if confirmResponse == "y" || confirmResponse == "ya" {
			res, err := repos[m.CurrSide()].Delete(viewConfig, item.Config.Type, viewConfig.Path, item.Name)
			switch {
			case err != nil:
				updateStatus(err.Error())
			case res:
				updateStatusf("Successfully deleted '%s' (%s).",
					item.Name, item.Config.Type.UserFriendlyString())
			default:
				updateStatusf("Couldn't delete '%s' (%s).",
					item.Name, item.Config.Type.UserFriendlyString())
			}
			showItem(side, viewConfig, ".")
		} else {
			updateStatusf("Canceled deleting of '%s' (%s).",
				item.Name, item.Config.Type.UserFriendlyString())
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

func filterItems(m *model.Model) {
	cf := m.CurrentFilter()
	dialogResult := askUserInput("Filter by: ", cf, false)
	m.SetCurrentFilter(dialogResult.inputAnswer)
}

func searchItem(m *model.Model) {
	m.SearchBy = ""
	dialogResult := askUserInput("Search by: ", m.SearchBy, false)
	m.SearchBy = dialogResult.inputAnswer
	if m.SearchBy != "" {
		found := m.SearchNext(m.SearchBy)
		if !found {
			notFoundStatus := fmt.Sprintf("Item '%s' not found.", m.SearchBy)
			updateStatus(notFoundStatus)
		}
	}
}

func searchNextItem(m *model.Model, reverse bool) {
	if m.SearchBy == "" {
		dialogResult := askUserInput("Search by: ", m.SearchBy, false)
		m.SearchBy = dialogResult.inputAnswer
	}
	if m.SearchBy != "" {
		var found bool
		if reverse {
			found = m.SearchPrev(m.SearchBy)
		} else {
			found = m.SearchNext(m.SearchBy)
		}
		if !found {
			notFoundStatus := fmt.Sprintf("Item '%s' not found.", m.SearchBy)
			updateStatus(notFoundStatus)
		}
	}
}

// saveDataPowerConfig saves current's DataPower domain configuration.
func saveDataPowerConfig(m *model.Model) error {
	logging.LogDebug("ui/saveDataPowerConfig()")
	side := m.CurrSide()
	viewConfig := m.ViewConfig(side)

	if side == model.Left && viewConfig.DpDomain != "" {
		confirmSave := askUserInput(
			fmt.Sprintf("Are you sure you want to save current DataPower configuration for domain '%s' (y/n): ",
				viewConfig.DpDomain),
			"", false)

		if confirmSave.dialogCanceled || confirmSave.inputAnswer != "y" {
			return errs.Errorf("Canceled saving of DataPower configuration for domain '%s'.", viewConfig.DpDomain)
		}

		err := dp.Repo.SaveConfiguration(viewConfig)
		if err != nil {
			return err
		}
		currView := workingModel.ViewConfig(workingModel.CurrSide())
		showItem(workingModel.CurrSide(), currView, ".")
		updateStatusf("Domain '%s' saved.", viewConfig.DpDomain)
		return nil
	}

	return errs.Error("To save DataPower configuration select DataPower domain first.")
}

// showStatusMessages shows history of status messages in viewer program.
func showStatusMessages(statuses []string) error {
	statusesText := ""
	for _, status := range statuses {
		statusesText = statusesText + status + "\n"
	}
	return extprogs.View("Status_Messages", []byte(statusesText))
}

// syncModeToggle toggles sync mode (off <-> on). Sync mode is used to copy
// local changes to DataPower.
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
		} else {
			refreshStatus()
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

// toggleObjectMode switches between (default) filestore mode and object mode
// for DataPower view.
func toggleObjectMode(m *model.Model) error {
	logging.LogDebugf("worker/toggleObjectMode(), dp.Repo.ObjectConfigMode: %t", dp.Repo.ObjectConfigMode)

	side := m.CurrSide()
	switch side {
	case model.Left:
		dp.Repo.ObjectConfigMode = !dp.Repo.ObjectConfigMode
		oldView := m.ViewConfig(side)

		if dp.Repo.ObjectConfigMode {
			if oldView.DpDomain == "" {
				logging.LogDebug("worker/toggleObjectMode(), can't switch to object mode, oldView: %v.", oldView)
				dp.Repo.ObjectConfigMode = false
				return errs.Errorf("Can't show object view if DataPower domain is not selected.")
			}
			newView := model.ItemConfig{
				Parent:      oldView,
				Type:        model.ItemDpObjectClassList,
				Path:        "Object classes",
				DpAppliance: oldView.DpAppliance,
				DpDomain:    oldView.DpDomain,
				DpFilestore: oldView.DpFilestore}
			return showItem(model.Left, &newView, newView.Path)
		}

		switch oldView.Type {
		case model.ItemDpObjectClassList:
			m.NavCurrentViewBack(side)
		case model.ItemDpObjectClass:
			m.NavCurrentViewBack(side)
			m.NavCurrentViewBack(side)
		default:
			logging.LogDebugf("worker/toggleObjectMode(), currentView of unexpected type: %v", oldView)
			return errs.Error("Internal error occured while switching back to filestore mode.")
		}
		firstNonObjectView := m.ViewConfig(side)
		return showItem(model.Left, firstNonObjectView, ".")
	default:
		logging.LogDebug("worker/toggleObjectMode(), To toggle object mode select DataPower view.")
		return errs.Error("To toggle object mode select DataPower view.")
	}
}

// showItemInfo shows information about current item.
func showItemInfo(m *model.Model) error {
	logging.LogDebugf("worker/showItemInfo(), dp.Repo.ObjectConfigMode: %t", dp.Repo.ObjectConfigMode)

	if !dp.Repo.ObjectConfigMode {
		return errs.Error("Can't show info for DataPower object if object mode is not active.")
	}

	currentItem := m.CurrItem()

	switch currentItem.Config.Type {
	case model.ItemDpObject:
		infoBytes, err := dp.Repo.GetInfo(currentItem)
		if err != nil {
			return err
		}
		if infoBytes != nil {
			err = extprogs.View("*."+currentItem.Name, infoBytes)
			if err != nil {
				return err
			}
		} else {
			return errs.Errorf("Can't show info for '%s' object.", currentItem.Name)
		}
	default:
		return errs.Error("Can't show info for non DataPower object.")
	}

	return nil
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
		if respErr, ok := err.(errs.UnexpectedHTTPResponse); !ok || respErr.StatusCode != 404 {
			logging.LogDebug("worker/updateDpFile(), couldn't get dp file - err: ", err)
			return false
		}
	}

	if bytes.Compare(localBytes, dpBytes) != 0 {
		changesMade = true
		res, err := dp.SyncRepo.UpdateFileByPath(m.SyncDpDomain, dpPath, localBytes)
		if err != nil {
			logging.LogDebug("worker/updateDpFile(), couldn't update dp file - err: ", err)
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

func refreshStatus() {
	logging.LogDebugf("worker/refreshStatus()")
	updateView := events.UpdateViewEvent{
		Type: events.UpdateViewShowStatus, Status: workingModel.LastStatus(), Model: &workingModel}
	out.DrawEvent(updateView)
}

func showProgressDialog(msg string) {
	progressDialogSession.value = 0
	progressDialogSession.visible = true
	progressDialogSession.msg = msg
	go runProgressDialog()
}

func showProgressDialogf(format string, v ...interface{}) {
	showProgressDialog(fmt.Sprintf(format, v...))
}

func hideProgressDialog() {
	progressDialogSession.visible = false
}

func runProgressDialog() {
	for progressDialogSession.visible {
		updateProgressDialog()
		time.Sleep(1 * time.Second)
	}
}

func updateProgressDialog() {
	if !progressDialogSession.waitUserInput {
		progressEvent := events.UpdateViewEvent{Type: events.UpdateViewShowProgress,
			Message:  progressDialogSession.msg,
			Progress: progressDialogSession.value}
		out.DrawEvent(progressEvent)
		progressDialogSession.value = progressDialogSession.value + 1
		if progressDialogSession.value > 99 {
			progressDialogSession.value = 0
		}
	}
}

func updateProgressDialogMessagef(format string, v ...interface{}) {
	progressDialogSession.msg = fmt.Sprintf(format, v...)
}

// getFileTypedName converts name without suffix to name with suffix - used for tmp
// file naming. Proper tmp file name can enable viewer / editor (vim) to highlight syntax.
func getObjectTmpName(objectName string) string {
	switch dp.Repo.GetManagementInterface() {
	case config.DpInterfaceRest:
		return "*." + objectName + ".json"
	case config.DpInterfaceSoma:
		return "*." + objectName + ".xml"
	default:
		return objectName
	}

}
