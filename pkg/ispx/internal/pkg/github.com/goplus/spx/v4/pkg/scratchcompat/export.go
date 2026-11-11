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

// export by github.com/goplus/ixgo/cmd/qexp

package scratchcompat

import (
	q "github.com/goplus/spx/v4/pkg/scratchcompat"

	"go/constant"
	"reflect"

	_ "embed"
	"github.com/goplus/ixgo"
	"github.com/goplus/ixgo/xgobuild/typesdata"
	spx "github.com/goplus/spx/v4"
)

//go:embed export.types
var exportTypesData []byte

func init() {
	ixgo.RegisterPackage(&ixgo.Package{
		Name: "scratchcompat",
		Path: "github.com/goplus/spx/v4/pkg/scratchcompat",
		Deps: map[string]string{
			"github.com/goplus/spx/v4":                       "spx",
			"github.com/goplus/spx/v4/internal/core/scratch": "scratch",
			"github.com/goplus/spx/v4/internal/engine":       "engine",
		},
		Interfaces: map[string]reflect.Type{
			"CostumeResolver":    reflect.TypeOf((*q.CostumeResolver)(nil)).Elem(),
			"SoundEffectChanger": reflect.TypeOf((*q.SoundEffectChanger)(nil)).Elem(),
			"SoundEffectSetter":  reflect.TypeOf((*q.SoundEffectSetter)(nil)).Elem(),
			"VolumeChanger":      reflect.TypeOf((*q.VolumeChanger)(nil)).Elem(),
			"VolumeSetter":       reflect.TypeOf((*q.VolumeSetter)(nil)).Elem(),
		},
		NamedTypes: map[string]reflect.Type{},
		AliasTypes: map[string]reflect.Type{
			"List": reflect.TypeOf((*q.List)(nil)).Elem(),
		},
		Vars: map[string]reflect.Value{},
		Funcs: map[string]reflect.Value{
			"ChangeSoundEffect":       reflect.ValueOf(q.ChangeSoundEffect),
			"ChangeVolume":            reflect.ValueOf(q.ChangeVolume),
			"CharAt":                  reflect.ValueOf(q.CharAt),
			"Compare":                 reflect.ValueOf(q.Compare),
			"Contains":                reflect.ValueOf(q.Contains),
			"Div":                     reflect.ValueOf(q.Div),
			"Equal":                   reflect.ValueOf(q.Equal),
			"FloorMod":                reflect.ValueOf(q.FloorMod),
			"KeyFromString":           reflect.ValueOf(q.KeyFromString),
			"LimitPrecision":          reflect.ValueOf(q.LimitPrecision),
			"NewList":                 reflect.ValueOf(q.NewList),
			"PenColorParamFromString": reflect.ValueOf(q.PenColorParamFromString),
			"Rand":                    reflect.ValueOf(q.Rand),
			"ResolveCostumeIndex":     reflect.ValueOf(q.ResolveCostumeIndex),
			"SetSoundEffect":          reflect.ValueOf(q.SetSoundEffect),
			"SetVolume":               reflect.ValueOf(q.SetVolume),
		},
		TypedConsts: map[string]ixgo.TypedConst{
			"Invalid": {Typ: reflect.TypeOf(q.Invalid), Value: constant.MakeInt64(int64(q.Invalid))},
			"Last":    {Typ: reflect.TypeOf(q.Last), Value: constant.MakeInt64(int64(q.Last))},
			"Random":  {Typ: reflect.TypeOf(q.Random), Value: constant.MakeInt64(int64(q.Random))},
		},
		UntypedConsts: map[string]ixgo.UntypedConst{
			"All": {Typ: "untyped int", Value: constant.MakeInt64(int64(q.All))},
		},
		Import: typesdata.ImportFunc("github.com/goplus/spx/v4/pkg/scratchcompat", exportTypesData),
	})
	ixgo.RegisterDirectCalls("github.com/goplus/spx/v4/pkg/scratchcompat", map[string]ixgo.DirectCallAdapter{
		"ChangeSoundEffect":       func_ChangeSoundEffect,
		"ChangeVolume":            func_ChangeVolume,
		"CharAt":                  func_CharAt,
		"Compare":                 func_Compare,
		"Contains":                func_Contains,
		"Div":                     func_Div,
		"Equal":                   func_Equal,
		"FloorMod":                func_FloorMod,
		"KeyFromString":           func_KeyFromString,
		"LimitPrecision":          func_LimitPrecision,
		"NewList":                 func_NewList,
		"PenColorParamFromString": func_PenColorParamFromString,
		"Rand":                    func_Rand,
		"ResolveCostumeIndex":     func_ResolveCostumeIndex,
		"SetSoundEffect":          func_SetSoundEffect,
		"SetVolume":               func_SetVolume,
	})
}

