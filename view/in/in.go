package in

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"github.com/croz-ltd/dpcmder/view/in/key"
	"os"
)

func Init(eventChan chan string) {
	fmt.Println("view/in/Init()")

	go keyPressedLoop(eventChan)
}

func Start(eventChan chan string) {
	fmt.Println("view/in/Start()")

	keyPressedLoop(eventChan)
}

func keyPressedLoop(eventChan chan string) {
	var bytesRead = make([]byte, 6)
	reader := bufio.NewReader(os.Stdin)

loop:
	for {
		bytesReadCount, err := reader.Read(bytesRead)
		if err != nil {
			panic(err)
		}
		hexBytesRead := hex.EncodeToString(bytesRead[0:bytesReadCount])

		eventChan <- hexBytesRead
		switch hexBytesRead {
		case key.Chq:
			break loop
		}
	}
}
