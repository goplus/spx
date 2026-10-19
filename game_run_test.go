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

package spx

import (
	"testing"
	"time"

	"github.com/goplus/spx/v3/internal/coroutine"
	"github.com/goplus/spx/v3/internal/engine"
	spxlog "github.com/goplus/spx/v3/internal/log"
)

func TestScheduling(t *testing.T) {
	for _, schedule := range []struct {
		name string
		call func() int
	}{
		{"SchedNow", SchedNow},
		{"Sched", Sched},
	} {
		for _, tt := range []struct {
			name    string
			offset  time.Duration
			demoted bool
		}{
			{"active", time.Minute, false},
			{"expired", -2 * mainExecTimeoutSec * time.Second, true},
		} {
			t.Run(schedule.name+"/Main/"+tt.name, func(t *testing.T) {
				co := setupRuntimeScheduler(t)
				returned := false
				thread := co.Create("main", func(thread coroutine.Thread) {
					started := time.Now().Add(tt.offset)
					end := thread.BeginMain(started)
					defer end()

					if got := schedule.call(); got != 0 {
						t.Errorf("%s() = %d, want 0", schedule.name, got)
					}
					want := started
					if tt.demoted {
						want = time.Time{}
					}
					if got := thread.MainStartedAt(); !got.Equal(want) {
						t.Errorf("MainStartedAt = %v, want %v", got, want)
					}
					returned = true
				})
				co.Join(thread)
				if !returned {
					t.Fatal("Main scheduling stopped the coroutine before its assertions completed")
				}
			})
		}

		t.Run(schedule.name+"/ExternalCaller", func(t *testing.T) {
			co := setupRuntimeScheduler(t)
			started := make(chan struct{})
			release := make(chan struct{})
			thread := co.Create("main", func(thread coroutine.Thread) {
				end := thread.BeginMain(time.Now().Add(-2 * mainExecTimeoutSec * time.Second))
				defer end()
				close(started)
				<-release
			})
			defer func() {
				close(release)
				co.Join(thread)
			}()
			select {
			case <-started:
			case <-time.After(time.Second):
				t.Fatal("active coroutine did not start")
			}

			result := make(chan any, 1)
			go func() {
				defer func() { result <- recover() }()
				schedule.call()
			}()
			select {
			case recovered := <-result:
				if recovered != nil {
					t.Fatalf("external %s panicked: %v", schedule.name, recovered)
				}
			case <-time.After(time.Second):
				t.Fatalf("external %s did not return", schedule.name)
			}
			if thread.Stopped() || thread.MainStartedAt().IsZero() {
				t.Fatalf("external %s changed the active coroutine", schedule.name)
			}
		})

		t.Run(schedule.name+"/WithoutManager", func(t *testing.T) {
			original := gco
			gco = nil
			defer func() { gco = original }()
			if got := schedule.call(); got != 0 {
				t.Errorf("%s() = %d, want 0", schedule.name, got)
			}
		})
	}
}

func TestSetDebugFlagsForwardsPerfMode(t *testing.T) {
	co := setupRuntimeScheduler(t)
	var game Game
	for _, flags := range []dbgFlags{
		DbgFlagInstr | DbgFlagEvent | DbgFlagPerf,
		DbgFlagInstr,
		DbgFlagEvent,
		DbgFlagPerf,
		0,
	} {
		game.setDebugFlags(flags)
		co.Update()
		if got, want := co.UpdateStats().GCStatsEnabled, flags&DbgFlagPerf != 0; got != want {
			t.Errorf("flags %v: performance statistics enabled = %v, want %v", flags, got, want)
		}
		if got, want := game.debugState.DebugInstr, flags&DbgFlagInstr != 0; got != want {
			t.Errorf("flags %v: DebugInstr = %v, want %v", flags, got, want)
		}
		if got, want := game.debugState.DebugEvent, flags&DbgFlagEvent != 0; got != want {
			t.Errorf("flags %v: DebugEvent = %v, want %v", flags, got, want)
		}
	}
}

func TestSetDebugUpdatesDefaultAndRunningGame(t *testing.T) {
	co := setupRuntimeScheduler(t)
	originalGame, originalFlags := engine.GetGame(), defaultDebugFlags.Load()
	t.Cleanup(func() {
		engine.SetGame(originalGame)
		defaultDebugFlags.Store(originalFlags)
		spxlog.SetLevel(spxlog.LevelInfo)
	})
	engine.SetGame(nil)
	SetDebug(DbgFlagPerf | dbgFlags(1<<20))
	co.Update()
	if defaultDebugFlags.Load() != uint32(DbgFlagPerf) || !co.UpdateStats().GCStatsEnabled {
		t.Fatal("default debug flags did not filter unknown flags or enable profiling")
	}
	var game Game
	engine.SetGame(&game)
	SetDebug(DbgFlagInstr | DbgFlagEvent)
	if !game.debugState.DebugInstr || !game.debugState.DebugEvent {
		t.Fatalf("game debug=%+v", game.debugState)
	}
}
