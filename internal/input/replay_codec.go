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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// EncodeInputReplay validates and encodes replay as JSON.
func EncodeInputReplay(replay InputReplay) ([]byte, error) {
	if err := replay.Validate(); err != nil {
		return nil, err
	}
	data, err := json.Marshal(replay)
	if err != nil {
		return nil, err
	}
	if len(data) > MaxInputReplayJSONSize {
		return nil, fmt.Errorf("input replay JSON size %d exceeds limit %d", len(data), MaxInputReplayJSONSize)
	}
	return data, nil
}

// DecodeInputReplay decodes and validates a strict replay JSON document.
func DecodeInputReplay(data []byte) (InputReplay, error) {
	if len(data) > MaxInputReplayJSONSize {
		return InputReplay{}, fmt.Errorf("input replay JSON size %d exceeds limit %d", len(data), MaxInputReplayJSONSize)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()

	var replay InputReplay
	if err := decoder.Decode(&replay); err != nil {
		return InputReplay{}, fmt.Errorf("decode input replay: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return InputReplay{}, fmt.Errorf("decode input replay: trailing JSON value")
		}
		return InputReplay{}, fmt.Errorf("decode input replay: trailing data: %w", err)
	}
	if err := replay.Validate(); err != nil {
		return InputReplay{}, err
	}
	return cloneInputReplay(replay), nil
}
