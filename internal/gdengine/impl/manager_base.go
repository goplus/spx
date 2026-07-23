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

package impl

//lint:file-ignore ST1001 Godot manager glue intentionally dot-imports engine API types.

import (
	. "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

var (
	mgrs []IManager
)

func addManager[T IManager](mgr T) T {
	mgrs = append(mgrs, mgr)
	return mgr
}

func CreateMgrs() []IManager {
	// Make repeated calls deterministic.
	mgrs = mgrs[:0]
	return createMgrs()
}

type baseMgr struct{}

func (pself *baseMgr) OnStart() {}

func (pself *baseMgr) OnUpdate(delta float64) {}

func (pself *baseMgr) OnFixedUpdate(delta float64) {}

func (pself *baseMgr) OnDestroy() {}

func (pself *baseMgr) OnPause(isPaused bool) {}
