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

package main

import (
	"os"
	"strings"
	"testing"
)

func TestInstallScriptDoesNotReferenceLegacyGengoTarget(t *testing.T) {
	content, err := os.ReadFile("install.sh")
	if err != nil {
		t.Fatalf("read install script: %v", err)
	}

	legacyTargets := []string{
		"internal/gengo/embedded_pkgs.go",
		"internal/embeddedpkgs",
	}
	for _, legacyTarget := range legacyTargets {
		if strings.Contains(string(content), legacyTarget) {
			t.Fatalf("install script should not reference removed target %s", legacyTarget)
		}
	}
}
