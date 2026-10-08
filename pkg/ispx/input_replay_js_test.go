//go:build js && wasm

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
	"encoding/json"
	"math"
	"reflect"
	"syscall/js"
	"testing"
	"time"

	spx "github.com/goplus/spx/v3"
)

func TestInputSessionStatusToJS(t *testing.T) {
	for _, test := range []struct {
		name   string
		status spx.InputSessionStatus
		want   string
	}{
		{
			name:   "idle",
			status: spx.InputSessionStatus{Mode: spx.InputSessionModeIdle},
			want:   `{"mode":"idle","phase":"","completed":false,"exhausted":false,"currentTick":null,"nextFrame":0,"frameCount":0}`,
		},
		{
			name: "running without current tick",
			status: spx.InputSessionStatus{
				Mode: spx.InputSessionModeReplaying, Phase: spx.InputSessionPhaseRunning,
				CurrentTick: 42, FrameCount: 3,
			},
			want: `{"mode":"replaying","phase":"running","completed":false,"exhausted":false,"currentTick":null,"nextFrame":0,"frameCount":3}`,
		},
		{
			name: "running at tick zero",
			status: spx.InputSessionStatus{
				Mode: spx.InputSessionModeRecording, Phase: spx.InputSessionPhaseRunning,
				CurrentTick: 0, HasCurrentTick: true, NextFrame: 1, FrameCount: 1,
			},
			want: `{"mode":"recording","phase":"running","completed":false,"exhausted":false,"currentTick":0,"nextFrame":1,"frameCount":1}`,
		},
		{
			name: "completed recording without exhaustion",
			status: spx.InputSessionStatus{
				Mode: spx.InputSessionModeRecording, Phase: spx.InputSessionPhaseCompleted,
				Completed: true, CurrentTick: 2, HasCurrentTick: true, NextFrame: 3, FrameCount: 3,
			},
			want: `{"mode":"recording","phase":"completed","completed":true,"exhausted":false,"currentTick":2,"nextFrame":3,"frameCount":3}`,
		},
		{
			name: "exhausted replay before completion",
			status: spx.InputSessionStatus{
				Mode: spx.InputSessionModeReplaying, Phase: spx.InputSessionPhaseRunning,
				Exhausted: true, CurrentTick: 2, HasCurrentTick: true, NextFrame: 3, FrameCount: 3,
			},
			want: `{"mode":"replaying","phase":"running","completed":false,"exhausted":true,"currentTick":2,"nextFrame":3,"frameCount":3}`,
		},
		{
			name: "completed and exhausted",
			status: spx.InputSessionStatus{
				Mode: spx.InputSessionModeReplaying, Phase: spx.InputSessionPhaseCompleted,
				Completed: true, Exhausted: true, CurrentTick: 2, HasCurrentTick: true, NextFrame: 3, FrameCount: 3,
			},
			want: `{"mode":"replaying","phase":"completed","completed":true,"exhausted":true,"currentTick":2,"nextFrame":3,"frameCount":3}`,
		},
		{
			name: "aborted with error",
			status: spx.InputSessionStatus{
				Mode: spx.InputSessionModeReplaying, Phase: spx.InputSessionPhaseAborted,
				CurrentTick: 1, HasCurrentTick: true, NextFrame: 2, FrameCount: 3, Error: "input session reset",
			},
			want: `{"mode":"replaying","phase":"aborted","completed":false,"exhausted":false,"currentTick":1,"nextFrame":2,"frameCount":3,"error":"input session reset"}`,
		},
		{
			name: "64-bit counters",
			status: spx.InputSessionStatus{
				Mode: spx.InputSessionModeRecording, Phase: spx.InputSessionPhaseRunning,
				CurrentTick: 1 << 40, HasCurrentTick: true, NextFrame: 1<<40 + 1, FrameCount: 4,
			},
			want: `{"mode":"recording","phase":"running","completed":false,"exhausted":false,"currentTick":1099511627776,"nextFrame":1099511627777,"frameCount":4}`,
		},
		{
			name: "counters retain JavaScript number rounding",
			status: spx.InputSessionStatus{
				Mode: spx.InputSessionModeRecording, Phase: spx.InputSessionPhaseRunning,
				CurrentTick: math.MaxInt64 - 1, HasCurrentTick: true, NextFrame: math.MaxInt64,
			},
			want: `{"mode":"recording","phase":"running","completed":false,"exhausted":false,"currentTick":9223372036854776000,"nextFrame":9223372036854776000,"frameCount":0}`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			assertInputSessionStatusJSON(t, inputSessionStatusToJS(test.status), test.want)
		})
	}
}

