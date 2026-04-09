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

package defaults

import "testing"

func TestOrDefault(t *testing.T) {
	if got := OrDefault[int](nil, 42); got != 42 {
		t.Fatalf("OrDefault(nil) = %v, want 42", got)
	}
	val := 7
	if got := OrDefault(&val, 42); got != 7 {
		t.Fatalf("OrDefault(value) = %v, want 7", got)
	}
}

func TestSetDefaultIfZero(t *testing.T) {
	v := 0
	SetDefaultIfZero(&v, 10)
	if v != 10 {
		t.Fatalf("SetDefaultIfZero zero = %v, want 10", v)
	}
	v = 5
	SetDefaultIfZero(&v, 10)
	if v != 5 {
		t.Fatalf("SetDefaultIfZero non-zero = %v, want 5", v)
	}
}
