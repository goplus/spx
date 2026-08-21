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
	"crypto/sha256"
	"fmt"
	"io"
)

func hashReader(reader io.Reader, maxSize int64) (string, int64, error) {
	hasher := sha256.New()
	count, err := io.Copy(hasher, io.LimitReader(reader, limitWithOverflow(maxSize)))
	if err != nil {
		return "", count, err
	}
	if count > maxSize {
		return "", count, fmt.Errorf("%w: file exceeds expected size %d", ErrDigestMismatch, maxSize)
	}
	return fmt.Sprintf("%x", hasher.Sum(nil)), count, nil
}
