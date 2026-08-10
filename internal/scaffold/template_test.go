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

package scaffold

import (
	"strings"
	"testing"

	"github.com/goplus/spx/v3/internal/release"
)

func TestGoModUsesDefaultSPXRelease(t *testing.T) {
	want := "require github.com/goplus/spx/v3 " + release.DefaultReleaseMeta().SPXVersion + " //xgo:class"
	if !strings.Contains(GoMod(), want) {
		t.Fatalf("go.mod template does not contain %q:\n%s", want, GoMod())
	}
}
