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
	Left           = Side(0)
	Right          = Side(1)
	maxStatusCount = 1000
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
	ItemNone            = ItemType('-')
)

// Item contains information about File, Directory, DataPower filestore,
// DataPower domain or DataPower configuration.
type Item struct {
	Name     string
	Size     string
	Modified string
	Selected bool
	Config   *ItemConfig
}

type ItemConfig struct {
	Type        ItemType
	Path        string
	DpAppliance string
	DpDomain    string
	DpFilestore string
	Parent      *ItemConfig
}

// ItemList is slice extended as a sortable list of Items (implements sort.Interface).
type ItemList []Item

// Model is a structure representing our dpcmder view of files,
// both left-side DataPower view and right-side local filesystem view.
type Model struct {
	viewConfig          [2]*ItemConfig
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
	SyncDomainDp        string
	SyncDirDp           string
	SyncDirLocal        string
	statuses            []string
}

// ItemConfig methods

// String method returns ItemConfig details.
func (ic ItemConfig) String() string {
	return fmt.Sprintf("IC(%s, '%s', '%s' (%s) %s)",
		ic.Type, ic.Path, ic.DpAppliance, ic.DpDomain, ic.DpFilestore)
}

// Equals method returns true if other object is refering to same ItemConfig.
func (ic *ItemConfig) Equals(other *ItemConfig) bool {
	if other == nil {
		return false
	}
	return ic.Path == other.Path && ic.DpAppliance == other.DpAppliance && ic.DpDomain == other.DpDomain && ic.DpFilestore == other.DpFilestore
}

// Item methods

// String method returns Item details.
func (item Item) String() string {
	return fmt.Sprintf("Item('%s', '%s', '%s', %t, %s)",
		item.Name, item.Size, item.Modified, item.Selected, item.Config)
}

// DisplayString method returns formatted string representing how item will be shown.
func (item *Item) DisplayString() string {
	return fmt.Sprintf("%s %10s %19s %s", item.GetDisplayableType(), item.Size, item.Modified, item.Name)
}

// GetDisplayableType retuns single character string representation of Item type.
func (item *Item) GetDisplayableType() string {
	return string(item.Config.Type)
}

// ItemList methods (implements sort.Interface)

// Len returns number of items in ItemList.
func (items ItemList) Len() int {
	return len(items)
}

// Less returns true if item at first index should be before second one.
func (items ItemList) Less(i, j int) bool {
	return items[i].Config.Type < items[j].Config.Type ||
		(items[i].Config.Type == items[j].Config.Type &&
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

// GetVisibleItemCount returns number of items which will be shown for given side.
func (m *Model) GetVisibleItemCount(side Side) int {
	visibleItemCount := len(m.items[side])
	logging.LogTrace("model/GetVisibleItemCount(", side, "), visibleItemCount: ", visibleItemCount, ", m.ItemMaxRows: ", m.ItemMaxRows)
	if m.ItemMaxRows < visibleItemCount {
		return m.ItemMaxRows
	}
	return visibleItemCount
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
	return m.ViewConfig(m.currSide).Path != ""
}

// CurrSide returns currently used Side.
func (m *Model) CurrSide() Side {
	return m.currSide
}

// CurrSide returns currently non-used Side.
func (m *Model) OtherSide() Side {
	return 1 - m.currSide
}

// Title returns title for given Side.
func (m *Model) Title(side Side) string {
	return m.title[side]
}

// ViewConfig returns view config for given Side.
func (m *Model) ViewConfig(side Side) *ItemConfig {
	return m.viewConfig[side]
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

// SetTitle sets title for given Side.
func (m *Model) SetCurrentView(side Side, viewConfig *ItemConfig, viewTitle string) {
	m.title[side] = viewTitle
	m.viewConfig[side] = viewConfig
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
	for idx, item := range m.items[m.currSide] {
		if item.Config.Equals(config) {
			itemIdx = idx
			break
		}
	}
	logging.LogDebugf("model/SetCurrItemForSideAndConfig(%v, %v), itemIdx: %v", side, config, itemIdx)
	m.currItemIdx[m.currSide] = itemIdx
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
	logging.LogDebug("model/navUpDown(), side: ", side, ", move: ", move, ", newCurr: ", newCurr, ", m.currFirstRowItemIdx[side]: ", m.currFirstRowItemIdx[side])

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

// SearchPrev moves cursor to previous item containing given searchStr and returns true if item is found.
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
