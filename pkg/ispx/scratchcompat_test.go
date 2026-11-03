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
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/goplus/ixgo"
	"github.com/goplus/ixgo/xgobuild"
	"github.com/goplus/spx/v4"
	"github.com/goplus/spx/v4/pkg/scratchcompat"
)

// Exercise bindings and type identity here; semantic matrices live in internal/scratch.
const scratchCompatSource = `import (
	"github.com/goplus/spx/v4"
	compat "github.com/goplus/spx/v4/pkg/scratchcompat"
)

type UserSprite struct { spx.SpriteImpl }
func (*UserSprite) Main() {}
type CustomSprite struct { UserSprite }
func (*CustomSprite) ResolveCostumeIndex(name string) int { return len(name) }
func stringify(v interface { String() string }) string { return v.String() }

func main() {
	const all int8 = compat.All
	if all != -3 || compat.Invalid != spx.Invalid || compat.Last != spx.Last || compat.Random != spx.Random {
		panic("list constant exports")
	}
	if compat.PenColorParamFromString("COLOR") != spx.PenHue || compat.KeyFromString("space") != spx.KeySpace {
		panic("compatible constants and parsing")
	}
	var sprite spx.Sprite = &UserSprite{}
	for _, resolver := range []compat.CostumeResolver{&spx.SpriteImpl{}, sprite} {
		if compat.ResolveCostumeIndex(resolver, "3") != 2 {
			panic("native and interpreted costume resolvers")
		}
	}
	var custom spx.Sprite = &CustomSprite{}
	if compat.ResolveCostumeIndex(custom, "costume") != 7 {
		panic("overridden costume resolver")
	}
	defer spx.ResetRandomSeed()
	spx.SetRandomSeed(17)
	want := spx.Rand__1(1, 2)
	spx.SetRandomSeed(17)
	if compat.Rand(spx.NewValue("1.0"), 2) != want {
		panic("wrapped random operand and runtime source")
	}
	if compat.CharAt("中文", 1) != "中" {
		panic("character binding")
	}
	number := spx.NewValue("03.5")
	var legacy spx.List = compat.NewList("APPLE", number)
	var items compat.List = legacy
	var valueNumber interface { Float() float64 } = number
	items.Set(compat.Last, number)
	if stringify(&items) != "APPLE 03.5" || valueNumber.Float() != 3.5 || items.At(1).Raw() != "03.5" {
		panic("alias identity and methods")
	}
	var copied compat.List
	copied.Init([]any{"init", 7}...)
	if copied.String() != "init 7" {
		panic("variadic list initialization")
	}
	copied.InitFrom(&legacy)
	var restored spx.List
	restored.InitFrom(&copied)
	if copied.At(1).Raw() != "03.5" || restored.String() != "APPLE 03.5" {
		panic("InitFrom pointer arguments across aliases")
	}
	if !compat.Equal(number, 3.5) || compat.Compare(items.At(0), "apple") != 0 || !compat.Contains("Scratch", "RAT") {
		panic("wrapped operator arguments")
	}
	if compat.Div(0, 0) != 0 || compat.FloorMod(-7, 3) != 2 || compat.LimitPrecision(42 + 5e-10) != 42 {
		panic("numeric operator bindings")
	}
	probe := spx.NewValue("dispatch probe")
	list := compat.NewList(probe)
	if list.String() != "probe" || stringify(&list) != "probe" || probe.String() != "probe" || stringify(probe) != "probe" {
		panic("static and interface method dispatch")
	}
}
`

// A native Stringer lets the test observe the real generated adapter without
// replacing global registrations or adding production instrumentation.
type scratchDispatchProbe func() string

func (p scratchDispatchProbe) String() string { return p() }

func scratchAdapterOnStack(adapter string) bool {
	var pcs [32]uintptr
	frames := runtime.CallersFrames(pcs[:runtime.Callers(2, pcs[:])])
	for {
		frame, more := frames.Next()
		if strings.HasSuffix(frame.Function, "/internal/scratch."+adapter) {
			return true
		}
		if !more {
			return false
		}
	}
}

