package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
)

type ServiceName string

const (
	ServiceApp    ServiceName = "app"
	ServiceWorker ServiceName = "worker"
)

type InitResult struct {
	Logger   *log.Logger
	LogFile  *os.File
	LogPath  string
	MultiOut io.Writer
}

func Init(service ServiceName) (*InitResult, error) {
	if service != ServiceApp && service != ServiceWorker {
		return nil, fmt.Errorf("invalid service name: %s", service)
	}

	logDir := filepath.Join("logs", string(service))
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, fmt.Errorf("create log dir failed: %w", err)
	}

	logPath := filepath.Join(logDir, fmt.Sprintf("%s.log", service))
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open log file failed: %w", err)
	}

	multiWriter := io.MultiWriter(os.Stdout, logFile)

	logger := log.New(multiWriter, fmt.Sprintf("[%s] ", service), log.LstdFlags|log.Lshortfile)

	// Redirect the default logger so existing log.Printf(...) calls also go here.
	log.SetOutput(multiWriter)
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.SetPrefix(fmt.Sprintf("[%s] ", service))

	return &InitResult{
		Logger:   logger,
		LogFile:  logFile,
		LogPath:  logPath,
		MultiOut: multiWriter,
	}, nil
}
