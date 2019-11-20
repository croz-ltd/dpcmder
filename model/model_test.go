package model

import (
	"fmt"
	"reflect"
	"testing"
)

// ItemType methods tests

func TestItemTypeString(t *testing.T) {
	types := []ItemType{ItemFile, ItemDirectory, ItemDpConfiguration, ItemDpDomain, ItemDpFilestore, ItemNone}
	wantArr := []string{"f", "d", "A", "D", "F", "-"}

	for idx, gotType := range types {
		got := gotType.String()
		want := wantArr[idx]
		assertValue(t, "ItemType.String()", want, got)
	}
}

// ItemConfig methods tests

func TestItemConfigEquals(t *testing.T) {
	itemDp1a := ItemConfig{DpAppliance: "mydp", DpDomain: "dom1", DpFilestore: "local:", Path: "local:/hello/dir", Parent: &ItemConfig{}}
	itemDp1b := ItemConfig{DpAppliance: "mydp", DpDomain: "dom1", DpFilestore: "local:", Path: "local:/hello/dir", Parent: &ItemConfig{DpAppliance: "other"}}
	itemDp2 := ItemConfig{DpAppliance: "mydp", DpDomain: "dom1", DpFilestore: "local:", Path: "local:/hello", Parent: &ItemConfig{}}
	itemLocal1a := ItemConfig{Path: "/hello/world/dir", Parent: &ItemConfig{}}
	itemLocal1b := ItemConfig{Path: "/hello/world/dir", Parent: &ItemConfig{Path: "/asdf"}}
	itemLocal2 := ItemConfig{Path: "/hello/world/dirother", Parent: &ItemConfig{Path: "/asdf"}}

	testDataMatrix := []struct {
		one   ItemConfig
		other *ItemConfig
		res   bool
	}{
		{itemDp1a, nil, false},
		{itemDp1a, &itemDp1b, true},
		{itemDp1a, &itemDp1a, true},
		{itemDp1b, &itemDp1a, true},
		{itemDp1a, &itemDp2, false},
		{itemDp1a, &itemLocal1a, false},
		{itemDp1a, &itemLocal2, false},
		{itemLocal1a, nil, false},
		{itemLocal1a, &itemLocal1a, true},
		{itemLocal1a, &itemLocal1b, true},
		{itemLocal1b, &itemLocal1a, true},
		{itemLocal1a, &itemLocal2, false},
		{itemLocal1a, &itemDp1a, false},
	}

	for _, testRow := range testDataMatrix {
		gotRes := testRow.one.Equals(testRow.other)
		extectedRes := testRow.res
		assertValue(t, "ItemConfig.Equals()", extectedRes, gotRes)
	}
}

func TestItemConfigString(t *testing.T) {
	itemDp := ItemConfig{Type: ItemDirectory, DpAppliance: "mydp", DpDomain: "dom1", DpFilestore: "local:", Path: "local:/hello/dir", Parent: &ItemConfig{}}
	assertValue(t, "ItemConfig.String()", "IC(d, 'local:/hello/dir', 'mydp' (dom1) local:)", itemDp.String())
}

// Item methods tests

func TestItemDisplayString(t *testing.T) {
	item := Item{Config: &ItemConfig{Type: ItemFile}, Name: "master", Size: "3000", Modified: "2019-02-06 14:06:10", Selected: true}

	got := item.DisplayString()
	want := "f       3000 2019-02-06 14:06:10 master"
	assertValue(t, "Item.GetDisplayableType()", want, got)
}
func TestItemGetDisplayableType(t *testing.T) {
	itemList := prepareAllTypesItemList()

	displayedType := []string{"A", "D", "F", "d", "f"}

	for idx := 0; idx < len(displayedType); idx++ {
		got := itemList[idx].GetDisplayableType()
		want := displayedType[idx]
		assertValue(t, "Item.GetDisplayableType()", want, got)
	}
}
func TestItemString(t *testing.T) {
	item := Item{Config: &ItemConfig{Type: ItemFile}, Name: "master", Size: "3000", Modified: "2019-02-06 14:06:10", Selected: true}
	assertValue(t, "Item.String()", "Item('master', '3000', '2019-02-06 14:06:10', true, IC(f, '', '' () ))", item.String())
}

