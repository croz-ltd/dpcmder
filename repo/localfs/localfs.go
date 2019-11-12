package localfs

import (
	"fmt"
	"github.com/croz-ltd/dpcmder/config"
	"github.com/croz-ltd/dpcmder/model"
	"github.com/croz-ltd/dpcmder/utils"
	"github.com/croz-ltd/dpcmder/utils/logging"
	"io/ioutil"
	"path/filepath"
	"sort"
	"strconv"
)

type LocalRepo struct {
	name string
}

// Repo is localfs implementation of repo/Repo interface.
var Repo = LocalRepo{name: "LocalRepo"}

// GetInitialView returns initialy opened local directory info.
func (r LocalRepo) GetInitialItem() model.Item {
	logging.LogDebug("repo/localfs/GetInitialItem()")
	currPath, err := filepath.Abs(*config.LocalFolderPath)
	if err != nil {
		logging.LogFatal("repo/localfs/GetInitialItem(): ", err)
	}

	initialItem := model.Item{Config: &model.ItemConfig{Path: currPath}}
	return initialItem
}

func (r LocalRepo) GetTitle(itemToShow model.Item) string {
	return itemToShow.Config.Path
}

// GetList returns list of items for current directory.
func (r LocalRepo) GetList(itemToShow model.Item) model.ItemList {
	logging.LogDebug(fmt.Sprintf("repo/localfs/GetList('%s')", itemToShow))
	currPath := itemToShow.Config.Path

	parentDir := model.Item{Name: "..",
		Config: &model.ItemConfig{
			Type: model.ItemDirectory, Path: utils.GetFilePath(currPath, "..")}}
	items := listFiles(currPath)

	itemsWithParentDir := make([]model.Item, 0)
	itemsWithParentDir = append(itemsWithParentDir, parentDir)
	itemsWithParentDir = append(itemsWithParentDir, items...)

	return itemsWithParentDir
}

func (r LocalRepo) InvalidateCache() {}

func listFiles(dirPath string) []model.Item {
	logging.LogDebug(fmt.Sprintf("repo/localfs/listFiles('%s')", dirPath))
	files, err := ioutil.ReadDir(dirPath)
	if err != nil {
		logging.LogFatal("repo/localfs/listFiles(): ", err)
	}

	items := make(model.ItemList, len(files))

	for idx, file := range files {
		var dirType model.ItemType
		if file.IsDir() {
			dirType = model.ItemDirectory
		} else {
			dirType = model.ItemFile
		}
		items[idx] = model.Item{Name: file.Name(), Size: strconv.FormatInt(file.Size(), 10),
			Modified: file.ModTime().Format("2006-01-02 15:04:05"),
			Config: &model.ItemConfig{
				Type: dirType, Path: utils.GetFilePath(dirPath, file.Name())}}
	}

	sort.Sort(items)

	return items
}
