package localfs

import (
	"fmt"
	"github.com/croz-ltd/dpcmder/config"
	"github.com/croz-ltd/dpcmder/model"
	"github.com/croz-ltd/dpcmder/utils"
	"github.com/croz-ltd/dpcmder/utils/logging"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	localSide = model.Right
)

type LocalRepo struct {
	name string
}

var Repo = LocalRepo{name: "LocalRepo"}

// LocalfsTree represents file/dir with all it's children and modification time.
type LocalfsTree struct {
	Dir          bool
	Name         string
	Path         string
	PathFromRoot string
	ModTime      time.Time
	Children     []LocalfsTree
}

func (t LocalfsTree) String() string {
	return fmt.Sprintf("{'%s', dir: '%t', rp: '%s', path: '%s', '%v'}", t.Name, t.Dir, t.PathFromRoot, t.Path, t.Children)
}

// FindChild finds child of same type and name as one we search for.
func (t LocalfsTree) FindChild(searchChild *LocalfsTree) *LocalfsTree {
	for _, child := range t.Children {
		if child.Dir == searchChild.Dir && child.Name == searchChild.Name {
			return &child
		}
	}

	return nil
}

// FileChanged check if this file is new or changed file comparing to saved info.
func (t LocalfsTree) FileChanged(anotherTree *LocalfsTree) bool {
	return anotherTree == nil || t.ModTime != anotherTree.ModTime
}

// LocalfsError records error on using local filesystem.
type LocalfsError struct {
	Message string
}

func (e *LocalfsError) Error() string {
	return e.Message
}

func (r LocalRepo) String() string {
	return r.name
}

func (r LocalRepo) ListFiles(m *model.Model, dirPath string) []model.Item {
	files, err := ioutil.ReadDir(dirPath)
	if err != nil {
		logging.LogFatal("localfs.ListFiles(): ", err)
	}

	items := make(model.ItemList, len(files))

	for idx, file := range files {
		var dirType byte
		if file.IsDir() {
			dirType = 'd'
		} else {
			dirType = 'f'
		}
		items[idx] = model.Item{Type: dirType, Name: file.Name(), Size: strconv.FormatInt(file.Size(), 10), Modified: file.ModTime().Format("2006-01-02 15:04:05"), Selected: false}
		idx++
	}

	sort.Sort(items)

	return items
}

func (r LocalRepo) LoadCurrent(m *model.Model) {
	currPath := m.CurrPathForSide(localSide)
	m.SetTitle(localSide, currPath)

	parentDir := model.Item{Type: 'd', Name: "..", Size: "", Modified: "", Selected: false}
	items := r.ListFiles(m, currPath)

	itemsWithParentDir := make([]model.Item, 0)
	itemsWithParentDir = append(itemsWithParentDir, parentDir)
	itemsWithParentDir = append(itemsWithParentDir, items...)

	m.SetItems(localSide, itemsWithParentDir)
}

func (r LocalRepo) InitialLoad(m *model.Model) {
	currPath, err := filepath.Abs(*config.LocalFolderPath)
	if err != nil {
		logging.LogFatal("localfs.InitialLoad(): ", err)
	}

	m.SetCurrPathForSide(localSide, currPath)

	r.LoadCurrent(m)
}

func (r LocalRepo) EnterCurrentDirectory(m *model.Model) {
	currPath := m.CurrPathForSide(localSide)
	dirName := m.CurrItemForSide(localSide).Name
	newCurrentItemName := ".."
	if dirName == ".." {
		lastSlashIdx := strings.LastIndex(currPath, "/")
		if lastSlashIdx != -1 {
			newCurrentItemName = currPath[lastSlashIdx+1:]
			currPath = currPath[:lastSlashIdx]
		}
	} else {
		currPath += "/" + dirName
	}

	m.SetCurrPathForSide(localSide, currPath)
	r.LoadCurrent(m)
	m.SetCurrItemForSide(localSide, newCurrentItemName)
}

func (r LocalRepo) GetFileName(filePath string) string {
	lastSlashIdx := strings.LastIndex(filePath, "/")
	if lastSlashIdx != -1 && len(filePath) > 1 {
		return filePath[lastSlashIdx+1:]
	} else {
		return ""
	}
}

func (r LocalRepo) GetFilePath(parentPath, fileName string) string {
	return utils.GetFilePath(parentPath, fileName)
}

func (r LocalRepo) GetFileTypeFromPath(m *model.Model, filePath string) byte {
	fi, err := os.Stat(filePath)
	if err != nil {
		return '0'
	}

	if fi.IsDir() {
		return 'd'
	} else if fi.Name() != "" {
		return 'f'
	} else {
		return '0'
	}
}

func (r LocalRepo) GetFileType(m *model.Model, parentPath, fileName string) byte {
	filePath := r.GetFilePath(parentPath, fileName)
	return r.GetFileTypeFromPath(m, filePath)
}

