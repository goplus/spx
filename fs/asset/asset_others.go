//go:build !android && !ios
// +build !android,!ios

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

package asset

import (
	"io"

	"github.com/goplus/spx/fs"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

// Open opens a local filesystem object.
func Open(base string) (fs.Dir, error) {
	return &FS{base: base + "/"}, nil
}

func openAsset(path string) (io.ReadSeekCloser, error) {
	return ebitenutil.OpenFile(path)
}
