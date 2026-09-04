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
	"fmt"
	"io"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/goplus/spx/v3/internal/driverbundle"
	"github.com/goplus/spx/v3/internal/release"
	"github.com/goplus/spx/v3/internal/runtimebundle"
)

type publishedDriverFixture struct {
	lock         release.RuntimeLock
	spec         release.HostRuntimeSpec
	manifest     driverbundle.Manifest
	manifestData []byte
	bundle       driverbundle.Bundle
	bundleData   []byte
	engine       []byte
	pack         []byte
	bridge       []byte
}

func newPublishedDriverFixture(t *testing.T) publishedDriverFixture {
	t.Helper()
	lock := release.DefaultRuntimeLock()
	spec, err := release.HostRuntimeSpecFor(lock, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	engine := []byte("published-engine-" + runtime.GOOS + "-" + runtime.GOARCH)
	pack := []byte("published-pack")
	bridge := []byte("published-bridge")
	bridgeName, err := bridgeFileName(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	bundlePath := filepath.Join(root, "bundle.zip")
	bundleData := writeRuntimeZip(t, bundlePath,
		runtimeZipEntry{Name: spec.RuntimeName, Mode: 0o755, Data: engine},
		runtimeZipEntry{Name: spec.PackName, Mode: 0o644, Data: pack},
		runtimeZipEntry{Name: bridgeName, Mode: 0o755, Data: bridge},
	)
	interfaceDigest, err := driverbundle.ComputeEngineInterfaceDigestFromSHA256(digestBytes(engine), digestBytes(pack))
	if err != nil {
		t.Fatal(err)
	}
	hostBundle := driverbundle.Bundle{
		GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, Name: "spx-driver-" + runtime.GOOS + "-" + runtime.GOARCH + ".zip",
		Size: int64(len(bundleData)), SHA256: digestBytes(bundleData), EngineInterfaceDigest: interfaceDigest,
		Files: []driverbundle.File{
			{Name: spec.RuntimeName, Mode: 0o755, Size: int64(len(engine)), SHA256: digestBytes(engine)},
			{Name: spec.PackName, Mode: 0o644, Size: int64(len(pack)), SHA256: digestBytes(pack)},
			{Name: bridgeName, Mode: 0o755, Size: int64(len(bridge)), SHA256: digestBytes(bridge)},
		},
	}
	bundles := []driverbundle.Bundle{hostBundle}
	for _, target := range []struct{ goos, goarch string }{{"darwin", "amd64"}, {"darwin", "arm64"}, {"linux", "amd64"}, {"windows", "amd64"}} {
		if target.goos == runtime.GOOS && target.goarch == runtime.GOARCH {
			continue
		}
		names := driverFixtureFileNames(lock.RuntimeVersion, target.goos, target.goarch)
		otherEngine, otherPack, otherBridge := []byte("other-engine"), []byte("other-pack"), []byte("other-bridge")
		otherInterface, err := driverbundle.ComputeEngineInterfaceDigestFromSHA256(digestBytes(otherEngine), digestBytes(otherPack))
		if err != nil {
			t.Fatal(err)
		}
		bundles = append(bundles, driverbundle.Bundle{
			GOOS: target.goos, GOARCH: target.goarch, Name: "spx-driver-" + target.goos + "-" + target.goarch + ".zip",
			Size: 100, SHA256: strings.Repeat("1", 64), EngineInterfaceDigest: otherInterface,
			Files: []driverbundle.File{
				{Name: names[0], Mode: 0o755, Size: int64(len(otherEngine)), SHA256: digestBytes(otherEngine)},
				{Name: names[1], Mode: 0o644, Size: int64(len(otherPack)), SHA256: digestBytes(otherPack)},
				{Name: names[2], Mode: 0o755, Size: int64(len(otherBridge)), SHA256: digestBytes(otherBridge)},
			},
		})
	}
	for i := 1; i < len(bundles); i++ {
		for j := i; j > 0 && bundles[j].GOOS+"/"+bundles[j].GOARCH < bundles[j-1].GOOS+"/"+bundles[j-1].GOARCH; j-- {
			bundles[j], bundles[j-1] = bundles[j-1], bundles[j]
		}
	}
	manifest := driverbundle.Manifest{
		Schema: driverbundle.ManifestSchema, SPXVersion: "v3.2.4",
		RuntimeVersion: lock.RuntimeVersion, Bundles: bundles,
	}
	manifestData, err := manifest.JSON()
	if err != nil {
		t.Fatal(err)
	}
	return publishedDriverFixture{lock: lock, spec: spec, manifest: manifest, manifestData: manifestData, bundle: hostBundle, bundleData: bundleData, engine: engine, pack: pack, bridge: bridge}
}

func driverFixtureFileNames(runtimeVersion, goos, goarch string) [3]string {
	engine := "gdspxrt" + runtimeVersion
	if goos == "windows" {
		engine += ".exe"
	}
	extension := map[string]string{"darwin": ".dylib", "linux": ".so", "windows": ".dll"}[goos]
	return [3]string{engine, "gdspxrt" + runtimeVersion + ".pck", "gdspx-" + goos + "-" + goarch + extension}
}

func (f publishedDriverFixture) fetcher(replacements map[string][]byte, calls *int) runtimebundle.FetchFunc {
	return func(ctx context.Context, url string, dst io.Writer) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		(*calls)++
		name := path.Base(url)
		data := replacements[name]
		if data == nil {
			switch name {
			case driverbundle.ManifestName:
				data = f.manifestData
			case f.bundle.Name:
				data = f.bundleData
			}
		}
		if len(data) == 0 {
			return fmt.Errorf("fixture has no driver asset %q", name)
		}
		_, err := io.Copy(dst, bytes.NewReader(data))
		return err
	}
}

func (f publishedDriverFixture) dependencies(cacheRoot string, fetch runtimebundle.FetchFunc) driverAssetDependencies {
	return driverAssetDependencies{fetch: fetch, cacheRoot: func() string { return cacheRoot }}
}

func publishedDriverTestConfig(cacheRoot string) Config {
	return Config{
		RuntimeCacheRoot: cacheRoot,
		Source: SourceIdentity{
			SelectedPath: driverbundle.SPXModulePath, EffectivePath: driverbundle.SPXModulePath,
			SelectedVersion: "v3.2.4", EffectiveVersion: "v3.2.4",
		},
	}
}
