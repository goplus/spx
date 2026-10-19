//go:build js || pure_engine
// +build js pure_engine

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

package coroutine

import "testing"

func TestWaitMainThreadDirectPlatform(t *testing.T) {
	co := New(nil)
	called := false

	co.WaitMainThread(func() { called = true })

	if !called {
		t.Fatal("WaitMainThread should execute immediately without a native bridge")
	}
}
