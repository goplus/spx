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
	"context"
	"os"
	"syscall"
	"testing"
	"time"
)

func TestRunCommandCancelsCleanupBeforeReturningSignal(t *testing.T) {
	cleaned := false
	status, err := RunCommand(context.Background(), func(ctx context.Context) (ProcessStatus, error) {
		defer func() { cleaned = true }()
		if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
			return ProcessStatus{}, err
		}
		select {
		case <-ctx.Done():
			return ProcessStatus{}, ctx.Err()
		case <-time.After(5 * time.Second):
			return ProcessStatus{}, context.DeadlineExceeded
		}
	})
	if err != nil || status.Signal != int(syscall.SIGTERM) {
		t.Fatalf("RunCommand() = (%+v, %v), want SIGTERM", status, err)
	}
	if !cleaned {
		t.Fatal("RunCommand returned before deferred cleanup")
	}
}
