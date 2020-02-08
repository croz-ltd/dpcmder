package model

import (
	"fmt"
	"github.com/croz-ltd/dpcmder/utils/assert"
	"reflect"
	"testing"
)

// ItemType methods tests

func TestItemTypeString(t *testing.T) {
	types := []ItemType{ItemFile, ItemDirectory, ItemDpConfiguration, ItemDpDomain,
		ItemDpFilestore, ItemDpObjectClassList, ItemDpObjectClass, ItemDpObject,
		ItemNone}
	wantArr := []string{"f", "d", "A", "D", "F", "L", "C", "O", "-"}

	for idx, gotType := range types {
		got := gotType.String()
		want := wantArr[idx]
		assert.DeepEqual(t, "ItemType.String()", got, want)
	}
}

func TestUserFriendlyString(t *testing.T) {
	types := []ItemType{ItemFile, ItemDirectory, ItemDpConfiguration, ItemDpDomain,
		ItemDpFilestore, ItemDpObjectClassList, ItemDpObjectClass, ItemDpObject,
		ItemNone}
	wantArr := []string{"file", "directory", "appliance configuration", "domain",
		"filestore", "object class list", "object class", "object", "-"}

	for idx, gotType := range types {
		got := gotType.UserFriendlyString()
		want := wantArr[idx]
		assert.DeepEqual(t, "ItemType.String()", got, want)
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
		assert.DeepEqual(t, "ItemConfig.Equals()", extectedRes, gotRes)
	}
}

func TestItemConfigString(t *testing.T) {
	itemDp := ItemConfig{Type: ItemDirectory, Name: "MyName", DpAppliance: "mydp", DpDomain: "dom1", DpFilestore: "local:", Path: "local:/hello/dir", Parent: &ItemConfig{Type: ItemNone}}
	assert.DeepEqual(t, "ItemConfig.String()",
		itemDp.String(),
		"IC(d, 'MyName' (local:/hello/dir), 'mydp' (dom1) local: IDOS(''/'', ''/'' ()) IC(-, '' (), '' ()  IDOS(''/'', ''/'' ()) <nil>))")
}

// Item methods tests

func TestItemDisplayString(t *testing.T) {
	item := Item{Config: &ItemConfig{Type: ItemFile}, Name: "master", Size: "3000", Modified: "2019-02-06 14:06:10", Selected: true}

	got := item.DisplayString()
	want := "f       3000 2019-02-06 14:06:10 master"
	assert.DeepEqual(t, "Item.GetDisplayableType()", got, want)
}
func TestItemGetDisplayableType(t *testing.T) {
	itemList := prepareAllTypesItemList()

	displayedType := []string{"A", "D", "F", "d", "f"}

	for idx := 0; idx < len(displayedType); idx++ {
		got := itemList[idx].GetDisplayableType()
		want := displayedType[idx]
		assert.DeepEqual(t, "Item.GetDisplayableType()", got, want)
	}
}
func TestItemString(t *testing.T) {
	item := Item{Config: &ItemConfig{Type: ItemFile}, Name: "master", Size: "3000", Modified: "2019-02-06 14:06:10", Selected: true}
	assert.DeepEqual(t,
		"Item.String()",
		item.String(),
		"Item('master', '3000', '2019-02-06 14:06:10', true, IC(f, '' (), '' ()  IDOS(''/'', ''/'' ()) <nil>))")
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
		Item{Config: &ItemConfig{Type: ItemDpConfiguration}, Name: "..", Size: "", Modified: "", Selected: false},
	}
}

