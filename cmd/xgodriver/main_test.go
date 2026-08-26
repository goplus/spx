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
	"context"
	"errors"
	"testing"

	"github.com/goplus/spx/v3/internal/xgodriver"
	"github.com/goplus/spx/v3/x/xgolauncher"
)

func TestCommandErrorMessageAddsOnePrefix(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
		want string
	}{
		{name: "unprefixed", err: errors.New("failed"), want: "xgodriver: failed"},
		{name: "prefixed", err: errors.New("xgodriver: failed"), want: "xgodriver: failed"},
		{name: "repeated", err: errors.New("xgodriver: xgodriver: failed"), want: "xgodriver: failed"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := commandErrorMessage(test.err); got != test.want {
				t.Fatalf("commandErrorMessage() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestCommandErrorStatusClassifiesRequestAndExecutionErrors(t *testing.T) {
	_, parseErr := xgodriver.Parse(nil)
	_, validationErr := xgodriver.Execute(context.Background(), xgodriver.Config{}, xgodriver.IO{})
	for _, test := range []struct {
		name   string
		status xgolauncher.ProcessStatus
		err    error
		want   xgolauncher.ProcessStatus
	}{
		{name: "protocol request", err: parseErr, want: xgolauncher.ProcessStatus{Code: 2}},
		{name: "live request", err: validationErr, want: xgolauncher.ProcessStatus{Code: 2}},
		{name: "execution", err: errors.New("build failed"), want: xgolauncher.ProcessStatus{Code: 1}},
		{name: "specific child status", status: xgolauncher.ProcessStatus{Code: 17}, err: errors.New("child failed"), want: xgolauncher.ProcessStatus{Code: 17}},
		{name: "request error with child status", status: xgolauncher.ProcessStatus{Code: 17}, err: parseErr, want: xgolauncher.ProcessStatus{Code: 17}},
		{name: "request error with child signal", status: xgolauncher.ProcessStatus{Signal: 15}, err: parseErr, want: xgolauncher.ProcessStatus{Signal: 15}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := commandErrorStatus(test.status, test.err); got != test.want {
				t.Fatalf("commandErrorStatus() = %+v, want %+v", got, test.want)
			}
		})
	}
}
