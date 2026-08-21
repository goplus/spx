//go:build windows

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
)

func runCommand(parent context.Context, run func(context.Context) (ProcessStatus, error)) (ProcessStatus, error) {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)
	interrupted := make(chan struct{}, 1)
	done := make(chan struct{})
	var wait sync.WaitGroup
	wait.Add(1)
	go func() {
		defer wait.Done()
		select {
		case <-signals:
			interrupted <- struct{}{}
			cancel()
		case <-done:
		}
	}()
	status, err := run(ctx)
	signal.Stop(signals)
	close(done)
	wait.Wait()
	select {
	case <-interrupted:
		return ProcessStatus{Code: 130}, nil
	default:
		return status, err
	}
}
