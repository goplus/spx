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

package runtimebundle

import (
	"fmt"
	"io"

	"github.com/goplus/spx/v3/internal/zippreflight"
)

func preflightZipArchive(reader io.ReaderAt, size int64, limits Limits) error {
	err := zippreflight.Check(reader, size, zippreflight.Limits{
		MaxArchiveBytes:          limits.MaxArchiveBytes,
		MaxCentralDirectoryBytes: limits.MaxCentralDirectoryBytes,
		MaxEntries:               limits.MaxEntries,
	})
	if zippreflight.IsLimit(err) {
		return fmt.Errorf("%w: %v", ErrArchiveLimit, err)
	}
	return err
}
