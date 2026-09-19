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

package gdengine

//lint:file-ignore ST1001 Godot linker glue intentionally dot-imports engine API types.

import (
	"sync"

	"github.com/goplus/spx/v3/internal/gdengine/binding/facade"
	engineimpl "github.com/goplus/spx/v3/internal/gdengine/impl"
	. "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
	gdspx "github.com/goplus/spx/v3/pkg/spx/pkg/gdspx"
)

var (
	mgrs                []IManager
	coreCallbacks       CoreCallbackInfo
	sprites             = make([]ISpriter, 0)
	isWebIntepreterMode bool
	activeLink          *LinkSession
	linkMu              sync.Mutex
)

// LinkSession owns the backend resources installed by one Link call.
type LinkSession struct {
	backend *facade.LinkSession
}

type linkerBridge struct{}

func init() {
	gdspx.SetLinkerBridge(linkerBridge{})
}

func (linkerBridge) IsWebIntepreterMode() bool {
	return IsWebIntepreterMode()
}

func (linkerBridge) Link(coreCallbackInfo CoreCallbackInfo) {
	Link(coreCallbackInfo)
}

func (linkerBridge) Unlink() {
	Unlink()
}

func IsWebIntepreterMode() bool {
	return isWebIntepreterMode
}

func Link(coreCallbackInfo CoreCallbackInfo) {
	PrepareLink(coreCallbackInfo).Run(nil)
}

// PrepareLink installs the process-wide callbacks and managers without
// entering the backend's potentially blocking run loop.
func PrepareLink(coreCallbackInfo CoreCallbackInfo) (session *LinkSession) {
	linkMu.Lock()
	defer linkMu.Unlock()
	if activeLink != nil {
		panic("gdengine: a link is already active")
	}

	backend, interpreter := facade.LinkFFI()
	session = &LinkSession{backend: backend}
	committed := false
	defer func() {
		if !committed {
			backend.Unlink()
			mgrs = nil
			coreCallbacks = CoreCallbackInfo{}
			isWebIntepreterMode = false
		}
	}()

	isWebIntepreterMode = interpreter
	coreCallbacks = coreCallbackInfo
	infos := bindCallbacks()
	facade.RegisterCallbacks(infos)
	mgrs = engineimpl.CreateMgrs()
	engineimpl.BindMgr(mgrs)
	activeLink = session
	committed = true
	return session
}

func (s *LinkSession) Run(ready func()) {
	if s == nil {
		if ready != nil {
			ready()
		}
		return
	}
	s.backend.Run(ready)
}

func (s *LinkSession) Unlink() {
	if s == nil {
		return
	}
	s.backend.Unlink()
	linkMu.Lock()
	defer linkMu.Unlock()
	if activeLink != s {
		return
	}
	activeLink = nil
	mgrs = nil
	coreCallbacks = CoreCallbackInfo{}
	isWebIntepreterMode = false
}

// Unlink closes the active legacy Link session.
func Unlink() {
	linkMu.Lock()
	session := activeLink
	linkMu.Unlock()
	session.Unlink()
}
