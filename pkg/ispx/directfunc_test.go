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
	"errors"
	"reflect"
	"testing"

	"github.com/goplus/ixgo"
	"github.com/goplus/spx/v4"
	"github.com/visualfc/funcval"
)

const directFuncSource = `package main

import "github.com/goplus/spx/v4"

func MakeCallbacks(count int, failure error) []any {
	var key spx.Key
	var text string
	var data any
	var sprite spx.Sprite
	return []any{
		func() { count++ },
		func() bool { return count % 2 == 0 },
		func() error { if count % 2 == 0 { return nil }; return failure },
		func(v spx.Key) { key = v; count++ },
		func(v string) { text = v; count++ },
		func(v any) { data = v; count++ },
		func(v spx.Sprite) { sprite = v; count++ },
		func(name string, v any) { text = name; data = v; count++ },
		func() []any { return []any{count, key, text, data, sprite} },
	}
}

func InvokeCallbacks(callbacks []any) []any {
	callbacks[0].(func())()
	even := callbacks[1].(func() bool)()
	failure := callbacks[2].(func() error)()
	callbacks[3].(func(spx.Key))(spx.KeyB)
	callbacks[4].(func(string))("event")
	callbacks[5].(func(any))(nil)
	callbacks[6].(func(spx.Sprite))(nil)
	callbacks[7].(func(string, any))("roundtrip", nil)
	return []any{even, failure}
}

func RecoverKey(mark func(bool)) func(spx.Key) {
	return func(key spx.Key) {
		defer func() { mark(recover() == key) }()
		panic(key)
	}
}

func PanicMessage() func(string, any) {
	return func(name string, data any) { panic(data) }
}

func HostPanic(stop func()) func(spx.Sprite) {
	return func(sprite spx.Sprite) { stop() }
}

func AbortCallbacks(hook, body func(), failure error) []any {
	return []any{
		func() { hook(); body() },
		func() bool { hook(); body(); return true },
		func() error { hook(); body(); return failure },
		func(v spx.Key) { hook(); body() },
		func(v string) { hook(); body() },
		func(v any) { hook(); body() },
		func(v spx.Sprite) { hook(); body() },
		func(name string, v any) { hook(); body() },
	}
}

func UpdateInventory(items *spx.List) []any {
	var previous spx.List
	previous.InitFrom(items)
	items.Insert(0, spx.NewValue("Apple"))
	price := spx.NewValue("03.5")
	price.Set(spx.NewValue("04.5"))
	items.Append(price)
	items.Set(items.IndexOf("coin"), "gold")
	items.Delete(items.IndexOf("potion"))
	return []any{
		&previous, items.String(), items.Len(), items.Contains("apple"),
		items.At(spx.Last).Float(), price.Int(), price.Equal(4.5), items.At(0).String(),
	}
}

func ClearInventory(items *spx.List) {
	items.Clear()
}

`

type directFuncCallbacks struct {
	values    []any
	increment func()
	even      func() bool
	snapshot  func() error
	key       func(spx.Key)
	text      func(string)
	data      func(any)
	sprite    func(spx.Sprite)
	message   func(string, any)
	state     func() []any
}

// No Sprite methods are needed: this verifies interface identity at the bridge.
type directFuncTestSprite struct{ spx.Sprite }

