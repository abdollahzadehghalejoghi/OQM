package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
)

var (
	infoLogger  *log.Logger
	errorLogger *log.Logger
	debugLogger *log.Logger
)

// Initialize sets up the logger
func Initialize(logFilePath string) error {
	// Create log directory if not exists
	dir := filepath.Dir(logFilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	// Open log file
	file, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	// Create multi-writer (file + stdout)
	multiWriter := io.MultiWriter(file, os.Stdout)

	// Initialize loggers
	infoLogger = log.New(multiWriter, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
	errorLogger = log.New(multiWriter, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)
	debugLogger = log.New(multiWriter, "DEBUG: ", log.Ldate|log.Ltime|log.Lshortfile)

	return nil
}

// InitializeSimple initializes logger with stdout only (for CLI usage)
func InitializeSimple() {
	infoLogger = log.New(os.Stdout, "", 0)
	errorLogger = log.New(os.Stderr, "ERROR: ", 0)
	debugLogger = log.New(os.Stdout, "DEBUG: ", 0)
}

// Info logs an info message
func Info(format string, v ...interface{}) {
	if infoLogger == nil {
		InitializeSimple()
	}
	infoLogger.Printf(format, v...)
}

// Error logs an error message
func Error(format string, v ...interface{}) {
	if errorLogger == nil {
		InitializeSimple()
	}
	errorLogger.Printf(format, v...)
}

// Debug logs a debug message
func Debug(format string, v ...interface{}) {
	if debugLogger == nil {
		InitializeSimple()
	}
	debugLogger.Printf(format, v...)
}

// Fatal logs an error message and exits
func Fatal(format string, v ...interface{}) {
	if errorLogger == nil {
		InitializeSimple()
	}
	errorLogger.Fatalf(format, v...)
}
