package out

import (
	"testing"
)

func TestBuildLine(t *testing.T) {
	testDataMatrix := []struct {
		first, middle, last string
		length              int
		result              string
	}{
		{"=", "=", "=", 20, "===================="},
		{"=", " ", "=", 20, "=                  ="},
		{"=+", " ", "+=", 20, "=+                +="},
		{"Č", "đ", "Ž", 20, "ČđđđđđđđđđđđđđđđđđđŽ"},
	}

	for _, row := range testDataMatrix {
		got := buildLine(row.first, row.middle, row.last, row.length)
		if got != row.result {
			t.Errorf("for BuildLine('%s', '%s', '%s', %d): got '%s', want '%s'", row.first, row.middle, row.last, row.length, got, row.result)
		}
	}
}

func TestScrollLineHoriz(t *testing.T) {
	testDataMatrix := []struct {
		line         string
		horizScroll  int
		lineScrolled string
	}{
		{"abcde1234567890", 0, "abcde1234567890"},
		{"abcde1234567890", 5, "1234567890"},
		{"abcde1234567890", 10, "67890"},
		{"abcde1234567890", 15, ""},
		{"abcde1234567890", 20, ""},
	}

	for _, row := range testDataMatrix {
		got := scrollLineHoriz(row.line, row.horizScroll)
		if got != row.lineScrolled {
			t.Errorf("for scrollLineHoriz('%s', %d): got '%s', want '%s'", row.line, row.horizScroll, got, row.lineScrolled)
		}
	}
}