// ItemList methods tests

func prepareItemList() ItemList {
	return ItemList{
		Item{Config: &ItemConfig{Type: ItemDirectory, Path: "/path1"}, Name: "ali", Size: "200", Modified: "2019-02-06 14:06:10", Selected: false},
		Item{Config: &ItemConfig{Type: ItemDirectory, Path: "/path2"}, Name: "Ajan", Size: "1000", Modified: "2019-02-06 12:06:10", Selected: false},
		Item{Config: &ItemConfig{Type: ItemFile, Path: "/path3"}, Name: "Micro", Size: "1000", Modified: "2019-02-06 12:06:10", Selected: false},
		Item{Config: &ItemConfig{Type: ItemFile, Path: "/path4"}, Name: "Macro", Size: "2000", Modified: "2019-02-06 13:06:10", Selected: false},
		Item{Config: &ItemConfig{Type: ItemFile, Path: "/path5"}, Name: "master", Size: "3000", Modified: "2019-02-06 14:06:10", Selected: false},
		Item{Config: &ItemConfig{Type: ItemFile, Path: "/path6"}, Name: "mister", Size: "3001", Modified: "2019-02-06 14:06:10", Selected: false},
		Item{Config: &ItemConfig{Type: ItemFile, Path: "/path7"}, Name: "Matter", Size: "3002", Modified: "2019-02-06 14:06:10", Selected: false},
		Item{Config: &ItemConfig{Type: ItemFile, Path: "/path8"}, Name: "Glob", Size: "3003", Modified: "2019-02-06 14:06:10", Selected: false},
		Item{Config: &ItemConfig{Type: ItemFile, Path: "/path9"}, Name: "Blob", Size: "3004", Modified: "2019-02-06 14:06:10", Selected: false},
	}
}

func prepareAllTypesItemList() ItemList {
	return ItemList{
		Item{Config: &ItemConfig{Type: ItemDpConfiguration}, Name: "Ajan", Size: "1000", Modified: "2019-02-06 12:06:10", Selected: false},
		Item{Config: &ItemConfig{Type: ItemDpDomain}, Name: "Ajan", Size: "1000", Modified: "2019-02-06 12:06:10", Selected: false},
		Item{Config: &ItemConfig{Type: ItemDpFilestore}, Name: "Ajan", Size: "1000", Modified: "2019-02-06 12:06:10", Selected: false},
		Item{Config: &ItemConfig{Type: ItemDirectory}, Name: "ali", Size: "200", Modified: "2019-02-06 14:06:10", Selected: false},
		Item{Config: &ItemConfig{Type: ItemFile}, Name: "Micro", Size: "1000", Modified: "2019-02-06 12:06:10", Selected: false},
	}
}
func TestItemListLen(t *testing.T) {
	itemList := prepareItemList()
	gotLen := itemList.Len()
	expectedLen := 9
	assertValue(t, "ItemList.Len()", expectedLen, gotLen)
}
func TestItemListLess(t *testing.T) {
	itemList := prepareItemList()

	testDataMatrix := []struct {
		i, j int
		res  bool
	}{
		{0, 1, false},
		{1, 0, true},
		{0, 2, true},
		{2, 0, false},
		{2, 3, false},
		{2, 4, false},
		{3, 4, true},
		{4, 3, false},
	}

	for _, testRow := range testDataMatrix {
		gotRes := itemList.Less(testRow.i, testRow.j)
		expectedRes := testRow.res
		assertValue(t, "ItemList.Less()", expectedRes, gotRes)
	}
}
func TestItemListSwap(t *testing.T) {
	itemList := prepareItemList()

	itemList.Swap(0, 4)

	gotItem := itemList[0]
	expectedItem := Item{Config: &ItemConfig{Type: ItemFile, Path: "/path5"}, Name: "master", Size: "3000", Modified: "2019-02-06 14:06:10", Selected: false}

	assertValue(t, "ItemList.Swap()", expectedItem, gotItem)
}

