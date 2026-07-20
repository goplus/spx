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

package spx

import (
	"sync"

	itime "github.com/goplus/spx/v2/internal/time"
)

// inputSessionEnvironment owns deterministic process settings for one Game
// lifecycle. Rendering pace remains a project/host concern and is not changed.
type inputSessionEnvironment struct {
	stopOnce sync.Once

	previousFixed        float64
	previousFixedEnabled bool
}

func (e *inputSessionEnvironment) start(fixedTimestep float64) {
	e.previousFixed, e.previousFixedEnabled = itime.FixedDeltaTime()
	itime.SetFixedDeltaTime(fixedTimestep)
}

func (e *inputSessionEnvironment) stop() {
	e.stopOnce.Do(func() {
		if e.previousFixedEnabled {
			itime.SetFixedDeltaTime(e.previousFixed)
		} else {
			itime.SetFixedDeltaTime(0)
		}
	})
}
