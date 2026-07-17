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

package coroutine

// UpdateJobsStats reports timings and job counts for one Update call. Durations
// are expressed in milliseconds.
type UpdateJobsStats struct {
	InitTime       float64 // Time spent preparing the update.
	LoopTime       float64 // Time spent in the update loop.
	MoveTime       float64 // Time spent promoting deferred jobs.
	WaitTime       float64 // Time spent waiting for runnable work.
	TaskProcessing float64 // Time spent processing jobs.
	GCPauses       float64 // GC pause time observed during the update.
	ExternalTime   float64 // Unaccounted time, including runtime scheduling overhead.
	TotalTime      float64 // Total update time.
	TimeDifference float64 // Difference between total and accounted time.
	TaskCounts     int     // Number of jobs processed.
	WaitFrameCount int     // Number of frame-wait jobs resumed.
	WaitMainCount  int     // Number of main-thread jobs executed.
	NextCount      int     // Number of jobs deferred at the end of the update loop.
	GCCount        int     // Number of GC cycles observed during the update.
	GCStatsEnabled bool    // Whether GCCount and GCPauses were collected.
	LoopIterations int     // Number of update-loop iterations.
}

// GetLastUpdateStats returns the package-wide statistics from the most recent
// Update call. GCCount and GCPauses are meaningful only when GCStatsEnabled is
// true.
func (*Coroutines) GetLastUpdateStats() UpdateJobsStats {
	return lastUpdateStats
}

var lastUpdateStats UpdateJobsStats
