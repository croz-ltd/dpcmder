package view

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"fmt"
	"github.com/croz-ltd/dpcmder/config"
	"github.com/croz-ltd/dpcmder/extprogs"
	"github.com/croz-ltd/dpcmder/help"
	"github.com/croz-ltd/dpcmder/model"
	"github.com/croz-ltd/dpcmder/repo"
	"github.com/croz-ltd/dpcmder/repo/dp"
	"github.com/croz-ltd/dpcmder/repo/localfs"
	"github.com/croz-ltd/dpcmder/utils/logging"
	"github.com/croz-ltd/dpcmder/view/key"
	"github.com/nsf/termbox-go"
	"os"
	"strings"
	"time"
)

const (
	fgNormal   = termbox.ColorDefault
	fgSelected = termbox.ColorRed
	bgNormal   = termbox.ColorDefault
	bgCurrent  = termbox.ColorGreen
)

var (
	repos                = []repo.Repo{model.Left: &dp.Repo, model.Right: &localfs.Repo}
	currentStatus string = ""
	horizScroll          = 0
	searchBy      string = ""
	syncModeOn    bool   = false
	syncDomainDp  string
	syncDirDp     string
	syncDirLocal  string
	syncInitial   bool = true
)

func Init() {
	err := termbox.Init()
	if err != nil {
		panic(err)
	}
	defer termbox.Close()

	m := &model.M

	repos[model.Left].InitialLoad(m)
	repos[model.Right].InitialLoad(m)

	setScreenSize(m)
	draw(m)

	keyPressedLoop(m)
}

func keyPressedLoop(m *model.Model) {
	var bytesRead = make([]byte, 6)
	reader := bufio.NewReader(os.Stdin)

loop:
	for {
		currentStatus = ""
		setScreenSize(m)
		bytesReadCount, err := reader.Read(bytesRead)
		if err != nil {
			logging.LogFatal(err)
		}
		hexBytesRead := hex.EncodeToString(bytesRead[0:bytesReadCount])

		switch hexBytesRead {
		case key.Chq:
			break loop
		case key.Return:
			enterCurrentDirectory(m)
		case key.Tab:
			m.ToggleSide()
		case key.Space:
			m.ToggleCurrItem()
		case key.Dot:
			enterPath(m)
		case key.ArrowLeft, key.Chj:
			horizScroll -= 10
			if horizScroll < 0 {
				horizScroll = 0
			}
		case key.ArrowRight, key.Chl:
			horizScroll += 10
		case key.ArrowUp, key.Chi:
			m.NavUp()
		case key.ArrowDown, key.Chk:
			m.NavDown()
		case key.ShiftArrowUp, key.ChI:
			m.ToggleCurrItem()
			m.NavUp()
		case key.ShiftArrowDown, key.ChK:
			m.ToggleCurrItem()
			m.NavDown()
		case key.PgUp, key.Chu:
			m.NavPgUp()
		case key.PgDown, key.Cho:
			m.NavPgDown()
		case key.ShiftPgUp, key.ChU:
			m.SelPgUp()
			m.NavPgUp()
		case key.ShiftPgDown, key.ChO:
			m.SelPgDown()
			m.NavPgDown()
		case key.Home, key.Cha:
			m.NavTop()
		case key.End, key.Chz:
			m.NavBottom()
		case key.ShiftHome, key.ChA:
			m.SelToTop()
			m.NavTop()
		case key.ShiftEnd, key.ChZ:
			m.SelToBottom()
			m.NavBottom()
		case key.Chf:
			cf := m.CurrentFilter()
			cf = userInput(m, fmt.Sprintf("Filter by: "), cf)
			m.SetCurrentFilter(cf)
		case key.Slash:
			searchBy = ""
			searchBy = userInput(m, fmt.Sprintf("Search by: "), searchBy)
			if searchBy != "" {
				found := m.SearchNext(searchBy)
				if !found {
					currentStatus = fmt.Sprintf("Item '%s' not found.", searchBy)
				}
			}
		case key.Chn:
			if searchBy == "" {
				searchBy = userInput(m, fmt.Sprintf("Search by: "), searchBy)
			}
			found := m.SearchNext(searchBy)
			if !found {
				currentStatus = fmt.Sprintf("Next item '%s' not found.", searchBy)
			}
		case key.Chp:
			if searchBy == "" {
				searchBy = userInput(m, fmt.Sprintf("Search by: "), searchBy)
			}
			found := m.SearchPrev(searchBy)
			if !found {
				currentStatus = fmt.Sprintf("Previous item '%s' not found.", searchBy)
			}
		case key.F2, key.Ch2:
			repos[m.CurrSide()].LoadCurrent(m)
			currentStatus = "Current directory refreshed."
		case key.F3, key.Ch3:
			viewCurrent(m)
		case key.F4, key.Ch4:
			editCurrent(m)
		case key.F5, key.Ch5:
			copyCurrent(m)
		case key.F7, key.Ch7:
			createDirectory(m)
		case key.Del, key.Chx:
			deleteCurrent(m)
		case key.Chs:
			syncModeToggle(m)
		default:
			help.Show()
			currentStatus = fmt.Sprintf("Key pressed hex value (before showing help): '%s'", hexBytesRead)
		}

		draw(m)
	}
}

