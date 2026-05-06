/*
 * Copyright (c) 2021 The XGo Authors (xgo.dev). All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package log

import (
	"fmt"
	"io"
	stdlog "log"
	"os"
	"strings"
	"sync"
	"sync/atomic"
)

type Level int32

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
	LevelNone
)

type Logger struct {
	mu           sync.Mutex
	level        Level
	stdoutLogger *stdlog.Logger
	stderrLogger *stdlog.Logger
	prefix       string
}

var defaultLogger = NewWithOutputs("SPX-CODEGEN", LevelInfo, os.Stdout, os.Stderr)

func New(prefix string, level Level, out io.Writer) *Logger {
	return NewWithOutputs(prefix, level, out, out)
}

func NewWithOutputs(prefix string, level Level, stdout, stderr io.Writer) *Logger {
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}
	return &Logger{
		level:        level,
		stdoutLogger: stdlog.New(stdout, "", stdlog.Ldate|stdlog.Ltime|stdlog.Lmicroseconds),
		stderrLogger: stdlog.New(stderr, "", stdlog.Ldate|stdlog.Ltime|stdlog.Lmicroseconds),
		prefix:       prefix,
	}
}

func (l *Logger) SetLevel(level Level) {
	atomic.StoreInt32((*int32)(&l.level), int32(level))
}

func (l *Logger) SetOutput(w io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if w == nil {
		w = io.Discard
	}
	l.stdoutLogger.SetOutput(w)
	l.stderrLogger.SetOutput(w)
}

func (l *Logger) targetLogger(level Level) *stdlog.Logger {
	if level >= LevelError {
		return l.stderrLogger
	}
	return l.stdoutLogger
}

func (l *Logger) log(level Level, format string, args ...any) {
	if Level(atomic.LoadInt32((*int32)(&l.level))) > level {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	if Level(atomic.LoadInt32((*int32)(&l.level))) > level {
		return
	}

	msg := format
	if len(args) > 0 {
		msg = fmt.Sprintf(format, args...)
	}
	l.targetLogger(level).Printf("[%s] [%s] %s", level.String(), l.prefix, msg)
}

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
	l.stderrLogger.Panicf("[%s] [%s] %s", LevelError.String(), l.prefix, msg)
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
