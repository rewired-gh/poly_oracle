// Package logger provides leveled structured logging.
package logger

import (
	"fmt"
	"log"
	"os"
	"strings"
)

// Level represents a logging level.
type Level int

const (
	DebugLevel Level = iota
	InfoLevel
	WarnLevel
	ErrorLevel
)

// Logger provides leveled logging.
type Logger struct {
	level  Level
	logger *log.Logger
}

var defaultLogger *Logger

// Init initializes the default logger with the specified level and format.
func Init(level string, format string) {
	var l Level
	switch strings.ToLower(level) {
	case "debug":
		l = DebugLevel
	case "info":
		l = InfoLevel
	case "warn":
		l = WarnLevel
	case "error":
		l = ErrorLevel
	default:
		l = InfoLevel
	}

	flags := log.LstdFlags | log.Lmicroseconds
	if strings.ToLower(format) == "text" {
		flags |= log.Lshortfile
	}

	defaultLogger = &Logger{
		level:  l,
		logger: log.New(os.Stderr, "", flags),
	}
}

func Debug(format string, args ...interface{}) {
	if defaultLogger != nil && defaultLogger.level <= DebugLevel {
		msg := fmt.Sprintf("[DEBUG] "+format, args...)
		_ = defaultLogger.logger.Output(2, msg)
	}
}

func Info(format string, args ...interface{}) {
	if defaultLogger != nil && defaultLogger.level <= InfoLevel {
		msg := fmt.Sprintf("[INFO] "+format, args...)
		_ = defaultLogger.logger.Output(2, msg)
	}
}

func Warn(format string, args ...interface{}) {
	if defaultLogger != nil && defaultLogger.level <= WarnLevel {
		msg := fmt.Sprintf("[WARN] "+format, args...)
		_ = defaultLogger.logger.Output(2, msg)
	}
}

func Error(format string, args ...interface{}) {
	if defaultLogger != nil && defaultLogger.level <= ErrorLevel {
		msg := fmt.Sprintf("[ERROR] "+format, args...)
		_ = defaultLogger.logger.Output(2, msg)
	}
}

func Fatal(format string, args ...interface{}) {
	msg := fmt.Sprintf("[FATAL] "+format, args...)
	if defaultLogger != nil {
		_ = defaultLogger.logger.Output(2, msg)
	}
	os.Exit(1)
}
