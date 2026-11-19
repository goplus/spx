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

package release

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestValidateEngineReuse(t *testing.T) {
	fixture := newRuntimeEngineFixture(t)
	source, _ := runtimeEngineManifestFixture(t)
	lock := DefaultRuntimeLock()
	source.Provenance.EngineSourceSHA256 = fixture.digest(t, "HEAD", lock)
	// A new pack and different Go tools do not change the compiled engine.
	source.Provenance.RuntimePackSourceSHA256 = strings.Repeat("0", 64)
	source.Provenance.BuildRecipeSHA256 = strings.Repeat("0", 64)
	target := cloneRuntimeLock(lock)
	target.RuntimeVersion = "99.0.0"
	target.Toolchain.Go = "1.99.0"
	target.Toolchain.XGo = "9.0.0"
	if err := ValidateEngineReuse(source, target, fixture.root, "HEAD"); err != nil {
		t.Fatalf("compatible engine rejected: %v", err)
	}
	for _, test := range []struct {
		name   string
		mutate func(*RuntimeManifest, *RuntimeLock)
		want   string
	}{
		{"unknown snapshot", func(m *RuntimeManifest, _ *RuntimeLock) { m.RuntimeVersion = "99.99.99" }, "no runtime lock snapshot"},
		{"source lock", func(m *RuntimeManifest, _ *RuntimeLock) { m.LockSHA256 = strings.Repeat("0", 64) }, "manifest lock SHA-256"},
		{"source repository", func(m *RuntimeManifest, _ *RuntimeLock) { m.ReleaseRepository = "example/runtime" }, "manifest repository"},
		{"target repository", func(_ *RuntimeManifest, l *RuntimeLock) { l.ReleaseRepository = "example/runtime" }, "engine source repository differs"},
		{"missing identity", func(m *RuntimeManifest, _ *RuntimeLock) { m.Provenance.EngineSourceSHA256 = "" }, "no engine identity"},
		{"different identity", func(m *RuntimeManifest, _ *RuntimeLock) { m.Provenance.EngineSourceSHA256 = strings.Repeat("0", 64) }, "engine source digest differs"},
		{"changed engine inputs", func(_ *RuntimeManifest, l *RuntimeLock) { l.RuntimeABI++ }, "engine source digest differs"},
		{"invalid target", func(_ *RuntimeManifest, l *RuntimeLock) { l.RuntimeABI = 0 }, "runtime ABI must be positive"},
		{"incomplete source", func(m *RuntimeManifest, _ *RuntimeLock) { m.Assets = m.Assets[1:] }, "release requires exactly"},
		{"additional target asset", func(_ *RuntimeManifest, l *RuntimeLock) {
			l.RequiredAssets = append(l.RequiredAssets, "zz-new-engine.zip")
		}, "release requires exactly"},
		{"different target assets", func(_ *RuntimeManifest, l *RuntimeLock) { l.RequiredAssets[0] = "another-engine.zip" }, "is not required"},
	} {
		t.Run(test.name, func(t *testing.T) {
			manifest, changed := source, cloneRuntimeLock(target)
			manifest.Assets = slices.Clone(source.Assets)
			test.mutate(&manifest, &changed)
			err := ValidateEngineReuse(manifest, changed, fixture.root, "HEAD")
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("reuse error = %v, want %q", err, test.want)
			}
		})
	}
	if err := ValidateEngineReuse(source, target, fixture.root, "missing"); err == nil || !strings.Contains(err.Error(), "list runtime engine source tree") {
		t.Fatalf("missing revision error = %v", err)
	}
}

