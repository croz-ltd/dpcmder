package ui

import (
	"bufio"
	"encoding/hex"
	"github.com/croz-ltd/dpcmder/ui/key"
	"github.com/croz-ltd/dpcmder/utils/logging"
	"io"
	"os"
)

// StartReadingKeys starts (blocking) reading user's input.
func StartReadingKeys() {
	logging.LogDebug("ui/in/Start()")

	keyPressedLoop()
}

// keyPressedLoop is main loop reading user's input.
func keyPressedLoop() {
	logging.LogDebug("ui/keyPressedLoop() starting")

loop:
	for {
		logging.LogTrace("ui/keyPressedLoop(), waiting to read")
		keyCode, err := kcr.readNext()

		switch err {
		case nil:
			logging.LogDebugf("ui/keyPressedLoop(), keyCode: %v", keyCode)
			err := ProcessInputEvent(keyCode)
			if err != nil {
				break loop
			}
		case QuitError:
			logging.LogDebug("ui/keyPressedLoop() received quit key.")
			break loop
		default:
			logging.LogDebug("ui/keyPressedLoop() unexpected error received.", err)
		}
	}
	logging.LogDebug("ui/keyPressedLoop() stopping")
}

////
// Internal code used to implement KeyCode reader.
////

// readResult contains either keyEvent created from user's input either error.
type readResult struct {
	keyCode key.KeyCode
	err     error
}

// kcr is reader instantiated for reading KeyCode from user.
var kcr = newKeyCodeReader()

// keyCodeReader is structure used to implement non-blocking user input reading.
type keyCodeReader struct {
	reader    io.Reader
	bytesRead []byte
}

// newTimeoutReader creates new keyCodeReader.
func newKeyCodeReader() *keyCodeReader {
	kcr := new(keyCodeReader)
	kcr.reader = bufio.NewReader(os.Stdin)
	kcr.bytesRead = make([]byte, 6)

	return kcr
}

// readNext returns readResult which can contain:
// 1) keyEvent created from successful user input
// 2) err passed from unsuccessful user input
func (kcr *keyCodeReader) readNext() (key.KeyCode, error) {
	logging.LogTrace("ui/in/TimeoutReader.readNext() begin")
	// bytesRead := make([]byte, 6)
	bytesReadCount, err := kcr.reader.Read(kcr.bytesRead)
	if err != nil {
		return key.None, err
	}
	// bytesReadCount, err := kcr.reader.Read(kcr.bytesRead)
	logging.LogTracef("ui/in/TimeoutReader.readNext(), bytesReadCount: %d, err: %v", bytesReadCount, err)

	hexBytesRead := hex.EncodeToString(kcr.bytesRead[0:bytesReadCount])
	keyCode := key.KeyCode(hexBytesRead)
	logging.LogTracef("ui/in/TimeoutReader.readNext() returning %v, %v", keyCode, err)
	return keyCode, err
}
