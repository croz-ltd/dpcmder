// Package model contains data required to show and use terminal user interface.
package model

import (
	"fmt"
	"github.com/croz-ltd/dpcmder/utils/logging"
	"reflect"
	"sort"
	"strings"
)

// Side type is used for Left/Right constants.
type Side int

// Left and Right constants for addressing side in slices.
const (
	Left  = Side(0)
	Right = Side(1)
)

// maxStatusCount - maximum number of statuses we keep in history
const maxStatusCount = 1000

// ItemType is used for defining type of Item (or current "directory")
type ItemType byte

func (it ItemType) String() string {
	return string(it)
}

// UserFriendlyString ruturns ItemType as text understandable to user.
func (it ItemType) UserFriendlyString() string {
	switch it {
	case ItemDirectory:
		return "directory"
	case ItemFile:
		return "file"
	case ItemDpConfiguration:
		return "appliance configuration"
	case ItemDpDomain:
		return "domain"
	case ItemDpFilestore:
		return "filestore"
	case ItemDpObjectClassList:
		return "object class list"
	case ItemDpObjectClass:
		return "object class"
	case ItemDpObject:
		return "object"
	case ItemDpStatusClassList:
		return "status class list"
	case ItemDpStatusClass:
		return "status class"
	case ItemDpStatus:
		return "status"
	case ItemNone:
		return "-"
	default:
		return string(it)
	}
}

// Available types of Item
const (
	ItemDirectory         = ItemType('d')
	ItemFile              = ItemType('f')
	ItemDpConfiguration   = ItemType('A')
	ItemDpDomain          = ItemType('D')
	ItemDpFilestore       = ItemType('F')
	ItemDpObjectClassList = ItemType('L')
	ItemDpObjectClass     = ItemType('O')
	ItemDpObject          = ItemType('o')
	ItemDpStatusClassList = ItemType('l')
	ItemDpStatusClass     = ItemType('S')
	ItemDpStatus          = ItemType('s')
	ItemNone              = ItemType('-')
	ItemAny               = ItemType('*')
)

// Item contains information about File, Directory, DataPower filestore,
// DataPower domain or DataPower configuration which is shown on screen.
type Item struct {
	Name     string
	Size     string
	Modified string
	Selected bool
	Config   *ItemConfig
}

// ItemConfig contains information about File, Directory, DataPower filestore,
// DataPower domain or DataPower configuration which is required to uniquely
// identify Item.
type ItemConfig struct {
	Type          ItemType
	Name          string
	Path          string
	DpAppliance   string
	DpDomain      string
	DpFilestore   string
	DpObjectState ItemDpObjectState
	Parent        *ItemConfig
}

// ItemDpObjectState contains info about DataPower object state.
type ItemDpObjectState struct {
	OpState     string
	AdminState  string
	EventCode   string
	ErrorCode   string
	ConfigState string
}

// ItemList is slice extended as a sortable list of Items (implements sort.Interface).
type ItemList []Item

// Model is a structure representing our dpcmder view of files,
// both left-side DataPower view and right-side local filesystem view.
type Model struct {
	// viewConfig          [2]*ItemConfig
	viewConfigHistory   [2][]*ItemConfig
	viewConfigCurrIdx   [2]int
	title               [2]string
	items               [2]ItemList
	allItems            [2]ItemList
	currentFilter       [2]string
	currItemIdx         [2]int
	currFirstRowItemIdx [2]int
	currSide            Side
	ItemMaxRows         int
	HorizScroll         int
	SearchBy            string
	SyncModeOn          bool
	SyncInitial         bool
	SyncDpDomain        string
	SyncDirDp           string
	SyncDirLocal        string
	statuses            []string
}

// ViewMode represent one of available DataPower view modes.
type DpViewMode byte

// Available DataPower view modes.
const (
	DpFilestoreMode = DpViewMode('f')
	DpObjectMode    = DpViewMode('o')
	DpStatusMode    = DpViewMode('s')
)

// DpViewMode methods
func (v DpViewMode) String() string {
	return string(v)
}

// ItemConfig methods

