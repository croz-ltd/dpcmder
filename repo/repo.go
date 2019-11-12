package repo

import (
	"github.com/croz-ltd/dpcmder/model"
)

// Repo is a common repository methods implemented by local filesystem and DataPower
type Repo interface {
	GetInitialItem() model.Item
	GetList(currentView model.Item) model.ItemList
	GetTitle(currentView model.Item) string
}
