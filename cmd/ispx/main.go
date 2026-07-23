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

import "github.com/goplus/spx/v3/pkg/ispx"

// This command validates the current SPX Web runtime. Builder AI remains under
// internal/ispxai for generation and package tests, but is not linked into this
// binary while its upstream module still depends on SPX v2.
func main() {
	if err := ispx.Init(nil); err != nil {
		panic(err)
	}
	select {}
}
