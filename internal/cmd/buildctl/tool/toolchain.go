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

package tool

import (
	"fmt"
	"strconv"

	"github.com/goplus/spx/v3/internal/release"
)

var (
	lockedToolchain      = release.DefaultRuntimeLock().Toolchain
	requiredSConsVersion = lockedToolchain.SCons
	requiredJDKMajor     = mustJDKMajor(lockedToolchain.JDK)
	requiredEMSDKVersion = lockedToolchain.EMSDK
	androidNDKVersion    = lockedToolchain.AndroidNDK
)

func mustJDKMajor(version string) int {
	major, err := strconv.Atoi(version)
	if err != nil || major <= 0 {
		panic(fmt.Sprintf("buildctl: unsupported locked JDK version %q", version))
	}
	return major
}

// Android publishes NDK archives under release aliases such as r23c while
// Godot verifies the corresponding full package revision. Keep that transport
// mapping fail-closed: changing runtime.lock cannot silently download a
// different NDK.
func androidNDKReleaseForVersion(version string) (string, error) {
	switch version {
	case "23.2.8568313":
		return "r23c", nil
	default:
		return "", fmt.Errorf("buildctl: no Android NDK archive mapping for locked version %q", version)
	}
}
