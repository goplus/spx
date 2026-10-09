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
	"sync/atomic"

	coreevent "github.com/goplus/spx/v3/internal/core/event"
	"github.com/goplus/spx/v3/internal/engine"
	"github.com/goplus/spx/v3/internal/ui"
)

type gameLifecycleState struct {
	RunOnce         sync.Once
	OncePathFinder  sync.Once
	BootstrapDone   atomic.Bool
	StartDispatched atomic.Bool
	IsRunned        atomic.Bool
}

type gameDisplayState struct {
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

type gameDialogState struct {
	AskPanel  *ui.UiAsk
	AnswerVal string
}

type gameDebugState struct {
	Debug      bool
	DebugPanel *ui.UiDebug
	DebugInstr bool
	DebugEvent bool
}

type gameEventQueueState struct {
	EventQueueMu     sync.Mutex
	EventQueuePolicy coreevent.QueuePolicy
	EventQueueStats  coreevent.QueueStats
}

type gamePathfindingState struct {
	PathCellSizeX int
	PathCellSizeY int
}

type gameAudioState struct {
	AudioAttenuation float64
	AudioMaxDistance float64
	SoundObj         engine.Object
}

// gameBootstrapState owns generation-scoped admission and task execution.
// Game coordinates lifecycle gates under the same mutex; callbacks run unlocked.
type gameBootstrapState struct {
	bootstrapMu      sync.Mutex
	bootstrapGen     uint64
	bootstrapStarted bool
	startScheduled   bool
	pendingBootstrap []func()
}

func (p *gameBootstrapState) queueBootstrap(generation uint64, call func()) bool {
	if call == nil {
		return false
	}
	p.bootstrapMu.Lock()
	defer p.bootstrapMu.Unlock()
	if generation != p.bootstrapGen {
		return false
	}
	p.pendingBootstrap = append(p.pendingBootstrap, call)
	return true
}

func (p *gameBootstrapState) bootstrapGeneration() uint64 {
	p.bootstrapMu.Lock()
	defer p.bootstrapMu.Unlock()
	return p.bootstrapGen
}

// Validate and publish under the reset lock so stale work cannot reopen a gate.
func (p *gameBootstrapState) markBootstrapFlag(generation uint64, flag *atomic.Bool) bool {
	p.bootstrapMu.Lock()
	defer p.bootstrapMu.Unlock()
	if generation != p.bootstrapGen {
		return false
	}
	flag.Store(true)
	return true
}

func (p *gameBootstrapState) takeBootstrapTasks(generation uint64) []func() {
	p.bootstrapMu.Lock()
	defer p.bootstrapMu.Unlock()
	if generation != p.bootstrapGen {
		return nil
	}
	tasks := p.pendingBootstrap
	p.pendingBootstrap = nil
	return tasks
}

func (p *gameBootstrapState) runBootstrapTasks(generation uint64) {
	for {
		tasks := p.takeBootstrapTasks(generation)
		if len(tasks) == 0 {
			return
		}
		// Also drain tasks queued by earlier tasks.
		for _, task := range tasks {
			if !p.isCurrentBootstrap(generation) {
				return
			}
			task()
		}
	}
}

func (p *gameBootstrapState) isCurrentBootstrap(generation uint64) bool {
	return generation == p.bootstrapGeneration()
}

func (p *gameBootstrapState) claimBootstrap(generation uint64) bool {
	p.bootstrapMu.Lock()
	defer p.bootstrapMu.Unlock()
	if generation != p.bootstrapGen || p.bootstrapStarted {
		return false
	}
	p.bootstrapStarted = true
	return true
}
