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

package ispx

import (
	"reflect"
	"testing"

	"github.com/goplus/ixgo"
	gdx "github.com/goplus/spx/v4/pkg/spx/pkg/engine"
)

func TestInterpretedEngineEvents(t *testing.T) {
	const source = `package main
import "github.com/goplus/spx/v4/pkg/spx/pkg/engine"

func Run(node *engine.UiNode) []int {
	var calls []int
	e := engine.NewEvent0()
	id := (*engine.Event0).Subscribe(e, func() { calls = append(calls, 1) })
	var dispatch interface { Trigger(); Unsubscribe(int); UnsubscribeAll() } = e
	dispatch.Trigger()
	dispatch.Unsubscribe(id)
	dispatch.Trigger()
	dispatch.UnsubscribeAll()
	next := e.Subscribe(func() { calls = append(calls, 2) })
	e.Trigger()
	toggle := node.OnUiToggleEvent
	toggle.Subscribe(func(on bool) {
		if on { calls = append(calls, 3) }
		toggle.UnsubscribeAll()
	})
	toggle.Trigger(true)
	toggle.Trigger(true)
	return append(calls, next)
}`
	for name, mode := range map[string]ixgo.Mode{"reflect": 0, "cached": ixgo.EnableCachedReg} {
		t.Run(name, func(t *testing.T) {
			ctx := ixgo.NewContext(mode | ixgo.SupportMultipleInterp)
			i, err := ctx.LoadInterp("events.go", source)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(i.UnsafeRelease)
			got := directFuncRun(t, i, "Run", &gdx.UiNode{OnUiToggleEvent: gdx.NewEvent1[bool]()})
			if want := []int{1, 2, 3, 1}; !reflect.DeepEqual(got, want) {
				t.Fatalf("event calls and next ID=%v, want %v", got, want)
			}
		})
	}
}
