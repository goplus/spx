package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goplus/spx/v4/internal/release"
)

func TestPackOnlyManifestKeepsEngineOrigin(t *testing.T) {
	root, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(strings.TrimSpace(string(root)))
	dir := t.TempDir()
	lock := release.DefaultRuntimeLock()
	args := []string{"--revision", "HEAD"}
	for _, name := range lock.RequiredAssets {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("asset:"+name), 0600); err != nil {
			t.Fatal(err)
		}
		args = append(args, "--asset", name+"="+path)
	}
	source := filepath.Join(dir, "source.json")
	if err := runReleaseManifest(append(append([]string{}, args...), "--output", source)); err != nil {
		t.Fatal(err)
	}
	// New pack version and bytes, identical engine assets.
	lock.RuntimeVersion = "99.0.0"
	data, err := lock.JSON()
	if err != nil {
		t.Fatal(err)
	}
	lockPath := filepath.Join(dir, "lock.json")
	if err := os.WriteFile(lockPath, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, release.RuntimePackAssetName), []byte("new pack"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := runReleaseManifest([]string{"--lock", lockPath, "--verify-engine-manifest", source, "--revision", "HEAD", "--asset-directory", dir}); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(dir, "target.json")
	targetArgs := append(append([]string{}, args...), "--lock", lockPath, "--engine-manifest", source, "--output", output)
	if err := runReleaseManifest(targetArgs); err != nil {
		t.Fatal(err)
	}
	manifest, err := release.LoadRuntimeManifest(output)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.RuntimeVersion != lock.RuntimeVersion || manifest.Provenance.EngineOrigin == nil || manifest.Provenance.EngineOrigin.RuntimeVersion != release.DefaultRuntimeLock().RuntimeVersion {
		t.Fatalf("lost source: %#v", manifest.Provenance)
	}
	if err := manifest.VerifyFiles(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "android.zip"), []byte("tampered"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := runReleaseManifest(targetArgs); err == nil {
		t.Fatal("modified engine accepted during assembly")
	}
}
