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

import spx "github.com/goplus/spx/v3"

const defaultHostInputRecordingFPS = 30

type hostInputSessionStatus struct {
	Mode           string
	Phase          string
	Completed      bool
	Exhausted      bool
	CurrentTick    int64
	HasCurrentTick bool
	NextFrame      int64
	FrameCount     int
	Error          string
}

func prepareHostInputRecording(fps float64, options ...spx.InputSessionOptions) (spx.InputSessionPreparation, error) {
	if fps == 0 {
		fps = defaultHostInputRecordingFPS
	}
	return spx.PrepareInputRecording(fps, options...)
}

func finishHostInputRecording() ([]byte, error) {
	encoded, err := spx.FinishInputRecordingJSON()
	if err != nil {
		return nil, err
	}
	return []byte(encoded), nil
}

func prepareHostInputReplay(data []byte, options ...spx.InputSessionOptions) (spx.InputSessionPreparation, error) {
	replay, err := spx.DecodeInputReplay(string(data))
	if err != nil {
		return spx.InputSessionPreparation{}, err
	}
	return spx.PrepareInputReplay(replay, options...)
}

func cancelPreparedHostInputSession(preparation spx.InputSessionPreparation) {
	preparation.Cancel()
}

func getHostInputSessionStatus() hostInputSessionStatus {
	status := spx.GetInputSessionStatus()
	return hostInputSessionStatus{
		Mode:           string(status.Mode),
		Phase:          string(status.Phase),
		Completed:      status.Completed,
		Exhausted:      status.Exhausted,
		CurrentTick:    status.CurrentTick,
		HasCurrentTick: status.HasCurrentTick,
		NextFrame:      status.NextFrame,
		FrameCount:     status.FrameCount,
		Error:          status.Error,
	}
}
