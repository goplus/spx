//go:build js && wasm

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

package main

import (
	"github.com/goplus/ixgo"
	"github.com/goplus/ixgo/xgobuild"
	"github.com/goplus/mod/modfile"
	"github.com/goplus/spx/v3/cmd/ispx/internal/ispxai"
	"github.com/goplus/spx/v3/pkg/ispx"
)

func init() {
	// The web interpreter exposes Builder AI as a command-scoped auto-import.
	xgobuild.RegisterProject(&modfile.Project{
		Ext:      ".spx",
		Class:    "Game",
		Works:    []*modfile.Class{{Ext: ".spx", Class: "SpriteImpl", Embedded: true}},
		PkgPaths: []string{"github.com/goplus/spx/v3", "math"},
		Import:   []*modfile.Import{{Name: "ai", Path: ispxai.ModulePath}},
	})
}

func main() {
	ixgoCtx := ixgo.NewContext(ixgo.SupportMultipleInterp | ixgo.EnableCachedReg)
	if err := ispxai.RegisterPatch(ixgoCtx); err != nil {
		panic(err)
	}
	if err := ispx.Init(ixgoCtx); err != nil {
		panic(err)
	}
	select {}
}
