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

import "fmt"

type resolvedLimits struct {
	maxEntries      int
	maxFileBytes    int64
	maxTotalBytes   int64
	maxArchiveBytes int64
}

func resolveLimits(limits Limits) (resolvedLimits, error) {
	resolved := resolvedLimits{
		maxEntries:      limits.MaxEntries,
		maxFileBytes:    limits.MaxFileBytes,
		maxTotalBytes:   limits.MaxTotalBytes,
		maxArchiveBytes: limits.MaxArchiveBytes,
	}
	if resolved.maxEntries == 0 {
		resolved.maxEntries = defaultMaxEntries
	}
	if resolved.maxFileBytes == 0 {
		resolved.maxFileBytes = defaultMaxFileBytes
	}
	if resolved.maxTotalBytes == 0 {
		resolved.maxTotalBytes = defaultMaxTotalBytes
	}
	if resolved.maxArchiveBytes == 0 {
		resolved.maxArchiveBytes = defaultMaxArchiveBytes
	}
	if resolved.maxEntries < 0 || resolved.maxFileBytes < 0 || resolved.maxTotalBytes < 0 || resolved.maxArchiveBytes < 0 {
		return resolvedLimits{}, fmt.Errorf("%w: limits must not be negative", ErrLimit)
	}
	const maxInt64 = int64(^uint64(0) >> 1)
	if resolved.maxFileBytes == maxInt64 || resolved.maxTotalBytes == maxInt64 {
		return resolvedLimits{}, fmt.Errorf("%w: file and total byte limits must be less than MaxInt64", ErrLimit)
	}
	return resolved, nil
}
