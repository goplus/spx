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

package state

import (
	"sync"
	"sync/atomic"
	"time"

	coreevent "github.com/goplus/spx/v3/internal/core/event"
	"github.com/goplus/spx/v3/internal/engine"
	"github.com/goplus/spx/v3/internal/ui"
)

type GameLifecycleState struct {
	RunOnce         sync.Once
	OncePathFinder  sync.Once
	BootstrapDone   atomic.Bool
	StartDispatched atomic.Bool
	IsRunned        atomic.Bool
}

type GameDisplayState struct {
	WorldWidth   int
	WorldHeight  int
	MinWorldX    int
	MinWorldY    int
	MapMode      int
	WindowWidth  int
	WindowHeight int
	WindowScale  float64
	StretchMode  bool
}

type GameDialogState struct {
	AskPanel  *ui.UiAsk
	AnswerVal string
}

type GameDebugState struct {
	Debug      bool
	DebugPanel *ui.UiDebug
	DebugInstr bool
	DebugEvent bool
	DebugPerf  bool
}

type GameRuntimeState struct {
	EnabledPhysics   bool
	IsSchedInMain    bool
	MainSchedTime    time.Time
	ImageSizeCache   sync.Map
	EventQueueMu     sync.Mutex
	EventQueuePolicy coreevent.QueuePolicy
	EventQueueStats  coreevent.QueueStats
}

type GamePathfindingState struct {
	PathCellSizeX int
	PathCellSizeY int
}

type GameAudioState struct {
	AudioAttenuation float64
	AudioMaxDistance float64
	SoundObj         engine.Object
}
