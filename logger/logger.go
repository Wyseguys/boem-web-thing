package logger

import (
	"io"
	"log"
	"os"
)

// Logger wraps standard log.Logger for info, error, and debug output.
type Logger struct {
	info    *log.Logger
	err     *log.Logger
	debug   *log.Logger
	verbose bool
}

// New creates a new Logger instance that writes to both a file and stdout.
func New(logPath string, verbose bool) (*Logger, error) {
	// Open or create the log file
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}

	// MultiWriter to write to both stdout and file
	multiInfo := io.MultiWriter(os.Stdout, f)
	multiErr := io.MultiWriter(os.Stderr, f)

	return &Logger{
		info:    log.New(multiInfo, "[INFO] ", log.Ldate|log.Ltime),
		err:     log.New(multiErr, "[ERROR] ", log.Ldate|log.Ltime|log.Lshortfile),
		debug:   log.New(multiInfo, "[DEBUG] ", log.Ldate|log.Ltime|log.Lshortfile),
		verbose: verbose,
	}, nil
}

// Info logs informational messages to both console and file.
func (l *Logger) Info(v ...interface{}) {
	l.info.Println(v...)
}

// Error logs error messages to both console and file.
func (l *Logger) Error(v ...interface{}) {
	l.err.Println(v...)
}

// Debug logs debug messages only if verbose is enabled.
func (l *Logger) Debug(v ...interface{}) {
	if l.verbose {
		l.debug.Println(v...)
	}
}
