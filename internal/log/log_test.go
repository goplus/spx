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
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"testing"
)

func TestLevels(t *testing.T) {
	for _, tc := range []struct {
		text, name string
		level      Level
	}{
		{"DEBUG", "DEBUG", LevelDebug}, {"info", "INFO", LevelInfo},
		{"warn", "WARN", LevelWarn}, {"warning", "WARN", LevelWarn},
		{"error", "ERROR", LevelError}, {"none", "NONE", LevelNone},
		{"invalid", "INFO", LevelInfo},
	} {
		if got := ParseLevel(tc.text); got != tc.level || got.String() != tc.name {
			t.Errorf("ParseLevel(%q) = %v, want %v", tc.text, got, tc.level)
		}
	}
	if got := Level(-1).String(); got != "UNKNOWN" {
		t.Fatalf("unknown level = %q", got)
	}
}

func TestLoggerRoutingAndThresholds(t *testing.T) {
	for minimum := LevelDebug; minimum <= LevelNone; minimum++ {
		t.Run(minimum.String(), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			logger := NewWithOutputs("test", minimum, &stdout, &stderr)
			for level, emit := range []func(string, ...any){logger.Debug, logger.Info, logger.Warn, logger.Error} {
				emit("message %d", level)
			}
			for level := LevelDebug; level <= LevelError; level++ {
				output := stdout.String()
				if level == LevelError {
					output = stderr.String()
				}
				if got := strings.Contains(output, fmt.Sprintf("[%s] [test] message %d", level, level)); got != (level >= minimum) {
					t.Errorf("level %s emitted=%v at threshold %s", level, got, minimum)
				}
			}
			if strings.Contains(stdout.String(), "[ERROR]") || strings.Contains(stderr.String(), "[INFO]") {
				t.Fatal("message routed to wrong stream")
			}
		})
	}
}

func TestDefaultLoggerOutputs(t *testing.T) {
	previous := defaultLogger
	t.Cleanup(func() { defaultLogger = previous })
	defaultLogger = New("test", LevelNone, nil)
	if Default() != defaultLogger {
		t.Fatal("default logger identity changed")
	}
	SetLevel(LevelDebug)
	var stdout, stderr bytes.Buffer
	SetStdoutOutput(&stdout)
	SetStderrOutput(&stderr)
	Debug("debug")
	Info("info")
	Warn("warn")
	Error("error")
	if strings.Count(stdout.String(), "[test]") != 3 || strings.Count(stderr.String(), "[test]") != 1 {
		t.Fatalf("stdout/stderr = %q / %q", stdout.String(), stderr.String())
	}
	var combined bytes.Buffer
	SetOutput(&combined)
	Info("literal")
	Error("formatted %s", "error")
	if !strings.Contains(combined.String(), "literal") || !strings.Contains(combined.String(), "formatted error") {
		t.Fatalf("formatting = %q", combined.String())
	}
	SetStdoutOutput(nil)
	SetStderrOutput(nil)
	Info("discard")
	Error("discard")
	SetOutput(nil)
	if strings.Contains(combined.String(), "discard") {
		t.Fatal("nil outputs retained previous writer")
	}
}

func TestLoggerPanic(t *testing.T) {
	for _, level := range []Level{LevelDebug, LevelNone} {
		t.Run(level.String(), func(t *testing.T) {
			var output bytes.Buffer
			logger := New("panic", level, &output)
			func() {
				defer func() {
					if got := recover(); got == nil || !strings.Contains(fmt.Sprint(got), "failure 42") {
						t.Errorf("panic = %v", got)
					}
				}()
				logger.Panicf("failure %d", 42)
			}()
			if got := strings.Contains(output.String(), "failure 42"); got != (level != LevelNone) {
				t.Errorf("panic logged=%v at level %s", got, level)
			}
			logger.SetLevel(LevelInfo)
			logger.Info("after panic")
			if !strings.Contains(output.String(), "after panic") {
				t.Fatal("panic did not release logger lock")
			}
		})
	}
}

func TestLoggerConcurrentReconfiguration(t *testing.T) {
	logger := NewWithOutputs("concurrent", LevelInfo, nil, nil)
	var workers sync.WaitGroup
	for range 4 {
		workers.Go(func() {
			for range 100 {
				logger.SetLevel(LevelDebug)
				logger.Info("message %d", 1)
				logger.SetOutput(nil)
				logger.SetStdoutOutput(nil)
				logger.SetStderrOutput(nil)
				logger.SetLevel(LevelNone)
			}
		})
	}
	workers.Wait()
}

func TestFatalLogging(t *testing.T) {
	if runtime.GOOS == "js" || runtime.GOOS == "wasip1" {
		t.Skip("fatal logging requires a subprocess")
	}
	if mode := os.Getenv("SPX_LOG_FATAL_TEST"); mode != "" {
		defaultLogger = New("fatal", ParseLevel(mode), os.Stderr)
		Fatalf("fatal %d", 42)
		return
	}
	for _, level := range []string{"info", "none"} {
		t.Run(level, func(t *testing.T) {
			command := exec.Command(os.Args[0], "-test.run=^TestFatalLogging$")
			command.Env = append(os.Environ(), "SPX_LOG_FATAL_TEST="+level)
			output, err := command.CombinedOutput()
			if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 1 {
				t.Fatalf("fatal exit = %v, output = %q", err, output)
			}
			if got := strings.Contains(string(output), "[ERROR] [fatal] fatal 42"); got != (level == "info") {
				t.Fatalf("fatal output = %q at level %s", output, level)
			}
		})
	}
}
