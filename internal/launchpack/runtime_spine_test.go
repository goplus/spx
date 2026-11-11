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

package launchpack

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/goplus/spx/v4/internal/runtimevariant"
)

func TestSpineAcquisitionNeverUsesPublishedRuntime(t *testing.T) {
	lock, spec := defaultHostRuntimeTest(t)
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	if err := os.MkdirAll(bin, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, runtimevariant.SpineMarker), nil, 0600); err != nil {
		t.Fatal(err)
	}
	calls := 0
	deps := localRuntimeTestDependencies(filepath.Join(root, "cache"), func(context.Context, string, io.Writer) error {
		calls++
		t.Error("published fetch attempted for Spine")
		return nil
	})
	env := []string{"SPX_SPINE=1", "GOPATH=" + root}
	if _, err := acquireRuntimeAssetsWith(context.Background(), Config{}, IO{Env: env}, lock, deps); err == nil {
		t.Fatal("missing local runtime should fail")
	}
	publishLocalRuntimeTest(t, bin, filepath.Join(root, "manifest.json"), spec, "spine-engine", "spine-pack")
	assets, err := acquireRuntimeAssetsWith(context.Background(), Config{}, IO{Env: env}, lock, deps)
	if err != nil {
		t.Fatal(err)
	}
	defer assets.Cleanup()
	if got := string(mustReadLaunchpackTestFile(t, assets.EnginePath)); got != "spine-engine" {
		t.Fatalf("selected engine = %q", got)
	}
	if calls != 0 {
		t.Fatalf("published fetches = %d", calls)
	}
	if _, err := acquireRuntimeAssetsWith(context.Background(), Config{}, IO{Env: []string{"GOPATH=" + root}}, lock, deps); err == nil {
		t.Fatal("marked workspace without flag accepted")
	}
	other := t.TempDir()
	manifest := publishLocalRuntimeTest(t, other, filepath.Join(other, "manifest.json"), spec, "default-engine", "default-pack")
	if _, err := acquireRuntimeAssetsWith(context.Background(), Config{RuntimeManifestPath: manifest}, IO{Env: env}, lock, deps); err == nil {
		t.Fatal("explicit default manifest accepted for Spine")
	}
}
