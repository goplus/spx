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
	"testing"

	"github.com/goplus/ixgo"
)

const procedureStopSource = `package main

import "github.com/goplus/spx/v3"

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
			got, err := i.RunFunc(test.name)
			if err != nil {
				t.Fatal(err)
			}
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
