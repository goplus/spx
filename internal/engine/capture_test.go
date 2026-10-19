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

package engine

import (
	"errors"
	"sync"
	"testing"

	itime "github.com/goplus/spx/v3/internal/time"
)

func TestCaptureCanCarryInputTickMetadata(t *testing.T) {
	SetGame(nil)
	ResetCaptures()
	defer ResetCaptures()

	var got CaptureRequest
	SetCaptureHandler(func(req CaptureRequest) error {
		got = req
		return nil
	})
	defer SetCaptureHandler(nil)

	tick := int64(0)
	if err := EnqueueCapture("tick", &tick); err != nil {
		t.Fatal(err)
	}
	if got.InputTick == nil || *got.InputTick != 0 {
		t.Fatalf("capture input tick = %v, want 0", got.InputTick)
	}
	if got.Frame != itime.Frame() {
		t.Fatalf("capture engine frame = %d, want %d", got.Frame, itime.Frame())
	}
}

func TestCaptureCopiesInputTickAtEnqueue(t *testing.T) {
	SetGame(struct{}{})
	defer SetGame(nil)
	ResetCaptures()
	defer ResetCaptures()

	var got CaptureRequest
	SetCaptureHandler(func(request CaptureRequest) error {
		got = request
		return nil
	})
	defer SetCaptureHandler(nil)

	tick := int64(7)
	if err := EnqueueCapture("copied", &tick); err != nil {
		t.Fatal(err)
	}
	tick = 8
	if err := FlushCaptures(); err != nil {
		t.Fatal(err)
	}
	if got.InputTick == nil || *got.InputTick != 7 {
		t.Fatalf("capture input tick = %v, want copied value 7", got.InputTick)
	}
}

func TestCaptureWithoutGameSupportsReentryAndReturnsErrors(t *testing.T) {
	SetGame(nil)
	ResetCaptures()
	defer ResetCaptures()
	SetCaptureHandler(nil)
	defer SetCaptureHandler(nil)

	if err := EnqueueCapture("missing-backend", nil); !errors.Is(err, errCaptureHandlerNotConfigured) {
		t.Fatalf("capture without backend = %v, want backend error", err)
	}
	backendErr := errors.New("capture failed")
	var got []CaptureRequest
	SetCaptureHandler(func(request CaptureRequest) error {
		got = append(got, request)
		SetCaptureHandler(func(request CaptureRequest) error {
			got = append(got, request)
			return backendErr
		})
		return EnqueueCapture("inner", nil)
	})
	if err := EnqueueCapture("outer", nil); !errors.Is(err, backendErr) {
		t.Fatalf("reentrant capture = %v, want backend error", err)
	}
	if len(got) != 2 || got[0].Name != "outer" || got[0].Sequence != 2 || got[1].Name != "inner" || got[1].Sequence != 3 {
		t.Fatalf("immediate captures = %v, want outer/2 then inner/3", got)
	}
	if HasPendingCaptures() {
		t.Fatal("capture without a game left queued requests")
	}
}

func TestConcurrentCaptureMetadataIsCoherentAndOrdered(t *testing.T) {
	SetGame(struct{}{})
	defer SetGame(nil)
	ResetCaptures()
	defer ResetCaptures()
	expectedFrame := itime.Frame()

	var got []CaptureRequest
	SetCaptureHandler(func(req CaptureRequest) error {
		got = append(got, req)
		return nil
	})
	defer SetCaptureHandler(nil)

	const count = 32
	var wg sync.WaitGroup
	wg.Add(count)
	for i := 0; i < count; i++ {
		go func(i int) {
			defer wg.Done()
			if err := EnqueueCapture("capture", nil); err != nil {
				t.Errorf("EnqueueCapture(%d): %v", i, err)
			}
		}(i)
	}
	wg.Wait()

	if !HasPendingCaptures() {
		t.Fatal("HasPendingCaptures = false with queued captures")
	}
	if err := FlushCaptures(); err != nil {
		t.Fatal(err)
	}
	if HasPendingCaptures() {
		t.Fatal("HasPendingCaptures = true after flush")
	}
	if len(got) != count {
		t.Fatalf("capture count = %d, want %d", len(got), count)
	}
	for i, req := range got {
		if req.Frame != expectedFrame {
			t.Fatalf("capture[%d].Frame = %d, want %d", i, req.Frame, expectedFrame)
		}
		if req.Sequence != uint64(i+1) {
			t.Fatalf("capture[%d].Sequence = %d, want %d", i, req.Sequence, i+1)
		}
	}
}

