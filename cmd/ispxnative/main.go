//go:build !js || !wasm

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
	"fmt"
	"os"
	"path/filepath"
	_ "unsafe"

	"github.com/goplus/spx/v3/pkg/ispx"
)

func main() {
	// The project directory is one level up from the current working directory
	// because this shared library runs from either:
	//   - .temp/ directory (spx run interpreted mode)
	//   - project/ directory (spx runnative/editor mode)
	//
	// In both cases, the spx source files (.spx) are in the parent directory.
	projDir, err := filepath.Abs("..")
	if err != nil {
		panic("Failed to get project directory: " + err.Error())
	}

	if err := ispx.Init(nil); err != nil {
		panic("Failed to initialize: " + err.Error())
	}

	if err := ispx.BuildFS(os.DirFS(projDir)); err != nil {
		panic("Failed to build: " + err.Error())
	}

	if exitCode, err := ispx.Run(); err != nil {
		panic(fmt.Sprintf("interpreter exited with code %d: %v", exitCode, err))
	}
}
