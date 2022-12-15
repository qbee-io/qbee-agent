package log

import "log"

const (
	ERROR = iota
	INFO
	DEBUG
)

var level = INFO

// Debugf logs message with DEBUG severity.
func Debugf(msg string, args ...any) {
	if level < DEBUG {
		return
	}

	log.Printf("[DEBUG] "+msg, args...)
}

// Infof logs message with INFO severity.
func Infof(msg string, args ...any) {
	if level < INFO {
		return
	}

	log.Printf("[INFO] "+msg, args...)
}

// Errorf logs message with ERROR severity.
func Errorf(msg string, args ...any) {
	log.Printf("[ERROR] "+msg, args...)
}

// SetLevel sets current log level.
func SetLevel(newLevel int) {
	level = newLevel
}