// String method returns ItemConfig details.
func (ic ItemConfig) String() string {
	return fmt.Sprintf("IC(%s, '%s' (%s), '%s' (%s) %s %v %v)",
		ic.Type, ic.Name, ic.Path, ic.DpAppliance, ic.DpDomain, ic.DpFilestore,
		ic.DpObjectState, ic.Parent)
}

// Equals method returns true if other object is refering to same ItemConfig.
func (ic ItemConfig) Equals(other *ItemConfig) bool {
	if other == nil {
		return false
	}
	return ic.Path == other.Path && ic.DpAppliance == other.DpAppliance &&
		ic.DpDomain == other.DpDomain && ic.DpFilestore == other.DpFilestore
}

// DpViewMode returns which DataPower view mode ItemConfig contains.
// Should be called only for DataPower config, all local file will
// return DpFilestoreMode.
func (ic ItemConfig) DpViewMode() DpViewMode {
	switch ic.Type {
	case ItemDpObjectClassList, ItemDpObjectClass:
		return DpObjectMode
	case ItemDpStatusClassList, ItemDpStatusClass:
		return DpStatusMode
	default:
		return DpFilestoreMode
	}
}

// ItemDpObjectState methods

// String method returns ItemDpObjectState details.
func (idos ItemDpObjectState) String() string {
	return fmt.Sprintf("IDOS('%s'/'%s', '%s'/'%s' (%s))",
		idos.OpState, idos.AdminState, idos.EventCode, idos.ErrorCode,
		idos.ConfigState)
}

// Item methods

// String method returns Item details.
func (item Item) String() string {
	return fmt.Sprintf("Item('%s', '%s', '%s', %t, %s)",
		item.Name, item.Size, item.Modified, item.Selected, item.Config)
}

// DisplayString method returns formatted string representing how item will be shown.
func (item Item) DisplayString() string {
	return fmt.Sprintf("%s %10s %19s %s",
		item.GetDisplayableType(), item.Size, item.Modified, item.Name)
}

// GetDisplayableType retuns single character string representation of Item type.
func (item Item) GetDisplayableType() string {
	return string(item.Config.Type)
}

// ItemList methods (implements sort.Interface)

// Len returns number of items in ItemList.
func (items ItemList) Len() int {
	return len(items)
}

// Less returns true if item at first index should be before second one.
func (items ItemList) Less(i, j int) bool {
	switch {
	case items[i].Name == ".." && items[j].Name != "..":
		return true
	case items[i].Name != ".." && items[j].Name == "..":
		return false
	case items[i].Config.Type == ItemDirectory && items[j].Config.Type == ItemFile:
		return true
	case items[i].Config.Type == ItemFile && items[j].Config.Type == ItemDirectory:
		return false
	case items[i].Config.Type == items[j].Config.Type:
		return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
	default:
		return false
	}
}

// Swap swaps items with given indices.
func (items ItemList) Swap(i, j int) {
	reflect.Swapper(items)(i, j)
}

// Model methods

// SetItems changes list of items for given side.
func (m *Model) SetItems(side Side, items []Item) {
	m.currFirstRowItemIdx[side] = 0
	m.allItems[side] = items
	m.items[side] = items
	m.applyFilter(side)
}

// GetVisibleItemCount returns number of items which will be shown for given side.
func (m *Model) GetVisibleItemCount(side Side) int {
	visibleItemCount := len(m.items[side])
	logging.LogTracef("model/GetVisibleItemCount(%v), visibleItemCount: %d, m.ItemMaxRows: %d",
		side, visibleItemCount, m.ItemMaxRows)
	if m.ItemMaxRows < visibleItemCount {
		return m.ItemMaxRows
	}
	return visibleItemCount
}

// GetVisibleItem returns (visible) item from given side at given index.
func (m *Model) GetVisibleItem(side Side, rowIdx int) Item {
	itemIdx := rowIdx + m.currFirstRowItemIdx[side]
	logging.LogTracef("model/GetVisibleItem(), rowIdx: %d, itemIdx: %d", rowIdx, itemIdx)

	item := m.items[side][itemIdx]

	return item
}

// IsSelectable returns true if we can select current item
// (can select directory, file or appliance config).
func (m *Model) IsSelectable() bool {
	switch m.CurrItem().Config.Type {
	case ItemFile, ItemDirectory, ItemDpConfiguration:
		return true
	default:
		return false
	}
}

