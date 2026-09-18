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
	"fmt"

	"github.com/goplus/spx/v3/internal/coroutine"
	spxlog "github.com/goplus/spx/v3/internal/log"
)

// DeferPanic reports a recovered panic.
func DeferPanic(name, stack string, shouldExit bool) {
	if value := recover(); value != nil {
		handlePanic(name, stack, value, shouldExit)
	}
}

// CheckPanic reports and exits on callback panic.
func CheckPanic() {
	if value := recover(); value != nil {
		handlePanic("", "", value, true)
	}
}

// OnPanic reports a coroutine panic.
func OnPanic(report coroutine.PanicReport) {
	stack := report.Stack
	if report.CreationStack != "" {
		stack += "\ncreated at:\n" + report.CreationStack
	}
	handlePanic(report.Name, stack, report.Value, true)
}

// Panic reports a runtime panic.
func Panic(args ...any) {
	handlePanic(fmt.Sprint(args...), "", nil, true)
}

// Panicf reports a formatted runtime panic.
func Panicf(format string, args ...any) {
	handlePanic(fmt.Sprintf(format, args...), "", nil, true)
}

func handlePanic(name, stack string, value any, shouldExit bool) {
	message := formatPanic(name, stack, value)
	if message != "" {
		spxlog.Error("%s", message)
	}

	Managers().ExtMgr.OnRuntimePanic(message)

	if shouldExit {
		RequestExit(1)
	}
}

func formatPanic(name, stack string, value any) string {
	message := name
	if value != nil {
		cause := fmt.Sprintf("panic: %v", value)
		if message == "" {
			message = cause
		} else {
			message += ": " + cause
		}
	}
	if stack != "" {
		message += "\nstack:\n" + stack
	}
	return message
}