func setScreenSize(m *model.Model) {
	width, height := termbox.Size()
	m.SetItemsMaxSize(height-3, width)
}

func draw(m *model.Model) {
	termbox.Clear(fgNormal, bgNormal)
	width, _ := termbox.Size()
	if m.IsCurrentSide(model.Left) {
		writeLine(0, 0, m.Title(model.Left), fgNormal, bgCurrent)
		writeLine(width/2, 0, m.Title(model.Right), fgNormal, bgNormal)
	} else {
		writeLine(0, 0, m.Title(model.Left), fgNormal, bgNormal)
		writeLine(width/2, 0, m.Title(model.Right), fgNormal, bgCurrent)
	}

	for idx := 0; idx < m.GetVisibleItemCount(model.Left); idx++ {
		item := m.GetVisibleItem(model.Left, idx)
		var fg = fgNormal
		var bg = bgNormal
		if m.IsCurrentRow(model.Left, idx) {
			bg = bgCurrent
		}
		if item.Selected {
			fg = fgSelected
		}
		writeLine(0, idx+2, item.String(), fg, bg)
	}
	for idx := 0; idx < m.GetVisibleItemCount(model.Right); idx++ {
		item := m.GetVisibleItem(model.Right, idx)
		var fg = fgNormal
		var bg = bgNormal
		if m.IsCurrentRow(model.Right, idx) {
			bg = bgCurrent
		}
		if item.Selected {
			fg = fgSelected
		}
		writeLine(width/2, idx+2, item.String(), fg, bg)
	}

	showStatus(m, currentStatus)

	termbox.Flush()
}

