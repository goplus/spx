package profiler

import (
	"fmt"
	"runtime/debug"
	stime "time"

	"github.com/goplus/spx/v2/internal/coroutine"
	"github.com/goplus/spx/v2/internal/time"
)

var (
	gco *coroutine.Coroutines
	fps float64 = 0

	debugLastTime  float64 = 0
	debugLastFrame int64   = 0
	prevGCStats    debug.GCStats
	timingData     = make(map[string]TimingInfo)

	// Controls whether to print detailed performance statistics
	printDetailedStats bool
	lastUpdateDuration float64
	totalStart         stime.Time
	Debug              bool
)

func Calcfps() float64 {
	curTime := time.RealTimeSinceStart()
	timeDiff := curTime - debugLastTime
	frameDiff := time.Frame() - debugLastFrame
	if timeDiff > 0.5 {
		fps = float64(frameDiff) / timeDiff
		debugLastFrame = time.Frame()
		debugLastTime = curTime
	}
	return fps
}

// TimingInfo records the execution time and related information of each module
type TimingInfo struct {
	PreCall    float64       // Preparation time before the call
	ActualCall float64       // Actual function execution time
	PostCall   float64       // Cleanup time after the call
	GCStats    debug.GCStats // GC statistics
}

func SetGco(co *coroutine.Coroutines) {
	gco = co
}

func BeginSample() {
	if !Enabled {
		return
	}
	totalStart = stime.Now()
	// Clear the timing data of the previous frame
	timingData = make(map[string]TimingInfo)
}

func EndSample() {
	if !Enabled {
		return
	}
	// Calculate the total time
	total := stime.Since(totalStart).Seconds() * 1000

	// print a brief message
	if total > 20 {
		fmt.Printf("Total time: %.3fms (GameUpdate: %.3fms, CoroUpdateJobs: %.3fms, GameRender: %.3fms)\n",
			total, timingData["GameUpdate"].ActualCall, timingData["CoroUpdateJobs"].ActualCall, timingData["GameRender"].ActualCall)
	}

	// If the coroutine update time exceeds the threshold, or the pre/post processing time is too long, print detailed information
	if printDetailedStats ||
		timingData["CoroUpdateJobs"].ActualCall > 20 ||
		(timingData["CoroUpdateJobs"].PreCall+timingData["CoroUpdateJobs"].PostCall) > 5 ||
		(gco != nil && timingData["CoroUpdateJobs"].ActualCall > 10 &&
			timingData["CoroUpdateJobs"].ActualCall > 2*getCoroStatsTotal()) {
		printTimingInfo()

		// If detailed statistics printing is enabled, print coroutine statistics
		if printDetailedStats && gco != nil {
			coroInfo := TimingInfo{
				PreCall:    timingData["CoroUpdateJobs"].PreCall,
				ActualCall: timingData["CoroUpdateJobs"].ActualCall,
				PostCall:   timingData["CoroUpdateJobs"].PostCall,
				GCStats:    timingData["CoroUpdateJobs"].GCStats,
			}
			printCoroStats(coroInfo)
		}
	}

	lastUpdateDuration = total
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
	debug.ReadGCStats(&prevGCStats)

	// Preparation phase ends
	preCallEnd := stime.Now()

	// Execute the actual function
	actualCallStart := stime.Now()
	fn()
	actualCallEnd := stime.Now()

	// Post-processing phase starts
	postCallStart := stime.Now()

	// Get the GC difference
	gcDiff := getGCDiff()

	// Post-processing phase ends
	postCallEnd := stime.Now()

	// Calculate the time for each phase (milliseconds)
	preCallTime := preCallEnd.Sub(preCallStart).Seconds() * 1000
	actualCallTime := actualCallEnd.Sub(actualCallStart).Seconds() * 1000
	postCallTime := postCallEnd.Sub(postCallStart).Seconds() * 1000

	// Store the timing information
	timingData[name] = TimingInfo{
		PreCall:    preCallTime,
		ActualCall: actualCallTime,
		PostCall:   postCallTime,
		GCStats:    gcDiff,
	}
}

