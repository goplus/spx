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

package dockercmd

import (
	"os"

	"github.com/goplus/spx/v2/internal/cmd/buildctl/shared"
)

var osStderr = os.Stderr

var errUsage = shared.ErrUsage

func findRepoRoot() (string, error) { return shared.FindRepoRoot() }
func fileExists(path string) bool   { return shared.FileExists(path) }
