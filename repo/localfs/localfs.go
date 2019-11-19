package localfs

import (
	"fmt"
	"github.com/croz-ltd/dpcmder/config"
	"github.com/croz-ltd/dpcmder/model"
	"github.com/croz-ltd/dpcmder/utils/logging"
	"github.com/croz-ltd/dpcmder/utils/paths"
	"io/ioutil"
	"os"
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
func (r localRepo) GetTitle(itemToShow *model.ItemConfig) string {
	return itemToShow.Path
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

func (r localRepo) UpdateFile(currentView *model.ItemConfig, fileName string, newFileContent []byte) (bool, error) {
	logging.LogDebugf("repo/localfs/UpdateFile(%v, '%s', ..)", currentView, fileName)
	parentPath := currentView.Path
	filePath := paths.GetFilePath(parentPath, fileName)
	err := ioutil.WriteFile(filePath, newFileContent, os.ModePerm)
	if err != nil {
		errMsg := fmt.Sprintf("Can't update file '%s' on path '%s'.", fileName, parentPath)
		logging.LogDebugf("repo/localfs/UpdateFile() - %s", errMsg)
		return false, err
	}

	return false, nil
}

func (r localRepo) GetFileType(viewConfig *model.ItemConfig, parentPath, fileName string) (model.ItemType, error) {
	logging.LogDebugf("repo/localfs/GetFileType(%v, '%s', '%s')", viewConfig, parentPath, fileName)
	filePath := r.GetFilePath(parentPath, fileName)
	return getFileTypeFromPath(filePath)
}

func (r localRepo) GetFilePath(parentPath, fileName string) string {
	logging.LogDebugf("repo/localfs/GetFilePath('%s', '%s', ..)", parentPath, fileName)
	return paths.GetFilePath(parentPath, fileName)
}

func (r localRepo) CreateDir(viewConfig *model.ItemConfig, parentPath, dirName string) (bool, error) {
	logging.LogDebugf("repo/localfs/CreateDir(%v, '%s', '%s')", viewConfig, parentPath, dirName)
	fi, err := os.Stat(parentPath)
	if err != nil {
		logging.LogDebugf("repo/localfs/CreateDir('%s', '%s') - %v", parentPath, dirName, err)
		return false, err
	}
	dirPath := r.GetFilePath(parentPath, dirName)
	err = os.Mkdir(dirPath, fi.Mode())
	if err != nil {
		logging.LogDebugf("repo/localfs/CreateDir('%s', '%s') - %v", parentPath, dirName, err)
		return false, err
	}
	return true, nil
}

func (r localRepo) Delete(currentView *model.ItemConfig, parentPath, fileName string) (bool, error) {
	logging.LogDebugf("repo/localfs/Delete(%v, '%s', '%s')", currentView, parentPath, fileName)
	fileType, err := r.GetFileType(currentView, parentPath, fileName)
	if err != nil {
		logging.LogDebugf("repo/localfs/Delete(), err: %v", err)
		return false, err
	}
	filePath := r.GetFilePath(parentPath, fileName)
	logging.LogDebugf("repo/localfs/Delete(), path: '%s', fileType: %v", parentPath, fileName, fileType)

	switch fileType {
	case model.ItemFile:
		os.Remove(filePath)
	case model.ItemDirectory:
		subFiles, err := ioutil.ReadDir(filePath)
		if err != nil {
			logging.LogDebugf("repo/localfs/Delete(), path: '%s', fileType: %v - err: %v", parentPath, fileName, fileType, err)
			return false, err
		}
		for _, subFile := range subFiles {
			r.Delete(currentView, filePath, subFile.Name())
		}
		os.Remove(filePath)
	default:
	}

	return true, nil

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
	logging.LogDebugf("repo/localfs/getFileByPath('%s')", filePath)
	result, err := ioutil.ReadFile(filePath)
	if err != nil {
		logging.LogDebugf("repo/localfs/getFileByPath('%s') - Error reading file (%v).", filePath, err)
	}
	return result, err
}

func getFileTypeFromPath(filePath string) (model.ItemType, error) {
	logging.LogDebugf("repo/localfs/getFileTypeFromPath('%s')", filePath)
	fi, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return model.ItemNone, nil
		}
		logging.LogDebugf("repo/localfs/getFileTypeFromPath('%s') - Error getting file's type (%#v).", filePath, err)
		return model.ItemNone, err
	}

	if fi.IsDir() {
		return model.ItemDirectory, nil
	} else if fi.Name() != "" {
		return model.ItemFile, nil
	} else {
		return model.ItemNone, nil
	}
}
