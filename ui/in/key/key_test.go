package key

import (
	"testing"
)

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
		{string(ArrowLeft), ""},
		{string(ArrowRight), ""},
		{string(ArrowUp), ""},
		{string(ArrowDown), ""},
		{string(Return), ""},
		{string(Esc), ""},
		{string(Del), ""},
		{string(Backspace), ""},
		{string(BackspaceWin), ""},
		{string(Home), ""},
		{string(End), ""},
	}

	for _, row := range testDataMatrix {
		input := row[0]
		got := ConvertKeyCodeStringToString(KeyCode(input))
		want := row[1]
		if got != want {
			t.Errorf("for ConvertKeyCodeStringToString('%s'): got '%s', want '%s'", input, got, want)
		}
	}
}
