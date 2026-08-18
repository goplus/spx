//go:build !windows

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

package xgolauncher

import (
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Exit is the only launcher command boundary. Signal exits are reproduced as
// signals so a parent observes WaitStatus.Signaled rather than 128+signal.
func Exit(status ProcessStatus) {
	if status.Signal != 0 {
		sig := syscall.Signal(status.Signal)
		signal.Reset(sig)
		if err := syscall.Kill(os.Getpid(), sig); err == nil {
			// Signal delivery is asynchronous with the Go runtime's signal
			// trampoline. Give the restored default disposition a chance to
			// terminate this process before using the numeric fallback.
			time.Sleep(time.Second)
		}
		// A malformed/unavailable signal must still produce failure.
		os.Exit(128 + status.Signal)
	}
	os.Exit(status.Code)
}