func TestVerifyEngineFiles(t *testing.T) {
	for _, test := range []struct {
		name, want string
		mutate     func(*testing.T, *RuntimeManifest, string)
	}{
		{"complete engines", "", func(t *testing.T, m *RuntimeManifest, dir string) {}},
		{"obsolete pack absent", "", func(t *testing.T, m *RuntimeManifest, dir string) {
			if err := os.Remove(filepath.Join(dir, RuntimeAssetZipName)); err != nil {
				t.Fatal(err)
			}
		}},
		{"missing engine", "verify runtime asset", func(t *testing.T, m *RuntimeManifest, dir string) {
			if err := os.Remove(filepath.Join(dir, m.Assets[0].Name)); err != nil {
				t.Fatal(err)
			}
		}},
		{"changed engine bytes", "SHA-256", func(t *testing.T, m *RuntimeManifest, dir string) {
			// Keep the same size so checksum verification must detect the change.
			if err := os.WriteFile(filepath.Join(dir, m.Assets[0].Name), []byte(strings.Repeat("x", int(m.Assets[0].Size))), 0o644); err != nil {
				t.Fatal(err)
			}
		}},
		{"truncated engine", "size =", func(t *testing.T, m *RuntimeManifest, dir string) {
			if err := os.Truncate(filepath.Join(dir, m.Assets[0].Name), 0); err != nil {
				t.Fatal(err)
			}
		}},
		{"invalid manifest", "manifest schema", func(t *testing.T, m *RuntimeManifest, dir string) { m.Schema = 0 }},
	} {
		t.Run(test.name, func(t *testing.T) {
			manifest, dir := runtimeEngineManifestFixture(t)
			test.mutate(t, &manifest, dir)
			err := manifest.VerifyEngineFiles(dir)
			if test.want == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("verify error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestInheritedEngineProvenanceAndBytes(t *testing.T) {
	source, _ := runtimeEngineManifestFixture(t)
	target := source
	target.RuntimeVersion = "99.0.0"
	target.Provenance.SPXCommit = strings.Repeat("f", 40)
	target.Assets = slices.Clone(source.Assets)
	for i := range target.Assets {
		if target.Assets[i].Name == RuntimeAssetZipName {
			target.Assets[i].Size++
			target.Assets[i].SHA256 = strings.Repeat("0", 64)
		}
	}
	if err := target.InheritEngine(source); err != nil {
		t.Fatal(err)
	}
	if target.Provenance.EngineOrigin.SPXCommit != source.Provenance.SPXCommit || target.Provenance.EngineOrigin.RuntimeVersion != source.RuntimeVersion {
		t.Fatal("lost original engine provenance")
	}
	next := target
	next.RuntimeVersion = "99.0.1"
	next.Provenance.SPXCommit = strings.Repeat("1", 40)
	if err := next.InheritEngine(target); err != nil {
		t.Fatal(err)
	}
	if *next.Provenance.EngineOrigin != *target.Provenance.EngineOrigin {
		t.Fatal("reuse chain lost original build")
	}
	next.Provenance.EngineOrigin.SPXCommit = strings.Repeat("2", 40)
	if target.Provenance.EngineOrigin.SPXCommit != source.Provenance.SPXCommit || source.Provenance.EngineOrigin != nil {
		t.Fatal("inherited origin aliases source manifest")
	}
}

func TestInheritEngineRejectsChangedSourceOrTarget(t *testing.T) {
	original, _ := runtimeEngineManifestFixture(t)
	for _, test := range []struct {
		name   string
		mutate func(*RuntimeManifest, *RuntimeManifest)
		want   string
	}{
		{"invalid source", func(s, _ *RuntimeManifest) { s.Schema = 0 }, "manifest schema"},
		{"missing identity", func(s, _ *RuntimeManifest) { s.Provenance.EngineSourceSHA256 = "" }, "copied engine identity differs"},
		{"different identity", func(_, m *RuntimeManifest) { m.Provenance.EngineSourceSHA256 = strings.Repeat("0", 64) }, "copied engine identity differs"},
		{"missing source engine", func(s, _ *RuntimeManifest) { s.Assets = s.Assets[1:] }, "reused engine asset"},
		{"changed engine checksum", func(_, m *RuntimeManifest) { m.Assets[0].SHA256 = strings.Repeat("0", 64) }, "reused engine asset"},
		{"changed engine size", func(_, m *RuntimeManifest) { m.Assets[0].Size++ }, "reused engine asset"},
		{"invalid target", func(_, m *RuntimeManifest) { m.Schema = 0 }, "manifest schema"},
		{"self origin", func(s, m *RuntimeManifest) { m.RuntimeVersion = s.RuntimeVersion }, "invalid engine origin"},
	} {
		t.Run(test.name, func(t *testing.T) {
			source, target := original, original
			source.Assets, target.Assets = slices.Clone(original.Assets), slices.Clone(original.Assets)
			target.RuntimeVersion = "99.0.0"
			test.mutate(&source, &target)
			err := target.InheritEngine(source)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("inherit error = %v, want %q", err, test.want)
			}
		})
	}
}

func runtimeEngineManifestFixture(t *testing.T) (RuntimeManifest, string) {
	t.Helper()
	lock, provenance, inputs, dir := runtimeManifestFixture(t)
	provenance.EngineSourceSHA256 = strings.Repeat("e", 64)
	manifest, err := GenerateRuntimeManifest(lock, provenance, inputs)
	if err != nil {
		t.Fatal(err)
	}
	return manifest, dir
}
