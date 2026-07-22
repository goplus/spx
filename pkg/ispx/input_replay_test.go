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

	spx "github.com/goplus/spx/v2"
)

func TestHostInputRecordingBridgePreparesNextGame(t *testing.T) {
	preparation, err := prepareHostInputRecording(60)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cancelPreparedHostInputSession(preparation) })
	status := getHostInputSessionStatus()
	if status.Mode != "recording" || status.Phase != "prepared" || status.Completed || status.NextFrame != 0 {
		t.Fatalf("prepared recording status = %+v", status)
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
	preparation, err := prepareHostInputReplay([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cancelPreparedHostInputSession(preparation) })
	status := getHostInputSessionStatus()
	if status.Mode != "replaying" || status.Phase != "prepared" || status.Completed || status.FrameCount != 0 || status.HasCurrentTick {
		t.Fatalf("prepared replay status = %+v", status)
	}
}

func TestHostInputRecordingBridgeRejectsInvalidFPS(t *testing.T) {
	for _, fps := range []float64{-1, math.NaN(), math.Inf(1)} {
		preparation, err := prepareHostInputRecording(fps)
		if err == nil {
			cancelPreparedHostInputSession(preparation)
			t.Fatalf("prepareHostInputRecording(%v) succeeded", fps)
		}
	}
}
