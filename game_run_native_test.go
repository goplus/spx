//go:build !js && !pure_engine

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
	"bytes"
	"os"
	"slices"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	coreruntime "github.com/goplus/spx/v3/internal/core/runtime"
	"github.com/goplus/spx/v3/internal/coroutine"
	"github.com/goplus/spx/v3/internal/engine"
	"github.com/goplus/spx/v3/internal/enginewrap"
	spxlog "github.com/goplus/spx/v3/internal/log"
	itime "github.com/goplus/spx/v3/internal/time"
	pkgengine "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

type schedTimeoutExtMgr struct {
	pkgengine.IExtMgr
	messages []string
	exits    []int64
}

func (m *schedTimeoutExtMgr) OnRuntimePanic(message string) {
	m.messages = append(m.messages, message)
}

func (m *schedTimeoutExtMgr) RequestExit(code int64) {
	m.exits = append(m.exits, code)
}

func TestSchedLoopTimeoutWaitsBeforeReporting(t *testing.T) {
	// Restore real frame timing after the bubble advances it with its fake clock.
	t.Cleanup(func() { itime.Start(nil) })
	synctest.Test(t, func(t *testing.T) {
		var warnings bytes.Buffer
		spxlog.SetStdoutOutput(&warnings)
		recorder := &schedTimeoutExtMgr{}
		originalExt := pkgengine.ExtMgr
		pkgengine.ExtMgr = recorder
		enginewrap.Init(engine.WaitMainThread)
		t.Cleanup(func() {
			pkgengine.ExtMgr = originalExt
			spxlog.SetStdoutOutput(os.Stdout)
		})
		co := setupRuntimeScheduler(t)
		timedOut := make(chan struct{})
		returned := false
		thread := co.Create("loop", func(coroutine.Thread) {
			// Start the frame's timeout clock, then expire it without advancing a frame.
			if got := Sched(); got != 0 {
				t.Errorf("initial Sched() = %d, want 0", got)
			}
			time.Sleep(schedTimeoutMs*time.Millisecond + time.Nanosecond)
			close(timedOut)
			if got := Sched(); got != 0 {
				t.Errorf("timed-out Sched() = %d, want 0", got)
			}
			returned = true
		})
		<-timedOut
		synctest.Wait()

		if got := warnings.String(); !strings.Contains(got, "[WARN] [SPX] "+coreruntime.LoopExecutionTimedOutMsg) {
			t.Fatalf("timeout warning = %q, want loop timeout warning", got)
		}
		// Processing the current frame must leave the timeout suspended.
		co.Update()
		synctest.Wait()
		if returned || len(recorder.messages) != 0 || len(recorder.exits) != 0 || thread.Stopped() {
			t.Fatalf("timeout reported before the next frame: returned=%v, messages=%v, exits=%v, stopped=%v",
				returned, recorder.messages, recorder.exits, thread.Stopped())
		}

		itime.Update(0, 0)
		co.Update()
		synctest.Wait()
		if want := []string{coreruntime.LoopExecutionTimedOutMsg}; !slices.Equal(recorder.messages, want) {
			t.Errorf("runtime panic messages = %v, want %v", recorder.messages, want)
		}
		if want := []int64{1}; !slices.Equal(recorder.exits, want) {
			t.Errorf("runtime exit codes = %v, want %v", recorder.exits, want)
		}
		if !returned || !thread.Stopped() {
			t.Errorf("timeout completion: returned=%v, stopped=%v, want both true", returned, thread.Stopped())
		}
	})
}
