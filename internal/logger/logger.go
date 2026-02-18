// Package logger provides leveled logging with support for debug, info, warn, and error levels.
// It wraps the standard log package to provide level-based filtering and formatted output.
package logger

import (
	"fmt"
	"log"
	"os"
	"strings"
)

// Level represents a logging level
type Level int

const (
	// DebugLevel logs are typically voluminous, and are usually disabled in production.
	DebugLevel Level = iota
	// InfoLevel is the default logging priority.
	InfoLevel
	// WarnLevel logs are more important than Info, but don't need individual human review.
	WarnLevel
	// ErrorLevel logs are high-priority. If an application is running smoothly, it shouldn't generate any error-level logs.
	ErrorLevel
)

// Logger provides leveled logging
type Logger struct {
	level  Level
	logger *log.Logger
}

var (
	// Global logger instance
	defaultLogger *Logger
)

// Init initializes the default logger with the specified level and format
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

	// Set log flags based on format
	flags := log.LstdFlags | log.Lmicroseconds
	if strings.ToLower(format) == "text" {
		flags |= log.Lshortfile
	}

	defaultLogger = &Logger{
		level:  l,
		logger: log.New(os.Stderr, "", flags),
	}
}

// Debug logs a message at DebugLevel
func Debug(format string, args ...interface{}) {
	if defaultLogger != nil && defaultLogger.level <= DebugLevel {
		msg := fmt.Sprintf("[DEBUG] "+format, args...)
		_ = defaultLogger.logger.Output(2, msg)
	}
}

// Info logs a message at InfoLevel
func Info(format string, args ...interface{}) {
	if defaultLogger != nil && defaultLogger.level <= InfoLevel {
		msg := fmt.Sprintf("[INFO] "+format, args...)
		_ = defaultLogger.logger.Output(2, msg)
	}
}

// Warn logs a message at WarnLevel
func Warn(format string, args ...interface{}) {
	if defaultLogger != nil && defaultLogger.level <= WarnLevel {
		msg := fmt.Sprintf("[WARN] "+format, args...)
		_ = defaultLogger.logger.Output(2, msg)
	}
}

// Error logs a message at ErrorLevel
func Error(format string, args ...interface{}) {
	if defaultLogger != nil && defaultLogger.level <= ErrorLevel {
		msg := fmt.Sprintf("[ERROR] "+format, args...)
		_ = defaultLogger.logger.Output(2, msg)
	}
}

// Fatal logs a message at ErrorLevel and exits
func Fatal(format string, args ...interface{}) {
	msg := fmt.Sprintf("[FATAL] "+format, args...)
	if defaultLogger != nil {
		_ = defaultLogger.logger.Output(2, msg)
	}
	// Use os.Exit directly instead of log.Fatal to avoid double-logging.
	// The message has already been written to defaultLogger above.
	os.Exit(1)
}
