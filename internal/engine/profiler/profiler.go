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
	"runtime/debug"
	stime "time"

	"github.com/goplus/spx/v4/internal/coroutine"
	spxlog "github.com/goplus/spx/v4/internal/log"
	"github.com/goplus/spx/v4/internal/time"
)

var (
	gco *coroutine.Coroutines
	fps float64 = 0

	debugLastTime  float64 = 0
	debugLastFrame int64   = 0
	gcStats        debug.GCStats
	timingData     = make(map[string]TimingInfo)

	lastUpdateDuration float64
	totalStart         stime.Time
	Debug              bool
	Enabled            bool
)

// TimingInfo records the execution time and related information of each module
type TimingInfo struct {
	PreCall     float64 // Preparation time before the call
	ActualCall  float64 // Actual function execution time
	PostCall    float64 // Cleanup time after the call
	GCCount     int64
	GCPauseTime stime.Duration
}

func Calcfps() float64 {
	curTime := time.RealTimeSinceStart()
	curFrame := time.Frame()
	if curFrame < debugLastFrame {
		// A new engine time session restarts the public frame counter. Rebase
		// profiling so the first interval cannot report negative FPS.
		debugLastFrame = curFrame
		debugLastTime = curTime
	}
	timeDiff := curTime - debugLastTime
	frameDiff := curFrame - debugLastFrame
	if timeDiff > 0.5 {
		fps = float64(frameDiff) / timeDiff
		debugLastFrame = curFrame
		debugLastTime = curTime
	}
	return fps
}

func SetGco(co *coroutine.Coroutines) {
	gco = co
}

func BeginSample() {
	if !Enabled {
		return
	}
	spxlog.Info("========== Begin profiling sample: Unnamed ==========")

	totalStart = stime.Now()
	// Clear the timing data of the previous frame
	clear(timingData)
}

func EndSample() {
	if !Enabled {
		return
	}
	// Calculate the total time
	total := stime.Since(totalStart).Seconds() * 1000

	// print a brief message
	if total > 20 {
		spxlog.Info("Total time: %.3fms (GameUpdate: %.3fms, CoroUpdateJobs: %.3fms, GameRender: %.3fms)",
			total, timingData["GameUpdate"].ActualCall, timingData["CoroUpdateJobs"].ActualCall, timingData["GameRender"].ActualCall)
	}

	// If the coroutine update time exceeds the threshold, or the pre/post processing time is too long, print detailed information
	if timingData["CoroUpdateJobs"].ActualCall > 20 ||
		(timingData["CoroUpdateJobs"].PreCall+timingData["CoroUpdateJobs"].PostCall) > 5 ||
		(gco != nil && timingData["CoroUpdateJobs"].ActualCall > 10 &&
			timingData["CoroUpdateJobs"].ActualCall > 2*getCoroStatsTotal()) {
		printTimingInfo()
	}

	lastUpdateDuration = total

	spxlog.Info("========== End profiling sample: Unnamed ==========")
}

func GetStats(name string) (TimingInfo, bool) {
	info, ok := timingData[name]
	return info, ok
}

// MeasureFunctionTime measures function execution time, including preparation and cleanup
func MeasureFunctionTime(name string, fn func()) {
	if !Enabled && !Debug {
		fn()
		return
	}
	// Record the time before the call
	preCallStart := stime.Now()

	// Get the GC statistics before the call
	debug.ReadGCStats(&gcStats)
	previousGCCount, previousGCPause := gcStats.NumGC, gcStats.PauseTotal

	// Preparation phase ends
	preCallEnd := stime.Now()

	// Execute the actual function
	actualCallStart := stime.Now()
	fn()
	actualCallEnd := stime.Now()

	// Post-processing phase starts
	postCallStart := stime.Now()

	// Get the GC difference
	debug.ReadGCStats(&gcStats)
	gcCount := gcStats.NumGC - previousGCCount
	gcPause := gcStats.PauseTotal - previousGCPause

	// Post-processing phase ends
	postCallEnd := stime.Now()

	// Calculate the time for each phase (milliseconds)
	preCallTime := preCallEnd.Sub(preCallStart).Seconds() * 1000
	actualCallTime := actualCallEnd.Sub(actualCallStart).Seconds() * 1000
	postCallTime := postCallEnd.Sub(postCallStart).Seconds() * 1000

	// Store the timing information
	timingData[name] = TimingInfo{
		PreCall:     preCallTime,
		ActualCall:  actualCallTime,
		PostCall:    postCallTime,
		GCCount:     gcCount,
		GCPauseTime: gcPause,
	}
}

// getCoroStatsTotal returns the total time of internal coroutine statistics
func getCoroStatsTotal() float64 {
	if gco == nil {
		return 0
	}

	stats := gco.UpdateStats()
	return stats.InitTime + stats.LoopTime + stats.MoveTime
}

