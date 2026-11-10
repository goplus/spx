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

package engine

import (
	"sync"

	"github.com/goplus/spx/v4/internal/engine/wrap"
	gdx "github.com/goplus/spx/v4/pkg/spx/pkg/engine"
)

type (
	Object = gdx.Object
	Array  = gdx.Array
)

type IGame interface {
	OnEngineStart()
	OnEngineUpdate(delta float64)
	OnEngineBeforeUpdate(delta float64)
	OnEngineRender(delta float64)
	OnEngineFrameEnd()
	OnEngineDestroy()
	OnEngineReset()
	OnEnginePause(isPaused bool)
}

const Float2IntFactor = gdx.Float2IntFactor

var logicMu sync.Mutex

// Main reserves the runtime until backend teardown.
// Call it outside managed scripts and runtime callbacks so failed startup can
// drain its work before releasing the session.
func Main(game IGame, owner any, initialize func()) error {
	binding, err := beginGame(game, owner)
	if err != nil {
		return err
	}
	committed := false
	finishStart := sync.OnceFunc(func() { close(binding.startDone) })
	defer func() {
		finishStart()
		if !committed {
			abortGameStart(binding)
		}
	}()

	if initialize != nil {
		initialize()
	}
	wrap.Init(WaitMainThread)
	callbacks := gdx.CoreCallbackInfo{
		OnEngineStart:     onStart,
		OnEngineUpdate:    onUpdate,
		OnEngineDestroy:   onDestroy,
		OnEngineDestroyed: onDestroyed,
		OnEngineReset:     onReset,
		OnEnginePause:     onPause,
		OnMousePressed:    onMousePressed,
		OnMouseReleased:   onMouseReleased,
		OnKeyPressed:      onKeyPressed,
		OnKeyReleased:     onKeyReleased,
	}
	if !activateGame(binding, callbacks) {
		return nil
	}
	binding.link.Run(finishStart)
	committed = true
	return nil
}

func Lock() {
	logicMu.Lock()
}

func Unlock() {
	logicMu.Unlock()
}
