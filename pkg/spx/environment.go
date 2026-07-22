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

package spx

import "sync/atomic"

var username atomic.Pointer[string]

// SetUsername sets the username exposed to the running project. An empty
// username represents an anonymous user.
func SetUsername(name string) {
	username.Store(&name)
}

// Username returns the configured username. It returns an empty string when no
// username has been configured.
func Username() string {
	name := username.Load()
	if name == nil {
		return ""
	}
	return *name
}