func userInputBoth(m *model.Model, question string, initialValue string, valueVisible bool) string {
	x, y := 0, 1
	w, _ := termbox.Size()
	if m.CurrSide() == model.Right {
		x = w / 2
	}
	cellx := writeLine(x, y, question, termbox.ColorDefault, termbox.ColorDefault)
	// Maximum length of question which can be visible on screen
	maxQlen := w - cellx
	termbox.Flush()
	// TODO: check if we pass len(rb)!!!
	var rb = make([]byte, 200)
	copy(rb[:], initialValue)
	// Show initial value
	writeLineWithCursor(cellx, y, initialValue, termbox.ColorDefault, termbox.ColorDefault, cellx+len(initialValue), termbox.AttrReverse, termbox.AttrReverse)

	termbox.Flush()
	rbIdx := len(initialValue)
	rbLen := len(initialValue)
loop:
	for {
		var bytesRead = make([]byte, 200)
		reader := bufio.NewReader(os.Stdin)
		bytesReadCount, err := reader.Read(bytesRead)
		if err != nil {
			logging.LogFatal(err)
		}
		hexBytesRead := hex.EncodeToString(bytesRead[0:bytesReadCount])
		logging.LogDebug("hexBytesRead: ", hexBytesRead)

		switch hexBytesRead {
		case key.Return:
			break loop
		case key.Backspace, key.BackspaceWin:
			// Remove character before cursor
			if rbIdx > 0 {
				rbSuffix := make([]byte, rbLen-rbIdx)
				copy(rbSuffix[:], rb[rbIdx:rbLen])
				copy(rb[rbIdx-1:], rbSuffix[:])
				rb[rbLen-1] = 0
				rbIdx--
				rbLen--
				if rbIdx < 0 {
					rbIdx = 0
				}
				if rbLen < 0 {
					rbLen = 0
				}
			}
		case key.Del:
			// Remove character at cursor
			if rbIdx < rbLen {
				rbSuffix := make([]byte, rbLen-rbIdx-1)
				copy(rbSuffix[:], rb[rbIdx+1:rbLen])
				copy(rb[rbIdx:], rbSuffix[:])
				rb[rbLen-1] = 0
				rbLen--
				if rbLen < 0 {
					rbLen = 0
				}
			}
		case key.ArrowLeft:
			rbIdx--
			if rbIdx < 0 {
				rbIdx = 0
			}
		case key.ArrowRight:
			rbIdx++
			if rbIdx > rbLen {
				rbIdx = rbLen
			}
		case key.Esc:
			return ""
		default:
			// Insert string in middle of current string
			rbSuffix := make([]byte, rbLen-rbIdx)
			copy(rbSuffix[:], rb[rbIdx:rbLen])
			copy(rb[rbIdx:rbIdx+bytesReadCount], bytesRead)
			copy(rb[rbIdx+bytesReadCount:], rbSuffix[:])
			rbIdx += bytesReadCount
			rbLen += bytesReadCount
			if rbLen > len(rb) {
				rbLen = len(rb)
			}
			if rbIdx > rbLen {
				rbIdx = rbLen
			}
		}

		firstQLetterIdx := 0
		// Make sure cursor is visible
		if rbIdx > maxQlen-2 {
			firstQLetterIdx = rbIdx - maxQlen + 2
		}
		s := string(rb[firstQLetterIdx:])
		if !valueVisible {
			s = strings.Repeat("*", rbLen)
		}
		// logging.LogDebug("---cellx: ", cellx, ", rbidx: ", rbIdx, ", firstQLetterIdx: ", firstQLetterIdx)
		writeLineWithCursor(cellx, y, s, termbox.ColorDefault, termbox.ColorDefault, cellx+rbIdx-firstQLetterIdx, termbox.AttrReverse, termbox.AttrReverse)
		termbox.Flush()
	}

	return strings.Trim(string(rb), "\x00")
}
func userInput(m *model.Model, question string, initialValue string) string {
	return userInputBoth(m, question, initialValue, true)
}
func userInputPassword(m *model.Model, question string, initialValue string) string {
	return userInputBoth(m, question, initialValue, false)
}

func writeLine(x, y int, line string, fg, bg termbox.Attribute) int {
	return writeLineWithCursor(x, y, line, fg, bg, -1, fg, bg)
}

