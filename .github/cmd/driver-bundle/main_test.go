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

package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/goplus/spx/v4/internal/driverbundle"
	"github.com/goplus/spx/v4/internal/release"
)

type driverShortWriter struct {
	count int
	err   error
}

func TestPackageAndVerifyIsDeterministic(t *testing.T) {
	for _, target := range driverbundle.SupportedTargets() {
		t.Run(target.GOOS+"/"+target.GOARCH, func(t *testing.T) {
			directory := t.TempDir()
			spec, err := driverbundle.HostSpecFor(release.DefaultRuntimeLock().RuntimeVersion, target.GOOS, target.GOARCH)
			if err != nil {
				t.Fatal(err)
			}
			enginePath := filepath.Join(directory, spec.Engine.Name)
			packPath := filepath.Join(directory, spec.Pack.Name)
			bridgePath := filepath.Join(directory, spec.Bridge.Name)
			for path, data := range map[string][]byte{
				enginePath: []byte("engine bytes"),
				packPath:   []byte("pack bytes"),
				bridgePath: []byte("bridge bytes"),
			} {
				writeDriverTestFile(t, path, data)
			}
			outputPath := filepath.Join(directory, spec.BundleName)
			descriptorPath := filepath.Join(directory, "bundle.json")
			args := []string{
				"--engine", enginePath, "--pack", packPath, "--bridge", bridgePath,
				"--output", outputPath, "--descriptor", descriptorPath,
				"--goos", target.GOOS, "--goarch", target.GOARCH,
			}
			runDriverMain(t, append([]string{"package"}, args...)...)
			firstZIP, err := os.ReadFile(outputPath)
			if err != nil {
				t.Fatal(err)
			}
			firstDescriptor, err := os.ReadFile(descriptorPath)
			if err != nil {
				t.Fatal(err)
			}
			runDriverMain(t, "verify", "--output", outputPath, "--descriptor", descriptorPath)

			secondOutput := filepath.Join(directory, "second", spec.BundleName)
			secondDescriptor := filepath.Join(directory, "second", "bundle.json")
			secondArgs := append([]string{}, args...)
			for i := range secondArgs {
				if secondArgs[i] == outputPath {
					secondArgs[i] = secondOutput
				}
				if secondArgs[i] == descriptorPath {
					secondArgs[i] = secondDescriptor
				}
			}
			if err := runPackage(secondArgs); err != nil {
				t.Fatalf("second package: %v", err)
			}
			secondZIP, err := os.ReadFile(secondOutput)
			if err != nil {
				t.Fatal(err)
			}
			secondDescriptorData, err := os.ReadFile(secondDescriptor)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(firstZIP, secondZIP) || !bytes.Equal(firstDescriptor, secondDescriptorData) {
				t.Fatal("package output is not deterministic")
			}

			bundle, err := driverbundle.ParseBundle(firstDescriptor)
			if err != nil {
				t.Fatal(err)
			}
			if len(bundle.Files) != 3 || bundle.Files[0].Mode != 0o755 || bundle.Files[1].Mode != 0o644 || bundle.Files[2].Mode != 0o755 {
				t.Fatalf("descriptor files = %#v", bundle.Files)
			}
			archive, err := zip.OpenReader(outputPath)
			if err != nil {
				t.Fatal(err)
			}
			defer archive.Close()
			if len(archive.File) != len(bundle.Files) {
				t.Fatalf("ZIP entry count = %d, want %d", len(archive.File), len(bundle.Files))
			}
			wantNames := []string{spec.Engine.Name, spec.Pack.Name, spec.Bridge.Name}
			wantContents := []string{"engine bytes", "pack bytes", "bridge bytes"}
			for i, entry := range archive.File {
				if entry.Name != wantNames[i] {
					t.Fatalf("ZIP entry %d = %q, want %q", i, entry.Name, wantNames[i])
				}
				reader, err := entry.Open()
				if err != nil {
					t.Fatal(err)
				}
				data, err := io.ReadAll(reader)
				reader.Close()
				if err != nil || string(data) != wantContents[i] {
					t.Fatalf("ZIP entry %q contents = %q, %v", entry.Name, data, err)
				}
				if entry.Name != bundle.Files[i].Name || entry.Mode().Perm() != os.FileMode(bundle.Files[i].Mode) {
					t.Fatalf("ZIP entry %d = %q mode %#o", i, entry.Name, entry.Mode().Perm())
				}
				if !entry.Modified.Equal(time.Date(1980, time.January, 1, 0, 0, 0, 0, time.UTC)) {
					t.Fatalf("ZIP entry %q modified = %s", entry.Name, entry.Modified)
				}
			}
		})
	}
}

