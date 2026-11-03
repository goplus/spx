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

package scratchcompat

import "github.com/goplus/spx/v4/internal/scratch"

// List stores values using 0-based indexes.
// Its elements are reported as [github.com/goplus/spx/v4.Value].
type List = scratch.List

// Special list positions retain their SPX values and types.
const (
	Invalid = scratch.ListInvalid
	Last    = scratch.ListLast
	All     = scratch.ListAll
	Random  = scratch.ListRandom
)

// NewList copies items into a new list, unwrapping one layer of spx.Value.
func NewList(items ...any) List {
	return scratch.NewList(items...)
}
