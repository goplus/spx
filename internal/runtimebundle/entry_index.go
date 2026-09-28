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

package runtimebundle

import (
	"fmt"
	"strings"
)

// entryIndex holds validated entries by their normalized, case-folded name.
// Checking a name does not insert it: ZIP names are checked before mode and
// content validation, and entries are recorded only after those checks pass.
type entryIndex map[string]Entry

func (entries entryIndex) checkName(key, name string) error {
	if previous, ok := entries[key]; ok {
		if previous.Name == name {
			return fmt.Errorf("%w: duplicate entry %q", ErrUnsafeArchive, name)
		}
		return fmt.Errorf("%w: case-fold/normalization collision between %q and %q", ErrUnsafeArchive, previous.Name, name)
	}
	return nil
}

func (entries entryIndex) checkParents() error {
	for parent, entry := range entries {
		for {
			index := strings.LastIndexByte(parent, '/')
			if index < 0 {
				break
			}
			parent = parent[:index]
			if parentEntry, ok := entries[parent]; ok && !parentEntry.isDir() {
				return fmt.Errorf("%w: file %q is also a parent of %q", ErrUnsafeArchive, parentEntry.Name, entry.Name)
			}
		}
	}
	return nil
}
