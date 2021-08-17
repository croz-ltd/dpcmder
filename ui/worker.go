package ui

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

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
  "github.com/gdamore/tcell/v2"
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
	err := dp.Repo.InitNetworkSettings(
		config.CurrentApplianceName, config.CurrentAppliance)
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
		case c == 'n', c == 'N':
			searchNextItem(&workingModel, c == 'N')
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
		case c == 'P':
			err = showObjectDetails(&workingModel)
		case c == 'h':
			err = extprogs.ShowHelp()

		default:
			err = extprogs.ShowHelp()
			updateStatusf("Key event value (before showing help): '%#v'", event)
		}
	case *tcell.EventResize:
		workingModel.ResizeView()
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
			runeIdx++
		}
		if answerLen == runeIdx && runeIdx == dialogSession.inputAnswerCursorIdx {
			changedAnswer = changedAnswer + string(c)
		}
		dialogSession.inputAnswer = changedAnswer
		dialogSession.inputAnswerCursorIdx++
	case k == tcell.KeyEsc:
		logging.LogDebugf("ui/processInputDialogInput() canceling user input: '%s'", dialogSession)
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
				runeIdx++
			}
			dialogSession.inputAnswer = changedAnswer
			dialogSession.inputAnswerCursorIdx--
		}
	case k == tcell.KeyDelete:
		if dialogSession.inputAnswerCursorIdx < utf8.RuneCountInString(dialogSession.inputAnswer) {
			changedAnswer := ""
			runeIdx := 0
			for _, runeVal := range dialogSession.inputAnswer {
				if runeIdx != dialogSession.inputAnswerCursorIdx {
					changedAnswer = changedAnswer + string(runeVal)
				}
				runeIdx++
			}
			dialogSession.inputAnswer = changedAnswer
		}
	case k == tcell.KeyLeft:
		if dialogSession.inputAnswerCursorIdx > 0 {
			dialogSession.inputAnswerCursorIdx--
		}
	case k == tcell.KeyRight:
		if dialogSession.inputAnswerCursorIdx < utf8.RuneCountInString(dialogSession.inputAnswer) {
			dialogSession.inputAnswerCursorIdx++
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

// showItem lists items in given view and select current item if possible. If
// changeHistory is false we just refresh current view without changing history.
// Current item is the item currently under cursor.
// Values for the currentItemName can be:
// - "" - don't know current item name - the current item by viewItemName
// - other - set currentItemName as the current item
// Values for the viewItemName (view containing new list of items) can be:
// - "." - refresh current view - keep the current item
// - ".." - go to parent view - use the previous view as the current item
// - other - adding new view - set the first item as the current item
func showView(side model.Side, itemConfig *model.ItemConfig,
	viewItemName, currentItemName string, changeHistory bool) error {
	logging.LogDebugf("ui/showView(%d, %v, '%s', '%s')",
		side, itemConfig, viewItemName, currentItemName)
	if itemConfig.Type == model.ItemDpConfiguration {
		applianceName := viewItemName
		if applianceName != ".." && applianceName != "." && applianceName != "" {
			applicanceConfig := config.Conf.DataPowerAppliances[applianceName]
			dpTransientPassword := config.DpTransientPasswordMap[applianceName]
			logging.LogDebugf("ui/showView(), applicanceConfig: '%s'", applicanceConfig)
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
		model.ItemDpStatusClassList, model.ItemDpStatusClass,
		model.ItemNone:
		itemList, err = r.GetList(itemConfig)
		if err != nil {
			return err
		}
	default:
		return errs.Errorf("ui/showView(), unknown type: %s", itemConfig.Type)
	}

	logging.LogDebug("ui/showView(), itemList: ", itemList)
	title := r.GetTitle(itemConfig)
	logging.LogDebug("ui/showView(), title: ", title)

	oldViewConfig := workingModel.ViewConfig(side)
	workingModel.SetItems(side, itemList)
	// If we are refreshing current view, showing a view from history
	// or navigating to previous/next view from history - don't
	// change view history.
	// If we are entering new directory/appliance/... add new view to history.
	switch changeHistory {
	case true:
		histIdx := workingModel.ViewConfigHistorySelectedIdx(side)
		prevView := workingModel.ViewConfigFromHistory(side, histIdx-1)
		nextView := workingModel.ViewConfigFromHistory(side, histIdx+1)
		logging.LogTracef("ui/showView(), itemConfig: %v", itemConfig)
		logging.LogTracef("ui/showView(), prevView: %v", prevView)
		logging.LogTracef("ui/showView(), nextView: %v", nextView)
		// If we navigate "manually" to prev/next view in history - don't change it.
		switch {
		case itemConfig.Equals(prevView):
			workingModel.NavCurrentViewBack(side)
			workingModel.SetTitle(side, title)
		case itemConfig.Equals(nextView):
			workingModel.NavCurrentViewForward(side)
			workingModel.SetTitle(side, title)
		default:
			workingModel.AddNextView(side, itemConfig, title)
		}
	default:
		workingModel.SetCurrentView(side, itemConfig, title)
	}

	if currentItemName != "" {
		workingModel.SetCurrItemForSide(side, currentItemName)
		return nil
	}

	switch viewItemName {
	case "..":
		workingModel.SetCurrItemForSideAndConfig(side, oldViewConfig)
	case ".":
		oldCurrItem := workingModel.CurrItemForSide(side)
		workingModel.SetCurrItemForSide(side, oldCurrItem.Name)
	default:
		workingModel.NavTopForSide(side)
	}

	return nil
}

func showItem(side model.Side, itemConfig *model.ItemConfig, itemName string) error {
	logging.LogDebugf("ui/showItem(%d, %v, '%s')", side, itemConfig, itemName)
	return showView(side, itemConfig, itemName, "", itemName != ".")
}

// showPrevView navigate to the previous view from the view history.
func showPrevView() error {
	logging.LogDebugf("ui/showPrevView()")
	side := workingModel.CurrSide()
	oldView := workingModel.ViewConfig(side)

	newView := workingModel.NavCurrentViewBack(side)
	logging.LogDebugf("ui/showPrevView(), side: %v, oldView: %v, newView: %s",
		side, oldView, newView)

	// If previous view in history requires filestore/object mode, switch mode.
	switch {
	case side == model.Left && dp.Repo.DpViewMode == model.DpObjectMode &&
		newView.Type != model.ItemDpObjectClassList &&
		newView.Type != model.ItemDpObjectClass:
		dp.Repo.DpViewMode = model.DpFilestoreMode
	case side == model.Left && dp.Repo.DpViewMode == model.DpStatusMode &&
		newView.Type != model.ItemDpStatusClassList &&
		newView.Type != model.ItemDpStatusClass:
		dp.Repo.DpViewMode = model.DpObjectMode
	}

	if newView == oldView {
		updateStatusf("Can't move back from the first view in the history.")
	}

	return showView(side, newView, "", oldView.Name, false)
}

// showNextView navigate to the next view from the view history.
func showNextView() error {
	logging.LogDebugf("ui/showNextView()")
	side := workingModel.CurrSide()
	oldView := workingModel.ViewConfig(side)

	newView := workingModel.NavCurrentViewForward(side)
	logging.LogDebugf("ui/showNextView(), side: %v, oldView: %v, newView: %s",
		side, oldView, newView)

	// If next view in history requires object/status mode, switch mode.
	switch {
	case side == model.Left && dp.Repo.DpViewMode == model.DpFilestoreMode &&
		newView.Type == model.ItemDpObjectClassList:
		dp.Repo.DpViewMode = model.DpObjectMode
	case side == model.Left && dp.Repo.DpViewMode == model.DpObjectMode &&
		newView.Type == model.ItemDpStatusClassList:
		dp.Repo.DpViewMode = model.DpStatusMode
	}

	if newView == oldView {
		updateStatusf("Can't move forward from the last view in the history.")
	}

	viewHistoryIdx := workingModel.ViewConfigHistorySelectedIdx(side)
	nextViewFromHistory := workingModel.ViewConfigFromHistory(side, viewHistoryIdx+1)
	nextViewName := ""
	if nextViewFromHistory != nil {
		nextViewName = nextViewFromHistory.Name
	}
	return showView(side, newView, "", nextViewName, false)
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
		list:         pathHistory,
		selectionIdx: workingModel.ViewConfigHistorySelectedIdx(side)}

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
		// If proper mode for new view (object mode vs filestore mode).
		if side == model.Left {
			switch newView.Type {
			case model.ItemDpObjectClassList, model.ItemDpObjectClass:
				dp.Repo.DpViewMode = model.DpObjectMode
			case model.ItemDpStatusClassList, model.ItemDpStatusClass:
				dp.Repo.DpViewMode = model.DpStatusMode
			default:
				dp.Repo.DpViewMode = model.DpFilestoreMode
			}
		}
		showView(side, newView, ".", "", false)
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
	case k == tcell.KeyEsc, c == 'H':
		logging.LogDebugf("ui/processSelectListDialogInput() canceling selection: '%s'", dialogSession)
		dialogSession.dialogCanceled = true
	case k == tcell.KeyEnter:
		logging.LogDebugf("ui/processSelectListDialogInput() accepting selection: '%s'", dialogSession)
		dialogSession.dialogSubmitted = true
	case k == tcell.KeyUp, c == 'i':
		if dialogSession.selectionIdx > 0 {
			dialogSession.selectionIdx--
		}
	case k == tcell.KeyDown, c == 'k':
		if dialogSession.selectionIdx < len(dialogSession.list)-1 {
			dialogSession.selectionIdx++
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
	viewConfig := m.ViewConfig(side)
	logging.LogDebugf("ui/refreshView() viewConfig: %v", viewConfig)
	err := showView(side, viewConfig, ".", "", false)
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
	if ci.Name == ".." {
		return errs.Errorf("Can't view parent directory '%s'.", ci.Name)
	}

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
	case model.ItemDpStatus:
		statusIdx, err := strconv.Atoi(ci.Config.Path)
		if err != nil {
			return err
		}
		statusContent, err :=
			dp.Repo.GetStatus(ci.Config.DpDomain, ci.Config.Parent.Name, statusIdx)
		if err != nil {
			return err
		}
		err = extprogs.View(getObjectTmpName(ci.Name), statusContent)
		if err != nil {
			return err
		}
	case model.ItemDpStatusClass:
		statusesContent, err :=
			dp.Repo.GetStatuses(ci.Config.DpDomain, ci.Config.Name)
		if err != nil {
			return err
		}
		err = extprogs.View(getObjectTmpName(ci.Name), statusesContent)
		if err != nil {
			return err
		}
	default:
		return errs.Errorf("Can't view item '%s' (%s)",
			ci.Name, ci.Config.Type.UserFriendlyString())
	}

	return err
}

func editCurrent(m *model.Model) error {
	ci := m.CurrItem()
	logging.LogDebugf("ui/editCurrent(), item: %v", ci)
	if ci.Name == ".." {
		return errs.Errorf("Can't edit parent directory '%s'.", ci.Name)
	}
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
	if dpItem.Name == ".." {
		return errs.Errorf("Can't diff dp parent directory '%s',", dpItem.Name)
	}
	localItem := m.CurrItemForSide(model.Right)
	if localItem.Name == ".." && dpItem.Config.Type != model.ItemDpObject {
		return errs.Errorf("Can't diff local parent directory '%s',", localItem.Name)
	}

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
		if m.CurrItem().Name != ".." {
			selectedItems = append(selectedItems, *m.CurrItem())
		}
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
		switch {
		case toRepo.String() == dp.Repo.String() && dp.Repo.DpViewMode == model.DpObjectMode:
			res, err = copyFileToObject(item.Config, item.Name, fromRepo, toRepo, fromViewConfig, toViewConfig, confirmOverwrite)
		case toRepo.String() == dp.Repo.String() && dp.Repo.DpViewMode == model.DpStatusMode:
			err = errs.Errorf("Can't copy to DataPower status.")
		default:
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
		logging.LogDebugf("ui/exportAppliance(), after asking password (%#v).", dialogResult)
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
	if currentItem.Name == ".." {
		return errs.Errorf("Can't clone parent directory '%s',", currentItem.Name)
	}

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
		var err error
		confirmResponse, err = deleteItem(repos[m.CurrSide()], viewConfig, item, confirmResponse)

		if err != nil {
			return err
		}
	}
	showItem(side, viewConfig, ".")

	return nil
}

func deleteItem(repo repo.Repo, parentItemConfig *model.ItemConfig, item model.Item, confirmResponse string) (string, error) {
	logging.LogDebugf("ui/deleteItem(%v, '%v')", item, confirmResponse)

	var confirmMsg string
	var successMsg string
	var errorMsg string
	var cancelMsg string
	switch item.Config.Type {
	case model.ItemDirectory, model.ItemFile, model.ItemDpConfiguration, model.ItemDpObject:
		if item.Name == ".." {
			return confirmResponse,
				errs.Errorf("Won't delete parent item '%s' (%s) at '%s', aborting...",
					item.Name, item.Config.Type.UserFriendlyString(), parentItemConfig.Path)
		}
		confirmMsg =
			fmt.Sprintf("Confirm deletion of '%s' (%s) at '%s' (y/ya/n/na): ",
				item.Name, item.Config.Type.UserFriendlyString(), parentItemConfig.Path)
		successMsg = fmt.Sprintf("Successfully deleted '%s' (%s).",
			item.Name, item.Config.Type.UserFriendlyString())
		errorMsg = fmt.Sprintf("Couldn't delete '%s' (%s).",
			item.Name, item.Config.Type.UserFriendlyString())
		cancelMsg = fmt.Sprintf("Canceled deleting of '%s' (%s).",
			item.Name, item.Config.Type.UserFriendlyString())
	case model.ItemDpStatusClass:
		switch item.Name {
		case "StylesheetCachingSummary", "DocumentCachingSummary",
			"DocumentCachingSummaryGlobal":
			confirmMsg =
				fmt.Sprintf("Confirm flushing all '%s' caches (y/ya/n/na): ",
					item.Name)
			successMsg = fmt.Sprintf("Successfully flushed caches '%s'.", item.Name)
			errorMsg = fmt.Sprintf("Couldn't flush caches '%s'", item.Name)
			cancelMsg = fmt.Sprintf("Canceled flushing caches '%s'.", item.Name)
		default:
			return confirmResponse,
				errs.Errorf("Can't flush caches '%s'.", item.Name)
		}
	case model.ItemDpStatus:
		switch parentItemConfig.Path {
		case "StylesheetCachingSummary", "DocumentCachingSummary",
			"DocumentCachingSummaryGlobal":
			confirmMsg =
				fmt.Sprintf("Confirm flushing cache of '%s' (%s) (y/ya/n/na): ",
					item.Name, parentItemConfig.Path)
			successMsg = fmt.Sprintf("Successfully flushed cache '%s'.", item.Name)
			errorMsg = fmt.Sprintf("Couldn't flush cache '%s'.", item.Name)
			cancelMsg = fmt.Sprintf("Canceled flushing cache of '%s'.", item.Name)
		default:
			return confirmResponse,
				errs.Errorf("Can't flush cache '%s' (%s) at '%s'.",
					item.Name, item.Config.Type.UserFriendlyString(), parentItemConfig.Path)
		}
	default:
		return confirmResponse,
			errs.Errorf("Can't delete '%s' (%s) at '%s'",
				item.Name, item.Config.Type.UserFriendlyString(), parentItemConfig.Path)
	}

	if confirmResponse != "ya" && confirmResponse != "na" {
		confirmResponse = "n"
		dialogResult := askUserInput(confirmMsg, "", false)
		if dialogResult.dialogSubmitted {
			confirmResponse = dialogResult.inputAnswer
		}
	}
	if confirmResponse == "y" || confirmResponse == "ya" {
		var res bool
		var err error
		switch item.Config.Type {
		case model.ItemDirectory, model.ItemFile, model.ItemDpConfiguration, model.ItemDpObject:
			res, err = repo.Delete(parentItemConfig, item.Config.Type, parentItemConfig.Path, item.Name)
		case model.ItemDpStatusClass:
			res, err = dp.Repo.FlushCache(
				parentItemConfig.DpDomain, item.Name, "", item.Config.Type)
		case model.ItemDpStatus:
			res, err = dp.Repo.FlushCache(
				parentItemConfig.DpDomain, parentItemConfig.Path, item.Name, item.Config.Type)
		default:
			return confirmResponse,
				errs.Errorf("Can't delete '%s' (%s) at '%s'",
					item.Name, item.Config.Type.UserFriendlyString(), parentItemConfig.Path)
		}
		switch {
		case err != nil:
			updateStatus(err.Error())
		case res:
			updateStatus(successMsg)
		default:
			updateStatus(errorMsg)
		}
	} else {
		updateStatus(cancelMsg)
	}

	return confirmResponse, nil
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
		return showView(side, newViewConfig, path, "", true)
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
			dp.SyncRepo.InitNetworkSettings(
				dpApplianceName, config.Conf.DataPowerAppliances[dpApplianceName])
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

// toggleObjectMode switches between (default) filestore mode, object mode and
// status mode for the DataPower view.
func toggleObjectMode(m *model.Model) error {
	logging.LogDebugf("worker/toggleObjectMode(), dp.Repo.DpViewMode: %s", dp.Repo.DpViewMode)

	side := m.CurrSide()
	switch side {
	case model.Left:
		oldView := m.ViewConfig(side)

		if oldView.DpDomain == "" {
			logging.LogDebugf("worker/toggleObjectMode(), can't switch to non-filestore mode, oldView: %v.", oldView)
			dp.Repo.DpViewMode = model.DpFilestoreMode
			return errs.Errorf("Can't show object or status view if DataPower domain is not selected.")
		}

		switch dp.Repo.DpViewMode {
		case model.DpFilestoreMode:
			dp.Repo.DpViewMode = model.DpObjectMode
		case model.DpObjectMode:
			dp.Repo.DpViewMode = model.DpStatusMode
		case model.DpStatusMode:
			dp.Repo.DpViewMode = model.DpFilestoreMode
		default:
			dp.Repo.DpViewMode = model.DpFilestoreMode
		}

		// When we switch to object config mode - add/open object class list.
		// When we switch to status mode - add/open status class list.
		switch dp.Repo.DpViewMode {
		case model.DpObjectMode:
			newView := model.ItemConfig{
				Parent:      oldView,
				Name:        "Object classes",
				Type:        model.ItemDpObjectClassList,
				Path:        "Object classes",
				DpAppliance: oldView.DpAppliance,
				DpDomain:    oldView.DpDomain,
				DpFilestore: oldView.DpFilestore}
			return showItem(model.Left, &newView, newView.Path)
		case model.DpStatusMode:
			newView := model.ItemConfig{
				Parent:      oldView,
				Name:        "Status classes",
				Type:        model.ItemDpStatusClassList,
				Path:        "Status classes",
				DpAppliance: oldView.DpAppliance,
				DpDomain:    oldView.DpDomain,
				DpFilestore: oldView.DpFilestore}
			return showItem(model.Left, &newView, newView.Path)
		case model.DpFilestoreMode:
			// When we switch from status mode navigate back to first filestore mode.
			ic := m.NavCurrentViewBack(side)
			for ic.DpViewMode() != model.DpFilestoreMode {
				ic = m.NavCurrentViewBack(side)
			}
			firstNonObjectView := m.ViewConfig(side)
			return showItem(model.Left, firstNonObjectView, ".")
		default:
			logging.LogDebugf("worker/toggleObjectMode(), unknown view mode %v.",
				dp.Repo.DpViewMode)
			return errs.Errorf("Unknown view mode %v.", dp.Repo.DpViewMode)
		}

	default:
		logging.LogDebug("worker/toggleObjectMode(), To toggle object mode select DataPower view.")
		return errs.Error("To toggle object mode select DataPower view.")
	}
}

// showItemInfo shows information about current item.
func showItemInfo(m *model.Model) error {
	logging.LogDebugf("worker/showItemInfo(), dp.Repo.DpViewMode: %s", dp.Repo.DpViewMode)

	currentItem := m.CurrItem()
	if currentItem.Name == ".." {
		return errs.Errorf("Can't show info for parent directory '%s'.", currentItem.Name)
	}

	switch currentItem.Config.Type {
	case model.ItemDpObject, model.ItemFile, model.ItemDirectory:
		infoBytes, err := repos[m.CurrSide()].GetItemInfo(currentItem.Config)
		if err != nil {
			return err
		}
		if infoBytes != nil {
			err = extprogs.View("*."+currentItem.Name, infoBytes)
			if err != nil {
				return err
			}
		} else {
			return errs.Errorf("Can't find info for '%s' item.", currentItem.Name)
		}
	default:
		return errs.Errorf("Can't show info for item '%s' of type %s.",
			currentItem.Name, currentItem.Config.Type.UserFriendlyString())
	}

	return nil
}

// showObjectDetails shows details (service, policy, matches, rules & actions)
// for the current object.
func showObjectDetails(m *model.Model) error {
	logging.LogDebugf("worker/showObjectPolicy(), dp.Repo.ObjectConfigMode: %s",
		dp.Repo.DpViewMode)

	if dp.Repo.DpViewMode != model.DpObjectMode {
		return errs.Error("Can't show policy for DataPower object if object mode is not active.")
	}

	currentItem := m.CurrItem()
	if currentItem.Name == ".." {
		return errs.Errorf("Can't show policy for parent directory '%s'.", currentItem.Name)
	}

	switch currentItem.Config.Type {
	case model.ItemDpObject:
		switch currentItem.Config.Path {
		case "B2BProfile",
			"MultiProtocolGateway",
			"WSGateway",
			"XMLFirewallService",
			"XSLProxyService",
			"WebAppFW",
			"WebTokenService",
			"WSStylePolicy",
			"StylePolicy",
			"Matching",
			"StylePolicyRule",
			"WSStylePolicyRule",
			"RequestStylePolicyRule",
			"ResponseStylePolicyRule",
			"ErrorStylePolicyRule":
		default:
			return errs.Errorf("Can't show policy for object of class '%s'.",
				currentItem.Config.Path)
		}

		updateStatusf("Fetching policy for object '%s' (%s) from domain '%s'.",
			currentItem.Config.Name, currentItem.Config.Path,
			currentItem.Config.DpDomain)
		showProgressDialogf("Exporting object '%s' (%s) from domain '%s'...",
			currentItem.Config.Name, currentItem.Config.Path,
			currentItem.Config.DpDomain)
		objectInfoBytes, err :=
			dp.Repo.GetObjectDetails(currentItem.Config.DpDomain,
				currentItem.Config.Path, currentItem.Config.Name)
		hideProgressDialog()
		if err != nil {
			return err
		}
		if objectInfoBytes != nil {
			err = extprogs.View("*."+currentItem.Name, objectInfoBytes)
			if err != nil {
				return err
			}
		} else {
			return errs.Errorf("Can't show policy info for '%s' object.", currentItem.Name)
		}
	default:
		return errs.Error("Can't show policy info for non DataPower object.")
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
		progressDialogSession.value++
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
