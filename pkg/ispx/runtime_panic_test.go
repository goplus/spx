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

package ispx

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/goplus/ixgo"
)

func TestLogRuntimePanicFields(t *testing.T) {
	var buf bytes.Buffer
	logger := newRuntimePanicLogger(&buf)

	logRuntimePanicFields(logger, runtimePanicFields{
		Error:    errors.New("boom"),
		Function: "main.(*Game).Main",
		File:     "Game.spx",
		Line:     12,
		Column:   3,
	})

	checkRuntimePanicLog(t, &buf, map[string]any{
		"error": "boom", "function": "main.(*Game).Main", "file": "Game.spx",
		"line": float64(12), "column": float64(3),
	})
}

func TestShouldLogRuntimePanic(t *testing.T) {
	tests := []struct {
		name string
		info *ixgo.PanicInfo
		want bool
	}{
		{
			name: "NilInfo",
			info: nil,
			want: false,
		},
		{
			name: "PanicError",
			info: &ixgo.PanicInfo{Error: ixgo.PanicError{}},
			want: true,
		},
		{
			name: "FatalWrappingRuntimeError",
			info: &ixgo.PanicInfo{Error: ixgo.FatalError{Value: ixgo.RuntimeError("boom")}},
			want: false,
		},
		{
			name: "FatalWrappingPlainError",
			info: &ixgo.PanicInfo{Error: ixgo.FatalError{Value: ixgo.PlainError("boom")}},
			want: false,
		},
		{
			name: "FatalWrappingPanicError",
			info: &ixgo.PanicInfo{Error: ixgo.FatalError{Value: ixgo.PanicError{}}},
			want: false,
		},
		{
			name: "FatalWrappingExternalValue",
			info: &ixgo.PanicInfo{Error: ixgo.FatalError{Value: "boom"}},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldLogRuntimePanic(tt.info); got != tt.want {
				t.Fatalf("shouldLogRuntimePanic() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLogRuntimePanic(t *testing.T) {
	for _, test := range []struct {
		name, body, wantError string
		line                  float64
	}{
		{"runtime error", "var p *int\n\tprintln(*p)", "runtime error: invalid memory address or nil pointer dereference", 4},
		{"explicit panic", `panic("boom")`, "boom", 3},
	} {
		t.Run(test.name, func(t *testing.T) {
			buf := captureRuntimePanicLog(t)
			ctx := ixgo.NewContext(0)
			ctx.SetPanic(logRuntimePanic)
			source := "package main\nfunc foo() {\n\t" + test.body + "\n}\nfunc main() { foo() }\n"
			if _, err := ctx.RunFile("main.go", source, nil); err == nil {
				t.Fatal("RunFile() error = nil, want panic error")
			}
			checkRuntimePanicLog(t, buf, map[string]any{
				"error": test.wantError, "function": "main.foo", "file": "main.go", "line": test.line,
			})
		})
	}
}

func TestLogRuntimePanicFromHostCallback(t *testing.T) {
	buf := captureRuntimePanicLog(t)
	ctx := ixgo.NewContext(0)
	ctx.SetPanic(logRuntimePanic)
	i, err := ctx.LoadInterp("host.go", "package main\nfunc Run(callback func()) { callback() }\n")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(i.UnsafeRelease)
	_, err = i.RunFunc("Run", func() { panic("host failure") })
	var fatal ixgo.FatalError
	if !errors.As(err, &fatal) || fatal.Value != "host failure" {
		t.Fatalf("host callback error = %v, want fatal carrying host failure", err)
	}
	checkRuntimePanicLog(t, buf, map[string]any{
		"error": "host failure", "function": "main.Run", "file": "host.go", "line": float64(2),
	})
}

func TestLogRuntimePanicSkipsAbsentOrAlreadyReportedErrors(t *testing.T) {
	buf := captureRuntimePanicLog(t)
	for _, info := range []*ixgo.PanicInfo{
		nil,
		{Error: ixgo.FatalError{Value: ixgo.RuntimeError("runtime error")}},
		{Error: ixgo.FatalError{Value: ixgo.PlainError("plain error")}},
		{Error: ixgo.FatalError{Value: ixgo.PanicError{}}},
	} {
		logRuntimePanic(info)
	}
	if buf.Len() != 0 {
		t.Fatalf("absent or previously reported errors were logged: %s", buf.String())
	}
	if got := runtimePanicFunction(nil); got != "" {
		t.Fatalf("missing panic function = %q, want empty", got)
	}
}

func captureRuntimePanicLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := runtimePanicLogger
	runtimePanicLogger = newRuntimePanicLogger(&buf)
	t.Cleanup(func() { runtimePanicLogger = prev })
	return &buf
}

func checkRuntimePanicLog(t *testing.T, buf *bytes.Buffer, want map[string]any) {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("log lines = %d, want 1\nlogs:\n%s", len(lines), buf.String())
	}
	var entry map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &entry); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	want["level"], want["msg"] = "ERROR", "panic"
	for key, value := range want {
		if entry[key] != value {
			t.Errorf("log %s = %v, want %v", key, entry[key], value)
		}
	}
}