// getCoroStatsTotal returns the total time of internal coroutine statistics
func getCoroStatsTotal() float64 {
	if gco == nil {
		return 0
	}

	stats := gco.GetLastUpdateStats()
	return stats.InitTime + stats.LoopTime + stats.MoveTime
}

// getGCDiff retrieves the difference in GC statistics
func getGCDiff() debug.GCStats {
	var currentStats debug.GCStats
	debug.ReadGCStats(&currentStats)

	diff := debug.GCStats{
		NumGC:          currentStats.NumGC - prevGCStats.NumGC,
		PauseTotal:     currentStats.PauseTotal - prevGCStats.PauseTotal,
		PauseQuantiles: currentStats.PauseQuantiles,
		PauseEnd:       currentStats.PauseEnd,
	}

	// Only copy the latest pause time
	if len(currentStats.Pause) > 0 && len(prevGCStats.Pause) > 0 {
		diff.Pause = make([]stime.Duration, len(currentStats.Pause))
		copy(diff.Pause, currentStats.Pause)
	}

	prevGCStats = currentStats
	return diff
}

// printTimingInfo prints timing information
func printTimingInfo() {
	fmt.Println("========== Engine Module Detailed Timing Information ==========")
	for name, info := range timingData {
		total := info.PreCall + info.ActualCall + info.PostCall
		fmt.Printf("%s: Total %.3fms (Preparation: %.3fms, Execution: %.3fms, Cleanup: %.3fms)\n",
			name, total, info.PreCall, info.ActualCall, info.PostCall)

		if info.GCStats.NumGC > 0 {
			fmt.Printf("  GC: %d times, Total pause: %.3fms\n",
				info.GCStats.NumGC,
				float64(info.GCStats.PauseTotal)/float64(stime.Millisecond))
		}
	}

	// If it's a coroutine update, also print the coroutine's internal detailed statistics
	if info, ok := timingData["CoroUpdateJobs"]; ok {
		printCoroStats(info)
	}

	fmt.Println("====================================")
}

