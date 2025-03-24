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
	LoopIterations int     // Number of loop iterations
}

// Global variable storing the most recent update statistics
var lastDebugUpdateStats UpdateJobsStats

// GetLastUpdateStats returns the most recent update statistics
func (p *Coroutines) GetLastUpdateStats() UpdateJobsStats {
	return lastDebugUpdateStats
}
