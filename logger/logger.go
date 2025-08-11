package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// Logger wraps standard loggers with different levels
type Logger struct {
	DebugLog *log.Logger
	InfoLog  *log.Logger
	ErrorLog *log.Logger
	file     *os.File
}

// New creates a new Logger writing to a file and stdout
func New(logDir, level string) (*Logger, error) {
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log dir: %w", err)
	}

	logPath := filepath.Join(logDir, "crawler.log")
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	multiOut := io.MultiWriter(os.Stdout, file)

	l := &Logger{
		DebugLog: log.New(multiOut, "[DEBUG] ", log.Ldate|log.Ltime|log.Lshortfile),
		InfoLog:  log.New(multiOut, "[INFO] ", log.Ldate|log.Ltime),
		ErrorLog: log.New(multiOut, "[ERROR] ", log.Ldate|log.Ltime|log.Lshortfile),
		file:     file,
	}

	// Set log level (naive filtering for now)
	level = strings.ToLower(level)
	if level != "debug" {
		// If not debug, make DebugLog a no-op
		l.DebugLog.SetOutput(io.Discard)
	}

	return l, nil
}

// Close closes the log file
func (l *Logger) Close() {
	if l.file != nil {
		_ = l.file.Close()
	}
}

func (l *Logger) Debug(v ...interface{}) {
	l.DebugLog.Println(v...)
}

func (l *Logger) Info(v ...interface{}) {
	l.InfoLog.Println(v...)
}

func (l *Logger) Error(v ...interface{}) {
	l.ErrorLog.Println(v...)
}