func TestPackageRejectsNonCanonicalBasename(t *testing.T) {
	directory := t.TempDir()
	outputDir := filepath.Join(directory, "output")
	err := runPackage([]string{
		"--engine", filepath.Join(directory, "engine"),
		"--pack", filepath.Join(directory, "pack"),
		"--bridge", filepath.Join(directory, "bridge"),
		"--output", filepath.Join(outputDir, "spx-driver-linux-amd64.zip"),
		"--descriptor", filepath.Join(directory, "bundle.json"),
		"--goos", "linux", "--goarch", "amd64",
	})
	if err == nil || !strings.Contains(err.Error(), "validate input basenames: Engine name") {
		t.Fatalf("basename rejection = %v", err)
	}
	if _, err := os.Stat(outputDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("output directory exists after basename rejection: %v", err)
	}
}

func TestValidateInputBasenames(t *testing.T) {
	for _, target := range driverbundle.SupportedTargets() {
		t.Run(target.GOOS+"/"+target.GOARCH, func(t *testing.T) {
			spec, err := driverbundle.HostSpecFor(release.DefaultRuntimeLock().RuntimeVersion, target.GOOS, target.GOARCH)
			if err != nil {
				t.Fatal(err)
			}
			inputs := [...]struct{ kind, name string }{
				{"Engine", spec.Engine.Name}, {"PCK", spec.Pack.Name},
				{"bridge", spec.Bridge.Name}, {"bundle", spec.BundleName},
			}
			var paths [4]string
			for i, input := range inputs {
				paths[i] = filepath.Join("input", "nested", input.name)
			}
			if err := validateInputBasenames(spec, paths[0], paths[1], paths[2], paths[3]); err != nil {
				t.Fatalf("canonical names in directories: %v", err)
			}
			for i, input := range inputs {
				for _, invalid := range []string{"", ".", "..", "wrong.zip", inputs[(i+1)%len(inputs)].name, input.name + " ", input.name + "\x00", input.name + "\n", input.name + "\\suffix"} {
					actual := paths
					actual[i] = invalid
					err := validateInputBasenames(spec, actual[0], actual[1], actual[2], actual[3])
					want := fmt.Sprintf("validate input basenames: %s name = %q, want %q", input.kind, filepath.Base(invalid), input.name)
					if err == nil || err.Error() != want {
						t.Fatalf("slot %d path %q: error = %v, want %q", i, invalid, err, want)
					}
				}
			}
			err = validateInputBasenames(spec, "bad-engine", "bad-pack", "bad-bridge", "bad.zip")
			if err == nil || !strings.Contains(err.Error(), "bundle name") {
				t.Fatalf("bundle name must be checked before component names: %v", err)
			}
		})
	}
}

func TestCommandsRejectInvalidArguments(t *testing.T) {
	for _, command := range []struct {
		name string
		run  func([]string) error
	}{
		{"package", runPackage}, {"verify", runVerify}, {"assemble", runAssemble}, {"verify-release", runVerifyRelease},
	} {
		t.Run(command.name, func(t *testing.T) {
			for _, test := range []struct {
				args []string
				want string
			}{
				{[]string{"--help"}, "help requested"},
				{[]string{"--unknown"}, "flag provided but not defined"},
				{[]string{"unexpected"}, "unexpected positional arguments"},
				{nil, "required"},
			} {
				if err := command.run(test.args); err == nil || !strings.Contains(err.Error(), test.want) {
					t.Fatalf("command(%q) = %v, want %q", test.args, err, test.want)
				}
			}
		})
	}
	for _, args := range [][]string{{"--manifest", "manifest.json"}, {"--manifest", "manifest.json", "--descriptor", " "}} {
		if err := runAssemble(args); err == nil {
			t.Fatalf("accepted %q", args)
		}
	}
	var inputs descriptorInputs
	if err := inputs.Set("one"); err != nil {
		t.Fatal(err)
	}
	if err := inputs.Set("two"); err != nil {
		t.Fatal(err)
	}
	if got := inputs.String(); got != "one,two" {
		t.Fatalf("descriptor inputs = %q", got)
	}
	var help bytes.Buffer
	usage(&help)
	if help.String() != "usage: driver-bundle package|verify|assemble|verify-release [flags]\n" {
		t.Fatalf("usage = %q", help.String())
	}
}

