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
	"strconv"
	"testing"

	"github.com/goplus/spx/v3/internal/release"
)

func TestToolchainComesFromRuntimeLock(t *testing.T) {
	lock := release.DefaultRuntimeLock()
	if requiredSConsVersion != lock.Toolchain.SCons {
		t.Fatalf("SCons version = %q, want locked %q", requiredSConsVersion, lock.Toolchain.SCons)
	}
	if requiredEMSDKVersion != lock.Toolchain.EMSDK {
		t.Fatalf("EMSDK version = %q, want locked %q", requiredEMSDKVersion, lock.Toolchain.EMSDK)
	}
	if androidNDKVersion != lock.Toolchain.AndroidNDK {
		t.Fatalf("Android NDK version = %q, want locked %q", androidNDKVersion, lock.Toolchain.AndroidNDK)
	}
	if got := strconv.Itoa(requiredJDKMajor); got != lock.Toolchain.JDK {
		t.Fatalf("JDK major = %q, want locked %q", got, lock.Toolchain.JDK)
	}
}

func TestAndroidNDKReleaseMapping(t *testing.T) {
	got, err := androidNDKReleaseForVersion("23.2.8568313")
	if err != nil {
		t.Fatal(err)
	}
	if got != "r23c" {
		t.Fatalf("release = %q, want r23c", got)
	}
	if _, err := androidNDKReleaseForVersion("99.0.0"); err == nil {
		t.Fatal("unknown Android NDK version did not fail closed")
	}
}
