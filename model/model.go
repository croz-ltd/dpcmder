package model

import (
	"fmt"
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

// Item contains information about File, Directory, DataPower filestore,
// DataPower domain or DataPower configuration.
type Item struct {
	Type     byte // d - directory; f - file; A - appliance configuration; D - domain; F - filestore
	Name     string
	Size     string
	Modified string
	Selected bool
}

// ItemList is slice extended as a sortable list of Items (implements sort.Interface).
type ItemList []Item

// Model is a structure representing our dpcmder view of files,
// both left-side DataPower view and right-side local filesystem view.
type Model struct {
	dpAppliance         string
	dpDomain            string
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

// String method returns formatted string representing how item will be shown.
func (item *Item) String() string {
	return fmt.Sprintf("%s %10s %19s %s", item.GetDisplayableType(), item.Size, item.Modified, item.Name)
}

// GetDisplayableType retuns "f" for files, "" for configuration and "d" for all other.
func (item *Item) GetDisplayableType() string {
	if item.Type == 'f' {
		return "f"
	} else if item.Type == 'd' {
		return "d"
	} else {
		return ""
	}
}

// IsFile returns true if Item is a file.
func (item *Item) IsFile() bool {
	return item.Type == 'f'
}

// IsDirectory returns true if Item is a directory.
func (item *Item) IsDirectory() bool {
	return item.Type == 'd'
}

// IsDpAppliance returns true if Item is a DataPower appliance configuration.
func (item *Item) IsDpAppliance() bool {
	return item.Type == 'A'
}

// IsDpDomain returns true if Item is a DataPower domain.
func (item *Item) IsDpDomain() bool {
	return item.Type == 'D'
}

// IsDpFilestore returns true if Item is a DataPower filestore (cert:, local:,..).
func (item *Item) IsDpFilestore() bool {
	return item.Type == 'F'
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

// DpAppliance returns name of configuration of current DataPower configuration.
func (m *Model) DpAppliance() string {
	return m.dpAppliance
}

// SetDpAppliance sets name of configuration of current DataPower configuration.
func (m *Model) SetDpAppliance(dpAppliance string) {
	m.dpAppliance = dpAppliance
}

// DpDomain returns name of current DataPower domain.
func (m *Model) DpDomain() string {
	return m.dpDomain
}

// SetDpDomain sets name of current DataPower domain.
func (m *Model) SetDpDomain(dpDomain string) {
	m.dpDomain = dpDomain
}

// SetItems changes list of items for given side.
func (m *Model) SetItems(side Side, items []Item) {
	m.allItems[side] = items
	m.items[side] = items
	m.applyFilter(side)
}

// SetItemsMaxSize sets maximum number of items which can be shown on screen.
func (m *Model) SetItemsMaxSize(itemMaxRows, itemMaxCols int) {
	m.itemMaxRows, m.itemMaxCols = itemMaxRows, itemMaxCols
}

// GetVisibleItemCount returns number of items which will be shown for given side.
func (m *Model) GetVisibleItemCount(side Side) int {
	visibleItemCount := len(m.items[side])
	if m.itemMaxRows < visibleItemCount {
		return m.itemMaxRows
	} else {
		return visibleItemCount
	}
}

// GetVisibleItem returns (visible) item from given side at given index.
func (m *Model) GetVisibleItem(side Side, rowIdx int) Item {
	itemIdx := rowIdx + m.currFirstRowItemIdx[side]

	item := m.items[side][itemIdx]

	return item
}

// IsSelectable returns true if we can select current item.
func (m *Model) IsSelectable() bool {
	return m.CurrPath() != ""
}

func (m *Model) CurrSide() Side {
	return m.currSide
}

func (m *Model) OtherSide() Side {
	return 1 - m.currSide
}

func (m *Model) Title(side Side) string {
	return m.title[side]
}

func (m *Model) SetTitle(side Side, title string) {
	m.title[side] = title
}

func (m *Model) IsCurrentSide(side Side) bool {
	return side == m.currSide
}

func (m *Model) IsCurrentItem(side Side, itemIdx int) bool {
	return itemIdx == m.currItemIdx[side]
}

func (m *Model) IsCurrentRow(side Side, rowIdx int) bool {
	// TODO: update with vertical scrolling
	return rowIdx+m.currFirstRowItemIdx[side] == m.currItemIdx[side]
}

func (m *Model) CurrItemForSide(side Side) *Item {
	return &m.items[side][m.currItemIdx[side]]
}

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

func (m *Model) CurrItem() *Item {
	return m.CurrItemForSide(m.currSide)
}

func (m *Model) CurrPathForSide(side Side) string {
	return m.currPath[side]
}

func (m *Model) CurrPath() string {
	return m.CurrPathForSide(m.currSide)
}

func (m *Model) SetCurrPathForSide(side Side, newPath string) {
	m.currPath[side] = newPath
}

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

func (m *Model) SetCurrentFilter(filterString string) {
	m.currentFilter[m.currSide] = filterString
	m.applyFilter(m.currSide)
}

func (m *Model) CurrentFilter() string {
	return m.currentFilter[m.currSide]
}

func (m *Model) ToggleCurrItem() {
	if m.IsSelectable() {
		item := m.CurrItem()
		item.Selected = !item.Selected
	}
}

func (m *Model) ToggleSide() {
	if m.currSide == Left {
		m.currSide = Right
	} else {
		m.currSide = Left
	}
}

func (m *Model) NavUp() {
	m.NavUpDown(m.currSide, -1)
}

func (m *Model) NavDown() {
	m.NavUpDown(m.currSide, 1)
}

func (m *Model) NavPgUp() {
	m.NavUpDown(m.currSide, -m.GetVisibleItemCount(m.currSide)+1)
}

func (m *Model) NavPgDown() {
	m.NavUpDown(m.currSide, m.GetVisibleItemCount(m.currSide)-1)
}

func (m *Model) NavTop() {
	m.NavUpDown(m.currSide, -len(m.items[m.currSide]))
}

func (m *Model) NavBottom() {
	m.NavUpDown(m.currSide, len(m.items[m.currSide]))
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

func (m *Model) SelPgUp() {
	firstIdx := m.currItemIdx[m.currSide] - m.GetVisibleItemCount(m.currSide) + 2
	lastIdx := m.currItemIdx[m.currSide]
	m.selRange(firstIdx, lastIdx)
}

func (m *Model) SelPgDown() {
	firstIdx := m.currItemIdx[m.currSide]
	lastIdx := m.currItemIdx[m.currSide] + m.GetVisibleItemCount(m.currSide) - 2
	m.selRange(firstIdx, lastIdx)
}

func (m *Model) SelToTop() {
	lastIdx := m.currItemIdx[m.currSide]
	m.selRange(0, lastIdx)
}

func (m *Model) SelToBottom() {
	firstIdx := m.currItemIdx[m.currSide]
	lastIdx := len(m.items[m.currSide]) - 1
	m.selRange(firstIdx, lastIdx)
}

func (m *Model) NavUpDown(side Side, move int) {
	newCurr := m.currItemIdx[side] + move

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

	m.currItemIdx[side] = newCurr
}

func (m *Model) SortSide(side Side) {
	sort.Sort(m.items[side])
}

func (m *Model) GetSelectedItems(side Side) []Item {
	var selItems = make([]Item, 0)
	for _, item := range m.items[side] {
		if item.Selected {
			selItems = append(selItems, item)
		}
	}
	return selItems
}

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
			m.NavUpDown(side, move)
			return move != 0
		}
	}
	return false
}

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
			m.NavUpDown(side, move)
			return move != 0
		}
	}
	return false
}

var M Model = Model{currSide: Left}
