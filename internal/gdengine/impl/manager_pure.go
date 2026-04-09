//go:build pure_engine

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

/*
------------------------------------------------------------------------------
//   Pure engine mode manager wrapper (no FFI).
//   This file is NOT auto-generated. It must be manually maintained.
//   Manager types embed enginewrap.*MgrImpl (from sync_pure.gen.go)
//   which are auto-generated and provide all interface method stubs.
//----------------------------------------------------------------------------
*/
package impl

import (
	"fmt"
	"reflect"

	"github.com/goplus/spx/v2/internal/enginewrap"
	. "github.com/goplus/spx/v2/pkg/spx/pkg/engine"
)

func BindMgr(mgrs []IManager) {
	for _, mgr := range mgrs {
		switch v := mgr.(type) {
		case IAudioMgr:
			AudioMgr = v
		case ICameraMgr:
			CameraMgr = v
		case IDebugMgr:
			DebugMgr = v
		case IExtMgr:
			ExtMgr = v
		case IInputMgr:
			InputMgr = v
		case INavigationMgr:
			NavigationMgr = v
		case IPenMgr:
			PenMgr = v
		case IPhysicsMgr:
			PhysicsMgr = v
		case IPlatformMgr:
			PlatformMgr = v
		case IResMgr:
			ResMgr = v
		case ISceneMgr:
			SceneMgr = v
		case ISpriteMgr:
			SpriteMgr = v
		case ITilemapMgr:
			TilemapMgr = v
		case ITilemapparserMgr:
			TilemapparserMgr = v
		case IUiMgr:
			UiMgr = v
		default:
			panic(fmt.Sprintf("engine init error : unknown manager type %s", reflect.TypeOf(mgr).String()))
		}
	}
}

type audioMgr struct {
	baseMgr
	enginewrap.AudioMgrImpl
}
type cameraMgr struct {
	baseMgr
	enginewrap.CameraMgrImpl
}
type debugMgr struct {
	baseMgr
	enginewrap.DebugMgrImpl
}
type extMgr struct {
	baseMgr
	enginewrap.ExtMgrImpl
}
type inputMgr struct {
	baseMgr
	enginewrap.InputMgrImpl
}
type navigationMgr struct {
	baseMgr
	enginewrap.NavigationMgrImpl
}
type penMgr struct {
	baseMgr
	enginewrap.PenMgrImpl
}
type physicsMgr struct {
	baseMgr
	enginewrap.PhysicsMgrImpl
}
type platformMgr struct {
	baseMgr
	enginewrap.PlatformMgrImpl
}
type resMgr struct {
	baseMgr
	enginewrap.ResMgrImpl
}
type sceneMgr struct {
	baseMgr
	enginewrap.SceneMgrImpl
}
type spriteMgr struct {
	baseMgr
	enginewrap.SpriteMgrImpl
}
type tilemapMgr struct {
	baseMgr
	enginewrap.TilemapMgrImpl
}
type tilemapparserMgr struct {
	baseMgr
	enginewrap.TilemapparserMgrImpl
}
type uiMgr struct {
	baseMgr
	enginewrap.UiMgrImpl
}

func createMgrs() []IManager {
	addManager(&audioMgr{})
	addManager(&cameraMgr{})
	addManager(&debugMgr{})
	addManager(&extMgr{})
	addManager(&inputMgr{})
	addManager(&navigationMgr{})
	addManager(&penMgr{})
	addManager(&physicsMgr{})
	addManager(&platformMgr{})
	addManager(&resMgr{})
	addManager(&sceneMgr{})
	addManager(&spriteMgr{})
	addManager(&tilemapMgr{})
	addManager(&tilemapparserMgr{})
	addManager(&uiMgr{})
	return mgrs
}