// CurrSide returns currently used Side.
func (m *Model) CurrSide() Side {
	return m.currSide
}

// OtherSide returns currently non-used Side.
func (m *Model) OtherSide() Side {
	return 1 - m.currSide
}

// Title returns title for given Side.
func (m *Model) Title(side Side) string {
	return m.title[side]
}

// SetTitle sets title for given Side.
func (m *Model) SetTitle(side Side, title string) {
	m.title[side] = title
}

// ViewConfig returns view config for given Side.
func (m *Model) ViewConfig(side Side) *ItemConfig {
	logging.LogDebugf("model/ViewConfig(%v), history len: %d, history idx: %d",
		side, len(m.viewConfigHistory[side]), m.viewConfigCurrIdx[side])
	return m.ViewConfigFromHistory(side, m.viewConfigCurrIdx[side])
}

// ViewConfigFromHistory returns view config for given Side and history position.
func (m *Model) ViewConfigFromHistory(side Side, idx int) *ItemConfig {
	logging.LogDebugf("model/ViewConfigFromHistory(%v, %d), history len: %d",
		side, idx, len(m.viewConfigHistory[side]))
	for i, viewConfig := range m.viewConfigHistory[side] {
		logging.LogTracef("model/ViewConfigFromHistory(%v, %d), config: %v",
			side, i, viewConfig)
	}
	switch {
	case len(m.viewConfigHistory[side]) < idx+1 || idx < 0:
		return nil
	default:
		return m.viewConfigHistory[side][idx]
	}
}

// ViewConfigHistorySize returns current size of view history.
func (m *Model) ViewConfigHistorySize(side Side) int {
	return len(m.viewConfigHistory[side])
}

// ViewConfigHistoryList returns all view history.
func (m *Model) ViewConfigHistoryList(side Side) []*ItemConfig {
	return m.viewConfigHistory[side]
}

// ViewConfigHistorySelectedIdx returns index of current view from history.
func (m *Model) ViewConfigHistorySelectedIdx(side Side) int {
	return m.viewConfigCurrIdx[side]
}

// SetCurrentView sets title and view config for given Side - overwrites current
// view in view history.
func (m *Model) SetCurrentView(side Side, viewConfig *ItemConfig, viewTitle string) {
	logging.LogDebugf("model/SetCurrentView(%v, .., '%s'), view history size: %d, idx: %d",
		side, viewTitle, m.ViewConfigHistorySize(side), m.viewConfigCurrIdx[side])
	m.title[side] = viewTitle
	switch {
	case len(m.viewConfigHistory[side]) == 0:
		m.viewConfigHistory[side] = append(m.viewConfigHistory[side], viewConfig)
		m.viewConfigCurrIdx[side] = 0
	case len(m.viewConfigHistory[side]) < m.viewConfigCurrIdx[side]+1:
		m.viewConfigHistory[side] = append(m.viewConfigHistory[side], viewConfig)
	default:
		viewConfigOld := m.ViewConfigFromHistory(side, m.viewConfigCurrIdx[side])
		logging.LogTracef("model/SetCurrentView(), viewConfig: %v, viewConfigOld: %v",
			viewConfig, viewConfigOld)
		if viewConfig != viewConfigOld {
			m.viewConfigHistory[side][m.viewConfigCurrIdx[side]] = viewConfig
			// reslice  a slice to remove "old view history" if we set current view
			// somewhere int the middle of view history.
			m.viewConfigHistory[side] = m.viewConfigHistory[side][:m.viewConfigCurrIdx[side]+1]
		}
	}
	logging.LogTracef("model/SetCurrentView(), view history size after: %d, idx: %d",
		m.ViewConfigHistorySize(side), m.viewConfigCurrIdx[side])
}

// AddNextView sets title and add next view config for given Side - appends new
// view to view history.
func (m *Model) AddNextView(side Side, viewConfig *ItemConfig, viewTitle string) {
	prevViewConfig := m.ViewConfigFromHistory(side, m.viewConfigCurrIdx[side]-1)
	switch viewConfig {
	case prevViewConfig:
		m.NavCurrentViewBack(side)
	default:
		m.viewConfigCurrIdx[side]++
	}
	m.SetCurrentView(side, viewConfig, viewTitle)
}