func writeLineWithCursor(x, y int, line string, fg, bg termbox.Attribute, cursorX int, cursorFg, cursorBg termbox.Attribute) int {
	var scrolledLine string
	var scrollh = horizScroll
	if len(line) < scrollh {
		scrollh = len(line)
		if scrollh < 0 {
			scrollh = 0
		}
	}
	scrolledLine = line[scrollh:]
	var xpos int
	for i := 0; i < len(scrolledLine); i++ {
		xpos = x + i
		if cursorX == xpos {
			termbox.SetCell(xpos, y, rune(scrolledLine[i]), cursorFg, cursorBg)
		} else {
			termbox.SetCell(xpos, y, rune(scrolledLine[i]), fg, bg)
		}
	}
	// logging.LogDebug("x: ", x, ", scrolledLine: ", scrolledLine, ", len(scrolledLine): ", len(scrolledLine))
	// logging.LogDebug("cursorX: ", cursorX, "xpos: ", xpos)
	if cursorX > xpos {
		termbox.SetCell(cursorX, y, rune(' '), cursorFg, cursorBg)
	} else if cursorX == -2 {
		// logging.LogDebug("------ ", "xpos+1: ", (xpos + 1))
		termbox.SetCell(xpos+1, y, rune(' '), cursorFg, cursorBg)
	}
	return xpos + 1
}

func enterCurrentDirectory(m *model.Model) {
	currItem := m.CurrItem()
	if currItem.IsDirectory() || currItem.IsDpAppliance() || currItem.IsDpDomain() || currItem.IsDpFilestore() {
		canContinue := true
		missingPassword := repos[m.CurrSide()].EnterCurrentDirectoryMissingPassword(m)
		if missingPassword {
			if m.CurrSide() == model.Left {
				selectedDpAppliance := m.CurrItemForSide(model.Left).Name
				question := fmt.Sprintf("Enter password for %s@%s: ", config.Conf.DataPowerAppliances[selectedDpAppliance].Username, selectedDpAppliance)
				answer := userInputPassword(m, question, "")
				canContinue = repos[m.CurrSide()].EnterCurrentDirectorySetPassword(m, answer)
			} else {
				logging.LogFatal("Don't expect password for localfs.")
			}
		}

		if canContinue {
			repos[m.CurrSide()].EnterCurrentDirectory(m)
			m.SetCurrentFilter("")
		}
	}
}

func viewCurrent(m *model.Model) {
	ci := m.CurrItem()
	logging.LogDebug("view.viewCurrent(), item: ", ci, ", type: ", ci.Type)
	if ci.IsFile() {
		fileContent := repos[m.CurrSide()].GetFile(m, m.CurrPath(), ci.Name)
		if fileContent != nil {
			extprogs.View(ci.Name, fileContent)
		} else {
			currentStatus = fmt.Sprintf("Can't fetch file '%s' from path '%s'.", ci.Name, m.CurrPath())
		}
	} else if ci.IsDpAppliance() {
		extprogs.View(ci.Name, config.Conf.GetDpApplianceConfig(ci.Name))
	}
}

func editCurrent(m *model.Model) {
	if m.CurrItem().IsFile() {
		fileName := m.CurrItem().Name
		fileContent := repos[m.CurrSide()].GetFile(m, m.CurrPath(), fileName)
		changed, newFileContent := extprogs.Edit(m.CurrItem().Name, fileContent)
		if changed {
			repos[m.CurrSide()].UpdateFile(m, m.CurrPath(), fileName, newFileContent)
			repos[m.CurrSide()].LoadCurrent(m)
		}
	}
}

func copyItem(m *model.Model, fromRepo, toRepo repo.Repo, fromBasePath, toBasePath string, item model.Item, confirmOverwrite string) string {
	if item.IsDirectory() {
		confirmOverwrite = copyDirs(m, fromRepo, toRepo, fromBasePath, toBasePath, item.Name, confirmOverwrite)
	} else if item.IsFile() {
		confirmOverwrite = copyFile(m, fromRepo, toRepo, fromBasePath, toBasePath, item.Name, confirmOverwrite)
	}
	return confirmOverwrite
}