func TestPackageFailureDoesNotLeaveTemporaryFiles(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(t *testing.T, paths map[string]string)
		want   string
	}{
		{"missing engine", func(t *testing.T, p map[string]string) {
			if err := os.Remove(p["engine"]); err != nil {
				t.Fatal(err)
			}
		}, "package Engine"},
		{"missing pack", func(t *testing.T, p map[string]string) {
			if err := os.Remove(p["pack"]); err != nil {
				t.Fatal(err)
			}
		}, "package PCK"},
		{"missing bridge", func(t *testing.T, p map[string]string) {
			if err := os.Remove(p["bridge"]); err != nil {
				t.Fatal(err)
			}
		}, "package bridge"},
		{"unsupported target", func(t *testing.T, p map[string]string) { p["goos"] = "unsupported" }, "unsupported"},
		{"output aliases descriptor", func(t *testing.T, p map[string]string) { p["descriptor"] = p["output"] }, "paths alias"},
		{"descriptor aliases input", func(t *testing.T, p map[string]string) { p["descriptor"] = p["engine"] }, "paths alias"},
		{"blocked ZIP parent", func(t *testing.T, p map[string]string) {
			p["output"] = filepath.Join(p["pack"], filepath.Base(p["output"]))
		}, "prepare ZIP output"},
		{"blocked descriptor parent", func(t *testing.T, p map[string]string) { p["descriptor"] = filepath.Join(p["pack"], "descriptor.json") }, "prepare descriptor output"},
		{"ZIP parent is read-only", func(t *testing.T, p map[string]string) {
			if runtime.GOOS == "windows" {
				t.Skip("requires Unix permission bits")
			}
			dir := filepath.Dir(p["output"])
			if err := os.Chmod(dir, 0o500); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { os.Chmod(dir, 0o700) })
			if probe, err := os.CreateTemp(dir, "probe-"); err == nil {
				probe.Close()
				t.Skip("user bypasses directory permission checks")
			} else if !errors.Is(err, os.ErrPermission) {
				t.Fatalf("permission probe: %v", err)
			}
		}, "create temporary ZIP"},
		{"ZIP destination is a directory", func(t *testing.T, p map[string]string) {
			if err := os.Mkdir(p["output"], 0o700); err != nil {
				t.Fatal(err)
			}
		}, "publish ZIP"},
		{"descriptor destination is a directory", func(t *testing.T, p map[string]string) {
			if err := os.Mkdir(p["descriptor"], 0o700); err != nil {
				t.Fatal(err)
			}
		}, "publish descriptor"},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			spec, err := driverbundle.HostSpecFor(release.DefaultRuntimeLock().RuntimeVersion, "linux", "amd64")
			if err != nil {
				t.Fatal(err)
			}
			paths := map[string]string{"engine": filepath.Join(dir, spec.Engine.Name), "pack": filepath.Join(dir, spec.Pack.Name), "bridge": filepath.Join(dir, spec.Bridge.Name), "output": filepath.Join(dir, "spx-driver-linux-amd64.zip"), "descriptor": filepath.Join(dir, "bundle.json"), "goos": "linux", "goarch": "amd64"}
			for _, key := range []string{"engine", "pack", "bridge"} {
				writeDriverTestFile(t, paths[key], []byte(key))
			}
			test.change(t, paths)
			var args []string
			for _, key := range []string{"engine", "pack", "bridge", "output", "descriptor", "goos", "goarch"} {
				args = append(args, "--"+key, paths[key])
			}
			if err := runPackage(args); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("runPackage error = %v, want %q", err, test.want)
			}
			entries, err := os.ReadDir(dir)
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range entries {
				if strings.Contains(entry.Name(), ".tmp-") {
					t.Fatalf("left temporary file %s", entry.Name())
				}
			}
		})
	}
}

