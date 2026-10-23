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

package debug

import (
	"bytes"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/goplus/spx/v4/internal/log"
)

func TestStackInformation(t *testing.T) {
	for _, index := range []int{-1, 0, 1, 100000} {
		stack, simple := GetStackInfo(index)
		if !strings.Contains(stack, "TestStackInformation") || !strings.HasSuffix(stack, "\n") {
			t.Fatalf("invalid stack: %q", stack)
		}
		if index == 1 && !strings.Contains(simple, "GetStackInfo") {
			t.Fatalf("first frame = %q", simple)
		}
		if index != 1 && simple != "" {
			t.Errorf("out-of-range index %d returned %q", index, simple)
		}
	}
	if trace := GetStackTrace(); !strings.Contains(trace, "TestStackInformation") {
		t.Fatalf("trace = %q", trace)
	}
}

func TestBufferedLogsAndStackOutput(t *testing.T) {
	var output bytes.Buffer
	log.SetOutput(&output)
	log.SetLevel(log.LevelDebug)
	debugSb.Reset()
	t.Cleanup(func() {
		debugSb.Reset()
		log.SetLevel(log.LevelInfo)
		log.SetStdoutOutput(os.Stdout)
		log.SetStderrOutput(os.Stderr)
	})
	Log("message", 42)
	if output.Len() != 0 {
		t.Fatal("buffered log was emitted early")
	}
	LogWithStack("stack message")
	FlushLog()
	for _, want := range []string{"message42", "stack message", "TestBufferedLogsAndStackOutput"} {
		if !strings.Contains(output.String(), want) {
			t.Errorf("output missing %q: %q", want, output.String())
		}
	}
	before := output.String()
	FlushLog()
	if output.String() != before || debugSb.Len() != 0 {
		t.Fatal("flush did not consume buffered logs exactly once")
	}
	PrintStackTrace()
	PrintAllStackTrace()
	if got := output.String(); !strings.Contains(got, "Stack trace:") || !strings.Contains(got, "All goroutine stack traces:") {
		t.Fatalf("stack output = %q", got)
	}
}

func TestConcurrentLogBuffer(t *testing.T) {
	debugSb.Reset()
	t.Cleanup(func() { debugSb.Reset() })
	var workers sync.WaitGroup
	for range 8 {
		workers.Go(func() {
			for range 25 {
				Log("entry")
			}
		})
	}
	workers.Wait()
	if got := strings.Count(debugSb.String(), "entry\n"); got != 200 {
		t.Fatalf("buffered entries = %d, want 200", got)
	}
}
