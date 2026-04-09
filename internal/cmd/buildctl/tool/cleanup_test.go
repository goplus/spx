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
	"os"
	"path/filepath"
	"testing"
)

func TestCleanInstalledAssetsRemovesKnownArtifactsOnly(t *testing.T) {
	root := t.TempDir()
	gopath := filepath.Join(root, "gopath")
	t.Setenv("GOPATH", gopath)
	version := mustDefaultRuntimeVersion(t)

	binDir := filepath.Join(gopath, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}

	toCreate := []string{
		"spx",
		"ispx",
		"ispx.wasm",
		"runtime.gdextension",
		"gdspx" + version,
		"gdspx" + version + "_webpack.zip",
		"gdspxrt" + version,
		"gdspxrt" + version + ".pck",
		"gdspxrt" + version + "_webnormal/engine.js",
	}
	for _, rel := range toCreate {
		dst := filepath.Join(binDir, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			t.Fatalf("MkdirAll(%s) returned error: %v", dst, err)
		}
		if err := os.WriteFile(dst, []byte("data"), 0o644); err != nil {
			t.Fatalf("WriteFile(%s) returned error: %v", dst, err)
		}
	}

	keep := filepath.Join(binDir, "xgotest")
	if err := os.WriteFile(keep, []byte("keep"), 0o644); err != nil {
		t.Fatalf("WriteFile(%s) returned error: %v", keep, err)
	}

	if err := cleanInstalledAssets(); err != nil {
		t.Fatalf("cleanInstalledAssets returned error: %v", err)
	}

	for _, rel := range []string{
		"spx",
		"ispx",
		"ispx.wasm",
		"runtime.gdextension",
		"gdspx" + version,
		"gdspx" + version + "_webpack.zip",
		"gdspxrt" + version,
		"gdspxrt" + version + ".pck",
		"gdspxrt" + version + "_webnormal",
	} {
		if fileExists(filepath.Join(binDir, rel)) {
			t.Fatalf("expected %s to be removed", rel)
		}
	}

	if !fileExists(keep) {
		t.Fatalf("expected unrelated file to remain: %s", keep)
	}
}
