/*
 * Copyright (c) 2021 The GoPlus Authors (goplus.org). All rights reserved.
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

package spx

import (
	"fmt"

	"github.com/goplus/spx/internal/engine"
	"github.com/goplus/spx/internal/time"
	"github.com/goplus/spx/internal/ui"
)

func init() {
	const dpi = 72

}
func (p *Game) showDebugPanel() {
	if p.debugObj == nil {
		p.debugObj = ui.NewUiDebug()
		p.addShape(p.debugObj)
	}
	engine.SetDebugMode(p.debug)
	msg := ""
	if p.debug {
		msg = fmt.Sprintf("FPS: %.f\n", time.FPS())
		msg += fmt.Sprintf("Shape: %v\n", len(p.items))
	}
	p.debugObj.Show(msg)
}
