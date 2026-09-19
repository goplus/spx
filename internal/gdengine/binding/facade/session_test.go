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

package facade

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestLinkSessionOwnsRunAndUnlink(t *testing.T) {
	var firstRuns, firstUnlinks, secondRuns, secondUnlinks atomic.Int32
	first := newLinkSession(func(ready func()) {
		firstRuns.Add(1)
		ready()
	}, func() { firstUnlinks.Add(1) })
	second := newLinkSession(func(ready func()) {
		secondRuns.Add(1)
		ready()
	}, func() { secondUnlinks.Add(1) })

	var readyCalls atomic.Int32
	first.Run(func() { readyCalls.Add(1) })
	first.Unlink()
	first.Unlink()
	first.Run(func() { readyCalls.Add(1) })
	second.Run(func() { readyCalls.Add(1) })

	if got := firstRuns.Load(); got != 1 {
		t.Fatalf("first runs = %d, want 1", got)
	}
	if got := firstUnlinks.Load(); got != 1 {
		t.Fatalf("first unlinks = %d, want 1", got)
	}
	if got := secondRuns.Load(); got != 1 {
		t.Fatalf("second runs = %d, want 1", got)
	}
	if got := secondUnlinks.Load(); got != 0 {
		t.Fatalf("first session unlinked its successor: calls = %d", got)
	}
	if got := readyCalls.Load(); got != 3 {
		t.Fatalf("ready calls = %d, want 3", got)
	}
}

func TestLinkSessionUnlinkWaitsForStartupCapture(t *testing.T) {
	startupEntered := make(chan struct{})
	allowCapture := make(chan struct{})
	unlinked := make(chan struct{})
	session := newLinkSession(func(ready func()) {
		close(startupEntered)
		<-allowCapture
		ready()
	}, func() { close(unlinked) })

	runDone := make(chan struct{})
	go func() {
		session.Run(nil)
		close(runDone)
	}()
	<-startupEntered
	unlinkDone := make(chan struct{})
	go func() {
		session.Unlink()
		close(unlinkDone)
	}()
	select {
	case <-unlinkDone:
		t.Fatal("unlink returned before startup captured its backend")
	case <-time.After(25 * time.Millisecond):
	}

	close(allowCapture)
	select {
	case <-runDone:
	case <-time.After(time.Second):
		t.Fatal("run did not finish after startup capture")
	}
	select {
	case <-unlinkDone:
	case <-time.After(time.Second):
		t.Fatal("unlink did not finish after startup capture")
	}
	select {
	case <-unlinked:
	default:
		t.Fatal("backend was not unlinked")
	}
}
