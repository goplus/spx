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

// Package xgolauncher verifies and runs a self-contained SPX payload offline.
package xgolauncher

import (
	"context"
	"io"
)

// ProcessStatus preserves a process exit code or host signal.
type ProcessStatus struct {
	Code   int
	Signal int
}

// Success reports whether the process exited normally with status zero.
func (s ProcessStatus) Success() bool { return s.Signal == 0 && s.Code == 0 }

// Config describes one embedded launcher invocation.
type Config struct {
	// Payload must remain unchanged while Run is active.
	Payload        []byte
	PayloadSHA256  string
	ManifestSHA256 string
	Args           []string
	Stdin          io.Reader
	Stdout         io.Writer
	Stderr         io.Writer
}

// Run verifies, materializes, and executes an embedded payload.
func Run(cfg Config) (ProcessStatus, error) {
	return RunContext(context.Background(), cfg)
}

// RunContext is Run with caller-controlled cancellation.
func RunContext(ctx context.Context, cfg Config) (ProcessStatus, error) {
	return run(ctx, cfg, "")
}
