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

package projectbundle

import (
	"fmt"
	"io"
	"os"
)

func readRegularFile(file *os.File, name string, maxFileBytes, remainingTotalBytes int64) ([]byte, error) {
	before, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("projectbundle: fstat source %q: %w", name, err)
	}
	if !before.Mode().IsRegular() {
		return nil, fmt.Errorf("%w: source %q has mode %s", ErrUnsafeFile, name, before.Mode())
	}
	if before.Size() < 0 || before.Size() > maxFileBytes {
		return nil, fmt.Errorf("%w: source %q exceeds %d bytes", ErrLimit, name, maxFileBytes)
	}
	if before.Size() > remainingTotalBytes {
		return nil, fmt.Errorf("%w: source %q exceeds remaining total allowance of %d bytes", ErrLimit, name, remainingTotalBytes)
	}
	readLimit := min(maxFileBytes, remainingTotalBytes)

	data, err := io.ReadAll(io.LimitReader(file, readLimit+1))
	if err != nil {
		return nil, fmt.Errorf("projectbundle: read source %q: %w", name, err)
	}
	if int64(len(data)) > readLimit {
		return nil, fmt.Errorf("%w: source %q grew beyond its byte allowance", ErrLimit, name)
	}
	after, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("projectbundle: second fstat source %q: %w", name, err)
	}
	if !after.Mode().IsRegular() || !os.SameFile(before, after) || before.Mode() != after.Mode() || before.Size() != after.Size() ||
		!before.ModTime().Equal(after.ModTime()) || int64(len(data)) != after.Size() {
		return nil, fmt.Errorf("%w: source %q changed while it was read", ErrUnsafeFile, name)
	}
	return data, nil
}
