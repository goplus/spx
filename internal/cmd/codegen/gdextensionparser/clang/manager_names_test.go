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

package clang

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestManagerNamesLongestPrefixAndFallback(t *testing.T) {
	names := []string{"foo", "foobar", "ui"}
	matcher := NewManagerNames(names)
	names[1] = "changed"
	for _, tt := range []struct{ name, want string }{
		{"GDExtensionSpxFooBarShow", "foobar"},
		{"GDExtensionSpxFooShow", "foo"},
		{"GDExtensionSpxUiBindNode", "ui"},
		{"GDExtensionSpxCameraGetPosition", "camera"},
	} {
		require.Equal(t, tt.want, matcher.Resolve(tt.name))
		require.Equal(t, tt.want, matcher.resolveASCII(tt.name))
	}
	// Preserve the two historical fallbacks for manually constructed identifiers.
	require.Equal(t, "ab", (ManagerNames{}).Resolve("GDExtensionSpxAbÉShow"))
	require.Equal(t, "abé", (ManagerNames{}).resolveASCII("GDExtensionSpxAbÉShow"))
}

func TestCollectManagerFunctionsUsesExactLongestMatch(t *testing.T) {
	ast, err := ParseCString(`typedef void (*GDExtensionSpxFooShow)();
 typedef void (*GDExtensionSpxFoobarShow)();`)
	require.NoError(t, err)
	names := NewManagerNames([]string{"foo", "foobar"})
	functions := ast.CollectGDExtensionManagerFunctions("foo", names)
	require.Len(t, functions, 1)
	require.Equal(t, "GDExtensionSpxFooShow", functions[0].Name)
}
