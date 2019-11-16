package key

import (
	"encoding/hex"
	"github.com/croz-ltd/dpcmder/utils/logging"
)

type KeyCode string

// Key hexadecimal constants caught by reading bytes from console.
const (
	Return         = KeyCode("0d")
	Tab            = KeyCode("09")
	Space          = KeyCode("20")
	Ch2            = KeyCode("32")
	Ch3            = KeyCode("33")
	Ch4            = KeyCode("34")
	Ch5            = KeyCode("35")
	Ch7            = KeyCode("37")
	Cha            = KeyCode("61")
	ChA            = KeyCode("41")
	Chd            = KeyCode("64")
	Chf            = KeyCode("66")
	Chi            = KeyCode("69")
	ChI            = KeyCode("49")
	Chj            = KeyCode("6a")
	Chk            = KeyCode("6b")
	ChK            = KeyCode("4b")
	Chl            = KeyCode("6c")
	Chn            = KeyCode("6e")
	Cho            = KeyCode("6f")
	ChO            = KeyCode("4f")
	Chp            = KeyCode("70")
	Chq            = KeyCode("71")
	Chs            = KeyCode("73")
	Chu            = KeyCode("75")
	ChU            = KeyCode("55")
	Chx            = KeyCode("78")
	Chz            = KeyCode("7a")
	ChZ            = KeyCode("5a")
	ArrowUp        = KeyCode("1b4f41")
	ArrowDown      = KeyCode("1b4f42")
	ArrowRight     = KeyCode("1b4f43")
	ArrowLeft      = KeyCode("1b4f44")
	ShiftArrowUp   = KeyCode("1b5b313b3241")
	ShiftArrowDown = KeyCode("1b5b313b3242")
	Home           = KeyCode("1b4f48")
	End            = KeyCode("1b4f46")
	ShiftHome      = KeyCode("1b5b313b3248")
	ShiftEnd       = KeyCode("1b5b313b3246")
	PgUp           = KeyCode("1b5b357e")
	PgDown         = KeyCode("1b5b367e")
	ShiftPgUp      = KeyCode("1b5b353b327e")
	ShiftPgDown    = KeyCode("1b5b363b327e")
	F2             = KeyCode("1b4f51")
	F3             = KeyCode("1b4f52")
	F4             = KeyCode("1b4f53")
	F5             = KeyCode("1b5b31357e")
	F7             = KeyCode("1b5b31387e")
	Del            = KeyCode("1b5b337e")
	Backspace      = KeyCode("7f")
	BackspaceWin   = KeyCode("08")
	Slash          = KeyCode("2f")
	Dot            = KeyCode("2e")
	Esc            = KeyCode("1b")
)

// ConvertKeyCodeStringToString converts KeyCode to printable string.
func ConvertKeyCodeStringToString(code KeyCode) string {
	logging.LogDebugf("utils/ConvertKeyCodeStringToString(%s)", code)
	switch code {
	case Esc, Return, Tab,
		ArrowDown, ArrowUp, ArrowLeft, ArrowRight,
		ShiftArrowUp, ShiftArrowDown,
		PgUp, PgDown, ShiftPgUp, ShiftPgDown,
		Backspace, BackspaceWin,
		Home, End, ShiftHome, ShiftEnd, Del:
		return ""
	default:
		res, err := hex.DecodeString(string(code))
		if err != nil {
			logging.LogDebugf("utils/ConvertKeyCodeStringToString(%s), err: %v", code, err)
		}
		return string(res)
	}
}
