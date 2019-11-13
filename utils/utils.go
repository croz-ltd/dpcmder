package utils

import (
	"os"
	"strings"
)

// Error type is used to create constant errors.
type Error string

func (e Error) Error() string { return string(e) }

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

func BuildLine(first, middle, last string, length int) string {
	middleLen := (length - len(first) - len(last)) / len(middle)
	return first + strings.Repeat(middle, middleLen) + last
}
