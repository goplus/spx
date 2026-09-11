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

// Package pathutil contains lexical path rules shared by archive formats.
package pathutil

import "strings"

// ComponentProblem identifies a non-portable character spelling.
// Callers retain their format-specific error types and messages.
type ComponentProblem uint8

const (
	ComponentOK ComponentProblem = iota
	TrailingDotOrSpace
	ReservedCharacter
	ControlCharacter
)

func CheckComponent(component string) ComponentProblem {
	if strings.HasSuffix(component, ".") || strings.HasSuffix(component, " ") {
		return TrailingDotOrSpace
	}
	if strings.ContainsAny(component, `<>:"|?*`) {
		return ReservedCharacter
	}
	for _, r := range component {
		if r < 0x20 {
			return ControlCharacter
		}
	}
	return ComponentOK
}

// IsDeviceBase checks an already normalized DOS basename. Uppercase and
// lowercase ASCII are accepted; Unicode folding remains the caller's policy.
func IsDeviceBase(base string) bool {
	switch base {
	case "CON", "PRN", "AUX", "NUL", "CLOCK$", "CONIN$", "CONOUT$",
		"con", "prn", "aux", "nul", "clock$", "conin$", "conout$":
		return true
	}
	if len(base) < 4 {
		return false
	}
	switch base[:3] {
	case "COM", "LPT", "com", "lpt":
		switch base[3:] {
		case "1", "2", "3", "4", "5", "6", "7", "8", "9", "¹", "²", "³":
			return true
		}
	}
	return false
}