// NavCurrentViewIdx sets the current view to the view at given index.
func (m *Model) NavCurrentViewIdx(side Side, idx int) *ItemConfig {
	switch {
	case idx < 0:
		m.viewConfigCurrIdx[side] = 0
	case idx >= len(m.viewConfigHistory[side]):
		m.viewConfigCurrIdx[side] = len(m.viewConfigHistory[side]) - 1
	default:
		m.viewConfigCurrIdx[side] = idx
	}

	return m.ViewConfig(side)
}

// NavCurrentViewBack sets the current view to the previous one from view
// history (if previous view exists).
func (m *Model) NavCurrentViewBack(side Side) *ItemConfig {
	return m.NavCurrentViewIdx(side, m.viewConfigCurrIdx[side]-1)
}

// NavCurrentViewForward sets the current view to the next one from view
// history (if next view exists).
func (m *Model) NavCurrentViewForward(side Side) *ItemConfig {
	return m.NavCurrentViewIdx(side, m.viewConfigCurrIdx[side]+1)
}

// AddStatus adds new status event to history of statuses.
func (m *Model) AddStatus(status string) {
	m.statuses = append(m.statuses, status)
	overflowStatusCount := len(m.statuses) - maxStatusCount
	if overflowStatusCount > 0 {
		// m.statuses = m.statuses[overflowStatusCount:]
		m.statuses = m.statuses[overflowStatusCount:]
	}
}

// LastStatus returns last status event added to history.
func (m *Model) LastStatus() string {
	statusCount := len(m.statuses)
	if statusCount > 0 {
		return m.statuses[statusCount-1]
	}
	return ""
}

// Statuses returns history of all status events.
func (m *Model) Statuses() []string {
	return m.statuses
}

// IsCurrentSide returns true if given side is currently used.
func (m *Model) IsCurrentSide(side Side) bool {
	return side == m.currSide
}

// IsCurrentItem returns true if item at given idx and side is the current item under cursor.
func (m *Model) IsCurrentItem(side Side, itemIdx int) bool {
	return itemIdx == m.currItemIdx[side]
}

// IsCurrentRow returns true if row at given idx and side is the current row under cursor.
func (m *Model) IsCurrentRow(side Side, rowIdx int) bool {
	// TODO: update with vertical scrolling
	return rowIdx+m.currFirstRowItemIdx[side] == m.currItemIdx[side]
}

// CurrItemForSide returns current item under cursor for given side.
func (m *Model) CurrItemForSide(side Side) *Item {
	if len(m.items[side]) == 0 {
		return nil
	}
	return &m.items[side][m.currItemIdx[side]]
}

// SetCurrItemForSide sets current item under cursor for given side and item name.
func (m *Model) SetCurrItemForSide(side Side, itemName string) {
	itemIdx := 0
	for idx, item := range m.items[side] {
		if item.Name == itemName {
			itemIdx = idx
			break
		}
	}
	m.currItemIdx[side] = itemIdx
}

// SetCurrItemForSideAndConfig sets current item under cursor for Side to ItemConfig.
func (m *Model) SetCurrItemForSideAndConfig(side Side, config *ItemConfig) {
	itemIdx := 0
	for idx, item := range m.items[side] {
		if item.Config.Path == config.Path &&
			item.Config.DpDomain == config.DpDomain &&
			item.Config.DpAppliance == config.DpAppliance {
			itemIdx = idx
			break
		}
	}

	logging.LogDebugf("model/SetCurrItemForSideAndConfig(%v, %v), itemIdx: %v",
		side, config, itemIdx)
	m.currItemIdx[side] = itemIdx
	m.navUpDown(side, 0)
}

// CurrItem returns current item under cursor for used side.
func (m *Model) CurrItem() *Item {
	return m.CurrItemForSide(m.currSide)
}

