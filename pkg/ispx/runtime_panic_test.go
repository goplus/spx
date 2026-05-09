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
	"testing"
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
