package repo

import (
	"github.com/croz-ltd/dpcmder/model"
)

// Repo is a common repository methods implemented by local filesystem and DataPower
type Repo interface {
	GetInitialItem() model.Item
	GetTitle(currentView model.Item) string
	GetList(currentView *model.ItemConfig) (model.ItemList, error)
	InvalidateCache()
	GetFile(currentView *model.ItemConfig, fileName string) ([]byte, error)
}