func copyDirs(m *model.Model, fromRepo, toRepo repo.Repo, fromParentPath, toParentPath, dirName, confirmOverwrite string) string {
	fromPath := fromRepo.GetFilePath(fromParentPath, dirName)
	toPath := toRepo.GetFilePath(toParentPath, dirName)
	createDirSuccess := true
	if toRepo.GetFileType(m, toParentPath, dirName) == '0' {
		createDirSuccess = toRepo.CreateDir(m, toParentPath, dirName)
	} else if toRepo.GetFileType(m, toParentPath, dirName) == 'f' {
		currentStatus = fmt.Sprintf("ERROR: '%s' exists as file, can't create dir.", toPath)
	}

	if createDirSuccess {
		currentStatus = fmt.Sprintf("Directory '%s' created.", toPath)
		items := fromRepo.ListFiles(m, fromPath)
		for _, item := range items {
			confirmOverwrite = copyItem(m, fromRepo, toRepo, fromPath, toPath, item, confirmOverwrite)
		}
	} else {
		currentStatus = fmt.Sprintf("ERROR: Directory '%s' not created.", toPath)
	}

	return confirmOverwrite
}

func copyFile(m *model.Model, fromRepo, toRepo repo.Repo, fromParentPath, toParentPath, fileName, confirmOverwrite string) string {
	targetFileType := toRepo.GetFileType(m, toParentPath, fileName)
	logging.LogDebug(fmt.Sprintf("view.copyFile(.., .., .., %v, %v, %v, %v)\n\n", fromParentPath, toParentPath, fileName, confirmOverwrite))
	logging.LogDebug(fmt.Sprintf("view targetFileType: %s\n", string(targetFileType)))

	if targetFileType == 'd' {
		currentStatus = fmt.Sprintf("ERROR: File '%s' could not be copied from '%s' to '%s' - directory with same name exists.", fileName, fromParentPath, toParentPath)
	} else {
		if targetFileType == 'f' {
			if confirmOverwrite != "ya" && confirmOverwrite != "na" {
				confirmOverwrite = userInput(m, fmt.Sprintf("Confirm overwrite of file '%s' at '%s' (y/ya/n/na): ", fileName, toParentPath), "")
			}
		}

		fBytes := fromRepo.GetFile(m, fromParentPath, fileName)
		copySuccess := toRepo.UpdateFile(m, toParentPath, fileName, fBytes)
		logging.LogDebug(fmt.Sprintf("view copySuccess: %v\n", copySuccess))
		if copySuccess {
			currentStatus = fmt.Sprintf("File '%s' copied from '%s' to '%s'.", fileName, fromParentPath, toParentPath)
		} else {
			currentStatus = fmt.Sprintf("ERROR: File '%s' not copied from '%s' to '%s'.", fileName, fromParentPath, toParentPath)
		}
	}
	return confirmOverwrite
}

func copyCurrent(m *model.Model) {
	fromSide := m.CurrSide()
	toSide := m.OtherSide()

	fromBasePath := m.CurrPathForSide(fromSide)
	toBasePath := m.CurrPathForSide(toSide)

	itemsToCopy := getSelectedOrCurrent(m)
	currentStatus = fmt.Sprintf("Copy from '%s' to '%s', items: %v", fromBasePath, toBasePath, itemsToCopy)
	confirmOverwrite := "n"
	for _, item := range itemsToCopy {
		confirmOverwrite = copyItem(m, repos[fromSide], repos[toSide], fromBasePath, toBasePath, item, confirmOverwrite)
	}

	repos[toSide].LoadCurrent(m)
}

func currentPath(m *model.Model, fileName string) string {
	parentPath := m.CurrPath()
	return repos[m.CurrSide()].GetFilePath(parentPath, fileName)
}

func getSelectedOrCurrent(m *model.Model) []model.Item {
	selectedItems := m.GetSelectedItems(m.CurrSide())
	if len(selectedItems) == 0 {
		selectedItems = make([]model.Item, 0)
		selectedItems = append(selectedItems, *m.CurrItem())
	}

	return selectedItems
}

