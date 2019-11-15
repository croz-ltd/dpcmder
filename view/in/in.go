package in

import (
	"bufio"
	"encoding/hex"
	"github.com/croz-ltd/dpcmder/events"
	"github.com/croz-ltd/dpcmder/utils/logging"
	"github.com/croz-ltd/dpcmder/view/in/key"
	"github.com/croz-ltd/dpcmder/worker"
	"os"
)

func Init(eventChan chan events.KeyPressedEvent) {
	logging.LogDebug("view/in/Init()")

	go keyPressedLoop(eventChan)
}

func Start(keyPressedEventChan chan events.KeyPressedEvent) {
	logging.LogDebug("view/in/Start()")

	keyPressedLoop(keyPressedEventChan)
}

func keyPressedLoop(keyPressedEventChan chan events.KeyPressedEvent) {
	logging.LogDebug("view/in/keyPressedLoop() starting")
	var bytesRead = make([]byte, 6)
	reader := bufio.NewReader(os.Stdin)

loop:
	for {
		logging.LogDebug("view/in/keyPressedLoop(), waiting to read...")
		bytesReadCount, err := reader.Read(bytesRead)
		logging.LogDebugf("view/in/keyPressedLoop(), bytesReadCount: %d, err: %v", bytesReadCount, err)
		if err != nil {
			panic(err)
		}
		hexBytesRead := hex.EncodeToString(bytesRead[0:bytesReadCount])
		keyCode := key.KeyCode(hexBytesRead)
		keyEvent := events.KeyPressedEvent{KeyCode: keyCode}
		logging.LogDebug("view/in/keyPressedLoop(), hexBytesRead: ", hexBytesRead, ", keyEvent: ", keyEvent, ", worker.IsQuitting(): ", worker.IsQuitting())

		if worker.IsQuitting() == true {
			break loop
		}

		keyPressedEventChan <- keyEvent
		logging.LogDebug("view/in/keyPressedLoop(), event sent to channel, worker.IsQuitting(): ", worker.IsQuitting())

		if worker.IsQuitting() == true {
			break loop
		}
	}
	logging.LogDebug("view/in/keyPressedLoop() stopping")
}