func (m *Model) applyFilter(side Side) {
	filterString := m.currentFilter[side]
	allItems := m.allItems[side]
	if filterString != "" {
		filteredItems := make([]Item, 0)
		searchStr := strings.ToLower(filterString)
		for _, item := range allItems {
			itemStr := strings.ToLower(item.DisplayString())
			if strings.Contains(itemStr, searchStr) || item.Name == ".." {
				filteredItems = append(filteredItems, item)
			}
		}
		m.items[side] = filteredItems
	} else {
		m.items[side] = m.allItems[side]
	}

	// Make sure we are not pointing outside filtered array
	if m.currItemIdx[m.currSide] >= len(m.items[side]) {
		m.currItemIdx[side] = 0
	}
	if m.currFirstRowItemIdx[side] >= len(m.items[side]) {
		m.currFirstRowItemIdx[side] = 0
	}
}

// ResizeView ensures view is showing proper items after resizing and current
// item is visible.
func (m *Model) ResizeView() {
	logging.LogDebugf("model/ResizeView()")

	for _, side := range []Side{Left, Right} {
		itemCount := len(m.items[side])
		maxRows := m.GetVisibleItemCount(side)
		currIdx := m.currItemIdx[side]

		if m.currFirstRowItemIdx[side]+maxRows > itemCount {
			m.currFirstRowItemIdx[side] = itemCount - maxRows
		}
		if currIdx < m.currFirstRowItemIdx[side] {
			m.currFirstRowItemIdx[side] = currIdx
		}
		if currIdx > m.currFirstRowItemIdx[side]+maxRows-1 {
			m.currFirstRowItemIdx[side] = currIdx - maxRows + 1
		}
		if m.currFirstRowItemIdx[side] < 0 {
			m.currFirstRowItemIdx[side] = 0
		}
	}
}

// SetCurrentFilter sets filter string to apply to current side.
func (m *Model) SetCurrentFilter(filterString string) {
	m.currentFilter[m.currSide] = filterString
	m.applyFilter(m.currSide)
}

// CurrentFilter returns filter string applied to current side.
func (m *Model) CurrentFilter() string {
	return m.currentFilter[m.currSide]
}

// ToggleCurrItem toggles selection of current item under cursor.
func (m *Model) ToggleCurrItem() {
	if m.IsSelectable() {
		item := m.CurrItem()
		item.Selected = !item.Selected
	}
}

// ToggleSide switch usage from one side to other.
func (m *Model) ToggleSide() {
	if m.currSide == Left {
		m.currSide = Right
	} else {
		m.currSide = Left
	}
}

// NavUp moves cursor one item up if possible.
func (m *Model) NavUp() {
	m.navUpDown(m.currSide, -1)
}

// NavDown moves cursor one item down if possible.
func (m *Model) NavDown() {
	m.navUpDown(m.currSide, 1)
}

// NavPgUp moves cursor one page up if possible.
func (m *Model) NavPgUp() {
	m.navUpDown(m.currSide, -m.GetVisibleItemCount(m.currSide)+1)
}

// NavPgDown moves cursor one page down if possible.
func (m *Model) NavPgDown() {
	m.navUpDown(m.currSide, m.GetVisibleItemCount(m.currSide)-1)
}

// NavTop moves cursor to first item for current side.
func (m *Model) NavTop() {
	m.NavTopForSide(m.currSide)
}

// NavTopForSide moves cursor to first item for Side.
func (m *Model) NavTopForSide(side Side) {
	m.navUpDown(side, -len(m.items[m.currSide]))
}

// NavBottom moves cursor to last item.
func (m *Model) NavBottom() {
	m.navUpDown(m.currSide, len(m.items[m.currSide]))
}

func (m *Model) selRange(firstIdx, lastIdx int) {
	if m.IsSelectable() {
		newSelected := !m.CurrItem().Selected
		if firstIdx < 0 {
			firstIdx = 0
		}
		if lastIdx > len(m.items[m.currSide])-1 {
			lastIdx = len(m.items[m.currSide]) - 1
		}
		for idx := firstIdx; idx <= lastIdx; idx++ {
			m.items[m.currSide][idx].Selected = newSelected
		}
	}
}

// SelPgUp selects all items one page up from current one.
func (m *Model) SelPgUp() {
	firstIdx := m.currItemIdx[m.currSide] - m.GetVisibleItemCount(m.currSide) + 2
	lastIdx := m.currItemIdx[m.currSide]
	m.selRange(firstIdx, lastIdx)
}