func deleteCurrent(m *model.Model) {
	selectedItems := getSelectedOrCurrent(m)

	confirmResponse := "n"
	for _, item := range selectedItems {
		if confirmResponse != "ya" && confirmResponse != "na" {
			confirmResponse = userInput(m, fmt.Sprintf("Confirm deletion of (%c) '%s' (y/ya/n/na): ", item.Type, item.Name), "")
		}
		if confirmResponse == "y" || confirmResponse == "ya" {
			if repos[m.CurrSide()].Delete(m, m.CurrPath(), item.Name) {
				currentStatus = "Successfully deleted file " + item.Name
			} else {
				currentStatus = "ERROR: couldn't delete file " + item.Name
			}
			repos[m.CurrSide()].LoadCurrent(m)
		} else {
			currentStatus = "Canceled deleting of file " + item.Name
		}
	}
}

func createDirectory(m *model.Model) {
	dirName := userInput(m, "Create directory: ", "")
	if dirName != "" {
		success := repos[m.CurrSide()].CreateDir(m, m.CurrPath(), dirName)
		if success {
			currentStatus = "Successfully created directory " + dirName
		} else {
			currentStatus = "ERROR: couldn't create directory " + dirName
		}
		repos[m.CurrSide()].LoadCurrent(m)
	} else {
		currentStatus = "Canceled creating directory " + dirName
	}
}

func showStatus(m *model.Model, status string) {
	var filterMsg string
	var syncMsg string
	if m.CurrentFilter() != "" {
		filterMsg = fmt.Sprintf("Filter: '%s' | ", m.CurrentFilter())
	}
	if syncModeOn {
		syncMsg = fmt.Sprintf("Sync: '%s' -> (%s) '%s' | ", syncDirLocal, syncDomainDp, syncDirDp)
	}

	statusMsg := fmt.Sprintf("%s%s%s", syncMsg, filterMsg, status)

	_, h := termbox.Size()
	writeLine(0, h-1, statusMsg, termbox.ColorDefault, termbox.ColorDefault)
	termbox.Flush()
}

func enterPath(m *model.Model) {
	if m.CurrSide() == model.Right {
		lPath := userInput(m, fmt.Sprintf("Enter path: "), m.CurrPath())
		if len(lPath) > 1 {
			lPath = strings.TrimRight(lPath, "/")
		}

		if lPath != "" {
			ft := repos[m.CurrSide()].GetFileTypeFromPath(m, lPath)
			switch ft {
			case 'd':
				m.SetCurrPath(lPath)
				repos[m.CurrSide()].LoadCurrent(m)
			case 'f':
				currentStatus = fmt.Sprintf("Can't show directory as given path '%s' is a file.", lPath)
			case '0':
				currentStatus = fmt.Sprintf("Can't show directory as given path '%s' doesn't exist.", lPath)
			}
		}
	} else {
		currentStatus = fmt.Sprintf("Path can be set only for local filesystem.")
	}
}

