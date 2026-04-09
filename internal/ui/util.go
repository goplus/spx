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

package ui

import (
	. "github.com/goplus/spbase/mathf"

	"github.com/goplus/spx/v2/internal/engine"
	"github.com/goplus/spx/v2/internal/enginewrap"
)

var (
	mgr *enginewrap.EngineManagers

	baseScreenWidth         int
	baseScreenHeight        int
	clampUIPositionInScreen bool
)

type UiNode struct {
	engine.UiNode
}

func Init(managers *enginewrap.EngineManagers) {
	mgr = managers
}

func SetBaseScreenSize(width, height int) {
	baseScreenWidth = width
	baseScreenHeight = height
}

func ClampUIPositionInScreen(isClamp bool) {
	clampUIPositionInScreen = isClamp
}

// ViewToUI converts centered view coordinates to UI coordinates.
func ViewToUI(pos Vec2) Vec2 {
	pos = pos.Mulf(engine.WindowScale())
	pos = NewVec2(pos.X, -pos.Y)
	viewport := mgr.CameraMgr.GetViewportRect()
	return pos.Add(viewport.Size.Mulf(0.5)).Sub(viewport.Position)
}
