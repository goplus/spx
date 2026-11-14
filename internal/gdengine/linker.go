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

import (
	"sync"

	"github.com/goplus/spx/v4/internal/engine/wrap"
	"github.com/goplus/spx/v4/internal/gdengine/binding/facade"
	engineimpl "github.com/goplus/spx/v4/internal/gdengine/impl"
	"github.com/goplus/spx/v4/pkg/spx/pkg/engine"
	"github.com/goplus/spx/v4/pkg/spx/pkg/gdspx"
)

// LinkSession owns the backend resources installed by one Link call.
type LinkSession struct {
	backend         *facade.LinkSession
	releaseManagers func()
}

type linkerBridge struct{}

var (
	coreCallbacks engine.CoreCallbackInfo
	activeLink    *LinkSession
	linkMu        sync.Mutex
)

func init() {
	gdspx.SetLinkerBridge(linkerBridge{})
}

func Link(coreCallbackInfo engine.CoreCallbackInfo) {
	PrepareLink(standaloneCallbacks(coreCallbackInfo)).Run(nil)
}

// PrepareLink installs callbacks and managers before Run enters the backend.
// Its OnEngineUpdate callback owns runtime advancement for each frame.
func PrepareLink(coreCallbackInfo engine.CoreCallbackInfo) *LinkSession {
	linkMu.Lock()
	defer linkMu.Unlock()
	if activeLink != nil {
		panic("gdengine: a link is already active")
	}

	session := &LinkSession{backend: facade.LinkFFI()}

	coreCallbacks = coreCallbackInfo
	facade.BindCallbacks(makeCallbacks())
	// These Go adapters have no lifecycle behavior; the backend owns its managers.
	session.releaseManagers = wrap.BindManagers(engineimpl.CreateManagers())
	activeLink = session
	return session
}

func Unlink() {
	linkMu.Lock()
	session := activeLink
	linkMu.Unlock()
	session.Unlink()
}

func (s *LinkSession) Run(ready func()) {
	s.backend.Run(ready)
}

func (s *LinkSession) Unlink() {
	if s == nil {
		return
	}
	s.backend.Unlink()
	linkMu.Lock()
	defer linkMu.Unlock()
	if s.releaseManagers != nil {
		s.releaseManagers()
		s.releaseManagers = nil
	}
	if activeLink != s {
		return
	}
	activeLink = nil
	coreCallbacks = engine.CoreCallbackInfo{}
}

func (linkerBridge) Link(coreCallbackInfo engine.CoreCallbackInfo) {
	Link(coreCallbackInfo)
}

func (linkerBridge) Unlink() {
	Unlink()
}
