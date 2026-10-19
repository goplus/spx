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
	"bytes"
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"syscall/js"
	"testing"
	"testing/synctest"
	"time"

	"github.com/goplus/ixgo"
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
	replay := hostInputReplayJSON(t, spx.InputReplayFrame{Frame: 0, Time: 0})
	replayBytes := copyBytesToJS([]byte(replay))

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
			if message := assertJSError(t, start.Invoke(test.input)); message != test.wantError {
				t.Fatalf("error message = %q, want %q", message, test.wantError)
			}
			if status := spx.GetInputSessionStatus(); status != idle {
				t.Fatalf("invalid input changed status to %+v, want idle", status)
			}
		})
	}
}

func TestStartFailureCancelsPreparedInputSession(t *testing.T) {
	isolateInterpreter(t)
	reported := make(chan string, 1)
	setJSFunction(t, "gdspx_ext_on_runtime_panic", func(_ js.Value, args []js.Value) any {
		reported <- args[0].String()
		return nil
	})
	setJSFunction(t, "gdspx_ext_request_reset", func(_ js.Value, _ []js.Value) any { return nil })
	global := js.Global()

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

func setJSFunction(t *testing.T, name string, fn func(js.Value, []js.Value) any) {
	t.Helper()
	global := js.Global()
	previous := global.Get(name)
	hadOwn := jsTypeObject.Get("prototype").Get("hasOwnProperty").Call("call", global, name).Bool()
	callback := js.FuncOf(fn)
	global.Set(name, callback)
	t.Cleanup(func() {
		if hadOwn {
			global.Set(name, previous)
		} else {
			global.Delete(name)
		}
		callback.Release()
	})
}

func assertJSError(t *testing.T, value js.Value) string {
	t.Helper()
	if value.Type() != js.TypeObject || !value.InstanceOf(jsTypeError) {
		t.Fatalf("result = %v, want JavaScript Error", value)
	}
	return value.Get("message").String()
}

func TestJSBinaryCopiesOwnTheirStorage(t *testing.T) {
	for _, kind := range []string{"string", "Uint8Array view", "ArrayBuffer"} {
		t.Run(kind, func(t *testing.T) {
			source := copyBytesToJS([]byte{9, 1, 2, 3, 8})
			view := source.Call("subarray", 1, 4)
			value := js.ValueOf("abc")
			want := []byte("abc")
			switch kind {
			case "Uint8Array view":
				value, want = view, []byte{1, 2, 3}
			case "ArrayBuffer":
				value, want = source.Get("buffer"), []byte{9, 1, 2, 3, 8}
			}
			data, err := copyJSReplayData(value)
			if err != nil || !bytes.Equal(data, want) {
				t.Fatalf("copied replay = %v, %v; want %v", data, err, want)
			}
			source.SetIndex(1, 255)
			if !bytes.Equal(data, want) {
				t.Fatal("JavaScript buffer mutation changed the Go copy")
			}
		})
	}
	original := []byte{1, 2, 3}
	copied := copyBytesToJS(original)
	original[0] = 255
	if copied.Index(0).Int() != 1 {
		t.Fatal("Go slice mutation changed the JavaScript copy")
	}
	copied.SetIndex(1, 255)
	if original[1] != 2 {
		t.Fatal("JavaScript mutation changed the original Go slice")
	}
	if empty := copyBytesToJS(nil); !empty.InstanceOf(jsTypeUint8Array) || empty.Length() != 0 {
		t.Fatalf("empty copy = %v, want empty Uint8Array", empty)
	}
}

func TestJSFilesConversionPreservesBinaryData(t *testing.T) {
	source := copyBytesToJS([]byte{9, 0, 255, 8})
	input := js.ValueOf(map[string]any{
		"view.bin":   source.Call("subarray", 1, 3),
		"buffer.bin": source.Get("buffer"),
		"empty.bin":  jsTypeUint8Array.New(0),
	})
	files, err := convertJSFilesToMap(input)
	want := map[string][]byte{"view.bin": {0, 255}, "buffer.bin": {9, 0, 255, 8}, "empty.bin": {}}
	if err != nil || !reflect.DeepEqual(files, want) {
		t.Fatalf("files = %v, %v; want %v", files, err, want)
	}
	source.SetIndex(1, 42)
	if !reflect.DeepEqual(files, want) {
		t.Fatal("JavaScript file mutation changed the Go snapshot")
	}
}

func TestExportedBuildValidatesAndBuildsFiles(t *testing.T) {
	isolateInterpreter(t)
	if err := Init(nil); err != nil {
		t.Fatal(err)
	}
	build := js.Global().Get("ispx_build")
	for _, test := range []struct {
		name string
		args []any
		want string
	}{
		{"missing files", nil, "missing files argument"},
		{"null", []any{nil}, "invalid files argument type"},
		{"array", []any{[]any{}}, "invalid files argument type"},
		{"text file", []any{map[string]any{"main.go": "package main"}}, `failed to convert files: unsupported file value type for "main.go"`},
	} {
		t.Run(test.name, func(t *testing.T) {
			if message := assertJSError(t, build.Invoke(test.args...)); !strings.Contains(message, test.want) {
				t.Fatalf("build error = %q, want %q", message, test.want)
			}
		})
	}
	// Compilation may perform asynchronous filesystem operations in Node, so
	// call the Go handler without suspending JavaScript's event loop in Invoke.
	invalid := js.ValueOf(map[string]any{"main.xgo": copyBytesToJS([]byte("func main( {"))})
	if err, ok := ispxBuild(js.Undefined(), []js.Value{invalid}).(error); !ok || !strings.Contains(err.Error(), "failed to build: failed to build XGo source") {
		t.Fatalf("invalid source error = %v", err)
	}
	files := js.ValueOf(map[string]any{"main.xgo": copyBytesToJS([]byte("func main() {}\n"))})
	if result := ispxBuild(js.Undefined(), []js.Value{files}); result != nil {
		t.Fatalf("successful build = %v, want nil", result)
	}
	mu.Lock()
	built := ixgoInterp != nil
	mu.Unlock()
	if !built {
		t.Fatal("JavaScript build did not publish an interpreter")
	}
}

func TestJSInputSessionDefaultsAndFinishErrors(t *testing.T) {
	for _, args := range [][]js.Value{nil, {js.Undefined()}, {js.Null()}} {
		preparation, err := prepareHostInputSession(args)
		if err != nil || preparation.Cancel() || spx.GetInputSessionStatus().Mode != spx.InputSessionModeIdle {
			t.Fatalf("absent input preparation = %+v, %v", preparation, err)
		}
	}
	input := js.ValueOf(map[string]any{"mode": "record", "fps": nil, "captureKey": nil})
	preparation, err := prepareHostInputSession([]js.Value{input})
	t.Cleanup(func() { preparation.Cancel() })
	if err != nil {
		t.Fatal(err)
	}
	before := spx.GetInputSessionStatus()
	finish := js.Global().Get("ispx_input_recording_finish")
	if message := assertJSError(t, finish.Invoke()); message != spx.ErrInputSessionNotRecording.Error() {
		t.Fatalf("finish error = %q", message)
	}
	if status := spx.GetInputSessionStatus(); status != before {
		t.Fatalf("failed finish changed status: %+v, want %+v", status, before)
	}
}

func TestJSLookupReportsErrorBeforeReset(t *testing.T) {
	var calls []string
	setJSFunction(t, "gdspx_ext_on_runtime_panic", func(_ js.Value, args []js.Value) any {
		calls = append(calls, args[0].String())
		return nil
	})
	setJSFunction(t, "gdspx_ext_request_reset", func(_ js.Value, args []js.Value) any {
		if len(args) != 1 || args[0].Int() != 1 {
			t.Errorf("reset args = %v, want exit code 1", args)
		}
		calls = append(calls, "reset")
		return nil
	})
	if dir, found := defaultIXGoContextLookup("unused", "missing/package"); dir != "" || found {
		t.Fatalf("lookup = %q, %v; want empty, false", dir, found)
	}
	want := []string{`failed to resolve package import "missing/package"`, "reset"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("lookup callbacks = %v, want %v", calls, want)
	}
}

func TestExportedStartReportsInterpreterFailure(t *testing.T) {
	isolateInterpreter(t)
	ctx := ixgo.NewContext(ixgo.SupportMultipleInterp)
	pkg, err := ctx.LoadFile("main.go", `package main; func main() { panic("bridge failure") }`)
	if err != nil {
		t.Fatal(err)
	}
	interp, err := ctx.NewInterp(pkg)
	if err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	ixgoCtx, ixgoInterp = ctx, interp
	mu.Unlock()
	messages := make(chan string, 1)
	reset := make(chan struct{}, 1)
	setJSFunction(t, "gdspx_ext_on_runtime_panic", func(_ js.Value, args []js.Value) any {
		messages <- args[0].String()
		return nil
	})
	setJSFunction(t, "gdspx_ext_request_reset", func(_ js.Value, _ []js.Value) any { reset <- struct{}{}; return nil })
	if result := js.Global().Get("ispx_start").Invoke(); !result.IsNull() {
		t.Fatalf("ispx_start = %v, want null", result)
	}
	select {
	case <-reset:
	case <-time.After(3 * time.Second):
		t.Fatal("interpreter failure did not request a runtime reset")
	}
	if message := <-messages; !strings.Contains(message, "interpreter exited with code") || !strings.Contains(message, "bridge failure") {
		t.Fatalf("interpreter failure = %q", message)
	}
	mu.Lock()
	running := runDone != nil
	mu.Unlock()
	if running {
		t.Fatal("failed interpreter still reports a running instance")
	}
}

func TestJSStopWithNoRunningInterpreter(t *testing.T) {
	isolateInterpreter(t)
	synctest.Test(t, func(t *testing.T) {
		if result := ispxStop(js.Undefined(), nil); result != nil {
			t.Fatalf("ispx_stop = %v, want nil", result)
		}
		synctest.Wait()
	})
}
