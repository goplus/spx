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

// Exit preserves signal termination at the launcher boundary.
func Exit(status ProcessStatus) {
	if status.Signal != 0 {
		sig := syscall.Signal(status.Signal)
		signal.Reset(sig)
		if err := syscall.Kill(os.Getpid(), sig); err == nil {
			// Let the restored disposition terminate before fallback.
			time.Sleep(time.Second)
		}
		os.Exit(128 + status.Signal)
	}
	os.Exit(status.Code)
}