// printCoroStats prints detailed statistics of the coroutine module
func printCoroStats(coroInfo TimingInfo) {
	// If gco is nil, return
	if gco == nil {
		fmt.Println("Coroutine system not initialized")
		return
	}

	// Import coroutine module statistics
	stats := gco.GetLastUpdateStats()

	// Calculate the difference between the engine's measured total time and the coroutine's internal measured time
	coroInfo, ok := timingData["CoroUpdateJobs"]
	if !ok {
		return
	}

	// Calculate the engine's measured total time
	engineMeasuredTotal := coroInfo.PreCall + coroInfo.ActualCall + coroInfo.PostCall

	// The coroutine's internal measured total time may contain two types of data:
	// 1. If there is a TotalTime field, use it as the coroutine's internal measured total time
	// 2. Otherwise, use the sum of the parts
	var coroInternalTotal float64
	var coroInternalParts float64

	if stats.TotalTime > 0 {
		coroInternalTotal = stats.TotalTime
		coroInternalParts = stats.InitTime + stats.LoopTime + stats.MoveTime
	} else {
		coroInternalTotal = stats.InitTime + stats.LoopTime + stats.MoveTime
		coroInternalParts = coroInternalTotal
	}

	// Calculate the difference
	difference := engineMeasuredTotal - coroInternalTotal

	// Calculate the coroutine's internal difference
	coroInternalDifference := 0.0
	if stats.TotalTime > 0 {
		coroInternalDifference = stats.TotalTime - coroInternalParts
	}

	fmt.Println("\n========== Coroutine Module Detailed Statistics ==========")
	fmt.Printf("Engine measured total time: %.3fms (Preparation: %.3fms, Execution: %.3fms, Cleanup: %.3fms Last total time %.3fms)\n",
		engineMeasuredTotal, coroInfo.PreCall, coroInfo.ActualCall, coroInfo.PostCall, lastUpdateDuration)
	fmt.Printf("Coroutine internal measured total time: %.3fms\n", coroInternalTotal)
	fmt.Printf("Time difference: %.3fms (%.2f%%)\n",
		difference, (difference/engineMeasuredTotal)*100)

	// If there is an internal difference, display it
	if stats.TotalTime > 0 && coroInternalDifference > 0.1 {
		fmt.Printf("Coroutine internal difference: %.3fms (%.2f%%)\n",
			coroInternalDifference, (coroInternalDifference/stats.TotalTime)*100)
	}

	fmt.Println("\nCoroutine internal detailed time distribution:")
	fmt.Printf("  Initialization: %.3fms (%.2f%%)\n",
		stats.InitTime, (stats.InitTime/coroInternalTotal)*100)
	fmt.Printf("  Main loop: %.3fms (%.2f%%)\n",
		stats.LoopTime, (stats.LoopTime/coroInternalTotal)*100)

	if stats.LoopIterations > 0 {
		fmt.Printf("    - Loop iterations: %d\n", stats.LoopIterations)
	}

	fmt.Printf("    - Task processing: %.3fms (Task count: %d)\n",
		stats.TaskProcessing, stats.TaskCounts)
	fmt.Printf("    - Wait time: %.3fms\n", stats.WaitTime)
	fmt.Printf("  Queue movement: %.3fms (%.2f%%, Next frame tasks: %d)\n",
		stats.MoveTime, (stats.MoveTime/coroInternalTotal)*100, stats.NextCount)

	if stats.ExternalTime > 0 {
		fmt.Printf("  External time: %.3fms (%.2f%%)\n",
			stats.ExternalTime, (stats.ExternalTime/coroInternalTotal)*100)
	}

	if stats.GCCount > 0 {
		fmt.Printf("  GC: %d times, Pause: %.3fms\n", stats.GCCount, stats.GCPauses)
	}

	// Analyze possible reasons for the difference
	if difference > 5 { // If the difference is greater than 5 milliseconds
		fmt.Println("\nPossible reasons for the performance difference:")

		// Check GC
		if coroInfo.GCStats.NumGC > 0 {
			fmt.Printf("  - Garbage collection: %d GCs occurred during the engine's measurement, total pause time %.3fms\n",
				coroInfo.GCStats.NumGC,
				float64(coroInfo.GCStats.PauseTotal)/float64(stime.Millisecond))
		}

		// Check coroutine internal GC
		if stats.GCCount > 0 {
			fmt.Printf("  - Coroutine internal GC: %d GCs occurred during the coroutine's measurement, total pause time %.3fms\n",
				stats.GCCount, stats.GCPauses)
		}

		// Check function call overhead
		if coroInfo.PreCall > 1 || coroInfo.PostCall > 1 {
			fmt.Printf("  - Function call overhead: Preparation phase %.3fms, Cleanup phase %.3fms\n",
				coroInfo.PreCall, coroInfo.PostCall)
		}

		// Check loop iterations
		if stats.LoopIterations > 100 {
			fmt.Printf("  - Loop iterations are too many: %d times\n", stats.LoopIterations)
		}

		// Check if there are unaccounted parts in the coroutine
		unaccountedTime := stats.LoopTime - (stats.TaskProcessing + stats.WaitTime)
		if unaccountedTime > 5 {
			fmt.Printf("  - There are unaccounted parts in the coroutine: approximately %.3fms\n", unaccountedTime)
		}

		// Check external time
		if stats.ExternalTime > 5 {
			fmt.Printf("  - External time is too long: %.3fms\n", stats.ExternalTime)
		}

		// Check Go runtime scheduling overhead
		fmt.Println("  - Go runtime scheduling: Possible goroutine scheduling delay")
		fmt.Println("  - External factors: Possible competition from other programs or system resources")
	}
}
