package logging

import (
	"fmt"
	"github.com/nsf/termbox-go"
	"log"
	"os"
	"time"
)

var (
	logFilePath string = "./dpcmder.log"
	// MaxEntrySize limits log line length.
	MaxEntrySize int = 1000
	// DebugLogFile enables writing of debug messages to dpcmder.log file in current folder.
	DebugLogFile bool = false
)

func LogFatal(v ...interface{}) {
	termbox.Close()
	log.Fatal(v...)
}

func LogDebug(v ...interface{}) {
	if DebugLogFile {
		msg := fmt.Sprintf("%v: %v\n", time.Now().Format("2006-01-02T15:04:05.999"), v)
		lineLen := len(msg)
		if lineLen > MaxEntrySize {
			msg = msg[:MaxEntrySize] + "\n"
		}

		f, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		if _, err := f.Write([]byte(msg)); err != nil {
			log.Fatal(err)
		}
	}
}
