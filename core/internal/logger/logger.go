package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"
)

// Level represents log level
type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
	FATAL
)

var levelNames = map[Level]string{
	DEBUG: "DEBUG",
	INFO:  "INFO",
	WARN:  "WARN",
	ERROR: "ERROR",
	FATAL: "FATAL",
}

// Logger structured logger
type Logger struct {
	level  Level
	output io.Writer
	prefix string
}

var defaultLogger *Logger

func init() {
	defaultLogger = New(INFO, os.Stderr, "TotalConnect")
}

// New creates new logger
func New(level Level, output io.Writer, prefix string) *Logger {
	return &Logger{
		level:  level,
		output: output,
		prefix: prefix,
	}
}

// SetLevel sets global log level
func SetLevel(level Level) {
	defaultLogger.level = level
}

// SetOutput sets global output
func SetOutput(output io.Writer) {
	defaultLogger.output = output
}

// SetLogFile sets output to file
func SetLogFile(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create log dir: %w", err)
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	// Write to both file and stderr
	defaultLogger.output = io.MultiWriter(file, os.Stderr)
	return nil
}

func (l *Logger) log(level Level, format string, args ...interface{}) {
	if level < l.level {
		return
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	levelName := levelNames[level]
	msg := fmt.Sprintf(format, args...)

	fmt.Fprintf(l.output, "[%s] %s %s: %s\n", timestamp, l.prefix, levelName, msg)

	if level == FATAL {
		os.Exit(1)
	}
}

// Debug logs debug message
func Debug(format string, args ...interface{}) {
	defaultLogger.log(DEBUG, format, args...)
}

// Info logs info message
func Info(format string, args ...interface{}) {
	defaultLogger.log(INFO, format, args...)
}

// Warn logs warning message
func Warn(format string, args ...interface{}) {
	defaultLogger.log(WARN, format, args...)
}

// Error logs error message
func Error(format string, args ...interface{}) {
	defaultLogger.log(ERROR, format, args...)
}

// Fatal logs fatal message and exits
func Fatal(format string, args ...interface{}) {
	defaultLogger.log(FATAL, format, args...)
}