// Model methods tests

func checkCurrItem(t *testing.T, model Model, want Item, msg string) {
	got := *model.CurrItem()
	assertValue(t, "Model.CurrItem()", want, got)
}

func TestModelTitle(t *testing.T) {
	model := Model{}

	checkTitle := func(side Side, wantedTitle string) {
		t.Helper()
		if model.Title(side) != wantedTitle {
			t.Errorf("Model Title(%v) should be '%s' but is '%s'.", side, wantedTitle, model.Title(side))
		}
	}
	checkViewConfig := func(side Side, wantedViewConfig *ItemConfig) {
		t.Helper()
		if model.ViewConfig(side) != wantedViewConfig {
			t.Errorf("Model ViewConfig(%v) should be '%s' but is '%s'.", side, wantedViewConfig, model.ViewConfig(side))
		}
	}
	checkTitle(Left, "")
	checkTitle(Right, "")
	checkViewConfig(Left, nil)
	checkViewConfig(Right, nil)

	itemConfig1 := &ItemConfig{Path: "/path/1"}
	itemConfig2 := &ItemConfig{Path: "/path/2"}
	model.SetCurrentView(Left, itemConfig1, "Left Title")
	model.SetCurrentView(Right, itemConfig2, "Right Title")
	checkTitle(Left, "Left Title")
	checkTitle(Right, "Right Title")
	checkViewConfig(Left, itemConfig1)
	checkViewConfig(Right, itemConfig2)
}

func TestModelToggleSide(t *testing.T) {
	model := Model{}

	if model.CurrSide() != Left {
		t.Errorf("Initial model currSide should be %v but is %v.", Left, model.CurrSide())
	}

	model.ToggleSide()
	if model.CurrSide() != Right {
		t.Errorf("After toggle currSide should be %v but is %v.", Right, model.CurrSide())
	}
	if model.IsCurrentSide(Right) != true {
		t.Errorf("After toggle IsCurrentSide(%v) should be true.", Right)
	}
	if model.OtherSide() != Left {
		t.Errorf("After toggle OtherSide() should be %v but is %v.", Left, model.OtherSide())
	}

	model.ToggleSide()
	if model.CurrSide() != Left {
		t.Errorf("After toggle currSide should be %v but is %v.", Left, model.CurrSide())
	}
	if model.IsCurrentSide(Left) != true {
		t.Errorf("After toggle IsCurrentSide(%v) should be true.", Left)
	}
	if model.OtherSide() != Right {
		t.Errorf("After toggle OtherSide() should be %v but is %v.", Right, model.OtherSide())
	}
}

func TestModelSetItemsAndFiltering(t *testing.T) {
	model := Model{}

	items := prepareItemList()

	side := model.CurrSide()

	model.SetItems(side, items)
	itemsCount := len(model.items[side])
	wantedCount := len(items)
	if itemsCount != wantedCount {
		t.Errorf("After setting items without filter there should be %d items but found %d items.", wantedCount, itemsCount)
	}

	model.NavBottom()

	filter := "cro"
	model.SetCurrentFilter(filter)
	model.SetItems(side, items)
	itemsCount = len(model.items[side])
	wantedCount = 2
	if itemsCount != wantedCount {
		t.Errorf("After setting items with '%s' filter there should be %d items but found %d items.", filter, wantedCount, itemsCount)
	}

	gotFilter := model.CurrentFilter()
	if gotFilter != filter {
		t.Errorf("Got filter '%s', want: '%s'", gotFilter, filter)
	}
}

