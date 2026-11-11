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

package driverbundle

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

const testDigest = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func TestManifestRoundTripLookupAndURL(t *testing.T) {
	want := testManifest()
	data, err := want.JSON()
	if err != nil {
		t.Fatal(err)
	}
	got, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if got.SPXVersion != want.SPXVersion || len(got.Bundles) != 4 {
		t.Fatalf("parsed manifest = %#v", got)
	}
	bundle, err := got.BundleFor("linux", "amd64")
	if err != nil || bundle.Name != "spx-driver-linux-amd64.zip" {
		t.Fatalf("BundleFor = %#v, %v", bundle, err)
	}
	wantURL := "https://github.com/goplus/spx/releases/download/v3.2.4/spx-driver-linux-amd64.zip"
	if gotURL, err := got.DownloadURL("spx-driver-linux-amd64.zip"); err != nil || gotURL != wantURL {
		t.Fatalf("DownloadURL = %q, %v, want %q", gotURL, err, wantURL)
	}
	wantManifestURL := "https://github.com/goplus/spx/releases/download/v3.2.4/" + ManifestName
	if gotURL, err := ManifestURL(got.SPXVersion); err != nil || gotURL != wantManifestURL {
		t.Fatalf("ManifestURL = %q, %v, want %q", gotURL, err, wantManifestURL)
	}
	if _, err := got.DownloadURL("../bundle.zip"); err == nil {
		t.Fatal("DownloadURL accepted an unsafe bundle name")
	}
	if _, err := ManifestURL("v3.2"); err == nil {
		t.Fatal("ManifestURL accepted an invalid SPX version")
	}
	if _, err := ManifestURL("v0.0.0-20200101000000-abcdefabcdef"); err == nil {
		t.Fatal("ManifestURL accepted a pseudo-version")
	}
}

func TestBundleRoundTrip(t *testing.T) {
	want := testManifest().Bundles[0]
	data, err := want.JSON()
	if err != nil {
		t.Fatal(err)
	}
	got, err := ParseBundle(data)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != want.Name || len(got.Files) != 3 {
		t.Fatalf("bundle = %#v", got)
	}
	duplicate := want
	duplicate.Files[1] = duplicate.Files[0]
	if err := duplicate.Validate(); err == nil {
		t.Fatal("accepted duplicate bundle file")
	}
}

func TestManifestParseIsStrict(t *testing.T) {
	data, err := testManifest().JSON()
	if err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(string) string{
		"unknown": func(value string) string {
			return strings.TrimSuffix(value, "\n")[:len(strings.TrimSuffix(value, "\n"))-1] + `,"unknown":true}`
		},
		"trailing": func(value string) string { return value + `{}` },
		"duplicate nested": func(value string) string {
			return strings.Replace(value, `"name": "spx-driver-darwin-arm64.zip"`, `"name": "spx-driver-darwin-arm64.zip", "name": "spx-driver-darwin-arm64.zip"`, 1)
		},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse([]byte(mutate(string(data)))); err == nil {
				t.Fatal("accepted ambiguous JSON")
			}
		})
	}
	if _, err := Parse(make([]byte, MaxManifestSize+1)); err == nil {
		t.Fatal("Parse accepted an oversized manifest")
	}
	if _, err := ParseBundle(make([]byte, MaxManifestSize+1)); err == nil {
		t.Fatal("ParseBundle accepted an oversized descriptor")
	}
}

func TestManifestValidationMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Manifest)
	}{
		{"bundle count", func(m *Manifest) { m.Bundles = nil }},
		{"bundle size", func(m *Manifest) { m.Bundles[0].Size = 0 }},
		{"bundle digest", func(m *Manifest) { m.Bundles[0].SHA256 = strings.Repeat("z", 64) }},
		{"bundle identity", func(m *Manifest) { m.Bundles[0].Name = "other.zip" }},
		{"file identity", func(m *Manifest) { m.Bundles[0].Files[0].Name = "other" }},
		{"file path", func(m *Manifest) { m.Bundles[0].Files[0].Name = "../bad" }},
		{"schema", func(m *Manifest) { m.Schema++ }},
		{"version", func(m *Manifest) { m.SPXVersion = "v3.2" }},
		{"runtime", func(m *Manifest) { m.RuntimeVersion = "v2.4.4" }},
		{"platform duplicate", func(m *Manifest) { m.Bundles[1].GOOS = m.Bundles[0].GOOS; m.Bundles[1].GOARCH = m.Bundles[0].GOARCH }},
		{"platform order", func(m *Manifest) { m.Bundles[0], m.Bundles[1] = m.Bundles[1], m.Bundles[0] }},
		{"bundle path", func(m *Manifest) { m.Bundles[0].Name = "../bundle.zip" }},
		{"file duplicate", func(m *Manifest) { m.Bundles[0].Files[1].Name = m.Bundles[0].Files[0].Name }},
		{"file mode", func(m *Manifest) { m.Bundles[0].Files[0].Mode = 0o100755 }},
		{"file size", func(m *Manifest) { m.Bundles[0].Files[0].Size = 0 }},
		{"file digest", func(m *Manifest) { m.Bundles[0].Files[0].SHA256 = "bad" }},
		{"file count", func(m *Manifest) { m.Bundles[0].Files = m.Bundles[0].Files[:2] }},
		{"interface digest", func(m *Manifest) { m.Bundles[0].EngineInterfaceDigest = "bad" }},
		{"interface identity", func(m *Manifest) { m.Bundles[0].Files[0].SHA256 = strings.Repeat("b", 64) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := testManifest()
			test.mutate(&candidate)
			if err := candidate.Validate(); err == nil {
				t.Fatal("accepted invalid manifest")
			}
			if _, err := candidate.JSON(); err == nil {
				t.Fatal("serialized invalid manifest")
			}
			if _, err := candidate.BundleFor("linux", "amd64"); err == nil {
				t.Fatal("looked up invalid manifest")
			}
			if err := candidate.ValidateVersions("v3.2.4", "2.4.4"); err == nil {
				t.Fatal("bound invalid manifest")
			}
			data, err := json.Marshal(candidate)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := Parse(data); err == nil {
				t.Fatal("parsed invalid manifest")
			}
		})
	}
	if _, err := testManifest().BundleFor("linux", "arm64"); !errors.Is(err, ErrBundleNotFound) {
		t.Fatalf("BundleFor missing error = %v", err)
	}
}

func TestManifestValidateVersions(t *testing.T) {
	manifest := testManifest()
	if err := manifest.ValidateVersions("v3.2.4", "2.4.4"); err != nil {
		t.Fatal(err)
	}
	if err := manifest.ValidateVersions("v3.2.5", "2.4.4"); err == nil || !strings.Contains(err.Error(), "SPX version") {
		t.Fatalf("SPX version mismatch error = %v", err)
	}
	if err := manifest.ValidateVersions("v3.2.4", "2.4.3"); err == nil || !strings.Contains(err.Error(), "runtime version") {
		t.Fatalf("runtime version mismatch error = %v", err)
	}
	if err := manifest.ValidateVersions("3.2.4", "2.4.4"); err == nil {
		t.Fatal("ValidateVersions accepted a non-canonical expected SPX version")
	}
}

func TestHostSpecMatchesCanonicalBundleComponents(t *testing.T) {
	spec, err := HostSpecFor("2.4.4", "linux", "amd64")
	if err != nil {
		t.Fatal(err)
	}
	if spec.BundleName != "spx-driver-linux-amd64.zip" || spec.Engine.Name != "gdspxrt2.4.4" || spec.Pack.Name != "gdspxrt2.4.4.pck" || spec.Bridge.Name != "gdspx-linux-amd64.so" {
		t.Fatalf("HostSpecFor = %#v", spec)
	}
	if _, err := HostSpecFor("2.4.4", "freebsd", "amd64"); err == nil {
		t.Fatal("HostSpecFor accepted unsupported target")
	}
	if got := SupportedTargets(); len(got) != 4 || got[0].GOOS != "darwin" || got[3].GOOS != "windows" {
		t.Fatalf("SupportedTargets = %#v", got)
	}
}

