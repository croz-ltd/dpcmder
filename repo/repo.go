package repo

import (
	"github.com/croz-ltd/dpcmder/model"
)

// Repo is a common repository methods implemented by local filesystem and DataPower
type Repo interface {
	GetInitialItem() model.Item
	GetTitle(currentView *model.ItemConfig) string
	GetList(currentView *model.ItemConfig) (model.ItemList, error)
	InvalidateCache()
	GetFile(currentView *model.ItemConfig, fileName string) ([]byte, error)
	UpdateFile(currentView *model.ItemConfig, fileName string, newFileContent []byte) (bool, error)
	GetFileType(currentView *model.ItemConfig, parentPath, fileName string) (model.ItemType, error)
	GetFilePath(parentPath, fileName string) string
	CreateDir(viewConfig *model.ItemConfig, parentPath, dirName string) (bool, error)
}