const procedureStopSource = `package main

import "github.com/goplus/spx/v4"

var game spx.Game
var sprite spx.SpriteImpl

func Run() int {
	n := 0
	spx.Procedure(func() {
		n++
		spx.Procedure(func() {
			n += 10
			game.Stop(spx.ThisScript)
			n += 1000
		})
		n += 100
		spx.Repeat(3, func() {
			sprite.Stop(spx.ThisScript)
			n += 1000
		})
		n += 1000
	})
	return n
}

func OtherPanic() (result string) {
	defer func() { result = recover().(string) }()
	spx.Procedure(func() { panic("user panic") })
	return "unreachable"
}

func NilGame() (panicked bool) {
	defer func() { panicked = recover() != nil }()
	var p *spx.Game
	spx.Procedure(func() { p.Stop(spx.ThisScript) })
	return
}

func NilSprite() (panicked bool) {
	defer func() { panicked = recover() != nil }()
	var p *spx.SpriteImpl
	spx.Procedure(func() { p.Stop(spx.ThisScript) })
	return
}

func PromotedAndDynamic() int {
	var p struct { spx.Game }
	var target interface { Stop(spx.StopKind) } = &p
	stop := target.Stop
	count := 0
	spx.Procedure(func() {
		p.Stop(spx.StopKind(12345)) // Unknown kinds remain a no-op.
		count++
		stop(spx.ThisScript)
		count++
	})
	return count
}

func RunStops(n int) int {
	count := 0
	for i := 0; i < n; i++ {
		spx.Procedure(func() {
			count++
			game.Stop(spx.ThisScript)
		})
	}
	return count
}
`

func TestDirectFuncCallbacks(t *testing.T) {
	for _, mode := range []struct {
		name string
		mode ixgo.Mode
	}{{"uncached", 0}, {"cached", ixgo.EnableCachedReg}} {
		t.Run(mode.name, func(t *testing.T) {
			i := newDirectFuncInterp(t, mode.mode)
			failure := errors.New("snapshot failed")
			left := loadDirectFuncCallbacks(t, i, "MakeCallbacks", 0, failure)
			right := loadDirectFuncCallbacks(t, i, "MakeCallbacks", 1, failure)
			if funcval.IsSupport {
				for _, callback := range left.values[:8] {
					if _, bridges := funcval.Get(callback); bridges != 0 {
						t.Errorf("callback %T uses %d reflection bridges", callback, bridges)
					}
				}
				// An unsupported signature still uses ixgo's reflection fallback.
				if _, bridges := funcval.Get(left.state); bridges != 1 {
					t.Errorf("fallback callback uses %d reflection bridges, want 1", bridges)
				}
			}
			if !left.even() || right.even() || left.snapshot() != nil {
				t.Fatal("initial callback results or independent closure environments changed")
			}
			left.increment()
			if left.even() || left.snapshot() != failure {
				t.Fatal("callbacks lost their captured count or error identity")
			}
			left.key(spx.KeyA)
			left.text("backdrop")
			payload := &struct{ N int }{42}
			left.data(payload)
			target := &directFuncTestSprite{}
			left.sprite(target)
			left.checkState(t, []any{5, spx.KeyA, "backdrop", payload, target})
			left.message("message", payload)
			left.checkState(t, []any{6, spx.KeyA, "message", payload, target})
			left.data(nil)
			left.sprite(nil)
			left.message("nil", nil)
			left.checkState(t, []any{9, spx.KeyA, "nil", nil, nil})
			var typedNil *directFuncTestSprite
			left.data(typedNil)
			left.sprite(typedNil)
			left.checkState(t, []any{11, spx.KeyA, "nil", typedNil, typedNil})
			if got := right.state()[0]; got != 1 {
				t.Fatalf("independent callback count = %v, want 1", got)
			}
		})
	}
}

func TestDirectFuncRoundTrip(t *testing.T) {
	i := newDirectFuncInterp(t, ixgo.EnableCachedReg)
	failure := errors.New("snapshot failed")
	callbacks := loadDirectFuncCallbacks(t, i, "MakeCallbacks", 0, failure)
	if got, want := directFuncRun(t, i, "InvokeCallbacks", callbacks.values), []any{false, failure}; !reflect.DeepEqual(got, want) {
		t.Fatalf("roundtrip results = %#v, want %#v", got, want)
	}
	callbacks.checkState(t, []any{6, spx.KeyB, "roundtrip", nil, nil})
	// The same dynamic call sites also accept ordinary native Go callbacks.
	var calls []any
	native := []any{
		func() { calls = append(calls, "void") },
		func() bool { calls = append(calls, "bool"); return true },
		func() error { calls = append(calls, "error"); return nil },
		func(key spx.Key) { calls = append(calls, key) },
		func(text string) { calls = append(calls, text) },
		func(data any) { calls = append(calls, data) },
		func(sprite spx.Sprite) { calls = append(calls, sprite) },
		func(name string, data any) { calls = append(calls, name, data) },
	}
	if got, want := directFuncRun(t, i, "InvokeCallbacks", native), []any{true, nil}; !reflect.DeepEqual(got, want) {
		t.Fatalf("native results = %#v, want %#v", got, want)
	}
	if want := []any{"void", "bool", "error", spx.KeyB, "event", nil, nil, "roundtrip", nil}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("native calls = %#v, want %#v", calls, want)
	}
}

