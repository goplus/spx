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

package project

import "encoding/json"

// MergeIndexRoot overlays packed index root fields on source fields.
func MergeIndexRoot(source, packed map[string]json.RawMessage) map[string]json.RawMessage {
	if len(source) == 0 {
		return packed
	}
	merged := make(map[string]json.RawMessage, len(source)+len(packed))
	for key, value := range source {
		merged[key] = value
	}
	for key, value := range packed {
		merged[key] = value
	}
	return merged
}

// DecodeIndexRoot decodes merged index root fields into a project config.
func DecodeIndexRoot(root map[string]json.RawMessage, dest any) error {
	if len(root) == 0 {
		return nil
	}
	data, err := json.Marshal(root)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dest)
}

// ParseIndexEntries decodes an optional index object section.
func ParseIndexEntries(raw json.RawMessage) (map[string]json.RawMessage, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var entries map[string]json.RawMessage
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}
