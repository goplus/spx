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

package projectpolicy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateConfig(t *testing.T) {
	projectDir := t.TempDir()
	if err := ValidateConfig(projectDir); err != nil {
		t.Fatalf("ValidateConfig() without .config: %v", err)
	}
	configPath := filepath.Join(projectDir, configName)
	if err := os.WriteFile(configPath, []byte(`{"name":"demo"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ValidateConfig(projectDir); err != nil {
		t.Fatalf("ValidateConfig() with supported fields: %v", err)
	}
	for _, key := range []string{"extasset", "ExtAsset", "EXTASSET"} {
		if err := os.WriteFile(configPath, []byte(`{"`+key+`":"../shared-assets"}`), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := ValidateConfig(projectDir); err == nil || !strings.Contains(err.Error(), "unsupported extasset") {
			t.Fatalf("ValidateConfig() key %q error = %v, want extasset rejection", key, err)
		}
	}
}

func TestValidateConfigRejectsSymlink(t *testing.T) {
	projectDir := t.TempDir()
	target := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(target, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(projectDir, configName)); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if err := ValidateConfig(projectDir); err == nil {
		t.Fatal("ValidateConfig accepted symlink .config")
	}
}
