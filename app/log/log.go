package log

import "log"

const (
	ERROR = iota
	WARNING
	INFO
	DEBUG
)

var levelPrefix = map[int]string{
	ERROR:   "[ERROR] ",
	WARNING: "[WARNING] ",
	INFO:    "[INFO] ",
	DEBUG:   "[DEBUG] ",
}

var level = INFO

func logf(msgLevel int, msg string, args ...any) {
	if level < msgLevel {
		return
	}

	log.Printf(levelPrefix[msgLevel]+msg, args...)
}

// Debugf logs message with DEBUG severity.
func Debugf(msg string, args ...any) {
	logf(DEBUG, msg, args...)
}

// Infof logs message with INFO severity.
func Infof(msg string, args ...any) {
	logf(INFO, msg, args...)
}

// Warnf logs message with WARNING severity.
func Warnf(msg string, args ...any) {
	logf(WARNING, msg, args...)
}

// Errorf logs message with ERROR severity.
func Errorf(msg string, args ...any) {
	logf(ERROR, msg, args...)
}

// SetLevel sets current log level.
func SetLevel(newLevel int) {
	level = newLevel
}
