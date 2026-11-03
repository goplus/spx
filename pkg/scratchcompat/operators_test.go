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

package scratchcompat_test

import (
	"fmt"
	"math"
	"testing"

	"github.com/goplus/spx/v4"
	"github.com/goplus/spx/v4/pkg/scratchcompat"
)

// The semantic matrices live in internal/scratch; this checks the public facade.
func TestOperatorDelegation(t *testing.T) {
	if got := scratchcompat.LimitPrecision(5e-10); got != 0 {
		t.Errorf("LimitPrecision = %v", got)
	}
	if got := scratchcompat.Div(0, 0); got != 0 {
		t.Errorf("Div = %v", got)
	}
	if got := scratchcompat.FloorMod(-7, 5); got != 3 {
		t.Errorf("FloorMod = %v", got)
	}
	if !scratchcompat.Contains("Scratch", "RaTc") {
		t.Error("Contains lost case-insensitive matching")
	}
	items := spx.NewList("01")
	if got := scratchcompat.Compare(items.At(0), 2); got != -1 {
		t.Errorf("Compare = %v", got)
	}
	if !scratchcompat.Equal(items.At(0), 1) {
		t.Error("Equal lost SPX value compatibility")
	}
}

func TestCharAtUsesOneBasedIndex(t *testing.T) {
	for _, tt := range []struct {
		index int
		want  string
	}{
		{1, "A"}, {2, "😀"}, {3, "B"}, {4, ""},
		{0, ""}, {-1, ""}, {math.MinInt, ""}, {math.MaxInt, ""},
	} {
		if got := scratchcompat.CharAt("A😀B", tt.index); got != tt.want {
			t.Errorf("CharAt(%d) = %q, want %q", tt.index, got, tt.want)
		}
	}
	if got := scratchcompat.CharAt("", 1); got != "" {
		t.Errorf("CharAt(empty) = %q", got)
	}
	if got := spx.CharAt("A😀B", 0); got != "A" {
		t.Errorf("spx.CharAt lost zero-based indexing: %q", got)
	}
}

func ExampleLimitPrecision() {
	x := 5e-10
	// Only the sprite's own position reporter applies precision correction.
	fmt.Println(scratchcompat.LimitPrecision(x))
	// Property queries and stored coordinates retain their original precision.
	fmt.Println(x)
	// Output:
	// 0
	// 5e-10
}
