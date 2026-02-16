package config

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	DefaultDataDir  = "/etc/oqm"
	DefaultDataFile = "data.json"
	DefaultLogFile  = "/var/log/oqm.log"
)

// GetDataFilePath returns the path to the data file
func GetDataFilePath() string {
	dataDir := os.Getenv("OQM_DATA_DIR")
	if dataDir == "" {
		dataDir = DefaultDataDir
	}
	return filepath.Join(dataDir, DefaultDataFile)
}

// GetLogFilePath returns the path to the log file
func GetLogFilePath() string {
	logFile := os.Getenv("OQM_LOG_FILE")
	if logFile == "" {
		logFile = DefaultLogFile
	}
	return logFile
}

// EnsureDataDir ensures the data directory exists
func EnsureDataDir() error {
	dataDir := os.Getenv("OQM_DATA_DIR")
	if dataDir == "" {
		dataDir = DefaultDataDir
	}

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	return nil
}