func prepareAllTypesItemList() ItemList {
	return ItemList{
		Item{Config: &ItemConfig{Type: ItemDpConfiguration}, Name: "Ajan DpConfig", Size: "1000", Modified: "2019-02-06 12:06:10", Selected: false},
		Item{Config: &ItemConfig{Type: ItemDpDomain}, Name: "Ajan DpDomain", Size: "1000", Modified: "2019-02-06 12:06:10", Selected: false},
		Item{Config: &ItemConfig{Type: ItemDpFilestore}, Name: "Ajan DpFilestore", Size: "1000", Modified: "2019-02-06 12:06:10", Selected: false},
		Item{Config: &ItemConfig{Type: ItemDirectory}, Name: "ali", Size: "200", Modified: "2019-02-06 14:06:10", Selected: false},
		Item{Config: &ItemConfig{Type: ItemFile}, Name: "Micro", Size: "1000", Modified: "2019-02-06 12:06:10", Selected: false},
	}
}
func TestItemListLen(t *testing.T) {
	itemList := prepareItemList()
	gotLen := itemList.Len()
	expectedLen := 10
	assert.DeepEqual(t, "ItemList.Len()", gotLen, expectedLen)
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
		assert.DeepEqual(t, "ItemList.Less()", gotRes, expectedRes)
	}
}
func TestItemListSwap(t *testing.T) {
	itemList := prepareItemList()

	itemList.Swap(0, 4)

	gotItem := itemList[0]
	expectedItem := Item{Config: &ItemConfig{Type: ItemFile, Path: "/path5"}, Name: "master", Size: "3000", Modified: "2019-02-06 14:06:10", Selected: false}

	assert.DeepEqual(t, "ItemList.Swap()", expectedItem, gotItem)
}

// Model methods tests

func checkCurrItem(t *testing.T, model Model, want Item, msg string) {
	got := *model.CurrItem()
	t.Helper()
	assert.DeepEqual(t, "Model.CurrItem()", got, want)
}

