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

package asset_test

import (
	"errors"
	iofs "io/fs"
	"testing"

	"github.com/goplus/spx/v4/fs/asset"
)

func TestOpenRejectsInvalidAssetPaths(t *testing.T) {
	dir, err := asset.Open("assets")
	if err != nil {
		t.Fatal(err)
	}
	defer dir.Close()
	for _, name := range []string{"../outside.png", "/shared/image.png", "C:/shared/image.png", `C:\shared\image.png`, "./C:/shared/image.png", "bad\x00/../shared/image.png"} {
		if _, err := dir.Open(name); !errors.Is(err, iofs.ErrInvalid) {
			t.Errorf("Open(%q) = %v, want invalid path", name, err)
		}
	}
}