func TestExportedInputSessionStatusReadsRuntime(t *testing.T) {
	status := js.Global().Get("ispx_input_session_status")
	if status.Type() != js.TypeFunction {
		t.Fatalf("ispx_input_session_status type = %v, want function", status.Type())
	}
	const idle = `{"mode":"idle","phase":"","completed":false,"exhausted":false,"currentTick":null,"nextFrame":0,"frameCount":0}`
	const recording = `{"mode":"recording","phase":"prepared","completed":false,"exhausted":false,"currentTick":null,"nextFrame":0,"frameCount":0}`
	const replaying = `{"mode":"replaying","phase":"prepared","completed":false,"exhausted":false,"currentTick":null,"nextFrame":0,"frameCount":1}`
	replay, err := spx.EncodeInputReplay(spx.InputReplay{
		Format: spx.InputReplayFormat, Version: spx.InputReplayVersion, FixedTimestep: 1.0 / 30,
		Frames: []spx.InputReplayFrame{{Frame: 0, Time: 0}},
	})
	if err != nil {
		t.Fatal(err)
	}
	replayBytes := jsTypeUint8Array.New(len(replay))
	js.CopyBytesToJS(replayBytes, []byte(replay))

	for _, test := range []struct {
		name  string
		input map[string]any
		want  string
	}{
		{"recording with default FPS", map[string]any{"mode": "record"}, recording},
		{"recording with explicit FPS and capture key", map[string]any{"mode": "record", "fps": 60, "captureKey": "p"}, recording},
		{"replay JSON string", map[string]any{"mode": "replay", "data": replay}, replaying},
		{"replay Uint8Array", map[string]any{"mode": "replay", "data": replayBytes}, replaying},
		{"replay ArrayBuffer", map[string]any{"mode": "replay", "data": replayBytes.Get("buffer")}, replaying},
	} {
		t.Run(test.name, func(t *testing.T) {
			assertInputSessionStatusJSON(t, status.Invoke(), idle)
			preparation, err := prepareHostInputSession([]js.Value{js.ValueOf(test.input)})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { preparation.Cancel() })
			assertInputSessionStatusJSON(t, status.Invoke(), test.want)
			if !preparation.Cancel() {
				t.Fatal("input session preparation was not cancelled")
			}
			assertInputSessionStatusJSON(t, status.Invoke(), idle)
		})
	}
}

func TestExportedStartRejectsInvalidInputSession(t *testing.T) {
	start := js.Global().Get("ispx_start")
	if start.Type() != js.TypeFunction {
		t.Fatalf("ispx_start type = %v, want function", start.Type())
	}
	for _, test := range []struct {
		name      string
		input     any
		wantError string
	}{
		{"non-object", []any{}, "input session must be an object"},
		{"non-string mode", map[string]any{"mode": 1}, "input session mode must be a string"},
		{"unknown mode", map[string]any{"mode": "unknown"}, `unsupported input session mode "unknown"`},
		{"non-number FPS", map[string]any{"mode": "record", "fps": "60"}, "input recording FPS must be a number"},
		{"missing replay data", map[string]any{"mode": "replay"}, "missing input replay data"},
		{"unsupported replay data", map[string]any{"mode": "replay", "data": 1}, "expected string, Uint8Array, or ArrayBuffer"},
		{"non-string capture key", map[string]any{"mode": "record", "captureKey": 1}, "input session captureKey must be a key name string"},
		{"non-specific capture key", map[string]any{"mode": "record", "captureKey": "Any"}, `unsupported input session captureKey "Any"`},
		{"unknown capture key", map[string]any{"mode": "record", "captureKey": "unknown"}, `unsupported input session captureKey "unknown"`},
	} {
		t.Run(test.name, func(t *testing.T) {
			idle := spx.InputSessionStatus{Mode: spx.InputSessionModeIdle}
			if status := spx.GetInputSessionStatus(); status != idle {
				t.Fatalf("initial input status = %+v, want idle", status)
			}
			result := start.Invoke(test.input)
			if result.Type() != js.TypeObject || !result.InstanceOf(jsTypeError) {
				t.Fatalf("ispx_start = %v, want JavaScript Error", result)
			}
			if message := result.Get("message").String(); message != test.wantError {
				t.Fatalf("error message = %q, want %q", message, test.wantError)
			}
			if status := spx.GetInputSessionStatus(); status != idle {
				t.Fatalf("invalid input changed status to %+v, want idle", status)
			}
		})
	}
}

