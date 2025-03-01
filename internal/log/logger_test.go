package log

import (
	"bytes"
	"log"
	"os"
	"strings"
	"testing"
)

func TestStandardLogger(t *testing.T) {
	// Redirect stdout to capture output
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stdout = w
	os.Stderr = w

	// Reset after test
	defer func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
	}()

	// Create a logger with debug mode enabled
	logger := NewStandardLogger(true)

	// Test debug logging
	logger.Debug("Debug message: %s", "test")
	
	// Test info logging
	logger.Info("Info message: %s", "test")
	
	// Test warning logging
	logger.Warn("Warning message: %s", "test")
	
	// Test error logging
	logger.Error("Error message: %s", "test")

	// Close the writer to flush the buffer
	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Check if all message levels were logged
	if !strings.Contains(output, "DEBUG: ") {
		t.Error("Debug message not logged")
	}
	if !strings.Contains(output, "INFO: ") {
		t.Error("Info message not logged")
	}
	if !strings.Contains(output, "WARN: ") {
		t.Error("Warning message not logged")
	}
	if !strings.Contains(output, "ERROR: ") {
		t.Error("Error message not logged")
	}

	// Check message contents
	if !strings.Contains(output, "Debug message: test") {
		t.Error("Debug message content incorrect")
	}
	if !strings.Contains(output, "Info message: test") {
		t.Error("Info message content incorrect")
	}
	if !strings.Contains(output, "Warning message: test") {
		t.Error("Warning message content incorrect")
	}
	if !strings.Contains(output, "Error message: test") {
		t.Error("Error message content incorrect")
	}
}

func TestDebugModeDisabled(t *testing.T) {
	// Create a custom writer to capture output
	var buf bytes.Buffer
	testLogger := log.New(&buf, "DEBUG: ", 0)
	
	// Create a logger with debug mode disabled
	logger := &StandardLogger{
		debugLogger: testLogger,
		infoLogger:  log.New(os.Stdout, "INFO: ", 0),
		warnLogger:  log.New(os.Stdout, "WARN: ", 0),
		errorLogger: log.New(os.Stderr, "ERROR: ", 0),
		debugMode:   false,
	}

	// Test debug logging when disabled
	logger.Debug("This should not be logged")

	// Check that nothing was logged
	if buf.Len() > 0 {
		t.Error("Debug message was logged when debug mode was disabled")
	}
}

func TestGlobalLogFunctions(t *testing.T) {
	// Save the original default logger
	originalLogger := defaultLogger
	defer func() {
		defaultLogger = originalLogger
	}()

	// Create a mock logger
	mockLogger := &mockLogger{}
	
	// Set as default logger
	SetDefaultLogger(mockLogger)
	
	// Test global functions
	Debug("debug message")
	Info("info message")
	Warn("warn message")
	Error("error message")

	// Check that all methods were called
	if !mockLogger.debugCalled {
		t.Error("Global Debug function did not call the underlying logger")
	}
	if !mockLogger.infoCalled {
		t.Error("Global Info function did not call the underlying logger")
	}
	if !mockLogger.warnCalled {
		t.Error("Global Warn function did not call the underlying logger")
	}
	if !mockLogger.errorCalled {
		t.Error("Global Error function did not call the underlying logger")
	}
}

func TestEnableDebugMode(t *testing.T) {
	// Create a logger with debug mode disabled
	logger := NewStandardLogger(false)
	SetDefaultLogger(logger)
	
	// Enable debug mode
	EnableDebugMode()
	
	// Verify the debug mode was enabled
	stdLogger, ok := defaultLogger.(*StandardLogger)
	if !ok {
		t.Error("Default logger is not a StandardLogger")
		return
	}
	
	if !stdLogger.debugMode {
		t.Error("EnableDebugMode did not enable debug mode")
	}
}

// mockLogger implements the Logger interface for testing
type mockLogger struct {
	debugCalled bool
	infoCalled  bool
	warnCalled  bool
	errorCalled bool
}

func (m *mockLogger) Debug(format string, args ...interface{}) {
	m.debugCalled = true
}

func (m *mockLogger) Info(format string, args ...interface{}) {
	m.infoCalled = true
}

func (m *mockLogger) Warn(format string, args ...interface{}) {
	m.warnCalled = true
}

func (m *mockLogger) Error(format string, args ...interface{}) {
	m.errorCalled = true
}