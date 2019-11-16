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
