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
	"os/signal"
	"sync"
	"syscall"

	"github.com/goplus/spx/v3/internal/processsupervisor"
)

func runCommand(parent context.Context, run func(context.Context) (ProcessStatus, error)) (ProcessStatus, error) {
	ctx, cancel := context.WithCancelCause(parent)
	defer cancel(nil)
	signals := make(chan os.Signal, 8)
	done := make(chan struct{})
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT)
	var (
		mu       sync.Mutex
		received syscall.Signal
		wait     sync.WaitGroup
	)
	wait.Add(1)
	go func() {
		defer wait.Done()
		for {
			select {
			case current := <-signals:
				unixSignal, ok := current.(syscall.Signal)
				if !ok {
					continue
				}
				mu.Lock()
				if received == 0 {
					received = unixSignal
					cancel(&processsupervisor.SignalCause{Signal: unixSignal})
				}
				mu.Unlock()
			case <-done:
				return
			}
		}
	}()
	status, err := run(ctx)
	signal.Stop(signals)
	close(done)
	wait.Wait()
	mu.Lock()
	original := received
	mu.Unlock()
	if original != 0 {
		return ProcessStatus{Signal: int(original)}, nil
	}
	return status, err
}
