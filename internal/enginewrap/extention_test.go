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

package enginewrap

import (
	"testing"
)

func TestCallInMainThreadRequiresInit(t *testing.T) {
	originalCallback := mainCallback
	t.Cleanup(func() {
		mainCallback = originalCallback
	})

	mainCallback = nil

	defer func() {
		if r := recover(); r != "enginewrap: Init must be called before using manager methods off the main thread" {
			t.Fatalf("panic = %v, want explicit Init requirement", r)
		}
	}()

	CallInMainThread(func() {})
}

func TestCallInMainThreadValue(t *testing.T) {
	originalCallback := mainCallback
	t.Cleanup(func() {
		mainCallback = originalCallback
	})

	calls := 0
	Init(func(call func()) {
		calls++
		call()
	})

	value := CallInMainThreadValue(func() int { return 42 })
	if value != 42 {
		t.Fatalf("value = %d, want 42", value)
	}
	if calls != 1 {
		t.Fatalf("main-thread callback calls = %d, want 1", calls)
	}
}