func TestModelGetVisibleItemCount(t *testing.T) {
	model := Model{}

	model.ItemMaxRows = 3
	got := model.GetVisibleItemCount(Left)
	want := 0
	if got != want {
		t.Errorf("Initial number of visible items should be %d but is %d.", want, got)
	}

	items := prepareItemList()
	model.SetItems(Left, items)
	got = model.GetVisibleItemCount(Left)
	want = 3
	if got != want {
		t.Errorf("Initial number of visible items should be %d but is %d.", want, got)
	}
}

func TestModelVisibleItems(t *testing.T) {
	model := Model{}
	side := model.CurrSide()

	model.ItemMaxRows = 4
	items := prepareItemList()
	model.SetItems(side, items)

	var checkItems = func(got, want []Item) {
		t.Helper()
		gotLen, wantLen := len(got), len(want)
		if gotLen != wantLen {
			t.Fatalf("Got %d items but want %d items.", gotLen, wantLen)
		}

		for idx := 0; idx < gotLen; idx++ {
			gotItem, wantItem := got[idx], want[idx]
			if gotItem != wantItem {
				t.Errorf("Got '%v' but want '%v'.", gotItem, wantItem)
			}
		}
	}

	got := []Item{
		model.GetVisibleItem(side, 0),
		model.GetVisibleItem(side, 1),
		model.GetVisibleItem(side, 2),
		model.GetVisibleItem(side, 3),
	}
	want := items[:4]
	checkItems(got, want)
}

func TestModelNavigate(t *testing.T) {
	model := Model{}
	side := model.CurrSide()

	model.ItemMaxRows = 4
	items := prepareItemList()
	model.SetItems(side, items)

	type navigationFunc func()

	checkCurr := func(navFuncCall navigationFunc, wantCurrIdx int, msg string) {
		t.Helper()
		if navFuncCall != nil {
			navFuncCall()
		}
		checkCurrItem(t, model, items[wantCurrIdx], msg)

		if model.IsCurrentItem(model.CurrSide(), wantCurrIdx) != true ||
			model.IsCurrentItem(model.CurrSide(), wantCurrIdx-1) != false ||
			model.IsCurrentItem(model.CurrSide(), wantCurrIdx+1) != false {
			t.Errorf("checkCurr - only %d should be current item.", wantCurrIdx)
		}
	}

	navigationTestMatrix := []struct {
		nf      navigationFunc
		itemIdx int
	}{
		{nil, 0},
		{model.NavUp, 0},
		{model.NavDown, 1},
		{model.NavDown, 2},
		{model.NavDown, 3},
		{model.NavDown, 4},
		{model.NavDown, 5},
		{model.NavDown, 6},
		{model.NavDown, 7},
		{model.NavDown, 8},
		{model.NavDown, 8},
		{model.NavUp, 7},
		{model.NavUp, 6},
		{model.NavUp, 5},
		{model.NavUp, 4},
		{model.NavUp, 3},
		{model.NavUp, 2},
		{model.NavPgDown, 5},
		{model.NavPgDown, 8},
		{model.NavPgDown, 8},
		{model.NavPgUp, 5},
		{model.NavPgUp, 2},
		{model.NavPgUp, 0},
		{model.NavPgUp, 0},
		{model.NavBottom, 8},
		{model.NavBottom, 8},
		{model.NavTop, 0},
		{model.NavTop, 0},
	}

	for idx, testCase := range navigationTestMatrix {
		checkCurr(testCase.nf, testCase.itemIdx, fmt.Sprintf("testCase[%d], itemIdx %d error", idx, testCase.itemIdx))
	}
}

