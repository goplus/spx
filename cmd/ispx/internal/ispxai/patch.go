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

package ispxai

import (
	"fmt"

	"github.com/goplus/ixgo"
)

const ModulePath = "github.com/goplus/builder/tools/ai"

const patchSource = `
package ai

import . %q

func XGot_Player_XGox_OnCmd[T any](p *Player, handler func(cmd T) error) {
	var cmd T
	PlayerOnCmd_(p, cmd, handler)
}

func XGot_Player_XGox_OnCmd__0[T any](p *Player, handler func(cmd T) error) {
	XGot_Player_XGox_OnCmd(p, handler)
}
`

func RegisterPatch(ctx *ixgo.Context) error {
	if err := ctx.RegisterPatch(ModulePath, fmt.Sprintf(patchSource, ModulePath)); err != nil {
		return fmt.Errorf("failed to register ai patch: %w", err)
	}
	return nil
}
