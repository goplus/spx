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

package input

import (
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"
)

func validInputReplay() InputReplay {
	return InputReplay{
		Format:        InputReplayFormat,
		Version:       InputReplayVersion,
		FixedTimestep: 1.0 / 30.0,
		Initial: InputReplayState{
			Mouse:    InputReplayMouse{X: 1, Y: 2},
			Buttons:  0,
			KeysDown: []int64{10},
		},
		Frames: []InputReplayFrame{
			{
				Frame: 0,
				Time:  0,
				State: InputReplayState{
					Mouse:    InputReplayMouse{X: 3, Y: 4},
					Buttons:  1,
					KeysDown: []int64{10, 20},
				},
				MouseEvents: []InputReplayMouseEvent{{Button: 1, Pressed: true}},
				KeyEvents:   []InputReplayKeyEvent{{Key: 20, Pressed: true}},
			},
			{
				Frame: 1,
				Time:  0.25,
				State: InputReplayState{
					Mouse:    InputReplayMouse{X: 5, Y: 6},
					Buttons:  0,
					KeysDown: []int64{20},
				},
				MouseEvents: []InputReplayMouseEvent{{Button: 1, Pressed: false}},
				KeyEvents:   []InputReplayKeyEvent{{Key: 10, Pressed: false}},
			},
		},
	}
}

func TestInputReplayValidateAcceptsCanonicalReplay(t *testing.T) {
	if err := validInputReplay().Validate(); err != nil {
		t.Fatalf("Validate() error: %v", err)
	}
}

func TestInputReplayValidateRejectsInvalidData(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*InputReplay)
		want   string
	}{
		{"format", func(r *InputReplay) { r.Format = "other" }, "format"},
		{"version", func(r *InputReplay) { r.Version++ }, "version"},
		{"negative fixed timestep", func(r *InputReplay) { r.FixedTimestep = -1 }, "fixed timestep"},
		{"non-finite fixed timestep", func(r *InputReplay) { r.FixedTimestep = math.Inf(1) }, "fixed timestep"},
		{"initial mouse", func(r *InputReplay) { r.Initial.Mouse.X = math.NaN() }, "mouse coordinates"},
		{"initial buttons", func(r *InputReplay) { r.Initial.Buttons = 8 }, "unsupported bits"},
		{"initial invalid key", func(r *InputReplay) { r.Initial.KeysDown = []int64{0} }, "invalid key"},
		{"initial unsorted keys", func(r *InputReplay) { r.Initial.KeysDown = []int64{2, 1} }, "strictly increasing"},
		{"initial duplicate keys", func(r *InputReplay) { r.Initial.KeysDown = []int64{1, 1} }, "strictly increasing"},
		{"frame gap", func(r *InputReplay) { r.Frames[1].Frame = 2 }, "frame 2"},
		{"frame zero time", func(r *InputReplay) { r.Frames[0].Time = 0.1 }, "want 0"},
		{"negative time", func(r *InputReplay) { r.Frames[1].Time = -1 }, "non-negative"},
		{"non-finite time", func(r *InputReplay) { r.Frames[1].Time = math.NaN() }, "finite"},
		{"time over clock range", func(r *InputReplay) { r.Frames[1].Time = maxInputReplayTime + 1 }, "finite"},
		{"decreasing time", func(r *InputReplay) {
			r.Frames[0].Time = 0
			r.Frames[1].Time = -0.0
			r.Frames = append(r.Frames, InputReplayFrame{Frame: 2, Time: 0.1, State: InputReplayState{}})
			r.Frames[1].Time = 0.2
			r.Frames[2].Time = 0.1
		}, "precedes"},
		{"frame state", func(r *InputReplay) { r.Frames[1].State.KeysDown = []int64{3, 2} }, "frame 1 state"},
		{"invalid mouse event", func(r *InputReplay) { r.Frames[0].MouseEvents[0].Button = 4 }, "invalid button"},
		{"duplicate mouse event", func(r *InputReplay) {
			r.Frames[0].MouseEvents = append(r.Frames[0].MouseEvents, InputReplayMouseEvent{Button: 1, Pressed: true})
		}, "does not change"},
		{"mouse event state mismatch", func(r *InputReplay) { r.Frames[0].State.Buttons = 0 }, "do not match buttons"},
		{"invalid key event", func(r *InputReplay) { r.Frames[0].KeyEvents[0].Key = 0 }, "invalid key"},
		{"key event state mismatch", func(r *InputReplay) { r.Frames[0].KeyEvents = nil }, "do not match keysDown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			replay := validInputReplay()
			tt.mutate(&replay)
			err := replay.Validate()
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Validate() error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestInputReplayJSONRoundTripIsStrict(t *testing.T) {
	want := validInputReplay()
	data, err := EncodeInputReplay(want)
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeInputReplay(data)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip = %+v, want %+v", got, want)
	}

	unknown := strings.Replace(string(data), `"version":1`, `"version":1,"unknown":true`, 1)
	if _, err := DecodeInputReplay([]byte(unknown)); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("unknown-field error = %v", err)
	}
	if _, err := DecodeInputReplay(append(data, []byte(` {}`)...)); err == nil || !strings.Contains(err.Error(), "trailing") {
		t.Fatalf("trailing-data error = %v", err)
	}
	if _, err := DecodeInputReplay(make([]byte, MaxInputReplayJSONSize+1)); err == nil || !strings.Contains(err.Error(), "exceeds limit") {
		t.Fatalf("oversize error = %v", err)
	}
}