func func_ChangeSoundEffect(ctx ixgo.DirectCallContext) {
	q.ChangeSoundEffect(ixgo.DirectCallArg[q.SoundEffectChanger](ctx, 0), ixgo.DirectCallArg[spx.SoundEffectKind](ctx, 1), ixgo.DirectCallArg[float64](ctx, 2))
}

func func_ChangeVolume(ctx ixgo.DirectCallContext) {
	q.ChangeVolume(ixgo.DirectCallArg[q.VolumeChanger](ctx, 0), ixgo.DirectCallArg[float64](ctx, 1))
}

func func_CharAt(ctx ixgo.DirectCallContext) {
	ctx.SetResult(q.CharAt(ixgo.DirectCallArg[string](ctx, 0), ixgo.DirectCallArg[int](ctx, 1)))
}

func func_Compare(ctx ixgo.DirectCallContext) {
	ctx.SetResult(q.Compare(ixgo.DirectCallArg[any](ctx, 0), ixgo.DirectCallArg[any](ctx, 1)))
}

func func_Contains(ctx ixgo.DirectCallContext) {
	ctx.SetResult(q.Contains(ixgo.DirectCallArg[string](ctx, 0), ixgo.DirectCallArg[string](ctx, 1)))
}

func func_Div(ctx ixgo.DirectCallContext) {
	ctx.SetResult(q.Div(ixgo.DirectCallArg[float64](ctx, 0), ixgo.DirectCallArg[float64](ctx, 1)))
}

func func_Equal(ctx ixgo.DirectCallContext) {
	ctx.SetResult(q.Equal(ixgo.DirectCallArg[any](ctx, 0), ixgo.DirectCallArg[any](ctx, 1)))
}

func func_FloorMod(ctx ixgo.DirectCallContext) {
	ctx.SetResult(q.FloorMod(ixgo.DirectCallArg[float64](ctx, 0), ixgo.DirectCallArg[float64](ctx, 1)))
}

func func_KeyFromString(ctx ixgo.DirectCallContext) {
	ctx.SetResult(q.KeyFromString(ixgo.DirectCallArg[string](ctx, 0)))
}

func func_LimitPrecision(ctx ixgo.DirectCallContext) {
	ctx.SetResult(q.LimitPrecision(ixgo.DirectCallArg[float64](ctx, 0)))
}

func func_NewList(ctx ixgo.DirectCallContext) {
	ctx.SetResult(q.NewList(ixgo.DirectCallArg[[]any](ctx, 0)...))
}

func func_PenColorParamFromString(ctx ixgo.DirectCallContext) {
	ctx.SetResult(q.PenColorParamFromString(ixgo.DirectCallArg[string](ctx, 0)))
}

func func_Rand(ctx ixgo.DirectCallContext) {
	ctx.SetResult(q.Rand(ixgo.DirectCallArg[any](ctx, 0), ixgo.DirectCallArg[any](ctx, 1)))
}

func func_ResolveCostumeIndex(ctx ixgo.DirectCallContext) {
	ctx.SetResult(q.ResolveCostumeIndex(ixgo.DirectCallArg[q.CostumeResolver](ctx, 0), ixgo.DirectCallArg[string](ctx, 1)))
}

func func_SetSoundEffect(ctx ixgo.DirectCallContext) {
	q.SetSoundEffect(ixgo.DirectCallArg[q.SoundEffectSetter](ctx, 0), ixgo.DirectCallArg[spx.SoundEffectKind](ctx, 1), ixgo.DirectCallArg[float64](ctx, 2))
}

func func_SetVolume(ctx ixgo.DirectCallContext) {
	q.SetVolume(ixgo.DirectCallArg[q.VolumeSetter](ctx, 0), ixgo.DirectCallArg[float64](ctx, 1))
}
