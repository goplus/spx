//go:build js

/*
 * Copyright (c) 2026 The XGo Authors (xgo.dev). All rights reserved.
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

package spx

import (
	"strings"
	"syscall/js"
	"testing"

	"github.com/goplus/spx/v3/internal/engine"
)

func TestParseCaptureBridgeResultError(t *testing.T) {
	for _, tt := range []struct {
		name   string
		result any
		want   string
	}{
		{name: "undefined", result: js.Undefined()},
		{name: "null"},
		{name: "true", result: true},
		{name: "false", result: false, want: "spx: JS capture bridge returned false"},
		{name: "empty string", result: ""},
		{name: "error string", result: "capture failed", want: "capture failed"},
		{name: "ok", result: map[string]any{"ok": true}},
		{name: "not ok", result: map[string]any{"ok": false}, want: "spx: JS capture bridge returned ok=false"},
		{name: "error precedes ok", result: map[string]any{"ok": true, "error": "compare failed"}, want: "compare failed"},
		{name: "error precedes not ok", result: map[string]any{"ok": false, "error": "compare failed"}, want: "compare failed"},
		{name: "empty error", result: map[string]any{"error": ""}},
		{name: "non-string error", result: map[string]any{"error": 1}},
		{name: "non-boolean ok", result: map[string]any{"ok": 0}},
		{name: "number", result: 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := parseCaptureBridgeResultError(js.ValueOf(tt.result))
			if tt.want == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			} else if err == nil || err.Error() != tt.want {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestDispatchCaptureToJSBridge(t *testing.T) {
	previous := js.Global().Get(jsCaptureBridgeName)
	t.Cleanup(func() { js.Global().Set(jsCaptureBridgeName, previous) })
	for _, value := range []any{js.Undefined(), nil, false, "not a function"} {
		js.Global().Set(jsCaptureBridgeName, value)
		err := dispatchCaptureToJSBridge(engine.CaptureRequest{Name: "missing"})
		if err == nil || !strings.Contains(err.Error(), "is not configured") {
			t.Fatalf("unconfigured bridge error = %v", err)
		}
	}

	var calls []js.Value
	bridge := js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) != 1 {
			t.Errorf("bridge received %d arguments, want 1", len(args))
			return false
		}
		calls = append(calls, args[0])
		return map[string]any{"ok": true}
	})
	defer bridge.Release()
	js.Global().Set(jsCaptureBridgeName, bridge)
	tick := int64(7)
	for _, inputTick := range []*int64{nil, &tick} {
		req := engine.CaptureRequest{Name: "frame.png", InputTick: inputTick, Frame: 23, Sequence: 4}
		if err := dispatchCaptureToJSBridge(req); err != nil {
			t.Fatalf("dispatch: %v", err)
		}
	}
	if len(calls) != 2 {
		t.Fatalf("bridge calls = %d, want 2", len(calls))
	}
	for _, call := range calls {
		if call.Get("name").String() != "frame.png" || call.Get("frame").Int() != 23 || call.Get("sequence").Int() != 4 {
			t.Fatalf("capture metadata was not forwarded: %v", call)
		}
	}
	if !calls[0].Get("inputTick").IsNull() || calls[1].Get("inputTick").Int() != 7 {
		t.Fatal("bridge did not preserve absent and explicit input ticks")
	}

	js.Global().Set(jsCaptureBridgeName, js.Global().Get("Function").New("throw new Error('capture rejected')"))
	if err := dispatchCaptureToJSBridge(engine.CaptureRequest{}); err == nil || !strings.Contains(err.Error(), "JS capture bridge failed") || !strings.Contains(err.Error(), "capture rejected") {
		t.Fatalf("throwing bridge error = %v", err)
	}
	js.Global().Set(jsCaptureBridgeName, js.Global().Get("Function").New("return {error: 'comparison failed'}"))
	if err := dispatchCaptureToJSBridge(engine.CaptureRequest{}); err == nil || err.Error() != "comparison failed" {
		t.Fatalf("bridge result error = %v, want comparison failed", err)
	}
}
