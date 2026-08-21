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

import "sync"

// Materialized is a verified cache path together with its shared use lease.
// The lease must remain open for as long as the caller may read or execute
// files below Path; Close is safe to call more than once and from concurrent
// goroutines.
type Materialized struct {
	Path string

	lease     LockLease
	closeOnce sync.Once
	closeErr  error
}

// Close releases the shared use lease. A zero Materialized is a no-op.
func (m *Materialized) Close() error {
	if m == nil {
		return nil
	}
	m.closeOnce.Do(func() {
		if m.lease != nil {
			m.closeErr = m.lease.Close()
			m.lease = nil
		}
	})
	return m.closeErr
}
