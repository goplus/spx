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

package profiler

import (
	"bytes"
	"fmt"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/goplus/spx/v3/internal/coroutine"
	spxlog "github.com/goplus/spx/v3/internal/log"
	itime "github.com/goplus/spx/v3/internal/time"
)

func TestEndSampleSkipsMissingCoroutineStats(t *testing.T) {
	output := captureProfile(t)

	BeginSample("no coroutine update")
	EndSample("no coroutine update")

	if !strings.Contains(output.String(), "End profiling sample: no coroutine update") {
		t.Fatalf("profiling output was not captured: %s", output)
	}
	if strings.Contains(output.String(), "Coroutine Module Detailed Statistics") {
		t.Fatalf("printed missing coroutine statistics: %s", output)
	}
}

func TestEndSampleReportsCoroutineStats(t *testing.T) {
	for _, updated := range []bool{false, true} {
		t.Run(fmt.Sprintf("updated=%v", updated), func(t *testing.T) {
			output := captureProfile(t)
			printDetailedStats = updated
			if updated {
				gco.SetPerfDebug(true)
				thread := gco.Create("profiled", func(me coroutine.Thread) {
					gco.WaitYield(me)
					runtime.GC()
				})
				gco.JoinFirstSlice(thread)
				gco.Update()
				gco.Join(thread)
			}
			stats := gco.UpdateStats()
			if updated && (stats.JobCount != 1 || stats.GCCount == 0) {
				t.Fatalf("update did not record its job and GC: %+v", stats)
			}
			BeginSample("coroutine update")
			timingData["CoroUpdateJobs"] = TimingInfo{ActualCall: stats.TotalTime + 11}
			EndSample("coroutine update")

			if count := strings.Count(output.String(), "Coroutine Module Detailed Statistics"); count != 1 {
				t.Errorf("coroutine statistics printed %d times, want 1: %s", count, output)
			}
			for _, want := range []string{
				fmt.Sprintf("Coroutine internal measured total time: %.3fms", stats.TotalTime),
				fmt.Sprintf("Task processing: %.3fms (Task count: %d)", stats.JobTime, stats.JobCount),
				fmt.Sprintf("Next frame tasks: %d", stats.DeferredCount),
				"Possible reasons for the performance difference:",
			} {
				if !strings.Contains(output.String(), want) {
					t.Errorf("missing %q in profile: %s", want, output)
				}
			}
			wantExternalReports := 0
			if stats.ExternalTime > 0 {
				wantExternalReports = 1
				want := fmt.Sprintf("External time: %.3fms (%.2f%%)", stats.ExternalTime, stats.ExternalTime/stats.TotalTime*100)
				if !strings.Contains(output.String(), want) {
					t.Errorf("missing %q in profile: %s", want, output)
				}
			}
			if count := strings.Count(output.String(), "External time:") + strings.Count(output.String(), "Coroutine internal difference:"); count != wantExternalReports {
				t.Errorf("external time printed %d times, want %d: %s", count, wantExternalReports, output)
			}
			if updated {
				for _, want := range []string{
					fmt.Sprintf("GC: %d times, Pause: %.3fms", stats.GCCount, stats.GCPauseTime),
					fmt.Sprintf("Coroutine internal GC: %d GCs", stats.GCCount),
				} {
					if !strings.Contains(output.String(), want) {
						t.Errorf("missing %q in profile: %s", want, output)
					}
				}
			}
		})
	}
}

func TestCalcfpsRebasesAfterEngineTimeRestart(t *testing.T) {
	itime.Start(nil)
	debugLastFrame = 100
	debugLastTime = -1
	fps = 30

	itime.Start(nil)

	if got := Calcfps(); got < 0 {
		t.Fatalf("Calcfps() after engine time restart = %v, want non-negative", got)
	}
	if debugLastFrame != 0 {
		t.Fatalf("debugLastFrame after reset = %d, want 0", debugLastFrame)
	}
}

func captureProfile(t *testing.T) *bytes.Buffer {
	t.Helper()
	previousCo, previousEnabled, previousDetailed := gco, Enabled, printDetailedStats
	previousData, previousStart, previousDuration := timingData, totalStart, lastUpdateDuration
	t.Cleanup(func() {
		gco, Enabled, printDetailedStats = previousCo, previousEnabled, previousDetailed
		timingData, totalStart, lastUpdateDuration = previousData, previousStart, previousDuration
		spxlog.SetStdoutOutput(os.Stdout)
	})
	output := new(bytes.Buffer)
	spxlog.SetStdoutOutput(output)
	gco, Enabled, printDetailedStats = coroutine.New(nil), true, true
	timingData = make(map[string]TimingInfo)
	return output
}
