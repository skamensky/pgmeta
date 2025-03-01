package log

import (
	"fmt"
	"log"
	"os"
)

// Logger defines the interface for logging operations
type Logger interface {
	Debug(format string, args ...interface{})
	Info(format string, args ...interface{})
	Warn(format string, args ...interface{})
	Error(format string, args ...interface{})
}

// StandardLogger implements Logger interface using Go's standard log package
type StandardLogger struct {
	debugLogger *log.Logger
	infoLogger  *log.Logger
	warnLogger  *log.Logger
	errorLogger *log.Logger
	debugMode   bool
}

// NewStandardLogger creates a new StandardLogger instance
func NewStandardLogger(debugMode bool) *StandardLogger {
	return &StandardLogger{
		debugLogger: log.New(os.Stdout, "DEBUG: ", log.Ldate|log.Ltime),
		infoLogger:  log.New(os.Stdout, "INFO: ", log.Ldate|log.Ltime),
		warnLogger:  log.New(os.Stdout, "WARN: ", log.Ldate|log.Ltime),
		errorLogger: log.New(os.Stderr, "ERROR: ", log.Ldate|log.Ltime),
		debugMode:   debugMode,
	}
}

// Debug logs a debug message
func (l *StandardLogger) Debug(format string, args ...interface{}) {
	if l.debugMode {
		l.debugLogger.Output(2, fmt.Sprintf(format, args...))
	}
}

// Info logs an info message
func (l *StandardLogger) Info(format string, args ...interface{}) {
	l.infoLogger.Output(2, fmt.Sprintf(format, args...))
}

// Warn logs a warning message
func (l *StandardLogger) Warn(format string, args ...interface{}) {
	l.warnLogger.Output(2, fmt.Sprintf(format, args...))
}

// Error logs an error message
func (l *StandardLogger) Error(format string, args ...interface{}) {
	l.errorLogger.Output(2, fmt.Sprintf(format, args...))
}

// Default logger instance
var defaultLogger Logger = NewStandardLogger(false)

// SetDefaultLogger sets the default logger
func SetDefaultLogger(logger Logger) {
	defaultLogger = logger
}

// EnableDebugMode enables debug logging
func EnableDebugMode() {
	if stdLogger, ok := defaultLogger.(*StandardLogger); ok {
		stdLogger.debugMode = true
	}
}

// Debug logs a debug message using the default logger
func Debug(format string, args ...interface{}) {
	defaultLogger.Debug(format, args...)
}

// Info logs an info message using the default logger
func Info(format string, args ...interface{}) {
	defaultLogger.Info(format, args...)
}

// Warn logs a warning message using the default logger
func Warn(format string, args ...interface{}) {
	defaultLogger.Warn(format, args...)
}

// Error logs an error message using the default logger
func Error(format string, args ...interface{}) {
	defaultLogger.Error(format, args...)
}