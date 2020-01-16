package localfs

import (
	"fmt"
	"github.com/croz-ltd/dpcmder/config"
	"github.com/croz-ltd/dpcmder/model"
	"github.com/croz-ltd/dpcmder/utils/assert"
	"github.com/croz-ltd/dpcmder/utils/errs"
	"testing"
	"time"
)

func TestLocalRepoString(t *testing.T) {
	assert.DeepEqual(t, "String()", Repo.String(), "local filesystem")
}

func TestLocalRepoGetInitialItem(t *testing.T) {
	initialDirParent := "/dir1/dir2"
	initialDirParentName := "dir2"
	initialDir := "/dir1/dir2/dir3"
	initialDirName := "dir3"
	config.LocalFolderPath = &initialDir

	item, _ := Repo.GetInitialItem()
	want := model.Item{Config: &model.ItemConfig{
		Type: model.ItemDirectory, Name: initialDirName, Path: initialDir,
		Parent: &model.ItemConfig{
			Type: model.ItemDirectory, Name: initialDirParentName, Path: initialDirParent}}}

	assert.DeepEqual(t, "GetInitialItem()", item, want)
}

func TestLocalRepoGetTitle(t *testing.T) {
	itemConfig := model.ItemConfig{Path: "/dir1/dir2/dir3"}
	want := "/dir1/dir2/dir3"
	assert.DeepEqual(t, "GetTitle()", Repo.GetTitle(&itemConfig), want)
}

func TestLocalRepoGetViewConfigByPath(t *testing.T) {
	testDataMatrix := []struct {
		currentView *model.ItemConfig
		dirPath     string
		newView     *model.ItemConfig
		err         error
	}{
		{&model.ItemConfig{Type: model.ItemNone}, "", nil, errs.Error("Given path '' is not directory.")},
		{&model.ItemConfig{Type: model.ItemDirectory}, "", nil, errs.Error("Given path '' is not directory.")},
	}

	for _, testCase := range testDataMatrix {
		newView, err := Repo.GetViewConfigByPath(testCase.currentView, testCase.dirPath)
		methodCall := fmt.Sprintf("GetViewConfigByPath(%v, '%s')", testCase.currentView, testCase.dirPath)

		assert.DeepEqual(t, methodCall, newView, testCase.newView)
		assert.DeepEqual(t, methodCall, err, testCase.err)
	}
}

func TestTreeString(t *testing.T) {
	tree := Tree{}
	want := "{'', dir: 'false', rp: '', path: '', '[]'}"

	assert.DeepEqual(t, "String()", tree.String(), want)
}

func TestTreeFindChild(t *testing.T) {
	treeWithWinner := Tree{Dir: true, Name: "root", Children: []Tree{
		Tree{Dir: false, Name: "some-file"},
		Tree{Dir: false, Name: "winner", Path: "/root/winner"},
		Tree{Dir: false, Name: "some other file"},
		Tree{Dir: true, Name: "dir2", Children: []Tree{}},
	}}
	treeWithoutWinner := Tree{Dir: true, Name: "root", Children: []Tree{}}
	searchChild := Tree{Dir: false, Name: "winner"}
	var wantNil *Tree
	wantWinner := Tree{Dir: false, Name: "winner", Path: "/root/winner"}

	assert.DeepEqual(t, "FindChild()", treeWithoutWinner.FindChild(&searchChild), wantNil)
	assert.DeepEqual(t, "FindChild()", treeWithWinner.FindChild(&searchChild), &wantWinner)
}

func TestTreeFileChanged(t *testing.T) {
	time1 := time.Now()
	time1a := time1.Add(time.Minute * 0)
	time2 := time1.Add(time.Minute * 10)
	tree1 := Tree{ModTime: time1}
	tree1a := Tree{ModTime: time1a}
	tree2 := Tree{ModTime: time2}
	tree3 := Tree{}

	assert.DeepEqual(t, "FileChanged()", tree1.FileChanged(&tree1a), false)
	assert.DeepEqual(t, "FileChanged()", tree1.FileChanged(&tree2), true)
	assert.DeepEqual(t, "FileChanged()", tree1.FileChanged(&tree3), true)
}
