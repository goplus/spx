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
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestNewColor(t *testing.T) {
	want := Color{0x12 / 255.0, 0x34 / 255.0, 0x56 / 255.0, 1}
	assertColor(t, NewColor(0x123456), want)
	assertColor(t, NewColor(float64(0x123456)), want)
	assertColor(t, NewColor("#123456"), want)
}

func TestNewColorUsesMathfPackedNumberBehavior(t *testing.T) {
	assertColor(t, NewColor(-1), Color{1, 1, 1, 1})
	assertColor(t, NewColor(float64(0x1000000)), Color{0, 0, 0, 1})
}

func writeTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func assertColor(t *testing.T, got, want Color) {
	t.Helper()
	const epsilon = 1e-12
	if math.Abs(got.X_0-want.X_0) > epsilon ||
		math.Abs(got.X_1-want.X_1) > epsilon ||
		math.Abs(got.X_2-want.X_2) > epsilon ||
		math.Abs(got.X_3-want.X_3) > epsilon {
		t.Fatalf("Color = %#v, want %#v", got, want)
	}
}