func TestInterpreterScratchCompat(t *testing.T) {
	for _, execution := range []string{"direct", "fallback"} {
		for _, optimization := range []string{"default", "optimized"} {
			t.Run(execution+"/"+optimization, func(t *testing.T) {
				isolateInterpreter(t)
				ctx := ixgo.NewContext(ixgo.SupportMultipleInterp | ixgo.EnableCachedReg | xgobuild.StaticLoad)
				called := make(map[string]bool)
				probeCalls := 0
				ctx.RegisterExternal("github.com/goplus/spx/v4.NewValue", func(v any) spx.Value {
					if value, ok := v.(string); ok && value == "dispatch probe" {
						v = scratchDispatchProbe(func() string {
							adapters := []string{"method_ptr_List_String", "method_ptr_List_String", "method_Value_String", "method_Value_String"}
							if probeCalls >= len(adapters) {
								t.Error("unexpected method probe call")
								return "probe"
							}
							seen := scratchAdapterOnStack(adapters[probeCalls])
							// Context overrides affect static calls; interface calls still
							// use the registered native adapter in both execution modes.
							wantDirect := execution == "direct" || probeCalls%2 == 1
							if seen != wantDirect {
								t.Errorf("method probe %d: direct adapter = %v, want %v", probeCalls, seen, wantDirect)
							}
							probeCalls++
							return "probe"
						})
					}
					return spx.NewValue(v)
				})
				if execution == "fallback" {
					// Context overrides bypass generated direct-call adapters.
					for name, fn := range map[string]any{
						"github.com/goplus/spx/v4/pkg/scratchcompat.CharAt":                  scratchcompat.CharAt,
						"github.com/goplus/spx/v4/pkg/scratchcompat.Compare":                 scratchcompat.Compare,
						"github.com/goplus/spx/v4/pkg/scratchcompat.Contains":                scratchcompat.Contains,
						"github.com/goplus/spx/v4/pkg/scratchcompat.Div":                     scratchcompat.Div,
						"github.com/goplus/spx/v4/pkg/scratchcompat.Equal":                   scratchcompat.Equal,
						"github.com/goplus/spx/v4/pkg/scratchcompat.FloorMod":                scratchcompat.FloorMod,
						"github.com/goplus/spx/v4/pkg/scratchcompat.KeyFromString":           scratchcompat.KeyFromString,
						"github.com/goplus/spx/v4/pkg/scratchcompat.LimitPrecision":          scratchcompat.LimitPrecision,
						"github.com/goplus/spx/v4/pkg/scratchcompat.NewList":                 scratchcompat.NewList,
						"github.com/goplus/spx/v4/pkg/scratchcompat.PenColorParamFromString": scratchcompat.PenColorParamFromString,
						"github.com/goplus/spx/v4/pkg/scratchcompat.Rand":                    scratchcompat.Rand,
						"github.com/goplus/spx/v4/pkg/scratchcompat.ResolveCostumeIndex":     scratchcompat.ResolveCostumeIndex,
						"(*github.com/goplus/spx/v4/internal/scratch.List).Init":             (*spx.List).Init,
						"(*github.com/goplus/spx/v4/internal/scratch.List).InitFrom":         (*spx.List).InitFrom,
						"(*github.com/goplus/spx/v4/internal/scratch.List).String":           (*spx.List).String,
						"(github.com/goplus/spx/v4/internal/scratch.Value).String":           spx.Value.String,
					} {
						called[name] = false
						native := reflect.ValueOf(fn)
						wrapped := reflect.MakeFunc(native.Type(), func(args []reflect.Value) []reflect.Value {
							called[name] = true
							if native.Type().IsVariadic() {
								return native.CallSlice(args)
							}
							return native.Call(args)
						})
						ctx.RegisterExternal(name, wrapped.Interface())
					}
				}
				var options []InitOption
				if optimization == "optimized" {
					options = append(options, WithSSAOptimization())
				}
				if err := Init(ctx, options...); err != nil {
					t.Fatal(err)
				}
				if err := Build(map[string][]byte{"main.xgo": []byte(scratchCompatSource)}); err != nil {
					t.Fatal(err)
				}
				if code, err := Run(); code != 0 || err != nil {
					t.Fatalf("Run() = %d, %v", code, err)
				}
				if probeCalls != 4 {
					t.Errorf("method probe calls = %d, want 4", probeCalls)
				}
				for name, seen := range called {
					if !seen {
						t.Errorf("%s fallback was not called", name)
					}
				}
			})
		}
	}
}