func TestInputReplayControllerRecordsTicksAndDeepCopies(t *testing.T) {
	var controller InputReplayController
	initial := InputReplayState{KeysDown: []int64{1}}
	if err := controller.StartRecording(initial, 1.0/60.0); err != nil {
		t.Fatal(err)
	}
	initial.KeysDown[0] = 999

	live0 := InputReplayState{
		Mouse:    InputReplayMouse{X: 10, Y: 20},
		Buttons:  1,
		KeysDown: []int64{1, 2},
	}
	events0 := []InputReplayKeyEvent{{Key: 2, Pressed: true}}
	mouseEvents0 := []InputReplayMouseEvent{{Button: 1, Pressed: true}}
	frame0, first, err := controller.ResolveWithMouseEvents(live0, mouseEvents0, events0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if !first || frame0.Frame != 0 || frame0.Time != 0 {
		t.Fatalf("first Resolve = (%+v, %v), want frame/time 0 and first", frame0, first)
	}
	live0.KeysDown[0] = 888
	events0[0].Key = 888
	mouseEvents0[0].Button = 3
	frame0.State.KeysDown[0] = 777
	frame0.MouseEvents[0].Button = 3
	frame0.KeyEvents[0].Key = 777

	frame1, first, err := controller.ResolveWithMouseEvents(
		InputReplayState{Mouse: InputReplayMouse{X: 11, Y: 21}, KeysDown: []int64{2}},
		[]InputReplayMouseEvent{{Button: 1, Pressed: false}},
		[]InputReplayKeyEvent{{Key: 1, Pressed: false}},
		0.25,
	)
	if err != nil {
		t.Fatal(err)
	}
	if first || frame1.Frame != 1 || frame1.Time != 0.25 {
		t.Fatalf("second Resolve = (%+v, %v), want frame 1/time .25", frame1, first)
	}
	frame2, _, err := controller.Resolve(InputReplayState{}, nil, 0.5)
	if err != nil {
		t.Fatal(err)
	}
	if frame2.Frame != 2 || frame2.Time != 0.75 {
		t.Fatalf("third Resolve = %+v, want frame 2/time .75", frame2)
	}

	recorded, err := controller.Recording()
	if err != nil {
		t.Fatal(err)
	}
	if recorded.Initial.KeysDown[0] != 1 {
		t.Fatalf("recorded initial keys = %v, want [1]", recorded.Initial.KeysDown)
	}
	if recorded.Frames[0].State.KeysDown[0] != 1 || recorded.Frames[0].MouseEvents[0].Button != 1 || recorded.Frames[0].KeyEvents[0].Key != 2 {
		t.Fatalf("recording aliased caller or result: %+v", recorded.Frames[0])
	}
	controller.Reset()
	if status := controller.Status(); status.Mode != InputSessionModeIdle {
		t.Fatalf("status after Reset = %+v", status)
	}
}

func TestInputReplayControllerReplaysAndFreezesLastState(t *testing.T) {
	var controller InputReplayController
	replay := validInputReplay()
	if err := controller.StartReplay(replay); err != nil {
		t.Fatal(err)
	}
	replay.Frames[0].State.KeysDown[0] = 999

	frame0, first, err := controller.Resolve(
		InputReplayState{Mouse: InputReplayMouse{X: math.NaN()}},
		[]InputReplayKeyEvent{{Key: 999, Pressed: true}},
		math.NaN(),
	)
	if err != nil {
		t.Fatalf("replay should ignore invalid live input: %v", err)
	}
	if !first || frame0.Frame != 0 || frame0.State.KeysDown[0] != 10 {
		t.Fatalf("first replay frame = (%+v, %v)", frame0, first)
	}
	frame0.State.KeysDown[0] = 999
	frame0.KeyEvents[0].Key = 999

	frame1, first, err := controller.Resolve(InputReplayState{}, nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	if first || frame1.Frame != 1 || frame1.State.KeysDown[0] != 20 {
		t.Fatalf("second replay frame = (%+v, %v)", frame1, first)
	}
	status := controller.Status()
	if !status.Exhausted || status.NextFrame != 2 || status.FrameCount != 2 {
		t.Fatalf("finished status = %+v", status)
	}

	frozen, first, err := controller.Resolve(
		InputReplayState{Mouse: InputReplayMouse{X: 999, Y: 999}, KeysDown: []int64{999}},
		[]InputReplayKeyEvent{{Key: 999, Pressed: true}},
		999,
	)
	if err != nil {
		t.Fatal(err)
	}
	if first || !reflect.DeepEqual(frozen.State, frame1.State) {
		t.Fatalf("frozen frame = (%+v, %v), want state %+v", frozen, first, frame1.State)
	}
	if len(frozen.KeyEvents) != 0 {
		t.Fatalf("frozen frame repeated key events: %+v", frozen.KeyEvents)
	}
	if len(frozen.MouseEvents) != 0 {
		t.Fatalf("frozen frame repeated mouse events: %+v", frozen.MouseEvents)
	}
	controller.Reset()
}

func TestInputReplayValidateAcceptsLegacyButtonSnapshots(t *testing.T) {
	replay := validInputReplay()
	for i := range replay.Frames {
		replay.Frames[i].MouseEvents = nil
	}
	if err := replay.Validate(); err != nil {
		t.Fatalf("legacy replay validation failed: %v", err)
	}
}

func TestInputReplayPreservesShortClickEdgesWithUnchangedHeldState(t *testing.T) {
	replay := InputReplay{
		Format:  InputReplayFormat,
		Version: InputReplayVersion,
		Frames: []InputReplayFrame{{
			Frame: 0,
			State: InputReplayState{Buttons: 0},
			MouseEvents: []InputReplayMouseEvent{
				{Button: 1, Pressed: true},
				{Button: 1, Pressed: false},
			},
		}},
	}
	data, err := EncodeInputReplay(replay)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeInputReplay(data)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded.Frames[0].MouseEvents, replay.Frames[0].MouseEvents) {
		t.Fatalf("short-click edges = %+v, want %+v", decoded.Frames[0].MouseEvents, replay.Frames[0].MouseEvents)
	}
}

func TestInputReplayControllerEmptyReplayFreezesInitialState(t *testing.T) {
	var controller InputReplayController
	replay := InputReplay{
		Format:  InputReplayFormat,
		Version: InputReplayVersion,
		Initial: InputReplayState{KeysDown: []int64{1}},
	}
	if err := controller.StartReplay(replay); err != nil {
		t.Fatal(err)
	}
	if controller.Status().Exhausted {
		t.Fatal("empty replay exhausted before its first effective tick")
	}
	frame, first, err := controller.Resolve(InputReplayState{KeysDown: []int64{9}}, nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !first || frame.Frame != -1 || !reflect.DeepEqual(frame.State.KeysDown, []int64{1}) {
		t.Fatalf("empty replay Resolve = (%+v, %v)", frame, first)
	}
	if !controller.Status().Exhausted {
		t.Fatal("empty replay did not exhaust after its first effective tick")
	}
	_, first, err = controller.Resolve(InputReplayState{KeysDown: []int64{9}}, nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	if first {
		t.Fatal("empty replay reported first tick more than once")
	}
}

func TestInputReplayControllerModeErrorsAndReset(t *testing.T) {
	var controller InputReplayController
	if _, err := controller.Recording(); !errors.Is(err, ErrInputSessionNotRecording) {
		t.Fatalf("Recording error = %v", err)
	}
	if err := controller.StartRecording(InputReplayState{}, 0); err != nil {
		t.Fatal(err)
	}
	if err := controller.StartReplay(validInputReplay()); !errors.Is(err, ErrInputSessionActive) {
		t.Fatalf("StartReplay while recording error = %v", err)
	}
	controller.Reset()
	if status := controller.Status(); status != (InputReplayControllerStatus{Mode: InputSessionModeIdle}) {
		t.Fatalf("status after Reset = %+v", status)
	}
	if err := controller.StartReplay(validInputReplay()); err != nil {
		t.Fatalf("StartReplay after Reset: %v", err)
	}
	controller.Reset()
	if status := controller.Status(); status.Mode != InputSessionModeIdle {
		t.Fatalf("status after Reset = %+v", status)
	}
}

func TestInputReplayControllerSynchronizesInitialBeforeTickZero(t *testing.T) {
	var controller InputReplayController
	if err := controller.StartRecording(InputReplayState{KeysDown: []int64{1}}, 0); err != nil {
		t.Fatal(err)
	}
	initial := InputReplayState{Mouse: InputReplayMouse{X: 4, Y: 5}, KeysDown: []int64{2}}
	if err := controller.SynchronizeRecordingInitial(initial); err != nil {
		t.Fatal(err)
	}
	if _, _, err := controller.Resolve(initial, nil, 1); err != nil {
		t.Fatal(err)
	}
	if err := controller.SynchronizeRecordingInitial(InputReplayState{}); err == nil {
		t.Fatal("SynchronizeRecordingInitial after tick zero succeeded")
	}
	replay, err := controller.Recording()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(replay.Initial, initial) {
		t.Fatalf("synchronized initial = %+v, want %+v", replay.Initial, initial)
	}
}

func TestInputReplayControllerIdleResolveReturnsIsolatedLiveFrame(t *testing.T) {
	var controller InputReplayController
	live := InputReplayState{KeysDown: []int64{1}}
	events := []InputReplayKeyEvent{{Key: 1, Pressed: true}}
	frame, first, err := controller.Resolve(live, events, 0.1)
	if err != nil {
		t.Fatal(err)
	}
	if first || frame.Frame != -1 || frame.Time != 0 {
		t.Fatalf("idle Resolve = (%+v, %v)", frame, first)
	}
	live.KeysDown[0] = 2
	events[0].Key = 2
	if frame.State.KeysDown[0] != 1 || frame.KeyEvents[0].Key != 1 {
		t.Fatalf("idle Resolve aliased live input: %+v", frame)
	}
}
