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

import "github.com/goplus/spx/v4"

var keyNames = map[string]spx.Key{
	"any":         spx.KeyAny,
	"space":       spx.KeySpace,
	"enter":       spx.KeyEnter,
	"up arrow":    spx.KeyUp,
	"down arrow":  spx.KeyDown,
	"left arrow":  spx.KeyLeft,
	"right arrow": spx.KeyRight,
	"backspace":   spx.KeyBackspace,
	"delete":      spx.KeyDelete,
	"shift":       spx.KeyShift,
	"caps lock":   spx.KeyCapsLock,
	"scroll lock": spx.KeyScrollLock,
	"control":     spx.KeyControl,
	"escape":      spx.KeyEscape,
	"insert":      spx.KeyInsert,
	"home":        spx.KeyHome,
	"end":         spx.KeyEnd,
	"page up":     spx.KeyPageUp,
	"page down":   spx.KeyPageDown,
}

// KeyFromString matches lowercase Scratch/TurboWarp key names exactly, falling
// back to spx.KeyFromString for other names. Unknown names return spx.KeyMax.
func KeyFromString(key string) spx.Key {
	if k, ok := keyNames[key]; ok {
		return k
	}
	return spx.KeyFromString(key)
}
