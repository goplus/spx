package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goplus/spx/v4/internal/release"
)

func TestEngineManifestRejectsMixedModes(t *testing.T) {
	for _, test := range []struct {
		name string
		args []string
		want string
	}{
		{"verification requires revision", []string{"--verify-engine-manifest", "source.json"}, "requires --revision"},
		{"generation requires revision", []string{"--engine-manifest", "source.json"}, "requires --revision"},
		{"two verification modes", []string{"--verify-engine-manifest", "source.json", "--verify-manifest", "target.json"}, "cannot be combined"},
		{"empty verification source", []string{"--verify-engine-manifest", "", "--revision", "HEAD"}, "must not be empty"},
		{"blank generation source", []string{"--engine-manifest", " ", "--revision", "HEAD"}, "must not be empty"},
		{"empty target manifest", []string{"--verify-manifest", ""}, "must not be empty"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := runReleaseManifest(test.args); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("runReleaseManifest(%q) = %v; want %q", test.args, err, test.want)
			}
		})
	}
	for _, mode := range []string{"--verify-manifest", "--verify-engine-manifest"} {
		for _, flag := range []string{"--engine-manifest", "--spx-commit", "--engine-source-sha256", "--asset", "--checksums", "--output"} {
			t.Run(mode+"/"+flag, func(t *testing.T) {
				args := []string{mode, "source.json", flag, "asset=path"}
				if err := runReleaseManifest(args); err == nil || !strings.Contains(err.Error(), "cannot be combined") {
					t.Fatalf("runReleaseManifest(%q) = %v; want incompatible inputs", args, err)
				}
			})
		}
	}
}

func TestPackOnlyManifestKeepsEngineOrigin(t *testing.T) {
	args := newEngineManifestFixture(t)
	verify := []string{"--lock", "runtime.lock.json", "--verify-engine-manifest", "source.json", "--revision", "HEAD"}
	// Source validation does not require local ZIPs. File validation deliberately
	// ignores the previous pack, which the target release has just rebuilt.
	for _, extra := range [][]string{nil, {"--asset-directory", "assets"}} {
		if err := runReleaseManifest(append(append([]string{}, verify...), extra...)); err != nil {
			t.Fatal(err)
		}
	}
	if err := runReleaseManifest(args); err != nil {
		t.Fatal(err)
	}
	manifest, err := release.LoadRuntimeManifest("target.json")
	if err != nil {
		t.Fatal(err)
	}
	source, err := release.LoadRuntimeManifest("source.json")
	if err != nil {
		t.Fatal(err)
	}
	origin := manifest.Provenance.EngineOrigin
	if manifest.RuntimeVersion != "99.0.0" || origin == nil || origin.RuntimeVersion != source.RuntimeVersion || origin.SPXCommit != source.Provenance.SPXCommit {
		t.Fatalf("lost source: %#v", manifest.Provenance)
	}
	if manifest.Provenance.SPXCommit == source.Provenance.SPXCommit || manifest.Provenance.RuntimePackSourceSHA256 == source.Provenance.RuntimePackSourceSHA256 {
		t.Fatal("target manifest did not record the new pack source")
	}
	if err := manifest.VerifyFiles("assets"); err != nil {
		t.Fatal(err)
	}
	writeManifestFixtureFile(t, filepath.Join("assets", "android.zip"), "tampered")
	if err := runReleaseManifest(verify); err != nil {
		t.Fatalf("provenance verification read local assets: %v", err)
	}
	if err := runReleaseManifest(append(verify, "--asset-directory", "assets")); err == nil {
		t.Fatal("modified engine passed file verification")
	}
	assertEngineManifestFailure(t, args, "differs from source")
}

func TestEngineManifestSourceFailuresPreserveOutputs(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*testing.T)
		want   string
	}{
		{"missing source", func(t *testing.T) {
			if err := os.Remove("source.json"); err != nil {
				t.Fatal(err)
			}
		}, "read runtime manifest"},
		{"invalid source", func(t *testing.T) {
			writeManifestFixtureFile(t, "source.json", "{")
		}, "decode runtime manifest"},
		{"different engine", func(t *testing.T) {
			source, err := release.LoadRuntimeManifest("source.json")
			if err != nil {
				t.Fatal(err)
			}
			source.Provenance.EngineSourceSHA256 = strings.Repeat("f", 64)
			if err := release.WriteRuntimeManifest("source.json", source); err != nil {
				t.Fatal(err)
			}
		}, "engine source digest differs"},
	} {
		t.Run(test.name, func(t *testing.T) {
			args := newEngineManifestFixture(t)
			test.change(t)
			verify := []string{"--lock", "runtime.lock.json", "--verify-engine-manifest", "source.json", "--revision", "HEAD"}
			for _, command := range [][]string{verify, args} {
				assertEngineManifestFailure(t, command, test.want)
			}
		})
	}
}

func newEngineManifestFixture(t *testing.T) []string {
	t.Helper()
	lock, git := newManifestRepository(t)
	args := append(manifestFixtureAssetArgs(t, lock), "--revision", "HEAD")
	if err := runReleaseManifest(append(append([]string{}, args...), "--output", "source.json")); err != nil {
		t.Fatal(err)
	}
	lock.RuntimeVersion = "99.0.0"
	data, err := lock.JSON()
	if err != nil {
		t.Fatal(err)
	}
	writeManifestFixtureFile(t, "runtime.lock.json", string(data))
	writeManifestFixtureFile(t, "cmd/spx/template/project/main.tscn", "[gd_scene format=3]\n; updated resources\n")
	git("add", "runtime.lock.json", "cmd/spx/template/project/main.tscn")
	git("commit", "--quiet", "-m", "new pack release")
	writeManifestFixtureFile(t, filepath.Join("assets", release.RuntimeAssetZipName), "new pack")
	return append(args, "--engine-manifest", "source.json", "--output", "target.json", "--checksums", "SHA256SUMS")
}

func assertEngineManifestFailure(t *testing.T, args []string, want string) {
	t.Helper()
	for _, path := range []string{"target.json", "SHA256SUMS"} {
		writeManifestFixtureFile(t, path, "previous output")
	}
	if err := runReleaseManifest(args); err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("runReleaseManifest(%q) = %v; want %q", args, err, want)
	}
	for _, path := range []string{"target.json", "SHA256SUMS"} {
		data, err := os.ReadFile(path)
		if err != nil || string(data) != "previous output" {
			t.Fatalf("failed generation changed %s: %q, %v", path, data, err)
		}
	}
}
