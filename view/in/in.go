package in

import (
	"bufio"
	"encoding/hex"
	"github.com/croz-ltd/dpcmder/events"
	"github.com/croz-ltd/dpcmder/utils/errs"
	"github.com/croz-ltd/dpcmder/utils/logging"
	"github.com/croz-ltd/dpcmder/view/in/key"
	"github.com/croz-ltd/dpcmder/worker"
	"io"
	"os"
	"time"
)

// Start starts (blocking) reading user's input.
func Start(keyPressedEventChan chan events.KeyPressedEvent) {
	logging.LogDebug("view/in/Start()")

	keyPressedLoop(keyPressedEventChan)
}

// keyPressedLoop is main loop reading user's input.
func keyPressedLoop(keyPressedEventChan chan events.KeyPressedEvent) {
	logging.LogDebug("view/in/keyPressedLoop() starting")
	reader := newTimeoutReader(bufio.NewReader(os.Stdin), 100*time.Millisecond)

loop:
	for {
		logging.LogTrace("view/in/keyPressedLoop(), waiting to read")
		readResult := reader.readNext()

		switch readResult.err {
		case readTimeout:
			logging.LogTrace("view/in/keyPressedLoop(), readTimeout err")
			if worker.IsQuitting() == true {
				break loop
			}
		case nil:
			logging.LogTracef("view/in/keyPressedLoop(), readResult: %v", readResult)
			keyEvent := readResult.keyEvent
			keyPressedEventChan <- keyEvent
		default:
			logging.LogFatal("view/in/keyPressedLoop() unexpected error received.", readResult.err)
		}
	}
	logging.LogDebug("view/in/keyPressedLoop() stopping")
}

////
// Internal code used to implement non-blocking reading of key pressed events.
////

// readTimeout is error used to make user input reading non blocking.
const readTimeout = errs.Error("ReadTimeout")

// readResult contains either keyEvent created from user's input either error.
type readResult struct {
	keyEvent events.KeyPressedEvent
	err      error
}

// timeoutReader is structure used to implement non-blocking user input reading.
type timeoutReader struct {
	reader            io.Reader
	readResultChannel chan (readResult)
	bytesRead         []byte
	bytesReadCount    int
	err               error
	waitForInput      time.Duration
	readFunc          func()
	readFuncIsRunning bool
}

// newTimeoutReader creates new timeoutReader.
func newTimeoutReader(reader io.Reader, timeout time.Duration) *timeoutReader {
	tr := new(timeoutReader)
	tr.reader = reader
	tr.waitForInput = timeout
	tr.bytesRead = make([]byte, 6)
	tr.readResultChannel = make(chan readResult, 1)

	tr.readFunc = func() {
		tr.readFuncIsRunning = true
		logging.LogTrace("view/in/TimeoutReader.readFunc() begin")
		// bytesRead := make([]byte, 6)
		bytesReadCount, err := tr.reader.Read(tr.bytesRead)
		logging.LogTracef("view/in/TimeoutReader.readFunc(), bytesReadCount: %d, err: %v", bytesReadCount, err)

		hexBytesRead := hex.EncodeToString(tr.bytesRead[0:bytesReadCount])
		keyCode := key.KeyCode(hexBytesRead)
		keyEvent := events.KeyPressedEvent{KeyCode: keyCode}
		result := readResult{keyEvent: keyEvent, err: err}
		tr.readResultChannel <- result
		logging.LogTrace("view/in/TimeoutReader.readFunc() end")
		tr.readFuncIsRunning = false
	}

	return tr
}

// readNext returns readResult which can contain:
// 1) keyEvent created from successful user input
// 2) err passed from unsuccessful user input
// 3) readTimeout error if user didn't input nothing during timeout period
func (tr *timeoutReader) readNext() readResult {
	logging.LogTracef("view/in/TimeoutReader.readNext(), tr.readFuncIsRunning: %v", tr.readFuncIsRunning)

	if !tr.readFuncIsRunning {
		go tr.readFunc()
	}

	logging.LogTracef("view/in/TimeoutReader.readNext(), waiting on channel.")
	select {
	case res := <-tr.readResultChannel:
		logging.LogTracef("view/in/TimeoutReader.readNext(), res: %v", res)
		return res
	case <-time.After(tr.waitForInput):
		logging.LogTrace("view/in/TimeoutReader.readNext(), timeout")
		return readResult{err: readTimeout}
	}
}