func TestDirectFuncPanic(t *testing.T) {
	i := newDirectFuncInterp(t, ixgo.EnableCachedReg)
	marks := 0
	key := directFuncRun(t, i, "RecoverKey", func(ok bool) {
		if !ok {
			t.Error("interpreted recover lost the typed key")
		}
		marks++
	}).(func(spx.Key))
	for range 3 {
		key(spx.KeyA)
	}
	if marks != 3 {
		t.Fatalf("recover called %d times, want 3", marks)
	}
	catch := func(call func()) (value any) {
		defer func() { value = recover() }()
		call()
		return
	}
	message := directFuncRun(t, i, "PanicMessage").(func(string, any))
	if got, ok := catch(func() { message("panic", "payload") }).(ixgo.PanicError); !ok || got.Value != "payload" {
		t.Fatalf("script panic = %#v, want PanicError carrying payload", got)
	}
	sentinel := new(int)
	sprite := directFuncRun(t, i, "HostPanic", func() { panic(sentinel) }).(func(spx.Sprite))
	for range 3 {
		if got := catch(func() { sprite(nil) }); got != sentinel {
			t.Fatalf("host panic = %v, want original sentinel %p", got, sentinel)
		}
	}
	key(spx.KeyB) // Interpreter remains usable after the host recovers.
}

func TestDirectFuncAbort(t *testing.T) {
	for _, tc := range []struct {
		name   string
		warmup int
		during bool
	}{
		{"cold/before", 0, false},
		{"cold/during", 0, true},
		{"warm/before", 2, false},
		{"warm/during", 2, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			i := newDirectFuncInterp(t, ixgo.EnableCachedReg)
			abort, hooks, bodies := false, 0, 0
			failure := errors.New("callback failed")
			callbacks := loadDirectFuncCallbacks(t, i, "AbortCallbacks", func() {
				hooks++
				if abort {
					i.Abort()
				}
			}, func() { bodies++ }, failure)
			invoke := func(wantBool bool, wantError error) {
				callbacks.increment()
				if got := callbacks.even(); got != wantBool {
					t.Errorf("bool callback = %v, want %v", got, wantBool)
				}
				if got := callbacks.snapshot(); got != wantError {
					t.Errorf("error callback = %v, want %v", got, wantError)
				}
				callbacks.key(spx.KeyA)
				callbacks.text("message")
				callbacks.data(nil)
				callbacks.sprite(nil)
				callbacks.message("message", nil)
			}
			for range tc.warmup {
				invoke(true, failure)
			}
			wantBodies := tc.warmup * len(callbacks.values)
			wantHooks := wantBodies
			if tc.during {
				abort = true
				if callbacks.even() {
					t.Error("bool callback aborted during execution returned true")
				}
				wantHooks++
			} else {
				i.Abort()
			}
			for range 3 {
				invoke(false, nil)
			}
			if hooks != wantHooks || bodies != wantBodies {
				t.Fatalf("after abort: hooks = %d, bodies = %d; want %d, %d", hooks, bodies, wantHooks, wantBodies)
			}
		})
	}
}

func BenchmarkDirectFuncEvents(b *testing.B) {
	b.Run("key", func(b *testing.B) {
		i := newDirectFuncInterp(b, ixgo.EnableCachedReg)
		key := loadDirectFuncCallbacks(b, i, "MakeCallbacks", 0, errors.New("unused")).key
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			key(spx.KeyA)
		}
	})
	b.Run("message", func(b *testing.B) {
		i := newDirectFuncInterp(b, ixgo.EnableCachedReg)
		message := loadDirectFuncCallbacks(b, i, "MakeCallbacks", 0, errors.New("unused")).message
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			message("message", nil)
		}
	})
}

