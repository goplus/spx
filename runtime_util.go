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

package spx

import (
	"math/rand"
	"strings"

	"github.com/goplus/spx/v2/internal/engine"
	itime "github.com/goplus/spx/v2/internal/time"
)

// Rand__0 returns a random integer between from and to (inclusive).
func Rand__0(from, to int) float64 {
	if to < from {
		to = from
	}
	return float64(from + rand.Intn(to-from+1))
}

// Rand__1 returns a random float64 between from and to.
func Rand__1(from, to float64) float64 {
	if to < from {
		to = from
	}
	return rand.Float64()*(to-from) + from
}

// Iround returns an integer value, while math.Round returns a float value.
func Iround(v float64) int {
	if v >= 0 {
		return int(v + 0.5)
	}
	return int(v - 0.5)
}

// Contains reports whether substr is within s.
func Contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// DeltaTime returns the time elapsed since the previous frame.
func DeltaTime() Seconds {
	return itime.DeltaTime()
}

// TimeSinceLevelLoad returns the time elapsed since the current level started.
func TimeSinceLevelLoad() Seconds {
	return itime.TimeSinceLevelLoad()
}

// CharAt returns the character at the given 1-based index in the string s.
// It returns an empty string if idx is out of range (< 1 or > len(s)).
// This matches Scratch's "letter (idx) of (s)" block behavior.
func CharAt(s string, idx int) string {
	r := []rune(s)
	if idx < 1 || idx > len(r) {
		return ""
	}
	return string(r[idx-1])
}

// Exit__0 exits the program with exit code 0.
func Exit__0() {
	engine.RequestExit(0)
}

// Exit__1 exits the program with the specified exit code.
func Exit__1(code int) {
	engine.RequestExit(int64(code))
}