// printTimingInfo prints timing information
func printTimingInfo() {
	spxlog.Info("========== Engine Module Detailed Timing Information ==========")
	for name, info := range timingData {
		total := info.PreCall + info.ActualCall + info.PostCall
		spxlog.Info("%s: Total %.3fms (Preparation: %.3fms, Execution: %.3fms, Cleanup: %.3fms)",
			name, total, info.PreCall, info.ActualCall, info.PostCall)

		if info.GCCount > 0 {
			spxlog.Info("  GC: %d times, Total pause: %.3fms",
				info.GCCount,
				float64(info.GCPauseTime)/float64(stime.Millisecond))
		}
	}

	// If it's a coroutine update, also print the coroutine's internal detailed statistics
	if info, ok := timingData["CoroUpdateJobs"]; ok {
		printCoroStats(info)
	}

	spxlog.Info("====================================")
}

// printCoroStats prints detailed statistics of the coroutine module
func printCoroStats(coroInfo TimingInfo) {
	// If gco is nil, return
	if gco == nil {
		spxlog.Warn("Coroutine system not initialized")
		return
	}

	// Import coroutine module statistics
	stats := gco.UpdateStats()

	// Calculate the engine's measured total time
	engineMeasuredTotal := coroInfo.PreCall + coroInfo.ActualCall + coroInfo.PostCall

	// Calculate the difference
	difference := engineMeasuredTotal - stats.TotalTime

	spxlog.Info("========== Coroutine Module Detailed Statistics ==========")
	spxlog.Info("Engine measured total time: %.3fms (Preparation: %.3fms, Execution: %.3fms, Cleanup: %.3fms Last total time %.3fms)",
		engineMeasuredTotal, coroInfo.PreCall, coroInfo.ActualCall, coroInfo.PostCall, lastUpdateDuration)
	spxlog.Info("Coroutine internal measured total time: %.3fms", stats.TotalTime)
	spxlog.Info("Time difference: %.3fms (%.2f%%)",
		difference, (difference/engineMeasuredTotal)*100)

	spxlog.Info("Coroutine internal detailed time distribution:")
	spxlog.Info("  Initialization: %.3fms (%.2f%%)",
		stats.InitTime, (stats.InitTime/stats.TotalTime)*100)
	spxlog.Info("  Main loop: %.3fms (%.2f%%)",
		stats.LoopTime, (stats.LoopTime/stats.TotalTime)*100)

	if stats.LoopIterations > 0 {
		spxlog.Info("    - Loop iterations: %d", stats.LoopIterations)
	}

	spxlog.Info("    - Task processing: %.3fms (Task count: %d)",
		stats.JobTime, stats.JobCount)
	spxlog.Info("    - Wait time: %.3fms", stats.WaitTime)
	spxlog.Info("  Queue movement: %.3fms (%.2f%%, Next frame tasks: %d)",
		stats.MoveTime, (stats.MoveTime/stats.TotalTime)*100, stats.DeferredCount)

	if stats.ExternalTime > 0 {
		spxlog.Info("  External time: %.3fms (%.2f%%)",
			stats.ExternalTime, (stats.ExternalTime/stats.TotalTime)*100)
	}

	if stats.GCCount > 0 {
		spxlog.Info("  GC: %d times, Pause: %.3fms", stats.GCCount, stats.GCPauseTime)
	}

	// Analyze possible reasons for the difference
	if difference > 5 { // If the difference is greater than 5 milliseconds
		spxlog.Info("Possible reasons for the performance difference:")

		// Check GC
		if coroInfo.GCCount > 0 {
			spxlog.Info("  - Garbage collection: %d GCs occurred during the engine's measurement, total pause time %.3fms",
				coroInfo.GCCount,
				float64(coroInfo.GCPauseTime)/float64(stime.Millisecond))
		}

		// Check coroutine internal GC
		if stats.GCCount > 0 {
			spxlog.Info("  - Coroutine internal GC: %d GCs occurred during the coroutine's measurement, total pause time %.3fms",
				stats.GCCount, stats.GCPauseTime)
		}

		// Check function call overhead
		if coroInfo.PreCall > 1 || coroInfo.PostCall > 1 {
			spxlog.Info("  - Function call overhead: Preparation phase %.3fms, Cleanup phase %.3fms",
				coroInfo.PreCall, coroInfo.PostCall)
		}

		// Check loop iterations
		if stats.LoopIterations > 100 {
			spxlog.Info("  - Loop iterations are too many: %d times", stats.LoopIterations)
		}

		// Check if there are unaccounted parts in the coroutine
		unaccountedTime := stats.LoopTime - (stats.JobTime + stats.WaitTime)
		if unaccountedTime > 5 {
			spxlog.Info("  - There are unaccounted parts in the coroutine: approximately %.3fms", unaccountedTime)
		}

		// Check external time
		if stats.ExternalTime > 5 {
			spxlog.Info("  - External time is too long: %.3fms", stats.ExternalTime)
		}

		// Check Go runtime scheduling overhead
		spxlog.Info("  - Go runtime scheduling: Possible goroutine scheduling delay")
		spxlog.Info("  - External factors: Possible competition from other programs or system resources")
	}
}
