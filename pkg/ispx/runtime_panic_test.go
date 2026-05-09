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

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if got, want := entry["level"], "ERROR"; got != want {
		t.Fatalf("level = %v, want %v", got, want)
	}
	if got, want := entry["msg"], "panic"; got != want {
		t.Fatalf("msg = %v, want %v", got, want)
	}
	if got, want := entry["error"], "boom"; got != want {
		t.Fatalf("error = %v, want %v", got, want)
	}
	if got, want := entry["function"], "main.(*Game).Main"; got != want {
		t.Fatalf("function = %v, want %v", got, want)
	}
	if got, want := entry["file"], "Game.spx"; got != want {
		t.Fatalf("file = %v, want %v", got, want)
	}
	if got, want := entry["line"], float64(12); got != want {
		t.Fatalf("line = %v, want %v", got, want)
	}
	if got, want := entry["column"], float64(3); got != want {
		t.Fatalf("column = %v, want %v", got, want)
	}
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

func TestLogRuntimePanic_RuntimeError(t *testing.T) {
	entry := runAndCaptureRuntimePanicLog(t, `package main
func foo() {
	var p *int
	println(*p)
}
func main() { foo() }
`)

	if got, want := entry["msg"], "panic"; got != want {
		t.Fatalf("msg = %v, want %v", got, want)
	}
	if got, want := entry["error"], "runtime error: invalid memory address or nil pointer dereference"; got != want {
		t.Fatalf("error = %v, want %v", got, want)
	}
	if got, want := entry["function"], "main.foo"; got != want {
		t.Fatalf("function = %v, want %v", got, want)
	}
	if got, want := entry["file"], "main.go"; got != want {
		t.Fatalf("file = %v, want %v", got, want)
	}
	if got, want := entry["line"], float64(4); got != want {
		t.Fatalf("line = %v, want %v", got, want)
	}
}

func TestLogRuntimePanic_ExplicitPanic(t *testing.T) {
	entry := runAndCaptureRuntimePanicLog(t, `package main
func foo() {
	panic("boom")
}
func main() { foo() }
`)

	if got, want := entry["error"], "boom"; got != want {
		t.Fatalf("error = %v, want %v", got, want)
	}
	if got, want := entry["function"], "main.foo"; got != want {
		t.Fatalf("function = %v, want %v", got, want)
	}
	if got, want := entry["file"], "main.go"; got != want {
		t.Fatalf("file = %v, want %v", got, want)
	}
	if got, want := entry["line"], float64(3); got != want {
		t.Fatalf("line = %v, want %v", got, want)
	}
}

func runAndCaptureRuntimePanicLog(t *testing.T, src string) map[string]any {
	t.Helper()

	var buf bytes.Buffer
	prev := runtimePanicLogger
	runtimePanicLogger = newRuntimePanicLogger(&buf)
	t.Cleanup(func() {
		runtimePanicLogger = prev
	})

	ctx := ixgo.NewContext(0)
	ctx.SetPanic(logRuntimePanic)

	if _, err := ctx.RunFile("main.go", src, nil); err == nil {
		t.Fatal("RunFile() error = nil, want panic error")
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if got, want := len(lines), 1; got != want {
		t.Fatalf("log lines = %d, want %d\nlogs:\n%s", got, want, buf.String())
	}

	var entry map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &entry); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	return entry
}
