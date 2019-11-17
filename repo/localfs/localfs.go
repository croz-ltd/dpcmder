package localfs

import (
	"github.com/croz-ltd/dpcmder/config"
	"github.com/croz-ltd/dpcmder/model"
	"github.com/croz-ltd/dpcmder/utils/logging"
	"github.com/croz-ltd/dpcmder/utils/paths"
	"io/ioutil"
	"path/filepath"
	"sort"
	"strconv"
)

type localRepo struct {
	name string
}

// Repo is instance or local filesystem repo/Repo interface implementation.
var Repo = localRepo{name: "LocalRepo"}

func (r *localRepo) String() string {
	return r.name
}

// GetInitialView returns initialy opened local directory info.
func (r localRepo) GetInitialItem() model.Item {
	logging.LogDebug("repo/localfs/GetInitialItem()")
	currPath, err := filepath.Abs(*config.LocalFolderPath)
	if err != nil {
		logging.LogFatal("repo/localfs/GetInitialItem(): ", err)
	}

	parentConfig := model.ItemConfig{Type: model.ItemDirectory, Path: paths.GetFilePath(currPath, "..")}
	initialItem := model.Item{Config: &model.ItemConfig{Type: model.ItemDirectory, Path: currPath, Parent: &parentConfig}}
	return initialItem
}

// GetTitle returns title for item to show.
func (r localRepo) GetTitle(itemToShow model.Item) string {
	return itemToShow.Config.Path
}

// GetList returns list of items for current directory.
func (r localRepo) GetList(itemToShow *model.ItemConfig) (model.ItemList, error) {
	logging.LogDebugf("repo/localfs/GetList('%s')", itemToShow)
	currPath := itemToShow.Path

	parentDir := model.Item{Name: "..",
		Config: &model.ItemConfig{
			Type: model.ItemDirectory, Path: paths.GetFilePath(currPath, "..")}}
	items, err := listFiles(currPath)
	if err != nil {
		return nil, err
	}

	itemsWithParentDir := make([]model.Item, 0)
	itemsWithParentDir = append(itemsWithParentDir, parentDir)
	itemsWithParentDir = append(itemsWithParentDir, items...)

	return itemsWithParentDir, nil
}

func (r localRepo) InvalidateCache() {}

func (r localRepo) GetFile(currentView *model.ItemConfig, fileName string) ([]byte, error) {
	logging.LogDebugf("repo/localfs/GetFile(%v, '%s')", currentView, fileName)
	parentPath := currentView.Path
	filePath := paths.GetFilePath(parentPath, fileName)

	return getFileByPath(filePath)
}

func listFiles(dirPath string) ([]model.Item, error) {
	logging.LogDebugf("repo/localfs/listFiles('%s')", dirPath)
	files, err := ioutil.ReadDir(dirPath)
	if err != nil {
		logging.LogDebug("repo/localfs/listFiles(): ", err)
		return nil, err
	}

	items := make(model.ItemList, len(files))

	for idx, file := range files {
		var dirType model.ItemType
		if file.IsDir() {
			dirType = model.ItemDirectory
		} else {
			dirType = model.ItemFile
		}
		parentConfig := model.ItemConfig{Type: model.ItemDirectory, Path: dirPath}
		items[idx] = model.Item{Name: file.Name(), Size: strconv.FormatInt(file.Size(), 10),
			Modified: file.ModTime().Format("2006-01-02 15:04:05"),
			Config: &model.ItemConfig{
				Type: dirType, Path: paths.GetFilePath(dirPath, file.Name()), Parent: &parentConfig}}
	}

	sort.Sort(items)

	return items, nil
}

func getFileByPath(filePath string) ([]byte, error) {
	result, err := ioutil.ReadFile(filePath)
	if err != nil {
		logging.LogDebugf("repo/localfs/GetFile() - Error reading file '%s'.", filePath, err)
	}
	return result, err
}