func TestVerifyRejectsInvalidDescriptorsAndArchives(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(t *testing.T, path string, bundle *driverbundle.Bundle) string
		want   string
	}{
		{"invalid descriptor", func(t *testing.T, path string, b *driverbundle.Bundle) string { b.Size = -1; return path }, "validate descriptor"},
		{"wrong basename", func(t *testing.T, path string, b *driverbundle.Bundle) string { return path + ".wrong" }, "ZIP basename"},
		{"missing archive", func(t *testing.T, path string, b *driverbundle.Bundle) string {
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			return path
		}, "open ZIP"},
		{"wrong size", func(t *testing.T, path string, b *driverbundle.Bundle) string { b.Size++; return path }, "ZIP size"},
		{"wrong digest", func(t *testing.T, path string, b *driverbundle.Bundle) string {
			b.SHA256 = strings.Repeat("0", 64)
			return path
		}, "ZIP SHA-256"},
		{"invalid ZIP", func(t *testing.T, path string, b *driverbundle.Bundle) string {
			data := []byte("not a ZIP")
			writeDriverTestFile(t, path, data)
			digest := sha256.Sum256(data)
			b.Size = int64(len(data))
			b.SHA256 = hex.EncodeToString(digest[:])
			return path
		}, "verify ZIP with runtimebundle"},
		{"wrong interface", func(t *testing.T, path string, b *driverbundle.Bundle) string {
			b.EngineInterfaceDigest = strings.Repeat("0", 64)
			return path
		}, "Engine interface digest"},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			bundle := writeReleaseTestBundle(t, dir, release.DefaultRuntimeLock(), "linux", "amd64")
			path := test.change(t, filepath.Join(dir, bundle.Name), &bundle)
			if err := verifyBundleFile(path, bundle); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("verify error = %v, want %q", err, test.want)
			}
		})
	}
	descriptor := filepath.Join(t.TempDir(), "bundle.json")
	args := []string{"--output", "spx-driver-linux-amd64.zip", "--descriptor", descriptor}
	if err := runVerify(args); err == nil || !strings.Contains(err.Error(), "read descriptor") {
		t.Fatalf("read error = %v", err)
	}
	writeDriverTestFile(t, descriptor, []byte("{"))
	if err := runVerify(args); err == nil || !strings.Contains(err.Error(), "parse descriptor") {
		t.Fatalf("parse error = %v", err)
	}
	if err := runVerifyRelease([]string{"--directory", t.TempDir(), "--spx-version", "v3.2.4"}); err == nil || !strings.Contains(err.Error(), "read SPX driver manifest") {
		t.Fatalf("missing release error = %v", err)
	}
}