func TestModelSelect(t *testing.T) {
	model := Model{}
	side := model.CurrSide()

	model.ItemMaxRows = 4
	items := prepareItemList()
	model.SetItems(side, items)

	type selectionFunc func()

	checkSelected := func(selectFuncCalls []selectionFunc, wantSelectedIdx []int, msg string) {
		t.Helper()
		for _, sfcall := range selectFuncCalls {
			sfcall()
		}
		gotSelected := model.GetSelectedItems(side)
		gotLen, wantLen := len(gotSelected), len(wantSelectedIdx)
		if gotLen != wantLen {
			t.Fatalf("checkSelected - got %d items but want %d items (%s).", gotLen, wantLen, msg)
		}
		for idx := 0; idx < gotLen; idx++ {
			gotItem, wantItem := gotSelected[idx], model.items[side][wantSelectedIdx[idx]]
			if gotItem != wantItem {
				t.Errorf("checkSelected[%d] - got '%v' but want '%v' (%s).", idx, gotItem, wantItem, msg)
			}
		}
	}

	model.SetCurrentView(side, &ItemConfig{Path: "/some/path"}, "Some Title")
	selectionTestMatrix := []struct {
		sf         []selectionFunc
		selItemIdx []int
	}{
		{[]selectionFunc{}, []int{}},
		{[]selectionFunc{model.ToggleCurrItem}, []int{0}},
		{[]selectionFunc{model.ToggleCurrItem}, []int{}},
		{[]selectionFunc{model.ToggleCurrItem, model.NavDown}, []int{0}},
		{[]selectionFunc{model.ToggleCurrItem, model.NavDown}, []int{0, 1}},
		{[]selectionFunc{model.SelPgDown, model.NavPgDown}, []int{0, 1, 2, 3, 4}},
		{[]selectionFunc{model.SelPgDown, model.NavPgDown}, []int{0, 1, 2, 3, 4, 5, 6, 7}},
		{[]selectionFunc{model.SelPgDown, model.NavPgDown}, []int{0, 1, 2, 3, 4, 5, 6, 7, 8}},
		{[]selectionFunc{model.SelPgDown, model.NavPgDown}, []int{0, 1, 2, 3, 4, 5, 6, 7}},
		{[]selectionFunc{model.NavUp}, []int{0, 1, 2, 3, 4, 5, 6, 7}},
		{[]selectionFunc{model.NavUp}, []int{0, 1, 2, 3, 4, 5, 6, 7}},
		{[]selectionFunc{model.SelPgUp, model.NavPgUp}, []int{0, 1, 2, 3, 7}},
		{[]selectionFunc{model.SelPgUp, model.NavPgUp}, []int{0, 7}},
		{[]selectionFunc{model.SelPgUp, model.NavPgUp}, []int{7}},
		{[]selectionFunc{model.SelToBottom, model.NavBottom}, []int{0, 1, 2, 3, 4, 5, 6, 7, 8}},
		{[]selectionFunc{model.SelToTop, model.NavTop}, []int{}},
		{[]selectionFunc{model.ToggleCurrItem}, []int{0}},
		{[]selectionFunc{model.SelToBottom, model.NavBottom}, []int{}},
	}
	for idx, testCase := range selectionTestMatrix {
		checkSelected(testCase.sf, testCase.selItemIdx, fmt.Sprintf("testCase[%d], selItemIdx arr: %v", idx, testCase.selItemIdx))
	}

}

func checkCurrentRow(t *testing.T, model Model, side Side, row int, want bool) {
	t.Helper()
	got := model.IsCurrentRow(side, row)
	if got != want {
		t.Errorf("IsCurrentRow(.., %d) should return %v.", row, want)
	}
}

func TestModelCurrentRow(t *testing.T) {
	model := Model{}
	side := model.CurrSide()

	model.ItemMaxRows = 4
	items := prepareItemList()
	model.SetItems(side, items)

	checkCurrentRow(t, model, side, 0, true)
	checkCurrentRow(t, model, side, 1, false)

	model.NavPgDown()
	checkCurrentRow(t, model, side, 0, false)
	checkCurrentRow(t, model, side, 3, true)

	model.NavPgDown()
	checkCurrentRow(t, model, side, 3, true)
}

