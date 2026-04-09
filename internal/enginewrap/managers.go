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

package enginewrap

// EngineManagers groups all engine-facing manager wrappers.
// This keeps runtime dependencies scoped to the game instead of package-level globals.
type EngineManagers struct {
	AudioMgr         AudioMgrImpl
	CameraMgr        CameraMgrImpl
	InputMgr         InputMgrImpl
	PhysicsMgr       PhysicsMgrImpl
	PlatformMgr      PlatformMgrImpl
	ResMgr           ResMgrImpl
	SceneMgr         SceneMgrImpl
	SpriteMgr        SpriteMgrImpl
	UiMgr            UiMgrImpl
	ExtMgr           ExtMgrImpl
	PenMgr           PenMgrImpl
	DebugMgr         DebugMgrImpl
	NavigationMgr    NavigationMgrImpl
	TilemapMgr       TilemapMgrImpl
	TilemapparserMgr TilemapparserMgrImpl
}