func TestRegularFileAndAtomicWriteFailures(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	writeDriverTestFile(t, source, []byte("original"))
	for _, path := range []string{dir, filepath.Join(dir, "missing")} {
		if _, err := readRegularFile(path); err == nil {
			t.Fatalf("read non-regular path %q", path)
		}
	}
	link := filepath.Join(dir, "symlink")
	if err := os.Symlink(source, link); err == nil {
		if _, err := readRegularFile(link); err == nil || !strings.Contains(err.Error(), "non-symlink") {
			t.Fatalf("symlink error = %v", err)
		}
	} else {
		t.Logf("symlink unavailable: %v", err)
	}
	alias := filepath.Join(dir, "hardlink")
	if err := os.Link(source, alias); err == nil {
		if err := rejectOutputAliases(alias, filepath.Join(dir, "descriptor"), source); err == nil || !strings.Contains(err.Error(), "paths alias") {
			t.Fatalf("hardlink alias error = %v", err)
		}
	} else {
		t.Logf("hardlink unavailable: %v", err)
	}
	large := filepath.Join(dir, "large")
	file, err := os.Create(large)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(driverbundle.MaxManifestSize + 1); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := readRegularFile(large); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversized descriptor error = %v", err)
	}
	for _, destination := range []string{source, filepath.Join(source, "nested"), dir} {
		if err := copyRegularFile(source, destination); err == nil {
			t.Fatalf("copy to %q succeeded", destination)
		}
	}
	if err := copyRegularFile(filepath.Join(dir, "missing"), filepath.Join(dir, "destination")); err == nil {
		t.Fatal("copied missing source")
	}
	for _, destination := range []string{filepath.Join(dir, "missing", "output"), dir} {
		if err := atomicWrite(destination, []byte("replacement"), 0o600); err == nil {
			t.Fatalf("atomicWrite(%q) succeeded", destination)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), ".tmp-") {
			t.Fatalf("left temporary file %s", entry.Name())
		}
	}
	data, err := os.ReadFile(source)
	if err != nil || string(data) != "original" {
		t.Fatalf("source was changed: %q, %v", data, err)
	}
	if err := syncDirectory(filepath.Join(dir, "missing")); err == nil {
		t.Fatal("synced missing directory")
	}
	t.Run("removed working directory", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Windows keeps the working directory open")
		}
		working := t.TempDir()
		t.Chdir(working)
		if err := os.Remove(working); err != nil {
			t.Fatal(err)
		}
		for _, test := range []struct{ name, output, input string }{
			{"relative output", "bundle.zip", source},
			{"relative input", filepath.Join(dir, "bundle.zip"), "source"},
		} {
			t.Run(test.name, func(t *testing.T) {
				err := rejectOutputAliases(test.output, filepath.Join(dir, "bundle.json"), test.input)
				if !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("removed working directory: %v", err)
				}
			})
		}
		data, err := os.ReadFile(source)
		if err != nil || string(data) != "original" {
			t.Fatalf("source changed: %q, %v", data, err)
		}
	})
}

func TestDigestWriterCountsOnlyWrittenBytes(t *testing.T) {
	failure := errors.New("write failed")
	for _, count := range []int{0, 3} {
		digest := sha256.New()
		writer := digestWriter{writer: driverShortWriter{count, failure}, digest: digest}
		written, err := writer.Write([]byte("abcdef"))
		want := sha256.Sum256([]byte("abcdef")[:count])
		if written != count || writer.size != int64(count) || !errors.Is(err, failure) || !bytes.Equal(digest.Sum(nil), want[:]) {
			t.Fatalf("Write = %d, %v, size = %d, digest = %x", written, err, writer.size, digest.Sum(nil))
		}
	}
}

func TestAddZipFilePropagatesWriterFailures(t *testing.T) {
	path := filepath.Join(t.TempDir(), "input")
	writeDriverTestFile(t, path, bytes.Repeat([]byte("data"), 4096))
	failure := errors.New("archive write failed")
	archive := zip.NewWriter(driverShortWriter{err: failure})
	if _, err := addZipFile(archive, path, "input", 0o644); !errors.Is(err, failure) {
		t.Fatalf("copy error = %v", err)
	}
	archive = zip.NewWriter(io.Discard)
	// ZIP names are limited to 16 bits; oversized names fail before copying.
	if _, err := addZipFile(archive, path, strings.Repeat("x", 1<<16), 0o644); err == nil {
		t.Fatal("accepted an oversized ZIP entry name")
	}
}

