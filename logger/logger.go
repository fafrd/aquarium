package logger

import (
	"fmt"
	"os"
	"time"
)

var logch chan string
var termch chan string

var logFile *os.File
var logTerminalFile *os.File

const (
	logFilename         = "aquarium.log"
	logTerminalFilename = "terminal.log"
)

func Init(_logch chan string, _termch chan string) {
	logch = _logch
	termch = _termch

	// Check if file exists
	if _, err := os.Stat(logFilename); os.IsNotExist(err) {
		// File doesn't exist, create new file
		logFile, err = os.Create(logFilename)
		if err != nil {
			panic(err)
		}

		_, err := logFile.WriteString(fmt.Sprintf("Starting new log session at %s\n", time.Now().Format("2006-01-02 15:04:05")))
		if err != nil {
			panic(err)
		}
	} else {
		// File already exists, open file for appending
		logFile, err = os.OpenFile(logFilename, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			panic(err)
		}

		_, err := logFile.WriteString(fmt.Sprintf("\nStarting new log session at %s\n", time.Now().Format("2006-01-02 15:04:05")))
		if err != nil {
			panic(err)
		}
	}

	// same thing, but replace logFilename with logTerminalFilename
	if _, err := os.Stat(logTerminalFilename); os.IsNotExist(err) {
		logTerminalFile, err = os.Create(logTerminalFilename)
		if err != nil {
			panic(err)
		}

		_, err := logTerminalFile.WriteString(fmt.Sprintf("Starting new log session at %s\n", time.Now().Format("2006-01-02 15:04:05")))
		if err != nil {
			panic(err)
		}
	} else {
		logTerminalFile, err = os.OpenFile(logTerminalFilename, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			panic(err)
		}

		_, err := logTerminalFile.WriteString(fmt.Sprintf("\nStarting new log session at %s\n", time.Now().Format("2006-01-02 15:04:05")))
		if err != nil {
			panic(err)
		}
	}
}

// Log sends the log message along the default logger channel
func Log(msg string, args ...interface{}) {
	if logch == nil {
		panic("logger not initialized")
	}
	msgFormatted := fmt.Sprintf(msg, args...)

	logch <- msgFormatted

	_, err := logFile.WriteString(msgFormatted)
	if err != nil {
		logch <- fmt.Sprintf("Error writing to log file: %s", err)
	}
}

// LogTerminal sends the log message along a different channel
func LogTerminal(msg string, args ...interface{}) {
	if termch == nil {
		panic("logger not initialized")
	}

	msgFormatted := fmt.Sprintf(msg, args...)

	termch <- msgFormatted

	_, err := logTerminalFile.WriteString(msgFormatted)
	if err != nil {
		termch <- fmt.Sprintf("Error writing to log file: %s", err)
	}
}
