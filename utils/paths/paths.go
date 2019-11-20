package paths

import (
	"os"
	"strings"
)

// GetFilePath generates local os correct path from parentPath and fileName.
func GetFilePath(parentPath, fileName string) string {
	return GetFilePathUsingSeparator(parentPath, fileName, string(os.PathSeparator))
}

// GetDpPath generates local os correct path from parentPath and fileName.
func GetDpPath(parentPath, fileName string) string {
	return GetFilePathUsingSeparator(parentPath, fileName, "/")
}

// GetFilePathUsingSeparator generates correct path from parentPath and fileName
//  using given path sepearator.
func GetFilePathUsingSeparator(parentPath, fileName, pathSeparator string) string {
	if fileName == ".." {
		lastSeparatorIdx := strings.LastIndex(parentPath, pathSeparator)
		if lastSeparatorIdx != -1 && len(parentPath) > 1 {
			if lastSeparatorIdx == 0 {
				return "/"
			}
			return parentPath[:lastSeparatorIdx]
		}
		return parentPath
	} else if parentPath == "" {
		return fileName
	}
	if strings.HasSuffix(parentPath, pathSeparator) {
		return parentPath + fileName
	}
	return parentPath + pathSeparator + fileName
}

// SplitDpPath splits DataPower path into splice where first element is
// filestore name and rest are directory names.
func SplitDpPath(path string) []string {
	if path == "" {
		return make([]string, 0)
	}
	return strings.Split(path, "/")
}
