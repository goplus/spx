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

	"github.com/goplus/spx/v3/internal/coroutine"
	"github.com/goplus/spx/v3/internal/engine"
)

func TestDeterministicRandomIsolatedPerCoroutine(t *testing.T) {
	co := coroutine.New(nil)
	original := gco
	gco = co
	engine.SetCoroutines(co)
	t.Cleanup(func() {
		gco = original
		engine.SetCoroutines(original)
		ResetRandomSeed()
	})

	runScenario := func(extraDraws int) []float64 {
		t.Helper()
		setDeterministicRandomSeed(123)

		doneA := make(chan struct{}, 1)
		co.CreateAndStart(true, "extra-draws", func(me coroutine.Thread) int {
			for range extraDraws {
				_ = Rand__1(0, 1)
			}
			doneA <- struct{}{}
			return 0
		})
		select {
		case <-doneA:
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for extra-draw coroutine")
		}

		doneB := make(chan []float64, 1)
		co.CreateAndStart(true, "captured-sequence", func(me coroutine.Thread) int {
			doneB <- []float64{Rand__1(0, 1), Rand__1(0, 1)}
			return 0
		})
		select {
		case values := <-doneB:
			return values
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for captured-sequence coroutine")
			return nil
		}
	}

	gotA := runScenario(1)
	gotB := runScenario(5)
	if len(gotA) != len(gotB) {
		t.Fatalf("sequence length mismatch: %v vs %v", gotA, gotB)
	}
	for i := range gotA {
		if gotA[i] != gotB[i] {
			t.Fatalf("coroutine-local deterministic random mismatch at %d: %v vs %v", i, gotA, gotB)
		}
	}
}

func TestDeterministicRandomIgnoresWaitToDoGoroutineDraws(t *testing.T) {
	co := coroutine.New(nil)
	original := gco
	gco = co
	engine.SetCoroutines(co)
	t.Cleanup(func() {
		gco = original
		engine.SetCoroutines(original)
		ResetRandomSeed()
	})

	runScenario := func(drawOutsideCoroutine bool) []float64 {
		t.Helper()
		setDeterministicRandomSeed(123)

		done := make(chan []float64, 1)
		co.CreateAndStart(true, "wait-to-do-random", func(me coroutine.Thread) int {
			engine.WaitToDo(func() {
				if drawOutsideCoroutine {
					_ = Rand__1(0, 1)
				}
			})
			done <- []float64{Rand__1(0, 1), Rand__1(0, 1)}
			return 0
		})

		select {
		case values := <-done:
			return values
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for wait-to-do-random coroutine")
			return nil
		}
	}

	gotA := runScenario(false)
	gotB := runScenario(true)
	for i := range gotA {
		if gotA[i] != gotB[i] {
			t.Fatalf("wait-to-do goroutine shifted coroutine random stream at %d: %v vs %v", i, gotA, gotB)
		}
	}
}
