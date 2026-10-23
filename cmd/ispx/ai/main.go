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
	"github.com/goplus/spx/v4/cmd/ispx/ai/internal/ispxai"
	"github.com/goplus/spx/v4/internal/spxproject"
	"github.com/goplus/spx/v4/pkg/ispx"
)

func init() {
	// The web interpreter exposes Builder AI as a command-scoped auto-import.
	project := spxproject.New()
	project.Import = append(project.Import, &modfile.Import{Name: "ai", Path: ispxai.ModulePath})
	xgobuild.RegisterProject(project)
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
