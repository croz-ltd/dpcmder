// Package logging implements methods used for logging to dpcmder.log file and
// adds timestamp to each log line.
package logging

import (
	"fmt"
	"log"
	"os"
	"time"
)

var (
	logFilePath = "./dpcmder.log"
	// MaxEntrySize limits log line length.
	MaxEntrySize = 1000
	// DebugLogFile enables writing of debug messages to dpcmder.log file in current folder.
	DebugLogFile = false
	// TraceLogFile enables writing of trace messages to dpcmder.log file in current folder.
	TraceLogFile = false
)

// LogFatal logs fatal error message to log file and exits dpcmder.
func LogFatal(v ...interface{}) {
	log.Fatal(v...)
}

// LogDebug logs debug message to log file.
func LogDebug(v ...interface{}) {
	if DebugLogFile || TraceLogFile {
		logInternal(fmt.Sprint(v...))
	}
}

// LogDebugf logs formatted debug message to log file.
func LogDebugf(format string, params ...interface{}) {
	if DebugLogFile || TraceLogFile {
		logInternal(fmt.Sprintf(format, params...))
	}
}

// LogTrace logs trace message to log file.
func LogTrace(v ...interface{}) {
	if TraceLogFile {
		logInternal(fmt.Sprint(v...))
	}
}

// LogTracef logs trace message to log file.
func LogTracef(format string, params ...interface{}) {
	if TraceLogFile {
		logInternal(fmt.Sprintf(format, params...))
	}
}

func logInternal(logMsg string) {
	timedMsg := fmt.Sprintf("%s: %s\n", time.Now().Format("2006-01-02T15:04:05.999"), logMsg)
	lineLen := len(timedMsg)
	if lineLen > MaxEntrySize {
		timedMsg = timedMsg[:MaxEntrySize] + "\n"
	}

	f, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	if _, err := f.Write([]byte(timedMsg)); err != nil {
		log.Fatal(err)
	}
}