func TestVerifyReleaseChecksVersionsAndBundles(t *testing.T) {
	lock, err := release.RuntimeLockForVersion("2.4.3")
	if err != nil {
		t.Fatal(err)
	}

	directory := t.TempDir()
	lockPath := filepath.Join(t.TempDir(), "runtime.lock.json")
	writeDriverTestLock(t, lockPath, lock)
	targets := [][2]string{{"darwin", "amd64"}, {"darwin", "arm64"}, {"linux", "amd64"}, {"windows", "amd64"}}
	bundles := make([]driverbundle.Bundle, 0, len(targets))
	for _, target := range targets {
		bundles = append(bundles, writeReleaseTestBundle(t, directory, lock, target[0], target[1]))
	}
	manifest := driverbundle.Manifest{
		Schema: driverbundle.ManifestSchema, SPXVersion: "v3.2.4", RuntimeVersion: lock.RuntimeVersion,
		Bundles: bundles,
	}
	manifestData, err := manifest.JSON()
	if err != nil {
		t.Fatal(err)
	}
	writeDriverTestFile(t, filepath.Join(directory, driverbundle.ManifestName), manifestData)

	args := []string{"--lock", lockPath, "--directory", directory, "--spx-version", manifest.SPXVersion}
	if err := runVerifyRelease([]string{"--lock", lockPath + ".missing", "--directory", directory, "--spx-version", manifest.SPXVersion}); err == nil || !strings.Contains(err.Error(), "read runtime lock") {
		t.Fatalf("missing lock error = %v", err)
	}
	if err := runVerifyRelease(args); err != nil {
		t.Fatal(err)
	}
	writeDriverTestFile(t, filepath.Join(directory, "spx_web.zip"), []byte("product"))
	if err := runVerifyRelease(args); err != nil {
		t.Fatalf("driver verification rejected another SPX release asset: %v", err)
	}
	if err := runVerifyRelease([]string{"--lock", lockPath, "--directory", directory, "--spx-version", "v9.9.9"}); err == nil {
		t.Fatal("accepted mismatched SPX version")
	}
	otherLock, err := release.RuntimeLockForVersion("2.4.2")
	if err != nil {
		t.Fatal(err)
	}
	otherLockPath := filepath.Join(t.TempDir(), "runtime.lock.json")
	writeDriverTestLock(t, otherLockPath, otherLock)
	if err := runVerifyRelease([]string{"--lock", otherLockPath, "--directory", directory, "--spx-version", manifest.SPXVersion}); err == nil {
		t.Fatal("accepted mismatched runtime version")
	}

	writeDriverTestFile(t, filepath.Join(directory, bundles[0].Name), []byte("tampered"))
	if err := runVerifyRelease(args); err == nil {
		t.Fatal("accepted tampered release")
	}
}

func TestAssemblePublishesSortedVerifiedRelease(t *testing.T) {
	args, descriptors, output := newAssemblyFixture(t)
	// Descriptor discovery order must not affect the release manifest.
	slices.Reverse(descriptors)
	for _, descriptor := range descriptors {
		args = append(args, "--descriptor", descriptor)
	}
	runDriverMain(t, append([]string{"assemble"}, args...)...)
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := driverbundle.ParseForVersions(data, "v3.2.4", release.DefaultRuntimeLock().RuntimeVersion)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.IsSortedFunc(manifest.Bundles, compareDriverBundles) {
		t.Fatalf("unsorted bundles: %#v", manifest.Bundles)
	}
	runDriverMain(t, "verify-release", "--lock", args[1], "--directory", filepath.Dir(output), "--spx-version", "v3.2.4")
}

func TestAssembleRejectsInvalidInputsWithoutManifest(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(t *testing.T, args, descriptors []string, output string)
		want   string
	}{
		{"missing lock", func(t *testing.T, args, _ []string, _ string) { args[1] += ".missing" }, "read runtime lock"},
		{"invalid lock", func(t *testing.T, args, _ []string, _ string) {
			writeDriverTestFile(t, args[1], []byte("{"))
		}, "decode runtime lock"},
		{"missing descriptor", func(t *testing.T, args, descriptors []string, _ string) {
			if err := os.Remove(descriptors[0]); err != nil {
				t.Fatal(err)
			}
		}, "read descriptor"},
		{"invalid descriptor", func(t *testing.T, args, descriptors []string, _ string) {
			writeDriverTestFile(t, descriptors[0], []byte("{"))
		}, "parse descriptor"},
		{"duplicate target", func(t *testing.T, args, descriptors []string, _ string) {
			args[len(args)-1] = descriptors[0]
		}, "duplicate driver target"},
		{"invalid SPX version", func(t *testing.T, args, _ []string, _ string) { args[3] = "invalid" }, "SPX version"},
		{"blocked output parent", func(t *testing.T, args, _ []string, output string) {
			writeDriverTestFile(t, filepath.Dir(output), []byte("blocked"))
		}, "prepare manifest output"},
		{"missing bundle", func(t *testing.T, args, descriptors []string, _ string) {
			path := filepath.Join(filepath.Dir(descriptors[0]), "spx-driver-darwin-amd64.zip")
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
		}, "copy darwin/amd64 bundle"},
		{"corrupted bundle", func(t *testing.T, args, descriptors []string, _ string) {
			path := filepath.Join(filepath.Dir(descriptors[0]), "spx-driver-darwin-amd64.zip")
			writeDriverTestFile(t, path, []byte("corrupted"))
		}, "verify copied darwin/amd64 bundle"},
		{"manifest is a directory", func(t *testing.T, args, _ []string, output string) {
			if err := os.MkdirAll(output, 0o700); err != nil {
				t.Fatal(err)
			}
		}, "write driver manifest"},
	} {
		t.Run(test.name, func(t *testing.T) {
			args, descriptors, output := newAssemblyFixture(t)
			for _, path := range descriptors {
				args = append(args, "--descriptor", path)
			}
			test.mutate(t, args, descriptors, output)
			if err := runAssemble(args); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("runAssemble error = %v, want %q", err, test.want)
			}
			if info, err := os.Stat(output); err == nil && info.Mode().IsRegular() {
				t.Fatal("published a manifest after assembly failed")
			}
		})
	}
}

