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

// BackdropResolver resolves a backdrop name or number to an index.
// [github.com/goplus/spx/v4.Game] values and custom stage implementations
// with the same method satisfy this interface.
type BackdropResolver interface {
	ResolveBackdropIndex(backdrop string) int
}

// ResolveBackdropIndex delegates backdrop lookup to the stage implementation.
// For SPX stages, exact names take precedence over 1-based integer strings and
// the result is zero-based. Invalid and negative results are returned unchanged.
func ResolveBackdropIndex(game BackdropResolver, backdrop string) int {
	return game.ResolveBackdropIndex(backdrop)
}
