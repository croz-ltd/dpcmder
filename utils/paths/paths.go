// Package paths implements DataPower and local filesystem path operations used
// to create proper paths recognized by DataPower and local filesystem.
package paths

import (
	"os"
	"strings"
)

// GetFilePath generates local os correct path from parentPath and fileName.
func GetFilePath(parentPath, fileName string) string {
	return getFilePathUsingSeparator(parentPath, fileName, string(os.PathSeparator))
}

// GetFileName extract local os name from fullPath.
func GetFileName(fullPath string) string {
	return getFileNameUsingSeparator(fullPath, string(os.PathSeparator))
}

// GetDpPath generates local os correct path from parentPath and fileName.
func GetDpPath(parentPath, fileName string) string {
	fullPath := getFilePathUsingSeparator(parentPath, fileName, "/")
	firstSeparatorIdx := strings.Index(fullPath, "/")
	filestoreEndIdx := strings.Index(fullPath, ":")
	switch {
	case filestoreEndIdx == -1 && firstSeparatorIdx == -1:
		return fullPath + ":"
	case filestoreEndIdx == -1 && firstSeparatorIdx != -1:
		return fullPath[0:firstSeparatorIdx] + ":" + fullPath[firstSeparatorIdx:]
	default:
		return fullPath
	}
}

// getFilePathUsingSeparator generates correct path from parentPath and fileName
//  using given path sepearator.
func getFilePathUsingSeparator(parentPath, fileName, pathSeparator string) string {
	switch {
	case fileName == ".", fileName == "":
		return parentPath
	case fileName == "..":
		lastSeparatorIdx := strings.LastIndex(parentPath, pathSeparator)
		if lastSeparatorIdx != -1 && len(parentPath) > 1 {
			if lastSeparatorIdx == 0 {
				return "/"
			}
			return parentPath[:lastSeparatorIdx]
		}
		return parentPath
	case parentPath == "":
		return fileName
	case strings.HasSuffix(parentPath, pathSeparator):
		return parentPath + fileName
	default:
		return parentPath + pathSeparator + fileName
	}
}

// getFileNameUsingSeparator extract fileName from full path
//  using given path sepearator.
func getFileNameUsingSeparator(fullPath, pathSeparator string) string {
	switch {
	case fullPath == "/":
		return "/"
	case strings.HasSuffix(fullPath, pathSeparator):
		fullPath = strings.TrimRight(fullPath, pathSeparator)
	}

	pathParts := strings.Split(fullPath, pathSeparator)

	partsNo := len(pathParts)
	fileName := "/"

	switch {
	case partsNo == 0:
	case pathParts[partsNo-1] == "":
	case pathParts[partsNo-1] == "." && partsNo > 1:
		fileName = pathParts[partsNo-2]
	case pathParts[partsNo-1] == ".." && partsNo > 2:
		fileName = pathParts[partsNo-3]
	case pathParts[partsNo-1] == ".." && partsNo > 1:
		fileName = pathParts[partsNo-2]
	default:
		fileName = pathParts[partsNo-1]
	}

	if fileName == "" {
		return "/"
	}

	return fileName
}

// SplitDpPath splits DataPower path into splice where first element is
// filestore name and rest are directory names.
func SplitDpPath(path string) []string {
	if path == "" {
		return make([]string, 0)
	}
	return strings.Split(path, "/")
}