func (w driverShortWriter) Write([]byte) (int, error) { return w.count, w.err }

func writeReleaseTestBundle(t *testing.T, directory string, lock release.RuntimeLock, goos, goarch string) driverbundle.Bundle {
	t.Helper()
	spec, err := release.HostRuntimeSpecFor(lock, goos, goarch)
	if err != nil {
		t.Fatal(err)
	}
	extension := map[string]string{"darwin": ".dylib", "linux": ".so", "windows": ".dll"}[goos]
	names := [3]string{spec.RuntimeName, spec.PackName, "gdspx-" + goos + "-" + goarch + extension}
	modes := [3]uint32{0o755, 0o644, 0o755}
	name := "spx-driver-" + goos + "-" + goarch + ".zip"
	path := filepath.Join(directory, name)
	output, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	archive := zip.NewWriter(output)
	files := make([]driverbundle.File, 0, len(names))
	for i, name := range names {
		data := []byte(name + " bytes")
		header := &zip.FileHeader{Name: name, Method: zip.Store, Modified: zipEpoch}
		header.SetMode(os.FileMode(modes[i]))
		writer, err := archive.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write(data); err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(data)
		files = append(files, driverbundle.File{Name: name, Mode: modes[i], Size: int64(len(data)), SHA256: hex.EncodeToString(digest[:])})
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := output.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(data)
	interfaceDigest, err := driverbundle.ComputeEngineInterfaceDigestFromSHA256(files[0].SHA256, files[1].SHA256)
	if err != nil {
		t.Fatal(err)
	}
	return driverbundle.Bundle{
		GOOS: goos, GOARCH: goarch, Name: name, Size: int64(len(data)), SHA256: hex.EncodeToString(digest[:]),
		EngineInterfaceDigest: interfaceDigest, Files: files,
	}
}

func newAssemblyFixture(t *testing.T) (args, descriptors []string, output string) {
	t.Helper()
	dir := t.TempDir()
	lock := release.DefaultRuntimeLock()
	lockPath := filepath.Join(dir, "runtime.lock.json")
	writeDriverTestLock(t, lockPath, lock)
	output = filepath.Join(dir, "release", driverbundle.ManifestName)
	args = []string{"--lock", lockPath, "--spx-version", "v3.2.4", "--manifest", output}
	for _, target := range driverbundle.SupportedTargets() {
		bundle := writeReleaseTestBundle(t, dir, lock, target.GOOS, target.GOARCH)
		data, err := bundle.JSON()
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, target.GOOS+"-"+target.GOARCH+".json")
		writeDriverTestFile(t, path, data)
		descriptors = append(descriptors, path)
	}
	return args, descriptors, output
}

func writeDriverTestFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeDriverTestLock(t *testing.T, path string, lock release.RuntimeLock) {
	t.Helper()
	data, err := lock.JSON()
	if err != nil {
		t.Fatal(err)
	}
	writeDriverTestFile(t, path, data)
}

func runDriverMain(t *testing.T, args ...string) {
	t.Helper()
	original := os.Args
	defer func() { os.Args = original }()
	os.Args = append([]string{"driver-bundle"}, args...)
	main()
}