func TestModelSetCurrentView(t *testing.T) {
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

func TestModelNavCurrentViewBackFw(t *testing.T) {
	model := Model{}

	checkViewConfig := func(side Side, wantedViewConfig *ItemConfig,
		wantedViewHistoryIdx, wantedViewHistorySize int) {
		t.Helper()
		gotViewConfig := model.ViewConfig(side)
		if gotViewConfig != wantedViewConfig {
			t.Errorf("Model ViewConfig(%v) should be '%s' but is '%s'.", side, wantedViewConfig, gotViewConfig)
		}
		gotViewHistorySize := model.ViewConfigHistorySize(side)
		if gotViewHistorySize != wantedViewHistorySize {
			t.Errorf("Model ViewConfigHistorySize(%v) should be %d but is %d.", side, wantedViewHistorySize, gotViewHistorySize)
		}
		gotViewHistoryIdx := model.ViewConfigHistorySelectedIdx(side)
		if gotViewHistorySize != wantedViewHistorySize {
			t.Errorf("Model ViewConfigHistorySelectedIdx(%v) should be %d but is %d.", side, wantedViewHistoryIdx, gotViewHistoryIdx)
		}
	}

	// View history navigation while there is no current view - no change
	checkViewConfig(Left, nil, 0, 0)
	checkViewConfig(Right, nil, 0, 0)
	assert.Equals(t, "History size left", model.ViewConfigHistorySize(Left), 0)
	assert.Equals(t, "History size right", model.ViewConfigHistorySize(Right), 0)

	model.NavCurrentViewBack(Left)
	model.NavCurrentViewBack(Right)
	checkViewConfig(Left, nil, 0, 0)
	checkViewConfig(Right, nil, 0, 0)
	assert.Equals(t, "History size left", model.ViewConfigHistorySize(Left), 0)
	assert.Equals(t, "History size right", model.ViewConfigHistorySize(Right), 0)

	assert.Equals(t, "History idx left", model.ViewConfigHistorySelectedIdx(Left), 0)
	assert.Equals(t, "History idx right", model.ViewConfigHistorySelectedIdx(Right), 0)

	model.NavCurrentViewForward(Left)
	model.NavCurrentViewForward(Right)
	checkViewConfig(Left, nil, 0, 0)
	checkViewConfig(Right, nil, 0, 0)

	// View history navigation while there is only 1 view - no change
	itemConfig1a := &ItemConfig{Path: "/path/1a"}
	itemConfig2a := &ItemConfig{Path: "/path/2a"}
	model.AddNextView(Left, itemConfig1a, "")
	model.AddNextView(Right, itemConfig2a, "")
	checkViewConfig(Left, itemConfig1a, 0, 1)
	checkViewConfig(Right, itemConfig2a, 0, 1)

	model.NavCurrentViewBack(Left)
	model.NavCurrentViewBack(Right)
	checkViewConfig(Left, itemConfig1a, 0, 1)
	checkViewConfig(Right, itemConfig2a, 0, 1)

	model.NavCurrentViewForward(Left)
	model.NavCurrentViewForward(Right)
	checkViewConfig(Left, itemConfig1a, 0, 1)
	checkViewConfig(Right, itemConfig2a, 0, 1)

	// View history navigation while there are more than 1 views - navigate
	// between existing history views
	itemConfig1b := &ItemConfig{Path: "/path/1b"}
	itemConfig2b := &ItemConfig{Path: "/path/2b"}
	model.AddNextView(Left, itemConfig1b, "")
	model.AddNextView(Right, itemConfig2b, "")
	checkViewConfig(Left, itemConfig1b, 1, 2)
	checkViewConfig(Right, itemConfig2b, 1, 2)

	itemConfig1c := &ItemConfig{Path: "/path/1c"}
	itemConfig2c := &ItemConfig{Path: "/path/2c"}
	model.AddNextView(Left, itemConfig1c, "")
	model.AddNextView(Right, itemConfig2c, "")
	checkViewConfig(Left, itemConfig1c, 2, 3)
	checkViewConfig(Right, itemConfig2c, 2, 3)

	model.NavCurrentViewBack(Left)
	model.NavCurrentViewBack(Right)
	checkViewConfig(Left, itemConfig1b, 1, 3)
	checkViewConfig(Right, itemConfig2b, 1, 3)

	model.NavCurrentViewBack(Left)
	model.NavCurrentViewBack(Right)
	checkViewConfig(Left, itemConfig1a, 0, 3)
	checkViewConfig(Right, itemConfig2a, 0, 3)

	model.NavCurrentViewBack(Left)
	model.NavCurrentViewBack(Right)
	checkViewConfig(Left, itemConfig1a, 0, 3)
	checkViewConfig(Right, itemConfig2a, 0, 3)

	model.NavCurrentViewForward(Left)
	model.NavCurrentViewForward(Right)
	checkViewConfig(Left, itemConfig1b, 1, 3)
	checkViewConfig(Right, itemConfig2b, 1, 3)

	model.NavCurrentViewForward(Left)
	model.NavCurrentViewForward(Right)
	checkViewConfig(Left, itemConfig1c, 2, 3)
	checkViewConfig(Right, itemConfig2c, 2, 3)

	model.NavCurrentViewForward(Left)
	model.NavCurrentViewForward(Right)
	checkViewConfig(Left, itemConfig1c, 2, 3)
	checkViewConfig(Right, itemConfig2c, 2, 3)

	gotLeftHistory := model.ViewConfigHistoryList(Left)
	gotRightHistory := model.ViewConfigHistoryList(Right)
	wantLeftHistory := []*ItemConfig{itemConfig1a, itemConfig1b, itemConfig1c}
	wantRightHistory := []*ItemConfig{itemConfig2a, itemConfig2b, itemConfig2c}
	assert.DeepEqual(t, "Left history list", wantLeftHistory, gotLeftHistory)
	assert.DeepEqual(t, "Right history list", wantRightHistory, gotRightHistory)

	// Move back in view history and check if navigation behaves as expected.
	model.NavCurrentViewBack(Left)
	model.NavCurrentViewBack(Right)
	model.NavCurrentViewBack(Left)
	model.NavCurrentViewBack(Right)
	checkViewConfig(Left, itemConfig1a, 0, 3)
	checkViewConfig(Right, itemConfig2a, 0, 3)

	// Create new view when we are back in view history - history should be rewritten.
	model.SetCurrentView(Left, itemConfig1a, "")
	model.SetCurrentView(Right, itemConfig2a, "")
	checkViewConfig(Left, itemConfig1a, 0, 3)
	checkViewConfig(Right, itemConfig2a, 0, 3)

	model.NavCurrentViewForward(Left)
	model.NavCurrentViewForward(Right)
	model.NavCurrentViewForward(Left)
	model.NavCurrentViewForward(Right)
	checkViewConfig(Left, itemConfig1c, 2, 3)
	checkViewConfig(Right, itemConfig2c, 2, 3)

	model.NavCurrentViewBack(Left)
	model.NavCurrentViewBack(Right)
	model.NavCurrentViewBack(Left)
	model.NavCurrentViewBack(Right)
	checkViewConfig(Left, itemConfig1a, 0, 3)
	checkViewConfig(Right, itemConfig2a, 0, 3)

	// Create new view when we are back in view history - history should be rewritten
	// (only if view is replaced with other view).
	itemConfig1d := &ItemConfig{Path: "/path/1d"}
	itemConfig2d := &ItemConfig{Path: "/path/2d"}
	model.AddNextView(Left, itemConfig1d, "")
	model.AddNextView(Right, itemConfig2d, "")
	checkViewConfig(Left, itemConfig1d, 1, 2)
	checkViewConfig(Right, itemConfig2d, 1, 2)

	model.NavCurrentViewForward(Left)
	model.NavCurrentViewForward(Right)
	checkViewConfig(Left, itemConfig1d, 1, 2)
	checkViewConfig(Right, itemConfig2d, 1, 2)

	model.NavCurrentViewBack(Left)
	model.NavCurrentViewBack(Right)
	checkViewConfig(Left, itemConfig1a, 2, 2)
	checkViewConfig(Right, itemConfig2a, 2, 2)

	model.NavCurrentViewBack(Left)
	model.NavCurrentViewBack(Right)
	checkViewConfig(Left, itemConfig1a, 0, 2)
	checkViewConfig(Right, itemConfig2a, 0, 2)
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
	wantedCount = 3
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
			t.Errorf("%s - %d should be current item.", msg, wantCurrIdx)
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
		{model.NavDown, 9},
		{model.NavDown, 9},
		{model.NavUp, 8},
		{model.NavUp, 7},
		{model.NavUp, 6},
		{model.NavUp, 5},
		{model.NavUp, 4},
		{model.NavUp, 3},
		{model.NavUp, 2},
		{model.NavPgDown, 5},
		{model.NavPgDown, 8},
		{model.NavPgDown, 9},
		{model.NavPgUp, 6},
		{model.NavPgUp, 3},
		{model.NavPgUp, 0},
		{model.NavPgUp, 0},
		{model.NavBottom, 9},
		{model.NavBottom, 9},
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
		{[]selectionFunc{model.SelPgDown, model.NavPgDown}, []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}},
		{[]selectionFunc{model.SelPgDown, model.NavPgDown}, []int{0, 1, 2, 3, 4, 5, 6, 7, 8}},
		{[]selectionFunc{model.NavUp}, []int{0, 1, 2, 3, 4, 5, 6, 7, 8}},
		{[]selectionFunc{model.NavUp}, []int{0, 1, 2, 3, 4, 5, 6, 7, 8}},
		{[]selectionFunc{model.SelPgUp, model.NavPgUp}, []int{0, 1, 2, 3, 4, 8}},
		{[]selectionFunc{model.SelPgUp, model.NavPgUp}, []int{0, 1, 8}},
		{[]selectionFunc{model.SelPgUp, model.NavPgUp}, []int{8}},
		{[]selectionFunc{model.SelToBottom, model.NavBottom}, []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}},
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
		items[9],
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

	model.SearchNext("..")
	checkCurrItem(t, model, items[9], "Next '..'")

	model.SearchNext("..")
	checkCurrItem(t, model, items[9], "Next '..'")
}

