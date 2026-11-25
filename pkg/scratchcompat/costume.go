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

import (
	"github.com/goplus/spx/v4/internal/base/numeric"
	"github.com/goplus/spx/v4/internal/core/scratch"
)

// CostumeResolver resolves a costume name or number to an index.
// [github.com/goplus/spx/v4.Sprite] values and pointers to
// [github.com/goplus/spx/v4.SpriteImpl] satisfy this interface.
type CostumeResolver interface {
	ResolveCostumeIndex(costume string) int
}

// ResolveCostumeIndex resolves strings by exact name first. Numeric selectors use
// the sprite's optional ResolveCostumeNumber(int) int method and wrap as 1-based
// costume numbers. Unsupported selectors return -1.
func ResolveCostumeIndex(sprite CostumeResolver, selector any) int {
	selector = scratch.UnwrapValue(selector)
	if name, ok := selector.(string); ok {
		return sprite.ResolveCostumeIndex(name)
	}
	resolver, canResolve := sprite.(interface{ ResolveCostumeNumber(int) int })
	number, isNumber := numeric.Int(selector)
	if !canResolve || !isNumber {
		return -1
	}
	// Int truncates floats. A fractional selector has no matching costume number.
	if value, _ := numeric.Float64(selector); value != float64(number) {
		return -1
	}
	return resolver.ResolveCostumeNumber(number)
}
