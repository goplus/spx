package log

import (
	"fmt"
	"io"
	stdlog "log"
	"os"
	"strings"
	"sync"
)

type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
	LevelNone
)

type Logger struct {
	mu     sync.Mutex
	level  Level
	logger *stdlog.Logger
	prefix string
}

var defaultLogger = New("SPX-CODEGEN", LevelInfo, os.Stdout)

func New(prefix string, level Level, out io.Writer) *Logger {
	return &Logger{
		level:  level,
		logger: stdlog.New(out, "", stdlog.Ldate|stdlog.Ltime|stdlog.Lmicroseconds),
		prefix: prefix,
	}
}

func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

func (l *Logger) SetOutput(w io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.logger.SetOutput(w)
}

func (l *Logger) log(level Level, format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.level > level {
		return
	}

	msg := format
	if len(args) > 0 {
		msg = fmt.Sprintf(format, args...)
	}
	l.logger.Printf("[%s] [%s] %s", level.String(), l.prefix, msg)
}

func (l *Logger) Panicf(format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.level > LevelError {
		panic(l.format(format, args...))
	}
	msg := l.format(format, args...)
	l.logger.Panicf("[%s] [%s] %s", LevelError, l.prefix, msg)
}

func (l *Logger) format(format string, args ...any) string {
	if len(args) > 0 {
		return fmt.Sprintf(format, args...)
	}
	return format
}

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

func Debug(format string, args ...any) {
	defaultLogger.log(LevelDebug, format, args...)
}

func Info(format string, args ...any) {
	defaultLogger.log(LevelInfo, format, args...)
}

func Warn(format string, args ...any) {
	defaultLogger.log(LevelWarn, format, args...)
}

func Error(format string, args ...any) {
	defaultLogger.log(LevelError, format, args...)
}

func Panicf(format string, args ...any) {
	defaultLogger.Panicf(format, args...)
}
