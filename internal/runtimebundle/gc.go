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

import "context"

// GCOptions describes the quota/age policy reserved for the cross-process
// collector. Cache.Remove already uses the OS-backed shared/exclusive lock;
// quota/age selection remains future work.
type GCOptions struct {
	MaxBytes int64
	MaxAge   int64 // seconds; kept scalar so callers need not import time here
}

// GarbageCollector is the future quota/lease-aware cache collector seam.
type GarbageCollector interface {
	Collect(context.Context, GCOptions) error
}

// Collect returns ErrGCUnsupported until quota/age selection is implemented.
// This explicit failure is preferable to a collector that can race a running
// provider.
func (c *Cache) Collect(ctx context.Context, options GCOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return ErrGCUnsupported
}
