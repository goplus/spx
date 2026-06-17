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

package spx

import (
	q "github.com/goplus/spx/v2/pkg/spx"

	"reflect"

	"github.com/goplus/ixgo"
)

func init() {
	ixgo.RegisterPackage(&ixgo.Package{
		Name: "spx",
		Path: "github.com/goplus/spx/v2/pkg/spx",
		Deps: map[string]string{
			"context": "context",
			"github.com/goplus/spx/v2/internal/engine": "engine",
			"time": "time",
		},
		Interfaces: map[string]reflect.Type{},
		NamedTypes: map[string]reflect.Type{},
		AliasTypes: map[string]reflect.Type{},
		Vars:       map[string]reflect.Value{},
		Funcs: map[string]reflect.Value{
			"Execute":            reflect.ValueOf(q.Execute),
			"ExecuteNative":      reflect.ValueOf(q.ExecuteNative),
			"Go":                 reflect.ValueOf(q.Go),
			"IsAbortThreadError": reflect.ValueOf(q.IsAbortThreadError),
			"IsInCoroutine":      reflect.ValueOf(q.IsInCoroutine),
			"Wait":               reflect.ValueOf(q.Wait),
			"WaitNextFrame":      reflect.ValueOf(q.WaitNextFrame),
		},
		TypedConsts:   map[string]ixgo.TypedConst{},
		UntypedConsts: map[string]ixgo.UntypedConst{},
	})
}