func TestDriverMainHelp(t *testing.T) {
	original := os.Stdout
	output, err := os.CreateTemp(t.TempDir(), "stdout-")
	if err != nil {
		t.Fatal(err)
	}
	defer output.Close()
	os.Stdout = output
	defer func() { os.Stdout = original }()
	for _, arg := range []string{"-h", "--help", "help"} {
		runDriverMain(t, arg)
	}
	data, err := os.ReadFile(output.Name())
	if err != nil {
		t.Fatal(err)
	}
	var expected bytes.Buffer
	usage(&expected)
	if string(data) != strings.Repeat(expected.String(), 3) {
		t.Fatalf("help output = %q", data)
	}
}

func TestReadOnlyFilesRejectWritesWithoutChangingInputs(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires Unix permission bits")
	}
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	writeDriverTestFile(t, source, []byte("original"))
	if err := os.Chmod(source, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(source, 0o600) })
	if file, err := os.Open(source); err == nil {
		file.Close()
		t.Skip("user bypasses file permission checks")
	}
	if _, _, err := openRegularFile(source); !errors.Is(err, os.ErrPermission) {
		t.Fatalf("unreadable source: %v", err)
	}
	if err := os.Chmod(source, 0o600); err != nil {
		t.Fatal(err)
	}
	locked := filepath.Join(dir, "locked")
	if err := os.Mkdir(locked, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(locked, 0o700) })
	destination := filepath.Join(locked, "destination")
	if err := copyRegularFile(source, destination); !errors.Is(err, os.ErrPermission) {
		t.Fatalf("read-only destination: %v", err)
	}
	entries, err := os.ReadDir(locked)
	if err != nil || len(entries) != 0 {
		t.Fatalf("failed copy left files: %v, %v", entries, err)
	}
	data, err := os.ReadFile(source)
	if err != nil || string(data) != "original" {
		t.Fatalf("source changed: %q, %v", data, err)
	}
}

type driverWriteFunc func([]byte) (int, error)

func (f driverWriteFunc) Write(data []byte) (int, error) { return f(data) }

func TestAddZipFileRejectsSourceChangedWhileReading(t *testing.T) {
	source := filepath.Join(t.TempDir(), "source")
	writeDriverTestFile(t, source, bytes.Repeat([]byte("original"), 4096))
	changed := false
	archive := zip.NewWriter(driverWriteFunc(func(data []byte) (int, error) {
		if !changed {
			changed = true
			if err := os.Truncate(source, 0); err != nil {
				return 0, err
			}
		}
		return len(data), nil
	}))
	if _, err := addZipFile(archive, source, "source", 0o644); err == nil || !strings.Contains(err.Error(), "source changed while reading") {
		t.Fatalf("changed source error = %v", err)
	}
	if !changed {
		t.Fatal("fixture did not change the source")
	}
}

func TestDriverMainFailures(t *testing.T) {
	const helper = "SPX_TEST_DRIVER_MAIN_FAILURE"
	if args := os.Getenv(helper); args != "" {
		if err := json.Unmarshal([]byte(args), &os.Args); err != nil {
			t.Fatal(err)
		}
		main()
		return
	}
	for _, test := range []struct {
		name string
		args []string
		code int
		want string
	}{
		{"missing command", nil, 2, "usage: driver-bundle"},
		{"unknown command", []string{"unknown"}, 1, `unknown command "unknown"`},
		{"package missing paths", []string{"package"}, 1, "engine, pack, bridge, output, and descriptor paths are required"},
		{"verify missing paths", []string{"verify"}, 1, "output and descriptor paths are required"},
		{"assemble missing manifest", []string{"assemble"}, 1, "manifest path is required"},
		{"verify release missing version", []string{"verify-release"}, 1, "--spx-version is required"},
	} {
		t.Run(test.name, func(t *testing.T) {
			args, err := json.Marshal(append([]string{"driver-bundle"}, test.args...))
			if err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(os.Args[0], "-test.run=^TestDriverMainFailures$")
			cmd.Env = append(os.Environ(), helper+"="+string(args))
			output, err := cmd.Output()
			exit, ok := err.(*exec.ExitError)
			if !ok || exit.ExitCode() != test.code || len(output) != 0 {
				t.Fatalf("main(%q): error = %v, stdout = %q; want exit %d and no output", test.args, err, output, test.code)
			}
			if !strings.Contains(string(exit.Stderr), test.want) {
				t.Fatalf("stderr = %q, want %q", exit.Stderr, test.want)
			}
		})
	}
}
