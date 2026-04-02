// Package log provides a unified logging system for SPX engine.
package log

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"sync/atomic"
)

// Level represents the logging level
type Level int32

const (
	// LevelDebug logs everything including debug information
	LevelDebug Level = iota
	// LevelInfo logs informational messages and above
	LevelInfo
	// LevelWarn logs warnings and errors
	LevelWarn
	// LevelError logs only errors
	LevelError
	// LevelNone disables all logging
	LevelNone
)

// String returns the string representation of the log level
func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	case LevelNone:
		return "NONE"
	default:
		return "UNKNOWN"
	}
}

// Logger represents a logger instance
type Logger struct {
	mu     sync.Mutex
	level  Level
	logger *log.Logger
	prefix string
}

var (
	defaultLogger *Logger
)

// init initializes the default logger
func init() {
	defaultLogger = New("SPX", LevelInfo, os.Stdout)
}

// New creates a new logger with the specified prefix, level, and output
func New(prefix string, level Level, out io.Writer) *Logger {
	return &Logger{
		level:  level,
		logger: log.New(out, "", log.Ldate|log.Ltime|log.Lmicroseconds),
		prefix: prefix,
	}
}

// Default returns the default logger
func Default() *Logger {
	return defaultLogger
}

// SetLevel sets the logging level for the default logger
func SetLevel(level Level) {
	defaultLogger.SetLevel(level)
}

// SetLevel sets the logging level for this logger
func (l *Logger) SetLevel(level Level) {
	atomic.StoreInt32((*int32)(&l.level), int32(level))
}

// SetOutput sets the output destination for the default logger
func SetOutput(w io.Writer) {
	defaultLogger.SetOutput(w)
}

// SetOutput sets the output destination for this logger
func (l *Logger) SetOutput(w io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.logger.SetOutput(w)
}

// log is the internal logging function
func (l *Logger) log(level Level, format string, args ...any) {
	if Level(atomic.LoadInt32((*int32)(&l.level))) > level {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if Level(atomic.LoadInt32((*int32)(&l.level))) > level {
		return
	}

	var msg string
	if len(args) > 0 {
		msg = fmt.Sprintf(format, args...)
	} else {
		msg = format
	}
	l.logger.Printf("[%s] [%s] %s", level, l.prefix, msg)
}

func (l *Logger) format(format string, args ...any) string {
	if len(args) > 0 {
		return fmt.Sprintf(format, args...)
	}
	return format
}

// Debug logs a debug message
func (l *Logger) Debug(format string, args ...any) {
	l.log(LevelDebug, format, args...)
}

// Info logs an informational message
func (l *Logger) Info(format string, args ...any) {
	l.log(LevelInfo, format, args...)
}

// Warn logs a warning message
func (l *Logger) Warn(format string, args ...any) {
	l.log(LevelWarn, format, args...)
}

// Error logs an error message
func (l *Logger) Error(format string, args ...any) {
	l.log(LevelError, format, args...)
}

// Fatalf exits with the formatted message, also logging it at error level if the current log level permits.
func (l *Logger) Fatalf(format string, args ...any) {
	msg := l.format(format, args...)
	if Level(atomic.LoadInt32((*int32)(&l.level))) > LevelError {
		os.Exit(1)
	}

	l.mu.Lock()
	if Level(atomic.LoadInt32((*int32)(&l.level))) > LevelError {
		l.mu.Unlock()
		os.Exit(1)
	}
	l.logger.Fatalf("[%s] [%s] %s", LevelError.String(), l.prefix, msg)
}

// Panicf panics with the formatted message, also logging it at error level if the current log level permits.
func (l *Logger) Panicf(format string, args ...any) {
	msg := l.format(format, args...)
	if Level(atomic.LoadInt32((*int32)(&l.level))) > LevelError {
		panic(msg)
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if Level(atomic.LoadInt32((*int32)(&l.level))) > LevelError {
		panic(msg)
	}
	l.logger.Panicf("[%s] [%s] %s", LevelError.String(), l.prefix, msg)
}

// Package-level convenience functions using the default logger

// Debug logs a debug message using the default logger
func Debug(format string, args ...any) {
	defaultLogger.Debug(format, args...)
}

// Info logs an informational message using the default logger
func Info(format string, args ...any) {
	defaultLogger.Info(format, args...)
}

// Warn logs a warning message using the default logger
func Warn(format string, args ...any) {
	defaultLogger.Warn(format, args...)
}

// Error logs an error message using the default logger
func Error(format string, args ...any) {
	defaultLogger.Error(format, args...)
}

// Fatalf exits with the formatted message, also logging it at error level if the current log level permits.
func Fatalf(format string, args ...any) {
	defaultLogger.Fatalf(format, args...)
}

// Panicf panics with the formatted message, also logging it at error level if the current log level permits.
func Panicf(format string, args ...any) {
	defaultLogger.Panicf(format, args...)
}

// ParseLevel converts a string to a log level
func ParseLevel(s string) Level {
	switch strings.ToLower(s) {
	case "debug":
		return LevelDebug
	case "info":
		return LevelInfo
	case "warn", "warning":
		return LevelWarn
	case "error":
		return LevelError
	case "none":
		return LevelNone
	default:
		return LevelInfo
	}
}