func syncModeToggle(m *model.Model) {
	var syncModeToggleConfirm string

	dpDomain := m.DpDomain()
	dpDir := m.CurrPathForSide(model.Left)

	if syncModeOn {
		syncModeToggleConfirm = userInput(m, fmt.Sprintf("Are you sure you want to disable sync mode (y/n): "), "")
	} else {
		if dpDomain != "" && dpDir != "" {
			syncModeToggleConfirm = userInput(m, fmt.Sprintf("Are you sure you want to enable sync mode (y/n): "), "")
		} else {
			currentStatus = fmt.Sprintf("Can't sync if DataPower domain (%s) or path (%s) are not selected.", dpDomain, dpDir)
		}
	}
	if syncModeToggleConfirm == "y" {
		syncModeOn = !syncModeOn
		if syncModeOn {
			syncDomainDp = dpDomain
			syncDirDp = dpDir
			syncDirLocal = m.CurrPathForSide(model.Right)
			syncInitial = true
			go syncLocalToDp(m)
		} else {
			syncDomainDp = ""
			syncDirDp = ""
			syncDirLocal = ""
		}
	}
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
	var treeOld localfs.LocalfsTree
	syncCheckTime := time.Duration(config.Conf.Sync.Seconds) * time.Second
	for syncModeOn {
		changesMade := false
		tree, err := localfs.Repo.LoadTree("", syncDirLocal)
		if err != nil {
			currentStatus = fmt.Sprintf("Sync err: %s.", err)
		}
		logging.LogDebug("syncLocalToDp(), tree: ", tree)

		if syncInitial {
			changesMade = syncLocalToDpInitial(&tree)
			logging.LogDebug("syncLocalToDp(), after initial sync - changesMade: ", changesMade)
			syncInitial = false
		} else {
			changesMade = syncLocalToDpLater(&tree, &treeOld)
			logging.LogDebug("syncLocalToDp(), after later sync - changesMade: ", changesMade)
		}

		treeOld = tree

		cnt++
		if changesMade {
			dp.Repo.LoadCurrent(m)
			draw(m)
		} else {
			// currentStatus = fmt.Sprintf("Sync cnt: %d.", cnt)
			showStatus(m, currentStatus)
			termbox.Flush()
		}
		time.Sleep(syncCheckTime)
	}
}

func syncLocalToDpInitial(tree *localfs.LocalfsTree) bool {
	changesMade := false
	logging.LogDebug(fmt.Sprintf("syncLocalToDpInitial(%v)", tree))
	m := &model.Model{}
	m.SetDpDomain(syncDomainDp)
	// logging.LogDebug("syncLocalToDpInitial(), syncDirDp: ", syncDirDp, ", tree.PathFromRoot: ", tree.PathFromRoot)

	if tree.Dir {
		dpPath := dp.Repo.GetFilePath(syncDirDp, tree.PathFromRoot)
		fileType := dp.Repo.GetFileTypeFromPath(m, dpPath)
		if fileType == '0' {
			dp.Repo.CreateDirByPath(m, dpPath)
			changesMade = true
		} else if fileType == 'f' {
			logging.LogFatal("In place of dir there is a file on dp: ", dpPath)
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

func updateDpFile(m *model.Model, tree *localfs.LocalfsTree) bool {
	changesMade := false
	localBytes := localfs.Repo.GetFileByPath(tree.Path)
	dpPath := dp.Repo.GetFilePath(syncDirDp, tree.PathFromRoot)
	dpBytes := dp.Repo.GetFileByPath(m, dpPath)

	if bytes.Compare(localBytes, dpBytes) != 0 {
		changesMade = true
		res := dp.Repo.UpdateFileByPath(m, dpPath, localBytes)
		if res {
			currentStatus = fmt.Sprintf("Dp '%s' updated.", dpPath)
		} else {
			currentStatus = fmt.Sprintf("Error updating '%s'.", dpPath)
		}
	}

	return changesMade
}

func syncLocalToDpLater(tree, treeOld *localfs.LocalfsTree) bool {
	changesMade := false
	logging.LogDebug(fmt.Sprintf("syncLocalToDpLater(%v, %v)", tree, treeOld))
	m := &model.Model{}
	m.SetDpDomain(syncDomainDp)

	if tree.Dir {
		if treeOld == nil {
			dpPath := dp.Repo.GetFilePath(syncDirDp, tree.PathFromRoot)
			fileType := dp.Repo.GetFileTypeFromPath(m, dpPath)
			if fileType == '0' {
				dp.Repo.CreateDirByPath(m, dpPath)
				changesMade = true
			} else if fileType == 'f' {
				logging.LogFatal("In place of dir there is a file on dp: ", dpPath)
			}
		}

		for _, child := range tree.Children {
			var childOld *localfs.LocalfsTree
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