func TestFlushCapturesContinuesAfterHandlerError(t *testing.T) {
	SetGame(struct{}{})
	defer SetGame(nil)
	ResetCaptures()
	defer ResetCaptures()

	firstErr := errors.New("first capture failed")
	thirdErr := errors.New("third capture failed")
	var got []string
	SetCaptureHandler(func(req CaptureRequest) error {
		got = append(got, req.Name)
		switch req.Name {
		case "first":
			return firstErr
		case "third":
			return thirdErr
		default:
			return nil
		}
	})
	defer SetCaptureHandler(nil)

	for _, name := range []string{"first", "second", "third"} {
		if err := EnqueueCapture(name, nil); err != nil {
			t.Fatal(err)
		}
	}
	err := FlushCaptures()
	if !errors.Is(err, firstErr) || !errors.Is(err, thirdErr) {
		t.Fatalf("FlushCaptures error = %v, want both handler errors", err)
	}
	want := []string{"first", "second", "third"}
	if len(got) != len(want) {
		t.Fatalf("dispatched captures = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("dispatched captures = %v, want %v", got, want)
		}
	}
}

func TestFlushCapturesSupportsReentrantResetAndHandlerChange(t *testing.T) {
	SetGame(struct{}{})
	defer SetGame(nil)
	ResetCaptures()
	defer ResetCaptures()

	firstErr := errors.New("first capture failed")
	var got []CaptureRequest
	firstHandlerCalls := 0
	SetCaptureHandler(func(request CaptureRequest) error {
		firstHandlerCalls++
		got = append(got, request)
		ResetCaptures()
		if err := EnqueueCapture("after-reset", nil); err != nil {
			t.Fatal(err)
		}
		SetCaptureHandler(func(request CaptureRequest) error {
			got = append(got, request)
			return nil
		})
		return firstErr
	})
	defer SetCaptureHandler(nil)

	for _, name := range []string{"first", "second"} {
		if err := EnqueueCapture(name, nil); err != nil {
			t.Fatal(err)
		}
	}
	if err := FlushCaptures(); !errors.Is(err, firstErr) {
		t.Fatalf("FlushCaptures error = %v, want first handler error", err)
	}
	if firstHandlerCalls != 1 {
		t.Fatalf("first handler calls = %d, want 1 before replacement", firstHandlerCalls)
	}
	if len(got) != 2 || got[0].Name != "first" || got[0].Sequence != 1 || got[1].Name != "second" || got[1].Sequence != 2 {
		t.Fatalf("flushed captures = %v, want original batch first/1 and second/2", got)
	}
	if !HasPendingCaptures() {
		t.Fatal("reentrant capture was not queued for the next flush")
	}
	if err := FlushCaptures(); err != nil {
		t.Fatalf("next flush did not use the new handler: %v", err)
	}
	if len(got) != 3 || got[2].Name != "after-reset" || got[2].Sequence != 1 {
		t.Fatalf("captures after next flush = %v, want after-reset/1 last", got)
	}
	if HasPendingCaptures() {
		t.Fatal("reentrant capture remained queued after the next flush")
	}
}

func TestResetCapturesClearsCapturesAndResetsSequence(t *testing.T) {
	SetGame(struct{}{})
	defer SetGame(nil)
	ResetCaptures()
	defer ResetCaptures()
	frame := itime.Frame()

	var got []CaptureRequest
	SetCaptureHandler(func(req CaptureRequest) error {
		got = append(got, req)
		return nil
	})
	defer SetCaptureHandler(nil)

	if err := EnqueueCapture("old.png", nil); err != nil {
		t.Fatal(err)
	}
	ResetCaptures()
	if HasPendingCaptures() {
		t.Fatal("ResetCaptures did not clear pending captures")
	}
	if err := FlushCaptures(); err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("captures after reset = %v, want none", got)
	}

	tick := int64(3)
	if err := EnqueueCapture("new.png", &tick); err != nil {
		t.Fatal(err)
	}
	if err := FlushCaptures(); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("captures after enqueue = %v, want one", got)
	}
	request := got[0]
	if request.Name != "new.png" || request.Sequence != 1 || request.Frame != frame || request.InputTick == nil || *request.InputTick != 3 {
		t.Fatalf("capture after reset = %+v, want new.png, sequence 1, frame %d, input tick 3", request, frame)
	}
}

func TestCaptureRequestQueuesForActiveGame(t *testing.T) {
	var got CaptureRequest
	SetGame(struct{}{})
	defer SetGame(nil)
	ResetCaptures()
	defer ResetCaptures()
	expectedFrame := itime.Frame()
	SetCaptureHandler(func(req CaptureRequest) error {
		got = req
		return nil
	})
	defer SetCaptureHandler(nil)

	if err := EnqueueCapture("step_001.png", nil); err != nil {
		t.Fatal(err)
	}
	if got.Name != "" {
		t.Fatalf("EnqueueCapture ran immediately: %+v", got)
	}

	if err := FlushCaptures(); err != nil {
		t.Fatal(err)
	}
	if got.Name != "step_001.png" {
		t.Fatalf("capture name = %q, want step_001.png", got.Name)
	}
	if got.Frame != expectedFrame {
		t.Fatalf("capture frame = %d, want %d", got.Frame, expectedFrame)
	}
	if got.InputTick != nil {
		t.Fatalf("capture input tick = %v, want nil", *got.InputTick)
	}
	if got.Sequence == 0 {
		t.Fatal("capture sequence was not assigned")
	}
}
