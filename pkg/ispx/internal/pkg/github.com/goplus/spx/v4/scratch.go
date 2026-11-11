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

package spx

import (
	"reflect"

	"github.com/goplus/ixgo"
	"github.com/goplus/spx/v4/internal/core/scratch"

	// This parent package can import the generated adapters beneath internal/.
	_ "github.com/goplus/spx/v4/pkg/ispx/internal/pkg/github.com/goplus/spx/v4/internal/core/scratch"
)

// This hand-written bridge keeps registration limited to the public aliases'
// native types while loading their generated direct-call adapters.
func init() {
	ixgo.RegisterPackage(&ixgo.Package{
		Name: "scratch",
		Path: "github.com/goplus/spx/v4/internal/core/scratch",
		NamedTypes: map[string]reflect.Type{
			"List":  reflect.TypeFor[scratch.List](),
			"Value": reflect.TypeFor[scratch.Value](),
		},
	})
}
