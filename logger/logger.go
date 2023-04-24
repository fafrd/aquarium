package logger

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"
)

var logch chan string
var termch chan string
var debug bool

var logFile *os.File
var logTerminalFile *os.File

const (
	logFilename         = "aquarium.log"
	logTerminalFilename = "terminal.log"
	debugFilename       = "debug.log"
)

func Init(_logch chan string, _termch chan string, _debug bool) {
	logch = _logch
	termch = _termch
	debug = _debug

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
		logTerminalFile, err = os.OpenFile(logTerminalFilename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			panic(err)
		}

		_, err := logTerminalFile.WriteString(fmt.Sprintf("\nStarting new log session at %s\n", time.Now().Format("2006-01-02 15:04:05")))
		if err != nil {
			panic(err)
		}
	}
}

// Logf sends the log message along the default logger channel
// and appends to aquarium.log
func Logf(msg string, args ...interface{}) {
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

// LogTerminalf sends the log message along a different channel
// and completely replaces the current terminal.log file with a new one
func LogTerminalf(msg string, args ...interface{}) {
	if termch == nil {
		panic("logger not initialized")
	}

	result := ""
	msgFormatted := fmt.Sprintf(msg, args...)
	// clean up the last.pid process tracking part
	regex := regexp.MustCompile(`(.+?@.+?:.+?\$ )/bin/bash -c "echo \\\$\\\$>/tmp/last.pid && exec (.+?)"?$`)
	regexUncutLine := regexp.MustCompile(`(.+?@.+?:.+?\$ )/bin/bash -c "echo \\\$\\\$>/tmp/last.pid && exec (.+?)"$`)
	previousPartialMatch := false
	for _, line := range strings.Split(msgFormatted, "\n") {
		if regex.MatchString(line) {
			result += strings.ReplaceAll(regex.ReplaceAllString(line, "${1}${2}"), "\"'\"'\"", "\"")
			if !regexUncutLine.MatchString(line) {
				previousPartialMatch = true
			} else {
				result += "\n"
			}
		} else {
			if previousPartialMatch {
				line = line[1 : len(line)-1]
			}
			previousPartialMatch = false
			result += strings.ReplaceAll(line, "\"'\"'\"", "\"") + "\n"
		}
	}

	termch <- result

	err := logTerminalFile.Truncate(0)
	if err != nil {
		termch <- fmt.Sprintf("Error writing to log file: %s", err)
	}

	_, err = logTerminalFile.Seek(0, 0)
	if err != nil {
		termch <- fmt.Sprintf("Error writing to log file: %s", err)
	}

	_, err = logTerminalFile.WriteString(msgFormatted)
	if err != nil {
		termch <- fmt.Sprintf("Error writing to log file: %s", err)
	}
}

func Debugf(msg string, args ...interface{}) {
	if !debug {
		return
	}
	var debugFile *os.File
	if _, err := os.Stat(debugFilename); os.IsNotExist(err) {
		debugFile, err = os.Create(debugFilename)
		if err != nil {
			panic(err)
		}
	} else {
		debugFile, err = os.OpenFile(debugFilename, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			panic(err)
		}
	}

	msgFormatted := fmt.Sprintf(msg, args...)
	_, err := debugFile.WriteString(msgFormatted)
	if err != nil {
		logch <- fmt.Sprintf("Error writing to log file: %s", err)
	}

}