func TestStartFailureCancelsPreparedInputSession(t *testing.T) {
	mu.Lock()
	previousInterp := ixgoInterp
	ixgoInterp = nil
	mu.Unlock()
	t.Cleanup(func() {
		mu.Lock()
		ixgoInterp = previousInterp
		mu.Unlock()
	})

	reported := make(chan string, 1)
	onPanic := js.FuncOf(func(_ js.Value, args []js.Value) any {
		reported <- args[0].String()
		return nil
	})
	onReset := js.FuncOf(func(_ js.Value, _ []js.Value) any { return nil })
	global := js.Global()
	previousPanic := global.Get("gdspx_ext_on_runtime_panic")
	previousReset := global.Get("gdspx_ext_request_reset")
	global.Set("gdspx_ext_on_runtime_panic", onPanic)
	global.Set("gdspx_ext_request_reset", onReset)
	t.Cleanup(func() {
		global.Set("gdspx_ext_on_runtime_panic", previousPanic)
		global.Set("gdspx_ext_request_reset", previousReset)
		onPanic.Release()
		onReset.Release()
	})

	if result := global.Get("ispx_start").Invoke(map[string]any{"mode": "record"}); !result.IsNull() {
		t.Fatalf("ispx_start = %v, want null", result)
	}
	timeout := time.NewTimer(5 * time.Second)
	defer timeout.Stop()
	var message string
	select {
	case message = <-reported:
	case <-timeout.C:
		t.Fatal("interpreter startup failure was not reported")
	}
	// The error callback runs before the deferred preparation cleanup.
	for spx.GetInputSessionStatus().Mode != spx.InputSessionModeIdle {
		select {
		case <-timeout.C:
			t.Fatal("failed startup left an input session prepared")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	if want := "interpreter exited with panic: ispx: not built"; message != want {
		t.Fatalf("runtime error = %q, want %q", message, want)
	}
}

func assertInputSessionStatusJSON(t *testing.T, value js.Value, want string) {
	t.Helper()
	if value.Type() != js.TypeObject || !isPlainJSObject(value) {
		t.Fatalf("status = %v, want a plain JavaScript object", value)
	}
	encoded := js.Global().Get("JSON").Call("stringify", value).String()
	// Compare raw JSON values to preserve exact JavaScript number rounding.
	var gotJSON, wantJSON map[string]json.RawMessage
	if err := json.Unmarshal([]byte(encoded), &gotJSON); err != nil {
		t.Fatalf("decode status JSON: %v", err)
	}
	if err := json.Unmarshal([]byte(want), &wantJSON); err != nil {
		t.Fatalf("decode expected JSON: %v", err)
	}
	if !reflect.DeepEqual(gotJSON, wantJSON) {
		t.Fatalf("status JSON = %s, want %s", encoded, want)
	}
	// JSON.stringify omits undefined fields, so also check the actual object
	// shape to ensure an empty error and the internal tick flag are absent.
	keys := jsTypeObject.Call("keys", value)
	if keys.Length() != len(wantJSON) {
		t.Fatalf("status has %d properties, want %d: %s", keys.Length(), len(wantJSON), encoded)
	}
}