func (r LocalRepo) GetFileByPath(filePath string) []byte {
	result, err := ioutil.ReadFile(filePath)
	if err != nil {
		logging.LogFatal(fmt.Sprintf("localfs.GetFileByPath('%s'): ", filePath), err)
	}
	return result
}

func (r LocalRepo) GetFile(m *model.Model, parentPath, fileName string) []byte {
	filePath := r.GetFilePath(parentPath, fileName)
	return r.GetFileByPath(filePath)
}
func (r LocalRepo) UpdateFile(m *model.Model, parentPath, fileName string, newFileContent []byte) bool {
	filePath := r.GetFilePath(parentPath, fileName)
	err := ioutil.WriteFile(filePath, newFileContent, os.ModePerm)
	if err != nil {
		logging.LogFatal(fmt.Sprintf("localfs.UpdateFile('%s', '%s'): ", parentPath, fileName), err)
	}

	return true
}

func (r LocalRepo) Delete(m *model.Model, parentPath, fileName string) bool {
	fileType := r.GetFileType(m, parentPath, fileName)
	filePath := r.GetFilePath(parentPath, fileName)
	logging.LogDebug(fmt.Sprintf("localfs.Delete(%s, %s), fileType: %s\n", parentPath, fileName, string(fileType)))

	if fileType == 'f' {
		os.Remove(filePath)
	} else if fileType == 'd' {
		subFiles, err := ioutil.ReadDir(filePath)
		if err != nil {
			logging.LogFatal(fmt.Sprintf("localfs.Delete('%s', '%s'): ", parentPath, fileName), err)
		}
		for _, subFile := range subFiles {
			r.Delete(m, filePath, subFile.Name())
		}
		os.Remove(filePath)
	}

	return true
}

func (r LocalRepo) CreateDir(m *model.Model, parentPath, dirName string) bool {
	fi, err := os.Stat(parentPath)
	if err != nil {
		logging.LogFatal(fmt.Sprintf("localfs.CreateDir('%s', '%s'): ", parentPath, dirName), err)
	}
	dirPath := r.GetFilePath(parentPath, dirName)
	err = os.Mkdir(dirPath, fi.Mode())
	if err != nil {
		logging.LogFatal(fmt.Sprintf("localfs.CreateDir('%s', '%s'): ", parentPath, dirName), err)
	}
	return true
}

func (r LocalRepo) IsEmptyDir(m *model.Model, parentPath, dirName string) bool {
	dirPath := r.GetFilePath(parentPath, dirName)
	files, err := ioutil.ReadDir(dirPath)
	if err != nil {
		logging.LogFatal("localfs.IsEmptyDir(): ", err)
	}

	return len(files) == 0
}

func (r LocalRepo) LoadTree(pathFromRoot, filePath string) (LocalfsTree, error) {
	tree := LocalfsTree{}
	var errorMsg string

	fi, err := os.Stat(filePath)
	if err != nil {
		errorMsg = fmt.Sprintf("localfs.LoadTree('%s', '%s'): %s", pathFromRoot, filePath, err.Error())
		logging.LogDebug("localfs.LoadTree(), err: ", err)
	}

	if fi.IsDir() {
		tree.Dir = true
	} else if fi.Name() == "" {
		errorMsg = fmt.Sprintf("localfs.LoadTree('%s', '%s'): fi.Name() not found.", pathFromRoot, filePath)
		logging.LogDebug("localfs.LoadTree() fi.Name() not found.")
	}

	tree.Name = fi.Name()
	tree.ModTime = fi.ModTime()
	tree.PathFromRoot = pathFromRoot
	tree.Path = filePath

	if tree.Dir {
		files, err := ioutil.ReadDir(filePath)
		if err != nil {
			errorMsg = fmt.Sprintf("localfs.LoadTree('%s', '%s'): %s", pathFromRoot, filePath, err.Error())
			logging.LogDebug("localfs.LoadTree(): ", err)
		}

		tree.Children = make([]LocalfsTree, len(files))
		for i := 0; i < len(files); i++ {
			file := files[i]
			childPath := r.GetFilePath(filePath, file.Name())
			childRelPath := r.GetFilePath(pathFromRoot, file.Name())
			tree.Children[i], err = r.LoadTree(childRelPath, childPath)
			if err != nil {
				errorMsg = fmt.Sprintf("localfs.LoadTree('%s', '%s'): %s", pathFromRoot, filePath, err.Error())
				logging.LogDebug("localfs.LoadTree(): ", err)
			}
		}
	}

	// logging.LogDebug("localfs.LoadTree(), tree: ", tree)
	if errorMsg != "" {
		return tree, &LocalfsError{errorMsg}
	} else {
		return tree, nil
	}
}
