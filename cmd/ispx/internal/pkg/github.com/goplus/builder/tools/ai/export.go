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

package ai

import (
	q "github.com/goplus/builder/tools/ai"

	"github.com/goplus/ixgo"
	"go/constant"
	"reflect"
)

func init() {
	ixgo.RegisterPackage(&ixgo.Package{
		Name: "ai",
		Path: "github.com/goplus/builder/tools/ai",
		Deps: map[string]string{
			"context":                          "context",
			"errors":                           "errors",
			"fmt":                              "fmt",
			"github.com/goplus/spx/v3/pkg/spx": "spx",
			"iter":                             "iter",
			"log":                              "log",
			"math":                             "math",
			"math/rand/v2":                     "rand",
			"reflect":                          "reflect",
			"slices":                           "slices",
			"strconv":                          "strconv",
			"strings":                          "strings",
			"sync":                             "sync",
			"time":                             "time",
		},
		Interfaces: map[string]reflect.Type{
			"Transport": reflect.TypeOf((*q.Transport)(nil)).Elem(),
		},
		NamedTypes: map[string]reflect.Type{
			"ArchivedHistory":      reflect.TypeOf((*q.ArchivedHistory)(nil)).Elem(),
			"CommandParamSpec":     reflect.TypeOf((*q.CommandParamSpec)(nil)).Elem(),
			"CommandResult":        reflect.TypeOf((*q.CommandResult)(nil)).Elem(),
			"CommandSpec":          reflect.TypeOf((*q.CommandSpec)(nil)).Elem(),
			"Player":               reflect.TypeOf((*q.Player)(nil)).Elem(),
			"Request":              reflect.TypeOf((*q.Request)(nil)).Elem(),
			"Response":             reflect.TypeOf((*q.Response)(nil)).Elem(),
			"TooManyRequestsError": reflect.TypeOf((*q.TooManyRequestsError)(nil)).Elem(),
			"Turn":                 reflect.TypeOf((*q.Turn)(nil)).Elem(),
		},
		AliasTypes: map[string]reflect.Type{},
		Vars: map[string]reflect.Value{
			"Break":              reflect.ValueOf(&q.Break),
			"ErrTransportNotSet": reflect.ValueOf(&q.ErrTransportNotSet),
		},
		Funcs: map[string]reflect.Value{
			"DefaultKnowledgeBase":    reflect.ValueOf(q.DefaultKnowledgeBase),
			"DefaultTransport":        reflect.ValueOf(q.DefaultTransport),
			"PlayerOnCmd_":            reflect.ValueOf(q.PlayerOnCmd_),
			"RetryAfterFromHeader":    reflect.ValueOf(q.RetryAfterFromHeader),
			"SetDefaultKnowledgeBase": reflect.ValueOf(q.SetDefaultKnowledgeBase),
			"SetDefaultTransport":     reflect.ValueOf(q.SetDefaultTransport),
		},
		TypedConsts: map[string]ixgo.TypedConst{},
		UntypedConsts: map[string]ixgo.UntypedConst{
			"GopPackage": {Typ: "untyped bool", Value: constant.MakeBool(bool(q.GopPackage))},
		},
	})
}
