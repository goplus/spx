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

package runtime

import "testing"

func TestRunOnMainThreadTracksState(t *testing.T) {
	if IsMainThread() {
		t.Fatal("IsMainThread() should be false before RunOnMainThread")
	}

	inside := false
	RunOnMainThread(func() {
		inside = IsMainThread()
	})

	if !inside {
		t.Fatal("IsMainThread() should be true inside RunOnMainThread")
	}
	if IsMainThread() {
		t.Fatal("IsMainThread() should be false after RunOnMainThread")
	}
}

func TestRunOnMainThreadSupportsNestedCalls(t *testing.T) {
	steps := 0
	RunOnMainThread(func() {
		if !IsMainThread() {
			t.Fatal("outer RunOnMainThread should set the main thread marker")
		}
		steps++
		RunOnMainThread(func() {
			if !IsMainThread() {
				t.Fatal("nested RunOnMainThread should keep the main thread marker")
			}
			steps++
		})
		if !IsMainThread() {
			t.Fatal("outer RunOnMainThread should still be active after nesting")
		}
	})

	if steps != 2 {
		t.Fatalf("steps = %d, want 2", steps)
	}
	if IsMainThread() {
		t.Fatal("main thread marker leaked after nested RunOnMainThread")
	}
}
