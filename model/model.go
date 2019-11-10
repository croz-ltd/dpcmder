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

// ItemType is used for defining type of Item (or current "directory")
type ItemType byte

func (it ItemType) String() string {
	return string(it)
}

// Available types of Item
const (
	ItemDirectory       = ItemType('d')
	ItemFile            = ItemType('f')
	ItemDpConfiguration = ItemType('A')
	ItemDpDomain        = ItemType('D')
	ItemDpFilestore     = ItemType('F')
	ItemNone            = ItemType('0')
)

// Item contains information about File, Directory, DataPower filestore,
// DataPower domain or DataPower configuration.
type Item struct {
	Type     ItemType
	Name     string
	Size     string
	Modified string
	Selected bool
}

// ItemList is slice extended as a sortable list of Items (implements sort.Interface).
type ItemList []Item

// CurrentView is a structure with information about currently showing view
// (DataPower or local filesystem panel). It can be one of: ItemDpConfiguration,
// ItemDpDomain, ItemDpFilestore, ItemDirectory or ItemNone.
type CurrentView struct {
	Type        ItemType
	Path        string
	DpAppliance string
	DpDomain    string
	DpFilestore string
}

func (cv CurrentView) String() string {
	return fmt.Sprintf("CurrentView(%s, '%s', '%s', '%s', '%s')", cv.Type, cv.Path, cv.DpAppliance, cv.DpDomain, cv.DpFilestore)
}

func (cv CurrentView) Clone() CurrentView {
	return CurrentView{Type: cv.Type, Path: cv.Path, DpAppliance: cv.DpAppliance, DpDomain: cv.DpDomain, DpFilestore: cv.DpFilestore}
}

// Model is a structure representing our dpcmder view of files,
// both left-side DataPower view and right-side local filesystem view.
type Model struct {
	currentView         [2]CurrentView
	title               [2]string
	items               [2]ItemList
	allItems            [2]ItemList
	currentFilter       [2]string
	currItemIdx         [2]int
	currFirstRowItemIdx [2]int
	currPath            [2]string
	currSide            Side
	screenSizeW         int
	screenSizeH         int
	itemMaxRows         int
	itemMaxCols         int
}

// Item methods

// String method returns Item details.
func (i Item) String() string {
	return fmt.Sprintf("Item(%s, '%s', '%s', '%s', %t)", i.Type, i.Name, i.Size, i.Modified, i.Selected)
}

// DisplayString method returns formatted string representing how item will be shown.
func (item *Item) DisplayString() string {
	return fmt.Sprintf("%s %10s %19s %s", item.GetDisplayableType(), item.Size, item.Modified, item.Name)
}

// GetDisplayableType retuns "f" for files, "" for configuration and "d" for all other.
func (item *Item) GetDisplayableType() string {
	if item.Type == ItemFile {
		return "f"
	} else if item.Type == ItemDirectory {
		return "d"
	} else {
		return ""
	}
}

// IsFile returns true if Item is a file.
func (item *Item) IsFile() bool {
	return item.Type == ItemFile
}

// IsDirectory returns true if Item is a directory.
func (item *Item) IsDirectory() bool {
	return item.Type == ItemDirectory
}

// IsDpAppliance returns true if Item is a DataPower appliance configuration.
func (item *Item) IsDpAppliance() bool {
	return item.Type == ItemDpConfiguration
}

// IsDpDomain returns true if Item is a DataPower domain.
func (item *Item) IsDpDomain() bool {
	return item.Type == ItemDpDomain
}

// IsDpFilestore returns true if Item is a DataPower filestore (cert:, local:,..).
func (item *Item) IsDpFilestore() bool {
	return item.Type == ItemDpFilestore
}

// ItemList methods (implements sort.Interface)

// Len returns number of items in ItemList.
func (items ItemList) Len() int {
	return len(items)
}

// Less returns true if item at first index should be before second one.
func (items ItemList) Less(i, j int) bool {
	return items[i].Type < items[j].Type ||
		(items[i].Type == items[j].Type &&
			strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name))
}

// Swap swaps items with given indices.
func (items ItemList) Swap(i, j int) {
	reflect.Swapper(items)(i, j)
}

// Model methods

// SetItems changes list of items for given side.
func (m *Model) SetItems(side Side, items []Item) {
	m.allItems[side] = items
	m.items[side] = items
	m.applyFilter(side)
}

// SetItemsMaxSize sets maximum number of items which can be shown on screen.
func (m *Model) SetItemsMaxSize(itemMaxRows, itemMaxCols int) {
	logging.LogTrace("model/SetItemsMaxSize(), itemMaxRows: ", itemMaxRows, ", itemMaxCols: ", itemMaxCols)
	m.itemMaxRows, m.itemMaxCols = itemMaxRows, itemMaxCols
}

// GetVisibleItemCount returns number of items which will be shown for given side.
func (m *Model) GetVisibleItemCount(side Side) int {
	visibleItemCount := len(m.items[side])
	logging.LogTrace("model/GetVisibleItemCount(", side, "), visibleItemCount: ", visibleItemCount, ", m.itemMaxRows: ", m.itemMaxRows)
	if m.itemMaxRows < visibleItemCount {
		return m.itemMaxRows
	} else {
		return visibleItemCount
	}
}

