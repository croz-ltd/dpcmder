package in

import (
	"bufio"
	"encoding/hex"
	"github.com/croz-ltd/dpcmder/events"
	"github.com/croz-ltd/dpcmder/utils/logging"
	"github.com/croz-ltd/dpcmder/view/in/key"
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
	logging.LogDebug("view/in/keyPressedLoop()")
	var bytesRead = make([]byte, 6)
	reader := bufio.NewReader(os.Stdin)

loop:
	for {
		bytesReadCount, err := reader.Read(bytesRead)
		if err != nil {
			panic(err)
		}
		hexBytesRead := hex.EncodeToString(bytesRead[0:bytesReadCount])
		keyEvent := events.KeyPressedEvent{HexBytes: hexBytesRead}
		logging.LogDebug("hexBytesRead: ", hexBytesRead, "keyEvent: ", keyEvent)

		keyPressedEventChan <- keyEvent
		switch hexBytesRead {
		case key.Chq:
			break loop
		}
	}
}
