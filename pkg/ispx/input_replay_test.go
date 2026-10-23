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
	"errors"
	"math"
	"testing"

	spx "github.com/goplus/spx/v4"
)

func TestHostInputRecordingBridge(t *testing.T) {
	for _, test := range []struct {
		name    string
		fps     float64
		wantErr bool
	}{
		{"explicit FPS", 60, false},
		{"default FPS", 0, false},
		{"negative FPS", -1, true},
		{"NaN FPS", math.NaN(), true},
		{"infinite FPS", math.Inf(1), true},
	} {
		t.Run(test.name, func(t *testing.T) {
			preparation, err := prepareHostInputRecording(test.fps)
			t.Cleanup(func() { preparation.Cancel() })
			if (err != nil) != test.wantErr {
				t.Fatalf("prepareHostInputRecording(%v) error = %v, want error = %v", test.fps, err, test.wantErr)
			}
			want := spx.InputSessionStatus{Mode: spx.InputSessionModeIdle}
			if !test.wantErr {
				want.Mode = spx.InputSessionModeRecording
				want.Phase = spx.InputSessionPhasePrepared
			}
			if status := spx.GetInputSessionStatus(); status != want {
				t.Fatalf("recording status = %+v, want %+v", status, want)
			}
		})
	}
}

func TestHostInputReplayBridgeValidatesAndPreparesNextGame(t *testing.T) {
	data := hostInputReplayJSON(t)
	for _, state := range []string{"idle", "prepared"} {
		t.Run(state, func(t *testing.T) {
			want := spx.InputSessionStatus{Mode: spx.InputSessionModeIdle}
			if state == "prepared" {
				preparation, err := prepareHostInputReplay([]byte(data))
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { preparation.Cancel() })
				want.Mode = spx.InputSessionModeReplaying
				want.Phase = spx.InputSessionPhasePrepared
			}
			if status := spx.GetInputSessionStatus(); status != want {
				t.Fatalf("replay status = %+v, want %+v", status, want)
			}
			preparation, err := prepareHostInputReplay([]byte("{"))
			t.Cleanup(func() { preparation.Cancel() })
			if err == nil {
				t.Fatal("malformed replay was accepted")
			}
			if status := spx.GetInputSessionStatus(); status != want {
				t.Fatalf("malformed replay changed status to %+v, want %+v", status, want)
			}
		})
	}
}

func hostInputReplayJSON(t *testing.T, frames ...spx.InputReplayFrame) string {
	t.Helper()
	data, err := spx.EncodeInputReplay(spx.InputReplay{
		Format: spx.InputReplayFormat, Version: spx.InputReplayVersion,
		FixedTimestep: 1.0 / 30, Frames: frames,
	})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestHostInputRecordingFinishRequiresActiveRecording(t *testing.T) {
	for _, mode := range []string{"idle", "recording prepared", "replay prepared"} {
		t.Run(mode, func(t *testing.T) {
			var preparation spx.InputSessionPreparation
			var err error
			switch mode {
			case "recording prepared":
				preparation, err = prepareHostInputRecording(30)
			case "replay prepared":
				preparation, err = prepareHostInputReplay([]byte(hostInputReplayJSON(t)))
			}
			t.Cleanup(func() { preparation.Cancel() })
			if err != nil {
				t.Fatal(err)
			}
			before := spx.GetInputSessionStatus()
			if data, err := finishHostInputRecording(); !errors.Is(err, spx.ErrInputSessionNotRecording) || data != nil {
				t.Fatalf("finish recording = %q, %v; want nil, ErrInputSessionNotRecording", data, err)
			}
			if got := spx.GetInputSessionStatus(); got != before {
				t.Fatalf("failed finish changed status: %+v, want %+v", got, before)
			}
		})
	}
}

func TestHostInputSessionForwardsOptions(t *testing.T) {
	for _, mode := range []string{"record", "replay"} {
		for _, test := range []struct {
			name    string
			options []spx.InputSessionOptions
			wantErr bool
		}{
			{"capture key", []spx.InputSessionOptions{{CaptureKey: spx.KeyP}}, false},
			{"invalid capture key", []spx.InputSessionOptions{{CaptureKey: spx.KeyMax}}, true},
			{"multiple options", []spx.InputSessionOptions{{}, {}}, true},
		} {
			t.Run(mode+"/"+test.name, func(t *testing.T) {
				var preparation spx.InputSessionPreparation
				var err error
				if mode == "record" {
					preparation, err = prepareHostInputRecording(30, test.options...)
				} else {
					preparation, err = prepareHostInputReplay([]byte(hostInputReplayJSON(t)), test.options...)
				}
				t.Cleanup(func() { preparation.Cancel() })
				if (err != nil) != test.wantErr {
					t.Fatalf("preparation error = %v, want error = %v", err, test.wantErr)
				}
				if test.wantErr && spx.GetInputSessionStatus().Mode != spx.InputSessionModeIdle {
					t.Fatal("invalid options prepared an input session")
				}
			})
		}
	}
}
