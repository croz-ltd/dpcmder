package utils

import (
	"github.com/croz-ltd/dpcmder/view/in/key"
	"testing"
)

func TestSplitOnFirst(t *testing.T) {
	testDataMatrix := [][]string{
		{"/usr/bin/share", "/", "", "usr/bin/share"},
		{"usr/bin/share", "/", "usr", "bin/share"},
		{"/share", "/", "", "share"},
		{"share", "/", "share", ""},
		{"my big testing task", " ", "my", "big testing task"},
	}
	for _, testCase := range testDataMatrix {
		gotPreffix, gotSuffix := SplitOnFirst(testCase[0], testCase[1])
		if gotPreffix != testCase[2] || gotSuffix != testCase[3] {
			t.Errorf("for SplitOnFirst('%s', '%s'): got ('%s', '%s'), want ('%s', '%s')", testCase[0], testCase[1], gotPreffix, gotSuffix, testCase[2], testCase[3])
		}
	}
}

func TestSplitOnLast(t *testing.T) {
	testDataMatrix := [][]string{
		{"/usr/bin/share", "/", "/usr/bin", "share"},
		{"usr/bin/share", "/", "usr/bin", "share"},
		{"/share", "/", "", "share"},
		{"share", "/", "share", ""},
		{"my big testing task", " ", "my big testing", "task"},
		{"local:/test1/test2", "/", "local:/test1", "test2"},
		{"local:/test1", "/", "local:", "test1"},
		{"local:", "/", "local:", ""},
	}
	for _, testCase := range testDataMatrix {
		gotPreffix, gotSuffix := SplitOnLast(testCase[0], testCase[1])
		if gotPreffix != testCase[2] || gotSuffix != testCase[3] {
			t.Errorf("for SplitOnLast('%s', '%s'): got ('%s', '%s'), want ('%s', '%s')", testCase[0], testCase[1], gotPreffix, gotSuffix, testCase[2], testCase[3])
		}
	}
}

func TestGetDpPath(t *testing.T) {
	testDataMatrix := [][]string{
		{"local:/dir1/dir2", "myfile", "local:/dir1/dir2/myfile"},
		{"local:", "myfile", "local:/myfile"},
		{"local:/dir1/dir2", "..", "local:/dir1"},
		{"local:/dir1", "..", "local:"},
		{"local:", "..", "local:"},
	}
	for _, testCase := range testDataMatrix {
		newPath := GetDpPath(testCase[0], testCase[1])
		if newPath != testCase[2] {
			t.Errorf("for GetFilePath('%s', '%s'): got '%s', want '%s'", testCase[0], testCase[1], newPath, testCase[2])
		}
	}
}

func TestGetFilePathUsingSeparator(t *testing.T) {
	testDataMatrix := [][]string{
		{"/usr/bin/share", "myfile", "/", "/usr/bin/share/myfile"},
		{"", "myfile", "/", "myfile"},
		{"/usr/bin/share", "..", "/", "/usr/bin"},
		{"/testdir", "..", "/", "/"},
		{"", "..", "/", ""},
		{"/", "..", "/", "/"},
		{"/", "testfile", "/", "/testfile"},
	}
	for _, testCase := range testDataMatrix {
		newPath := GetFilePathUsingSeparator(testCase[0], testCase[1], testCase[2])
		if newPath != testCase[3] {
			t.Errorf("for GetFilePathUsingSeparator('%s', '%s', '%s'): got '%s', want '%s'", testCase[0], testCase[1], testCase[2], newPath, testCase[3])
		}
	}
}

func TestBuildLine(t *testing.T) {
	testDataMatrix := []struct {
		first, middle, last string
		length              int
		result              string
	}{
		{"=", "=", "=", 20, "===================="},
		{"=", " ", "=", 20, "=                  ="},
		{"=+", " ", "+=", 20, "=+                +="},
	}

	for _, row := range testDataMatrix {
		got := BuildLine(row.first, row.middle, row.last, row.length)
		if got != row.result {
			t.Errorf("for BuildLine('%s', '%s', '%s', %d): got '%s', want '%s'", row.first, row.middle, row.last, row.length, got, row.result)
		}
	}
}

func TestConvertKeyCodeStringToString(t *testing.T) {
	testDataMatrix := [][]string{
		{"6e", "n"},
		{"65", "e"},
		{"32", "2"},
		{"20", " "},
		{"41", "A"},
		{"0d", ""},
		{"09", ""},
		{"1b", ""},
		{"Z", ""},
		{string(key.ArrowLeft), ""},
		{string(key.ArrowRight), ""},
		{string(key.ArrowUp), ""},
		{string(key.ArrowDown), ""},
		{string(key.Return), ""},
		{string(key.Esc), ""},
		{string(key.Del), ""},
		{string(key.Backspace), ""},
		{string(key.BackspaceWin), ""},
		{string(key.Home), ""},
		{string(key.End), ""},
	}

	for _, row := range testDataMatrix {
		input := row[0]
		got := ConvertKeyCodeStringToString(key.KeyCode(input))
		want := row[1]
		if got != want {
			t.Errorf("for ConvertKeyCodeStringToString('%s'): got '%s', want '%s'", input, got, want)
		}
	}
}