// GetVisibleItem returns (visible) item from given side at given index.
func (m *Model) GetVisibleItem(side Side, rowIdx int) Item {
	itemIdx := rowIdx + m.currFirstRowItemIdx[side]
	logging.LogTrace("model/GetVisibleItem(), rowIdx: ", rowIdx, ", itemIdx: ", itemIdx)

	item := m.items[side][itemIdx]

	return item
}

// IsSelectable returns true if we can select current item.
func (m *Model) IsSelectable() bool {
	return m.CurrPath() != ""
}

// CurrSide returns currently used Side.
func (m *Model) CurrSide() Side {
	return m.currSide
}

// CurrSide returns currently non-used Side.
func (m *Model) OtherSide() Side {
	return 1 - m.currSide
}

// CurrentView returns current view for given Side.
func (m *Model) CurrentView(side Side) CurrentView {
	return m.currentView[side]
}

// SetCurrentView sets current view for given Side.
func (m *Model) SetCurrentView(side Side, view CurrentView) {
	m.currentView[side] = view
}

// Title returns title for given Side.
func (m *Model) Title(side Side) string {
	return m.title[side]
}

// SetTitle sets title for given Side.
func (m *Model) SetTitle(side Side, title string) {
	m.title[side] = title
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
	return &m.items[side][m.currItemIdx[side]]
}

// SetCurrItemForSide sets current item under cursor for given side and item name.
func (m *Model) SetCurrItemForSide(side Side, itemName string) {
	itemIdx := 0
	for idx, item := range m.items[side] {
		// DebugInfo += "itemName: " + itemName + ", idx: " + strconv.Itoa(idx) + ", item.Name: " + item.Name + "\n"
		if item.Name == itemName {
			itemIdx = idx
			break
		}
	}
	m.currItemIdx[side] = itemIdx
}

// CurrItem returns current item under cursor for used side.
func (m *Model) CurrItem() *Item {
	return m.CurrItemForSide(m.currSide)
}

// CurrPathForSide returns current path for given side.
func (m *Model) CurrPathForSide(side Side) string {
	return m.currPath[side]
}

// CurrPath returns current path for used side.
func (m *Model) CurrPath() string {
	return m.CurrPathForSide(m.currSide)
}

// SetCurrPathForSide sets current path for given side.
func (m *Model) SetCurrPathForSide(side Side, newPath string) {
	m.currPath[side] = newPath
}

// SetCurrPath sets current path for used side.
func (m *Model) SetCurrPath(newPath string) {
	m.SetCurrPathForSide(m.currSide, newPath)
}

func (m *Model) applyFilter(side Side) {
	filterString := m.currentFilter[side]
	allItems := m.allItems[side]
	if filterString != "" {
		filteredItems := make([]Item, 0)
		searchStr := strings.ToLower(filterString)
		for _, item := range allItems {
			itemStr := strings.ToLower(item.String())
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

// NavTop moves cursor to first item.
func (m *Model) NavTop() {
	m.navUpDown(m.currSide, -len(m.items[m.currSide]))
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
	logging.LogTrace("model/navUpDown(), side: ", side, ", move: ", move, ", newCurr: ", newCurr, ", m.currFirstRowItemIdx[side]: ", m.currFirstRowItemIdx[side])

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
	logging.LogTrace("model/navUpDown(), newCurr: ", newCurr, ", minIdx: ", minIdx, ", maxIdx: ", maxIdx, ", maxRows: ", maxRows, ", m.currFirstRowItemIdx[side]: ", m.currFirstRowItemIdx[side])

	m.currItemIdx[side] = newCurr
}

// SortSide sorts all items in given side.
func (m *Model) SortSide(side Side) {
	sort.Sort(m.items[side])
}

// GetSelectedItems returns all selected items for given side.
func (m *Model) GetSelectedItems(side Side) []Item {
	var selItems = make([]Item, 0)
	for _, item := range m.items[side] {
		if item.Selected {
			selItems = append(selItems, item)
		}
	}
	return selItems
}

// SearchNext moves cursor to next item containing given searchStr and returns true if item is found.
func (m *Model) SearchNext(searchStr string) bool {
	side := m.CurrSide()
	nextItemIdx := m.currItemIdx[side] + 1
	if nextItemIdx >= len(m.items[side]) {
		nextItemIdx = len(m.items[side]) - 1
	}
	searchStr = strings.ToLower(searchStr)
	for idx := nextItemIdx; idx < len(m.items[side]); idx++ {
		item := m.items[side][idx]
		itemStr := strings.ToLower(item.String())
		if strings.Contains(itemStr, searchStr) {
			move := idx - m.currItemIdx[side]
			m.navUpDown(side, move)
			return move != 0
		}
	}
	return false
}

// SearchNext moves cursor to previous item containing given searchStr and returns true if item is found.
func (m *Model) SearchPrev(searchStr string) bool {
	side := m.CurrSide()
	prevItemIdx := m.currItemIdx[side] - 1
	if prevItemIdx < 0 {
		prevItemIdx = 0
	}
	searchStr = strings.ToLower(searchStr)
	for idx := prevItemIdx; idx >= 0; idx-- {
		item := m.items[side][idx]
		// for idx, item := range m.items[side][nextItemIdx:] {
		itemStr := strings.ToLower(item.String())
		if strings.Contains(itemStr, searchStr) {
			move := idx - m.currItemIdx[side]
			m.navUpDown(side, move)
			return move != 0
		}
	}
	return false
}