// SelPgDown selects all items one page down from current one.
func (m *Model) SelPgDown() {
	firstIdx := m.currItemIdx[m.currSide]
	lastIdx := m.currItemIdx[m.currSide] + m.GetVisibleItemCount(m.currSide) - 2
	m.selRange(firstIdx, lastIdx)
}

// SelToTop selects all items from current one to first one.
func (m *Model) SelToTop() {
	lastIdx := m.currItemIdx[m.currSide]
	m.selRange(0, lastIdx)
}

// SelToBottom selects all items from current one to last one.
func (m *Model) SelToBottom() {
	firstIdx := m.currItemIdx[m.currSide]
	lastIdx := len(m.items[m.currSide]) - 1
	m.selRange(firstIdx, lastIdx)
}

func (m *Model) navUpDown(side Side, move int) {
	newCurr := m.currItemIdx[side] + move
	logging.LogDebugf(
		"model/navUpDown(), side: %d, move: %d, newCurr: %d, m.currFirstRowItemIdx[side]: %d",
		side, move, newCurr, m.currFirstRowItemIdx[side])

	if newCurr < 0 {
		newCurr = 0
	} else if newCurr >= len(m.items[side]) {
		newCurr = len(m.items[side]) - 1
	}

	maxRows := m.GetVisibleItemCount(side)
	minIdx := m.currFirstRowItemIdx[side]
	maxIdx := m.currFirstRowItemIdx[side] + maxRows - 1
	if newCurr > maxIdx {
		m.currFirstRowItemIdx[side] = newCurr - maxRows + 1
	} else if newCurr < minIdx {
		m.currFirstRowItemIdx[side] = newCurr
	}
	logging.LogTracef(
		"model/navUpDown(), newCurr: %d, minIdx: %d, maxIdx: %d, maxRows: %d, m.currFirstRowItemIdx[side]: %d",
		newCurr, minIdx, maxIdx, maxRows, m.currFirstRowItemIdx[side])

	m.currItemIdx[side] = newCurr
}

// SortSide sorts all items in given side.
func (m *Model) SortSide(side Side) {
	sort.Sort(m.items[side])
}

// GetSelectedItems returns all selected items for given side.
// It skips parent directories since we don't want to perform any
// actions (except navigation) on parent directory.
func (m *Model) GetSelectedItems(side Side) []Item {
	var selItems = make([]Item, 0)
	for _, item := range m.items[side] {
		if item.Selected && item.Name != ".." {
			selItems = append(selItems, item)
		}
	}
	return selItems
}

// SearchNext moves cursor to next item containing given searchStr and returns true if item is found.
func (m *Model) SearchNext(searchStr string) bool {
	logging.LogDebugf("model/SearchNext('%s')", searchStr)
	side := m.CurrSide()
	nextItemIdx := m.currItemIdx[side] + 1
	if nextItemIdx >= len(m.items[side]) {
		nextItemIdx = len(m.items[side]) - 1
	}
	searchStr = strings.ToLower(searchStr)
	for idx := nextItemIdx; idx < len(m.items[side]); idx++ {
		item := m.items[side][idx]
		itemStr := strings.ToLower(item.DisplayString())
		if strings.Contains(itemStr, searchStr) {
			move := idx - m.currItemIdx[side]
			m.navUpDown(side, move)
			return move != 0
		}
	}
	return false
}

// SearchPrev moves cursor to previous item containing given searchStr
// and returns true if item is found.
func (m *Model) SearchPrev(searchStr string) bool {
	logging.LogDebugf("model/SearchPrev('%s')", searchStr)
	side := m.CurrSide()
	prevItemIdx := m.currItemIdx[side] - 1
	if prevItemIdx < 0 {
		prevItemIdx = 0
	}
	searchStr = strings.ToLower(searchStr)
	for idx := prevItemIdx; idx >= 0; idx-- {
		item := m.items[side][idx]
		// for idx, item := range m.items[side][nextItemIdx:] {
		itemStr := strings.ToLower(item.DisplayString())
		if strings.Contains(itemStr, searchStr) {
			move := idx - m.currItemIdx[side]
			m.navUpDown(side, move)
			return move != 0
		}
	}
	return false
}
