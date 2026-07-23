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

// Package debug provides debugging utilities for stack traces and logging.
package debug

import (
	"fmt"
	"runtime"
	"strings"
	"sync"

	"github.com/goplus/spx/v3/internal/log"
)

const (
	defaultStackBufSize = 4096
	largeStackBufSize   = 1 << 20 // 1MB for all goroutines
)

var (
	debugSb  strings.Builder
	logMutex sync.Mutex

	// Buffer pools for reducing allocations.
	smallBufPool = sync.Pool{
		New: func() any {
			buf := make([]byte, defaultStackBufSize)
			return &buf
		},
	}

	largeBufPool = sync.Pool{
		New: func() any {
			buf := make([]byte, largeStackBufSize)
			return &buf
		},
	}
)

// getSmallBuffer retrieves a buffer from the pool
func getSmallBuffer() *[]byte {
	return smallBufPool.Get().(*[]byte)
}

// putSmallBuffer returns a buffer to the pool
func putSmallBuffer(buf *[]byte) {
	smallBufPool.Put(buf)
}

// getLargeBuffer retrieves a large buffer from the pool
func getLargeBuffer() *[]byte {
	return largeBufPool.Get().(*[]byte)
}

// putLargeBuffer returns a large buffer to the pool
func putLargeBuffer(buf *[]byte) {
	largeBufPool.Put(buf)
}

// GetStackInfo returns the full stack trace and a simplified version.
// lastStackIdx specifies which stack frame to include in the simplified version.
func GetStackInfo(lastStackIdx int) (stack, stackSimple string) {
	bufPtr := getSmallBuffer()
	defer putSmallBuffer(bufPtr)

	buf := *bufPtr
	n := runtime.Stack(buf, false)
	stack = string(buf[:n]) + "\n"

	// Extract simplified stack info
	lines := strings.Split(stack, "\n")
	if lastStackIdx*2 <= len(lines) && lastStackIdx > 0 {
		stackSimple = lines[lastStackIdx*2-1] + " " + lines[lastStackIdx*2]
	}
	return
}

// Log accumulates debug messages in a buffer for later output.
// Messages are not immediately printed but stored until FlushLog is called.
func Log(args ...any) {
	logMutex.Lock()
	defer logMutex.Unlock()
	debugSb.WriteString(fmt.Sprint(args...))
	debugSb.WriteString("\n")
}

// LogWithStack logs a message along with the current stack trace.
func LogWithStack(args ...any) {
	Log(args...)
	logStackTrace()
}

// logStackTrace appends the current stack trace to the debug buffer.
func logStackTrace() {
	bufPtr := getSmallBuffer()
	defer putSmallBuffer(bufPtr)

	buf := *bufPtr
	n := runtime.Stack(buf, false)
	debugSb.WriteString("\n")
	debugSb.WriteString(string(buf[:n]))
	debugSb.WriteString("\n")
}

// GetStackTrace returns the current stack trace as a string.
func GetStackTrace() string {
	bufPtr := getSmallBuffer()
	defer putSmallBuffer(bufPtr)

	buf := *bufPtr
	stackSize := runtime.Stack(buf, false)
	return string(buf[:stackSize]) + "\n"
}

// PrintStackTrace prints the current goroutine's stack trace.
func PrintStackTrace() {
	bufPtr := getSmallBuffer()
	defer putSmallBuffer(bufPtr)

	buf := *bufPtr
	stackSize := runtime.Stack(buf, false)
	log.Debug("Stack trace:\n%s", string(buf[:stackSize]))
}

// PrintAllStackTrace prints stack traces for all goroutines.
func PrintAllStackTrace() {
	bufPtr := getLargeBuffer()
	defer putLargeBuffer(bufPtr)

	buf := *bufPtr
	stackSize := runtime.Stack(buf, true)
	log.Debug("All goroutine stack traces:\n%s", string(buf[:stackSize]))
}

// FlushLog outputs all accumulated debug messages and clears the buffer.
func FlushLog() {
	logMutex.Lock()
	defer logMutex.Unlock()

	logs := debugSb.String()
	if logs != "" {
		log.Debug("Buffered debug logs:\n%s", logs)
		debugSb.Reset()
	}
}