func TestModelSetCurrItemForSide(t *testing.T) {
	model := Model{}
	side := model.CurrSide()

	model.ItemMaxRows = 4
	items := prepareItemList()
	model.SetItems(side, items)

	checkCurrentRow(t, model, side, 0, true)

	model.SetCurrItemForSide(side, "Matter")
	checkCurrentRow(t, model, side, 0, false)
	checkCurrentRow(t, model, side, 3, false)

	checkCurrItem(t, model, items[6], "")
}

func TestModelSetCurrItemForSideAndConfig(t *testing.T) {
	model := Model{}
	side := model.CurrSide()

	model.ItemMaxRows = 4
	items := prepareItemList()
	model.SetItems(side, items)

	checkCurrentRow(t, model, side, 0, true)

	itemConfig := ItemConfig{Path: "/path4"}
	model.SetCurrItemForSideAndConfig(side, &itemConfig)
	checkCurrentRow(t, model, side, 0, false)
	checkCurrentRow(t, model, side, 3, true)

	checkCurrItem(t, model, items[3], "")
}

func TestModelSortSide(t *testing.T) {
	model := Model{}
	side := model.CurrSide()

	items := prepareItemList()
	want := ItemList{
		items[1],
		items[0],
		items[8],
		items[7],
		items[3],
		items[4],
		items[6],
		items[2],
		items[5],
	}

	model.SetItems(side, items)
	model.SortSide(side)

	if reflect.DeepEqual(model.items[side], want) != true {
		t.Errorf("After sorting items should be:\n%v\nbut was:\n%v.", want, model.items[side])
	}
}

func TestModelSearch(t *testing.T) {
	model := Model{}
	side := model.CurrSide()

	items := prepareItemList()

	model.SetItems(side, items)

	checkCurrItem(t, model, items[0], "Before search")

	model.SearchNext("er")
	checkCurrItem(t, model, items[4], "Next 'er'")

	model.SearchNext("er")
	checkCurrItem(t, model, items[5], "Next 'er'")

	model.SearchNext("er")
	checkCurrItem(t, model, items[6], "Next 'er'")

	model.SearchNext("er")
	checkCurrItem(t, model, items[6], "Next 'er'")

	model.SearchPrev("er")
	checkCurrItem(t, model, items[5], "Prev 'er'")

	model.SearchPrev("er")
	checkCurrItem(t, model, items[4], "Prev 'er'")

	model.SearchPrev("er")
	checkCurrItem(t, model, items[4], "Prev 'er'")

	model.SearchNext("Blob")
	checkCurrItem(t, model, items[8], "Next 'Blob'")

	model.SearchNext("Blob")
	checkCurrItem(t, model, items[8], "Next 'Blob'")

	model.SearchPrev("ali")
	checkCurrItem(t, model, items[0], "Prev 'ali'")

	model.SearchPrev("ali")
	checkCurrItem(t, model, items[0], "Prev 'ali'")
}

func TestStatusHandling(t *testing.T) {
	model := Model{}
	assertValue(t, "LastStatus()", "", model.LastStatus())
	testStatuses := []string{"Status event no 1",
		"Status event no 2",
		"Status event no 3"}
	model.AddStatus(testStatuses[0])
	model.AddStatus(testStatuses[1])
	model.AddStatus(testStatuses[2])

	assertValue(t, "LastStatus()", testStatuses[2], model.LastStatus())
	assertValue(t, "Statuses()", testStatuses, model.Statuses())

	for index := 0; index < maxStatusCount; index++ {
		model.AddStatus(fmt.Sprintf("Status event new no %d", index))
	}
	assertValue(t, "LastStatus()", fmt.Sprintf("Status event new no %d", maxStatusCount-1), model.LastStatus())
	assertValue(t, "Statuses() size", maxStatusCount, len(model.Statuses()))
}

func assertValue(t *testing.T, testName, want, got interface{}) {
	if !reflect.DeepEqual(want, got) {
		t.Errorf("%s should be: '%v' but was: '%v'.", testName, want, got)
	}
}
