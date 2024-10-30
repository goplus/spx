/*
 * Copyright (c) 2021 The GoPlus Authors (goplus.org). All rights reserved.
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
	"fmt"
	"sync"

	"github.com/goplus/spx/internal/timeline"
)

type TimelineMgr struct {
	timelines sync.Map // map: ID => ITimeline
}

func (p *TimelineMgr) AddTimeline(t timeline.ITimeline) {
	if t != nil {
		p.timelines.Store(t.GetTimeline().ID, t)
	}
}

func (p *TimelineMgr) Update(deltaTime float64) {
	timelines := &p.timelines
	remove := []int64{}
	timelines.Range(func(id, v interface{}) bool {
		if it, ok := v.(timeline.ITimeline); ok {
			t := deltaTime
			running := it.Step(&t)
			if running == nil {
				remove = append(remove, it.GetTimeline().ID)
			}
		}
		return true
	})
	for _, id := range remove {
		fmt.Printf("TimelineMgr.Update del %X\n", id)
		timelines.Delete(id)
	}
}
