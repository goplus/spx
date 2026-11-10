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
	"testing"
	"time"

	"github.com/goplus/spx/v4/internal/coroutine"
)

func TestNewValueUnwrapsOnce(t *testing.T) {
	var list List
	list.Init(NewValue("nested"))
	value := NewValue(list.At(0))
	if got := value.Raw(); got != NewValue("nested") {
		t.Fatalf("NewValue must unwrap exactly one layer: got %#v", got)
	}
}

func TestListRandomPositionSharesScriptRandomSource(t *testing.T) {
	t.Cleanup(ResetRandomSeed)
	list := NewList(0, 1, 2, 3, 4, 5, 6)
	SetRandomSeed(91)
	var indices [32]int32
	for i := range indices {
		indices[i] = randomInt31n(int32(list.Len()))
	}
	wantNext := Rand__1(0, 1)
	SetRandomSeed(91)
	for i, want := range indices {
		if got := list.At(Random).Int(); got != int(want) {
			t.Fatalf("random item %d = %d, want %d", i, got, want)
		}
	}
	if got := Rand__1(0, 1); got != wantNext {
		t.Fatalf("random list reads shifted the shared stream: got %v, want %v", got, wantNext)
	}

	SetRandomSeed(91)
	wantFirst := Rand__1(0, 1)
	SetRandomSeed(91)
	list.Clear()
	if list.At(Random).Raw() != nil {
		t.Fatal("empty list has a random item")
	}
	if got := Rand__1(0, 1); got != wantFirst {
		t.Fatalf("empty random list read consumed the shared stream: got %v, want %v", got, wantFirst)
	}
}

func TestListRandomPositionKeepsCoroutineSource(t *testing.T) {
	t.Cleanup(ResetRandomSeed)
	currentScheduler := setupRuntimeScheduler(t)
	list := NewList(0, 1, 2, 3, 4, 5, 6)
	run := func(useList bool, extraDraws int) []float64 {
		t.Helper()
		setDeterministicRandomSeed(91)
		noiseDone := make(chan struct{}, 1)
		currentScheduler.Create("other-random-draws", func(me coroutine.Thread) {
			for range extraDraws {
				_ = Rand__1(0, 1)
			}
			noiseDone <- struct{}{}
		})
		select {
		case <-noiseDone:
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for unrelated random draws")
		}

		done := make(chan []float64, 1)
		currentScheduler.Create("random-list-items", func(me coroutine.Thread) {
			values := make([]float64, 17)
			for i := range values[:16] {
				if useList {
					values[i] = list.At(Random).Float()
				} else {
					values[i] = float64(randomInt31n(int32(list.Len())))
				}
			}
			if useList {
				var empty List
				_ = empty.At(Random)
			}
			values[16] = Rand__1(0, 1)
			done <- values
		})
		select {
		case values := <-done:
			return values
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for random list reads")
			return nil
		}
	}
	want, got := run(false, 1), run(true, 5)
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("coroutine draw %d = %v, want %v", i, got[i], want[i])
		}
	}
}
