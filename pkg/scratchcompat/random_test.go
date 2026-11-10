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
	"math"
	"testing"

	"github.com/goplus/spx/v4"
	"github.com/goplus/spx/v4/pkg/scratchcompat"
)

// Scalar classification and range boundaries are covered in internal/core/scratch.
// Keep public input types, Value unwrapping, and runtime random integration here.
func TestRandPreservesOperandTypes(t *testing.T) {
	t.Cleanup(spx.ResetRandomSeed)
	spx.SetRandomSeed(17)
	unit := spx.Rand__1(0, 1)
	items := scratchcompat.NewList("1", "1.0", "1e-2")
	for _, tt := range []struct {
		name     string
		from, to any
		want     float64
	}{
		{"integer numbers", 1, 2, 1 + math.Floor(unit*2)},
		{"fractional numbers", 1.5, 2.5, 1.5 + unit},
		{"integer strings", "1", "2", 1 + math.Floor(unit*2)},
		{"decimal string", "1.0", "2", 1 + unit},
		{"true", true, 2, 1 + math.Floor(unit*2)},
		{"nil", nil, 2, unit * 2},
		{"wrapped integer string", spx.NewValue("1"), spx.NewValue("2"), 1 + math.Floor(unit*2)},
		{"wrapped decimal string", spx.NewValue("1.0"), 2, 1 + unit},
		{"wrapped boolean", spx.NewValue(true), 2, 1 + math.Floor(unit*2)},
		{"wrapped NaN", spx.NewValue(math.NaN()), 2, math.Floor(unit * 3)},
		{"zero Value", spx.Value{}, 2, unit * 2},
		{"list integer string", items.At(0), 2, 1 + math.Floor(unit*2)},
		{"list decimal string", items.At(1), 2, 1 + unit},
		{"list exponent string", items.At(2), "1", 0.01 + math.Floor(unit*(2-0.01))},
	} {
		t.Run(tt.name, func(t *testing.T) {
			spx.SetRandomSeed(17)
			if got := scratchcompat.Rand(tt.from, tt.to); got != tt.want {
				t.Fatalf("Rand(%#v, %#v) = %.17g, want %.17g", tt.from, tt.to, got, tt.want)
			}
		})
	}
}

// The matching-card project draws float-valued IDs, allowing two cards per ID.
// Excluding the upper bound traps its warp loop after dealing only 22 cards.
func TestRandCanDealAllCardPairs(t *testing.T) {
	spx.SetRandomSeed(1042497735)
	t.Cleanup(spx.ResetRandomSeed)
	var counts [12]int
	dealt := 0
	for attempts := 0; attempts < 1024 && dealt < 24; attempts++ {
		index := int(scratchcompat.Rand(1, float64(len(counts)))) - 1
		if index < 0 || index >= len(counts) {
			t.Fatalf("card index %d is out of range", index)
		}
		if counts[index] < 2 {
			counts[index]++
			dealt++
		}
	}
	if dealt != 24 {
		t.Fatalf("dealing stalled after %d cards: %v", dealt, counts)
	}
}

func TestRandSharesScriptRandomSource(t *testing.T) {
	t.Cleanup(spx.ResetRandomSeed)
	spx.SetRandomSeed(17)
	first, second := spx.Rand__1(0, 1), spx.Rand__1(0, 1)
	items := scratchcompat.NewList("7")
	for _, tt := range []struct {
		from, to any
		wantNext float64
	}{
		{1, 6, second},
		{6, 1, second},
		{1.5, 2.5, second},
		{2.5, 1.5, second},
		{7, 7, first},
		{0.5, 0.5, first},
		{1e20, 1e20, first},
		{"1", "2", second},
		{"1.0", "2", second},
		{nil, 2, second},
		{math.NaN(), 2, second},
		{"7", 7, first},
		{"1.0", true, first},
		{nil, false, first},
		{math.NaN(), 0, first},
		{spx.NewValue("7"), 7, first},
		{items.At(0), 7, first},
		{math.Inf(1), math.Inf(1), first},
	} {
		spx.SetRandomSeed(17)
		_ = scratchcompat.Rand(tt.from, tt.to)
		if got := spx.Rand__1(0, 1); got != tt.wantNext {
			t.Fatalf("Rand(%v, %v) shifted the shared source: next = %v, want %v", tt.from, tt.to, got, tt.wantNext)
		}
	}
}
