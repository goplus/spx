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

package input

import (
	"testing"
	"time"
)

func TestClickGateAllowHonorsIntervalPerObject(t *testing.T) {
	now := time.Unix(0, 0)
	gate := ClickGate{
		now: func() time.Time { return now },
	}
	gate.Init(50 * time.Millisecond)

	if !gate.Allow(1) {
		t.Fatal("expected first click to be allowed")
	}
	if !gate.Allow(2) {
		t.Fatal("expected independent object click to be allowed")
	}

	now = now.Add(25 * time.Millisecond)
	if gate.Allow(1) {
		t.Fatal("expected click inside interval to be blocked")
	}

	now = now.Add(25 * time.Millisecond)
	if !gate.Allow(1) {
		t.Fatal("expected click at interval boundary to be allowed")
	}
}

func TestClickGatePrunesExpiredEntriesAndRemove(t *testing.T) {
	now := time.Unix(0, 0)
	gate := ClickGate{
		now: func() time.Time { return now },
	}
	gate.Init(50 * time.Millisecond)

	if !gate.Allow(1) {
		t.Fatal("expected first click to be allowed")
	}
	now = now.Add(60 * time.Millisecond)
	if !gate.Allow(2) {
		t.Fatal("expected second click to be allowed")
	}
	if len(gate.lastClickMs) != 1 {
		t.Fatalf("lastClickMs len = %d, want 1 after pruning", len(gate.lastClickMs))
	}

	gate.Remove(2)
	if len(gate.lastClickMs) != 0 {
		t.Fatalf("lastClickMs len = %d, want 0 after remove", len(gate.lastClickMs))
	}
}
