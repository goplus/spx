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
	"math"
	"testing"

	spx "github.com/goplus/spx/v3"
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
	replay := spx.InputReplay{
		Format:        spx.InputReplayFormat,
		Version:       spx.InputReplayVersion,
		FixedTimestep: 1.0 / 30,
	}
	data, err := spx.EncodeInputReplay(replay)
	if err != nil {
		t.Fatal(err)
	}
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
