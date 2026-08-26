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
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
)

var publishedAssetPaths = []struct {
	name string
	path func(Assets) string
}{
	{"Engine", func(a Assets) string { return a.EnginePath }},
	{"PCK", func(a Assets) string { return a.PackPath }},
	{"bridge", func(a Assets) string { return a.BridgePath }},
}

func TestLauncherPayloadRejectsSameSizePublishedMutationAfterAcquire(t *testing.T) {
	for _, test := range publishedAssetPaths {
		t.Run(test.name, func(t *testing.T) {
			fixture := newPublishedDriverFixture(t)
			cfg, project := publishedPayloadConfig(t)
			assets, err := acquirePublishedDriverWith(context.Background(), cfg, IO{}, fixture.lock, fixture.dependencies(cfg.RuntimeCacheRoot, fixture.fetcher(nil, new(int))))
			if err != nil {
				t.Fatal(err)
			}
			defer assets.Cleanup()
			mutateSameSizeFile(t, test.path(assets))
			var payload bytes.Buffer
			if _, _, err := writeLauncherPayload(t.TempDir(), &payload, cfg, assets, project, IO{}); err == nil || !strings.Contains(err.Error(), "SHA-256") {
				t.Fatalf("same-size %s mutation error = %v", test.name, err)
			}
		})
	}
}

func TestAssetsVerifyRejectsSameSizePublishedMutation(t *testing.T) {
	fixture := newPublishedDriverFixture(t)
	for _, test := range publishedAssetPaths {
		t.Run(test.name, func(t *testing.T) {
			assets := publishedPayloadAssets(t, fixture)
			if err := assets.Verify(); err != nil {
				t.Fatal(err)
			}
			mutateSameSizeFile(t, test.path(assets))
			if err := assets.Verify(); err == nil || !strings.Contains(err.Error(), "SHA-256") {
				t.Fatalf("same-size %s mutation verification error = %v", test.name, err)
			}
		})
	}
}

func mutateSameSizeFile(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	offset := len(data) / 2
	if len(data) == 0 {
		t.Fatal("cannot mutate an empty file")
	}
	data[offset] ^= 1
	file, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, writeErr := file.WriteAt(data[offset:offset+1], int64(offset))
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		t.Fatalf("mutate %s: write=%v close=%v", path, writeErr, closeErr)
	}
}
