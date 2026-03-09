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

package input

import (
	"time"

	engine "github.com/goplus/spx/v2/pkg/spx/pkg/engine"
)

// ClickGate suppresses repeated click handling within a minimum interval.
type ClickGate struct {
	minIntervalMs int64
	lastClickMs   map[engine.Object]int64
	now           func() time.Time
}

func (g *ClickGate) Init(minInterval time.Duration) {
	g.minIntervalMs = int64(minInterval / time.Millisecond)
	g.lastClickMs = make(map[engine.Object]int64)
	if g.now == nil {
		g.now = time.Now
	}
}

func (g *ClickGate) Allow(id engine.Object) bool {
	currentTimeMs := g.nowMs()
	if lastTimeMs, ok := g.lastClickMs[id]; ok {
		if currentTimeMs-lastTimeMs < g.minIntervalMs {
			return false
		}
	}
	g.lastClickMs[id] = currentTimeMs
	return true
}

func (g *ClickGate) nowMs() int64 {
	if g.now == nil {
		g.now = time.Now
	}
	return g.now().UnixNano() / int64(time.Millisecond)
}
