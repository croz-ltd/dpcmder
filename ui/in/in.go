package in

import (
	"bufio"
	"encoding/hex"
	"github.com/croz-ltd/dpcmder/ui/in/key"
	"github.com/croz-ltd/dpcmder/utils/errs"
	"github.com/croz-ltd/dpcmder/utils/logging"
	"github.com/croz-ltd/dpcmder/worker"
	"io"
	"os"
	"time"
)

// Start starts (blocking) reading user's input.
func Start() {
	logging.LogDebug("ui/in/Start()")

	keyPressedLoop()
}

// keyPressedLoop is main loop reading user's input.
func keyPressedLoop() {
	logging.LogDebug("ui/in/keyPressedLoop() starting")
	reader := newTimeoutReader(bufio.NewReader(os.Stdin), 100*time.Millisecond)

loop:
	for {
		logging.LogTrace("ui/in/keyPressedLoop(), waiting to read")
		readResult := reader.readNext()

		switch readResult.err {
		case readTimeout:
			logging.LogTrace("ui/in/keyPressedLoop(), readTimeout err")
			if worker.IsQuitting() == true {
				break loop
			}
		case nil:
			logging.LogDebugf("ui/in/keyPressedLoop(), readResult: %v", readResult)
			keyCode := readResult.keyCode
			worker.ProcessInputEvent(keyCode)
		default:
			logging.LogFatal("ui/in/keyPressedLoop() unexpected error received.", readResult.err)
		}
	}
	logging.LogDebug("ui/in/keyPressedLoop() stopping")
}

////
// Internal code used to implement non-blocking reading of key pressed events.
////

// readTimeout is error used to make user input reading non blocking.
const readTimeout = errs.Error("ReadTimeout")

// readResult contains either keyEvent created from user's input either error.
type readResult struct {
	keyCode key.KeyCode
	err     error
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
		logging.LogTrace("ui/in/TimeoutReader.readFunc() begin")
		// bytesRead := make([]byte, 6)
		bytesReadCount, err := tr.reader.Read(tr.bytesRead)
		logging.LogTracef("ui/in/TimeoutReader.readFunc(), bytesReadCount: %d, err: %v", bytesReadCount, err)

		hexBytesRead := hex.EncodeToString(tr.bytesRead[0:bytesReadCount])
		keyCode := key.KeyCode(hexBytesRead)
		result := readResult{keyCode: keyCode, err: err}
		tr.readResultChannel <- result
		logging.LogTrace("ui/in/TimeoutReader.readFunc() end")
		tr.readFuncIsRunning = false
	}

	return tr
}

// readNext returns readResult which can contain:
// 1) keyEvent created from successful user input
// 2) err passed from unsuccessful user input
// 3) readTimeout error if user didn't input nothing during timeout period
func (tr *timeoutReader) readNext() readResult {
	logging.LogTracef("ui/in/TimeoutReader.readNext(), tr.readFuncIsRunning: %v", tr.readFuncIsRunning)

	if !tr.readFuncIsRunning {
		go tr.readFunc()
	}

	logging.LogTracef("ui/in/TimeoutReader.readNext(), waiting on channel.")
	select {
	case res := <-tr.readResultChannel:
		logging.LogTracef("ui/in/TimeoutReader.readNext(), res: %v", res)
		return res
	case <-time.After(tr.waitForInput):
		logging.LogTrace("ui/in/TimeoutReader.readNext(), timeout")
		return readResult{err: readTimeout}
	}
}