func TestManifestBindingAndDigestInputs(t *testing.T) {
	manifest := testManifest()
	data, err := manifest.JSON()
	if err != nil {
		t.Fatal(err)
	}
	if got, err := ParseForVersions(data, manifest.SPXVersion, manifest.RuntimeVersion); err != nil || got.SPXVersion != manifest.SPXVersion {
		t.Fatalf("bound manifest=%+v,%v", got, err)
	}
	for _, tc := range []struct {
		data         []byte
		spx, runtime string
	}{{[]byte("bad"), "v3.2.4", "2.4.4"}, {data, "v3.2.5", "2.4.4"}, {data, "v3.2.4", "bad"}} {
		if _, err := ParseForVersions(tc.data, tc.spx, tc.runtime); err == nil {
			t.Fatal("invalid binding accepted")
		}
	}
	for _, b := range manifest.Bundles {
		if err := b.ValidateForRuntime(manifest.RuntimeVersion); err != nil {
			t.Fatal(err)
		}
		if err := b.ValidateForRuntime("bad"); err == nil {
			t.Fatal("invalid runtime accepted")
		}
	}
	for _, b := range []Bundle{{}, {GOOS: "INVALID", GOARCH: "amd64"}} {
		if _, err := b.JSON(); err == nil {
			t.Fatal("serialized invalid bundle")
		}
		data, err := json.Marshal(b)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := ParseBundle(data); err == nil {
			t.Fatal("parsed invalid bundle")
		}
	}
	if _, err := ParseBundle([]byte("invalid")); err == nil {
		t.Fatal("invalid bundle JSON accepted")
	}
	engine, pack := []byte("engine"), []byte("pack")
	a, b := sha256.Sum256(engine), sha256.Sum256(pack)
	want, err := ComputeEngineInterfaceDigestFromSHA256(hex.EncodeToString(a[:]), hex.EncodeToString(b[:]))
	if err != nil {
		t.Fatal(err)
	}
	if got := ComputeEngineInterfaceDigest(engine, pack); got != want || got == ComputeEngineInterfaceDigest(pack, engine) {
		t.Fatal("digest lost pair identity or ordering")
	}
	for _, pair := range [][2]string{{"bad", testDigest}, {testDigest, "bad"}} {
		if _, err := ComputeEngineInterfaceDigestFromSHA256(pair[0], pair[1]); err == nil {
			t.Fatal("invalid component digest accepted")
		}
	}
}

func testManifest() Manifest {
	file := func(name string, mode uint32) File {
		return File{Name: name, Mode: mode, Size: 10, SHA256: testDigest}
	}
	bundle := func(goos, goarch, name string) Bundle {
		names := expectedFileNames("2.4.4", goos, goarch)
		interfaceDigest, err := ComputeEngineInterfaceDigestFromSHA256(testDigest, testDigest)
		if err != nil {
			panic(err)
		}
		return Bundle{
			GOOS: goos, GOARCH: goarch, Name: name, Size: 100, SHA256: testDigest,
			EngineInterfaceDigest: interfaceDigest,
			Files:                 []File{file(names[0], 0o755), file(names[1], 0o644), file(names[2], 0o755)},
		}
	}
	return Manifest{
		Schema: ManifestSchema, SPXVersion: "v3.2.4", RuntimeVersion: "2.4.4",
		Bundles: []Bundle{
			bundle("darwin", "amd64", "spx-driver-darwin-amd64.zip"),
			bundle("darwin", "arm64", "spx-driver-darwin-arm64.zip"),
			bundle("linux", "amd64", "spx-driver-linux-amd64.zip"),
			bundle("windows", "amd64", "spx-driver-windows-amd64.zip"),
		},
	}
}