func TestInterpretedProcedureStop(t *testing.T) {
	i := newProcedureStopInterp(t)
	for _, test := range []struct {
		name string
		want any
	}{
		{"Run", 111},
		{"OtherPanic", "user panic"},
		{"NilGame", true},
		{"NilSprite", true},
		{"PromotedAndDynamic", 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := directFuncRun(t, i, test.name)
			if got != test.want {
				t.Fatalf("result = %v, want %v", got, test.want)
			}
		})
	}
}

func BenchmarkInterpretedProcedureStop(b *testing.B) {
	i := newProcedureStopInterp(b)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		got, err := i.RunFunc("RunStops", 1000)
		if err != nil || got != 1000 {
			b.Fatalf("RunStops = %v, %v", got, err)
		}
	}
}

func TestInterpretedListMutation(t *testing.T) {
	i := newDirectFuncInterp(t, ixgo.EnableCachedReg)
	items := spx.NewList("coin", "potion")
	result := directFuncRun(t, i, "UpdateInventory", &items).([]any)
	previous := result[0].(*spx.List)
	if got := previous.String(); got != "coin potion" {
		t.Fatalf("snapshot changed with the inventory: %q", got)
	}
	want := []any{"Apple gold 04.5", 3, true, 4.5, 4, true, "Apple"}
	if !reflect.DeepEqual(result[1:], want) {
		t.Fatalf("interpreted inventory = %#v, want %#v", result[1:], want)
	}
	directFuncRun(t, i, "ClearInventory", previous)
	if previous.Len() != 0 || items.String() != "Apple gold 04.5" {
		t.Fatalf("clearing the returned snapshot changed inventory: snapshot=%q inventory=%q", previous.String(), items.String())
	}
	directFuncRun(t, i, "ClearInventory", &items)
	if items.Len() != 0 {
		t.Fatal("interpreter did not clear the host inventory")
	}
}

func (c directFuncCallbacks) checkState(t *testing.T, want []any) {
	t.Helper()
	if got := c.state(); !reflect.DeepEqual(got, want) {
		t.Fatalf("callback state = %#v, want %#v", got, want)
	}
}

func newDirectFuncInterp(t testing.TB, mode ixgo.Mode) *ixgo.Interp {
	t.Helper()
	ctx := ixgo.NewContext(mode | ixgo.SupportMultipleInterp)
	ctx.SetLeastCallForEnablePool(0)
	i, err := ctx.LoadInterp("callbacks.go", directFuncSource)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(i.UnsafeRelease)
	return i
}

func directFuncRun(t testing.TB, i *ixgo.Interp, name string, args ...any) any {
	t.Helper()
	value, err := i.RunFunc(name, args...)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func loadDirectFuncCallbacks(t testing.TB, i *ixgo.Interp, name string, args ...any) directFuncCallbacks {
	t.Helper()
	values := directFuncRun(t, i, name, args...).([]any)
	c := directFuncCallbacks{
		values:    values,
		increment: values[0].(func()),
		even:      values[1].(func() bool),
		snapshot:  values[2].(func() error),
		key:       values[3].(func(spx.Key)),
		text:      values[4].(func(string)),
		data:      values[5].(func(any)),
		sprite:    values[6].(func(spx.Sprite)),
		message:   values[7].(func(string, any)),
	}
	if len(values) == 9 {
		c.state = values[8].(func() []any)
	}
	return c
}

func newProcedureStopInterp(t testing.TB) *ixgo.Interp {
	t.Helper()
	ctx := ixgo.NewContext(ixgo.EnableCachedReg | ixgo.SupportMultipleInterp)
	i, err := ctx.LoadInterp("main.go", procedureStopSource)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(i.UnsafeRelease)
	return i
}
