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
			name: "prepared recording",
			status: spx.InputSessionStatus{
				Mode: spx.InputSessionModeRecording, Phase: spx.InputSessionPhasePrepared,
			},
			want: `{"mode":"recording","phase":"prepared","completed":false,"exhausted":false,"currentTick":null,"nextFrame":0,"frameCount":0}`,
		},
		{
			name: "prepared replay",
			status: spx.InputSessionStatus{
				Mode: spx.InputSessionModeReplaying, Phase: spx.InputSessionPhasePrepared, FrameCount: 3,
			},
			want: `{"mode":"replaying","phase":"prepared","completed":false,"exhausted":false,"currentTick":null,"nextFrame":0,"frameCount":3}`,
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
			name: "finishing",
			status: spx.InputSessionStatus{
				Mode: spx.InputSessionModeRecording, Phase: spx.InputSessionPhaseFinishing,
				CurrentTick: 2, HasCurrentTick: true, NextFrame: 3, FrameCount: 3,
			},
			want: `{"mode":"recording","phase":"finishing","completed":false,"exhausted":false,"currentTick":2,"nextFrame":3,"frameCount":3}`,
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
			name: "aborted without error",
			status: spx.InputSessionStatus{
				Mode: spx.InputSessionModeRecording, Phase: spx.InputSessionPhaseAborted,
			},
			want: `{"mode":"recording","phase":"aborted","completed":false,"exhausted":false,"currentTick":null,"nextFrame":0,"frameCount":0}`,
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
	assertInputSessionStatusJSON(t, status.Invoke(), idle)

	preparation, err := spx.PrepareInputRecording(60)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { preparation.Cancel() })
	assertInputSessionStatusJSON(t, status.Invoke(), `{"mode":"recording","phase":"prepared","completed":false,"exhausted":false,"currentTick":null,"nextFrame":0,"frameCount":0}`)
	if !preparation.Cancel() {
		t.Fatal("recording preparation was not cancelled")
	}
	assertInputSessionStatusJSON(t, status.Invoke(), idle)

	replayPreparation, err := spx.PrepareInputReplay(spx.InputReplay{
		Format: spx.InputReplayFormat, Version: spx.InputReplayVersion, FixedTimestep: 1.0 / 30,
		Frames: []spx.InputReplayFrame{{Frame: 0, Time: 0}},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { replayPreparation.Cancel() })
	assertInputSessionStatusJSON(t, status.Invoke(), `{"mode":"replaying","phase":"prepared","completed":false,"exhausted":false,"currentTick":null,"nextFrame":0,"frameCount":1}`)
	if !replayPreparation.Cancel() {
		t.Fatal("replay preparation was not cancelled")
	}
	assertInputSessionStatusJSON(t, status.Invoke(), idle)
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
