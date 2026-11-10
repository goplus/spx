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

package scratchcompat_test

import (
	"testing"

	"github.com/goplus/spx/v4"
	"github.com/goplus/spx/v4/pkg/scratchcompat"
)

// All remains untyped, so callers can use it with other integer types.
const _ int8 = scratchcompat.All

func TestListTypeIdentity(t *testing.T) {
	//lint:ignore SA1019 Verify type identity and interoperability with the legacy API.
	var native spx.List = scratchcompat.NewList("first", "second")
	var compatible scratchcompat.List = native
	//lint:ignore SA1019 Verify type identity and interoperability with the legacy API.
	var nativePointer *spx.List = &compatible
	var compatiblePointer *scratchcompat.List = nativePointer
	if compatible.At(0).String() != "first" || compatible.At(1).String() != "second" {
		t.Fatal("list indexes must remain 0-based")
	}
	nativePointer.Set(scratchcompat.Last, "last")
	if compatiblePointer.At(1).String() != "last" || native.At(1).String() != "last" {
		t.Fatal("list aliases did not share element storage")
	}

	items := []any{"original"}
	compatiblePointer.Init(items...)
	items[0] = "changed"
	if nativePointer.At(0).String() != "changed" {
		t.Fatal("Init must retain the supplied element storage")
	}
}

func TestListInitFromAcrossPackages(t *testing.T) {
	//lint:ignore SA1019 Verify type identity and interoperability with the legacy API.
	var source spx.List
	source.Init(spx.NewValue("first"), "second")
	var copied scratchcompat.List
	copied.InitFrom(&source)
	if got := copied.At(0).Raw(); got != spx.NewValue("first") {
		t.Fatalf("InitFrom must preserve wrapped elements: got %#v", got)
	}
	source.Set(0, "source")
	copied.Set(1, "copy")
	if source.String() != "source second" || copied.String() != "first copy" {
		t.Fatalf("InitFrom shared storage: source=%q, copy=%q", source.String(), copied.String())
	}
	//lint:ignore SA1019 Verify type identity and interoperability with the legacy API.
	var nativeCopy spx.List
	nativeCopy.InitFrom(&copied)
	copied.Set(0, "changed")
	if nativeCopy.At(0).String() != "first" {
		t.Fatal("InitFrom into spx.List shared element storage")
	}
}

func TestNewListOwnsStorageAndUnwrapsOnce(t *testing.T) {
	for name, newList := range map[string]func(...any) scratchcompat.List{
		//lint:ignore SA1019 Verify both constructors retain the same storage and unwrapping semantics.
		"spx":           spx.NewList,
		"scratchcompat": scratchcompat.NewList,
	} {
		t.Run(name, func(t *testing.T) {
			var nested scratchcompat.List
			nested.Init(spx.NewValue("nested"))
			items := []any{spx.NewValue("wrapped"), "tail", nested.At(0)}
			list := newList(items...)
			if got := list.At(0).Raw(); got != "wrapped" {
				t.Fatalf("NewList retained a wrapped value: %T(%v)", got, got)
			}
			if got, ok := list.At(2).Raw().(spx.Value); !ok || got.Raw() != "nested" {
				t.Fatal("NewList must unwrap exactly one Value layer")
			}

			items[1] = "caller"
			list.Set(0, "list")
			if got := list.At(1).String(); got != "tail" {
				t.Fatalf("caller changed list storage: got %q", got)
			}
			if got, ok := items[0].(spx.Value); !ok || got.Raw() != "wrapped" {
				t.Fatal("NewList or Set changed the supplied element storage")
			}
		})
	}
}
