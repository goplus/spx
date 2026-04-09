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

// UpdateJobsStats stores detailed statistics of coroutine updates
type UpdateJobsStats struct {
	InitTime       float64 // Initialization time
	LoopTime       float64 // Main loop time
	MoveTime       float64 // Queue move time
	WaitTime       float64 // Wait time
	TaskProcessing float64 // Task processing time
	GCPauses       float64 // GC pause time
	ExternalTime   float64 // External time (may include scheduling overhead)
	TotalTime      float64 // Total time
	TimeDifference float64 // Time difference
	TaskCounts     int     // Number of tasks processed
	WaitFrameCount int     // Number of frames waited
	WaitMainCount  int     // Number of times waiting for the main thread
	NextCount      int     // Number of next frame queue
	GCCount        int     // Number of GC occurrences
	GCStatsEnabled bool    // Whether GCCount and GCPauses were collected this update
	LoopIterations int     // Number of loop iterations
}

// Global variable storing the most recent update statistics
var lastDebugUpdateStats UpdateJobsStats

// GetLastUpdateStats returns the most recent update statistics.
// GCCount and GCPauses are meaningful only when GCStatsEnabled is true.
func (p *Coroutines) GetLastUpdateStats() UpdateJobsStats {
	return lastDebugUpdateStats
}
