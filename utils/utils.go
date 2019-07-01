package utils

import (
	"os"
	"strings"
)

// SplitOnFirst splits given string in two parts (prefix, suffix) where prefix is
// part of the string before first found splitterString and suffix is part of string
// after first found splitterString.
func SplitOnFirst(wholeString string, splitterString string) (string, string) {
	prefix := wholeString
	suffix := ""

	lastIdx := strings.Index(wholeString, splitterString)
	if lastIdx != -1 {
		prefix = wholeString[:lastIdx]
		suffix = wholeString[lastIdx+1:]
	}

	return prefix, suffix
}

// SplitOnLast splits given string in two parts (prefix, suffix) where prefix is
// part of the string before last found splitterString and suffix is part of string
// after last found splitterString.
func SplitOnLast(wholeString string, splitterString string) (string, string) {
	prefix := wholeString
	suffix := ""

	lastIdx := strings.LastIndex(wholeString, splitterString)
	if lastIdx != -1 {
		prefix = wholeString[:lastIdx]
		suffix = wholeString[lastIdx+1:]
	}

	return prefix, suffix
}

// GetFilePath generates local os correct path from parentPath and fileName.
func GetFilePath(parentPath, fileName string) string {
	if fileName == ".." {
		lastSeparatorIdx := strings.LastIndex(parentPath, string(os.PathSeparator))
		if lastSeparatorIdx != -1 && len(parentPath) > 1 {
			return parentPath[:lastSeparatorIdx]
		}
		return parentPath
	} else if parentPath == "" {
		return fileName
	}
	return parentPath + string(os.PathSeparator) + fileName
}
