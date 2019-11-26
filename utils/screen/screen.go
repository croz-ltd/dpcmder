package screen

import (
	"github.com/nsf/termbox-go"
)

// TermboxClose cleans up terminal session if needed.
func TermboxClose() {
	if termbox.IsInit {
		termbox.Close()
	}
}
