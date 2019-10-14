package repo

import (
	"github.com/croz-ltd/dpcmder/model"
)

// Repo is a common repository methods implemented by local filesystem and DataPower
type Repo interface {
	InitialLoad(m *model.Model)
	LoadCurrent(m *model.Model)
	EnterCurrentDirectory(m *model.Model)
	ListFiles(m *model.Model, dirPath string) []model.Item
	GetFileType(m *model.Model, parentPath, fileName string) byte
	GetFileTypeFromPath(m *model.Model, filePath string) byte
	GetFileName(filePath string) string
	GetFilePath(parentPath, fileName string) string
	GetFile(m *model.Model, parentPath, fileName string) []byte
	UpdateFile(m *model.Model, parentPath, fileName string, newFileContent []byte) bool
	Delete(m *model.Model, parentPath, fileName string) bool
	CreateDir(m *model.Model, parentPath, dirName string) bool
	IsEmptyDir(m *model.Model, parentPath, dirName string) bool
}