func TestModelIsSelectable(t *testing.T) {
	model := Model{}
	itemList := prepareAllTypesItemList()
	model.SetItems(Left, itemList)

	selectableExpected := []bool{true, false, false, true, true, true}

	for idx := 0; idx < len(itemList); idx++ {
		itemName := itemList[idx].Name
		model.SetCurrItemForSide(Left, itemName)
		got := model.IsSelectable()
		want := selectableExpected[idx]
		assert.DeepEqual(t, fmt.Sprintf("Model.IsSelectable(), item: '%s'", itemName), got, want)
	}
}

func TestModelStatusHandling(t *testing.T) {
	model := Model{}
	assert.DeepEqual(t, "LastStatus()", "", model.LastStatus())
	testStatuses := []string{"Status event no 1",
		"Status event no 2",
		"Status event no 3"}
	model.AddStatus(testStatuses[0])
	model.AddStatus(testStatuses[1])
	model.AddStatus(testStatuses[2])

	assert.DeepEqual(t, "LastStatus()", testStatuses[2], model.LastStatus())
	assert.DeepEqual(t, "Statuses()", testStatuses, model.Statuses())

	for index := 0; index < maxStatusCount; index++ {
		model.AddStatus(fmt.Sprintf("Status event new no %d", index))
	}
	assert.DeepEqual(t, "LastStatus()", fmt.Sprintf("Status event new no %d", maxStatusCount-1), model.LastStatus())
	assert.DeepEqual(t, "Statuses() size", maxStatusCount, len(model.Statuses()))
}
