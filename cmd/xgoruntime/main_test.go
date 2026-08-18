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

package main

import (
	"errors"
	"testing"
)

func TestCommandErrorMessageAddsOnePrefix(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
		want string
	}{
		{name: "unprefixed", err: errors.New("failed"), want: "xgoruntime: failed"},
		{name: "prefixed", err: errors.New("xgoruntime: failed"), want: "xgoruntime: failed"},
		{name: "repeated", err: errors.New("xgoruntime: xgoruntime: failed"), want: "xgoruntime: failed"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := commandErrorMessage(test.err); got != test.want {
				t.Fatalf("commandErrorMessage() = %q, want %q", got, test.want)
			}
		})
	}
}
