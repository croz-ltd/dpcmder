package repo

import (
	"github.com/croz-ltd/dpcmder/model"
)

// Repo is a common repository methods implemented by local filesystem and DataPower
type Repo interface {
	GetInitialView() model.CurrentView
	GetList(currentView model.CurrentView) model.ItemList
	GetTitle(view model.CurrentView) string
	NextView(currView model.CurrentView, selectedItem model.Item) model.CurrentView
}
