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

package ispx

import (
	"github.com/goplus/ixgo"
	"github.com/goplus/spx/v4"
)

// DirectFuncVal must be the first field of each value receiver so ixgo can
// recover the interpreted function and its captured environment.
type directResult[T any] struct {
	ixgo.DirectFuncVal
}

type directAction[T any] struct {
	ixgo.DirectFuncVal
}

type directMessageFunc struct {
	ixgo.DirectFuncVal
}

// Register the callback signatures used by SPX scripts on every interpreter
// platform. func() is already built into ixgo. Other signatures retain ixgo's
// reflective fallback.
func init() {
	ixgo.RegisterDirectFuncFor(bindDirectResult[bool])
	ixgo.RegisterDirectFuncFor(bindDirectResult[error])
	ixgo.RegisterDirectFuncFor(bindDirectAction[spx.Key])
	ixgo.RegisterDirectFuncFor(bindDirectAction[string])
	ixgo.RegisterDirectFuncFor(bindDirectAction[any])
	ixgo.RegisterDirectFuncFor(bindDirectAction[spx.Sprite])
	ixgo.RegisterDirectFuncFor(bindDirectMessage)
}

func (c directResult[T]) call() T {
	value := c.Call()
	if value == nil {
		// Abort can stop before return. Preserve zero results, and allow a
		// Snapshot callback to return a nil error normally.
		var zero T
		return zero
	}
	return value.(T)
}

func (c directAction[T]) call(arg T) {
	c.CallDiscard(arg)
}

func (c directMessageFunc) call(name string, data any) {
	c.CallDiscard(name, data)
}

func bindDirectResult[T any](c ixgo.DirectFuncVal) func() T {
	return directResult[T]{c}.call
}

func bindDirectAction[T any](c ixgo.DirectFuncVal) func(T) {
	return directAction[T]{c}.call
}

func bindDirectMessage(c ixgo.DirectFuncVal) func(string, any) {
	return directMessageFunc{c}.call
}
