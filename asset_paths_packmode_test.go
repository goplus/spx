//go:build packmode

/*
 * Copyright (c) 2026 The XGo Authors (xgo.dev). All rights reserved.
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
	"github.com/goplus/spx/v4/internal/engine"
	pkgengine "github.com/goplus/spx/v4/pkg/spx/pkg/engine"
)

const testFilesystemMode = false

// Direct runtime tests bypass loadGame, which normally installs archive roots.
func setupTestAssetPaths() {
	previous := pkgengine.ResMgr
	pkgengine.ResMgr = testAssetResMgr{}
	engine.SetAssetDir("assets")
	pkgengine.ResMgr = previous
}

type testAssetResMgr struct{ pkgengine.IResMgr }

func (testAssetResMgr) SetLoadMode(bool)    {}
func (testAssetResMgr) HasFile(string) bool { return false }
