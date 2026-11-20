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
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRuntimeEngineSourceLockIdentity(t *testing.T) {
	fixture := newRuntimeEngineFixture(t)
	lock := DefaultRuntimeLock()
	baseline := fixture.digest(t, "HEAD", lock)
	for _, test := range []struct {
		name    string
		mutate  func(*RuntimeLock)
		changes bool
	}{
		{"runtime version", func(l *RuntimeLock) { l.RuntimeVersion = "9.9.9" }, false},
		{"Go", func(l *RuntimeLock) { l.Toolchain.Go = "1.99.0" }, false},
		{"XGo", func(l *RuntimeLock) { l.Toolchain.XGo = "9.0.0" }, false},
		{"Godot ref", func(l *RuntimeLock) { l.Godot.Ref = "other-ref" }, false},
		{"ABI", func(l *RuntimeLock) { l.RuntimeABI++ }, true},
		{"Godot repository", func(l *RuntimeLock) { l.Godot.Repository = "https://github.com/example/godot.git" }, true},
		{"Godot commit", func(l *RuntimeLock) { l.Godot.Commit = strings.Repeat("1", 40) }, true},
		{"Godot version", func(l *RuntimeLock) { l.Godot.Version = "9.0" }, true},
		{"SCons", func(l *RuntimeLock) { l.Toolchain.SCons = "9.0.0" }, true},
		{"EMSDK", func(l *RuntimeLock) { l.Toolchain.EMSDK = "9.0.0" }, true},
		{"Android NDK", func(l *RuntimeLock) { l.Toolchain.AndroidNDK = "99.0.0" }, true},
		{"JDK", func(l *RuntimeLock) { l.Toolchain.JDK = "21" }, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			changed := lock
			test.mutate(&changed)
			if got := fixture.digest(t, "HEAD", changed); (got != baseline) != test.changes {
				t.Fatalf("engine identity changed = %t, want %t", got != baseline, test.changes)
			}
		})
	}
}

func TestRuntimeEngineSourceTrackedInputs(t *testing.T) {
	fixture := newRuntimeEngineFixture(t)
	lock := DefaultRuntimeLock()
	baseline := fixture.digest(t, "HEAD", lock)
	for _, name := range []string{"monitor.go", "cmd/spx/internal/command/env.go", "cmd/spx/internal/command/dispatch.go", "cmd/spx/internal/logutil/logutil.go", "cmd/spx/template/project/main.tscn", ".github/scripts/runtime/build_pack.sh"} {
		fixture.write(t, name, "independent")
		fixture.commit(t)
		if fixture.digest(t, "HEAD", lock) != baseline {
			t.Fatalf("%s invalidated engine", name)
		}
	}
	for _, name := range []string{"register_types.cpp", "register_types.h", "SCsub", "spx_scons_profile.json", "generated/binding.inc"} {
		fixture.write(t, lock.Module.Path+"/"+name, "changed")
		fixture.commit(t)
		next := fixture.digest(t, "HEAD", lock)
		if next == baseline {
			t.Fatalf("native input %s did not invalidate engine", name)
		}
		baseline = next
	}
	workflow := engineWorkflowFixture(strings.Repeat("1", 40))
	fixture.write(t, ".github/workflows/build_release.yml", strings.Replace(workflow, "  engine:\n", "  engine:\n    name: Renamed job\n", 1))
	fixture.commit(t)
	if fixture.digest(t, "HEAD", lock) != baseline {
		t.Fatal("workflow presentation invalidated engine")
	}
	fixture.write(t, ".github/workflows/build_release.yml", engineWorkflowFixture(strings.Repeat("2", 40)))
	fixture.commit(t)
	if fixture.digest(t, "HEAD", lock) == baseline {
		t.Fatal("workflow implementation did not invalidate engine")
	}
	baseline = fixture.digest(t, "HEAD", lock)
	fixture.write(t, ".github/scripts/runtime/build_contract.py", "changed contract")
	fixture.commit(t)
	if fixture.digest(t, "HEAD", lock) == baseline {
		t.Fatal("SCons contract change did not invalidate engine")
	}
}

func TestRuntimeEngineSourceUsesRequestedRevision(t *testing.T) {
	fixture := newRuntimeEngineFixture(t)
	lock := DefaultRuntimeLock()
	revision := fixture.git(t, "rev-parse", "HEAD")
	baseline := fixture.digest(t, revision, lock)
	fixture.write(t, lock.Module.Path+"/register_types.cpp", "new engine")
	fixture.write(t, ".github/scripts/runtime/build_contract.py", "new contract")
	fixture.write(t, ".github/workflows/build_release.yml", engineWorkflowFixture(strings.Repeat("2", 40)))
	fixture.commit(t)
	current := fixture.digest(t, "HEAD", lock)
	if current == baseline {
		t.Fatal("new engine inputs did not change identity")
	}
	fixture.write(t, lock.Module.Path+"/register_types.cpp", "uncommitted engine")
	fixture.write(t, lock.Module.Path+"/untracked.cpp", "untracked input")
	fixture.write(t, ".github/scripts/runtime/build_contract.py", "uncommitted contract")
	fixture.write(t, ".github/workflows/build_release.yml", "uncommitted invalid workflow")
	if fixture.digest(t, "HEAD", lock) != current || fixture.digest(t, revision, lock) != baseline {
		t.Fatal("worktree or later commit contaminated requested engine revision")
	}
}

func TestRuntimeEngineSourcePinsMovingRef(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX Git wrapper")
	}
	fixture := newRuntimeEngineFixture(t)
	fixture.git(t, "branch", "engine-source")
	lock := DefaultRuntimeLock()
	original := fixture.digest(t, "engine-source", lock)
	fixture.write(t, ".github/workflows/build_release.yml", engineWorkflowFixture(strings.Repeat("2", 40)))
	fixture.commit(t)

	// Advance the branch immediately after Git has listed its original tree.
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	script := `#!/bin/sh
"$SPX_REAL_GIT" "$@" || exit $?
if [ "$3" = ls-tree ]; then
    "$SPX_REAL_GIT" -C "$2" update-ref refs/heads/engine-source HEAD
fi
`
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SPX_REAL_GIT", realGit)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	if got := fixture.digest(t, "engine-source", lock); got != original {
		t.Fatal("engine digest combined inputs from different revisions")
	}
	if got := fixture.digest(t, "engine-source", lock); got == original {
		t.Fatal("updated branch did not change the engine digest")
	}
}

func TestRuntimeEngineSourceRejectsIncompleteInputs(t *testing.T) {
	for _, test := range []struct {
		name, remove, want string
	}{
		{"module", DefaultRuntimeLock().Module.Path, "runtime engine module tree contains no tracked inputs"},
		{"contract", ".github/scripts/runtime/build_contract.py", "runtime engine contract tree contains no tracked inputs"},
		{"workflow", ".github/workflows/build_release.yml", "read engine build workflow"},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newRuntimeEngineFixture(t)
			fixture.git(t, "rm", "-r", test.remove)
			fixture.commit(t)
			_, err := RuntimeEngineSourceSHA256(fixture.root, "HEAD", DefaultRuntimeLock())
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("missing input error = %v, want %q", err, test.want)
			}
		})
	}
	fixture := newRuntimeEngineFixture(t)
	if _, err := RuntimeEngineSourceSHA256(fixture.root, "missing", DefaultRuntimeLock()); err == nil {
		t.Fatal("missing revision accepted")
	}
	invalid := DefaultRuntimeLock()
	invalid.Module.Path = "../outside"
	if _, err := RuntimeEngineSourceSHA256(fixture.root, "HEAD", invalid); err == nil {
		t.Fatal("invalid lock accepted")
	}
}

func TestRuntimeEngineSourceRequiresPinnedWorkflow(t *testing.T) {
	for _, test := range []struct {
		name, workflow, want string
	}{
		{"absent reference", "jobs: {}\n", "found 0"},
		{"unpinned reference", engineWorkflowFixture("main"), "found 0"},
		{"duplicate reference", engineWorkflowFixture(strings.Repeat("1", 40)) + "  another:\n    uses: goplus/godot/.github/workflows/release_builds.yml@" + strings.Repeat("2", 40) + "\n", "found 2"},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newRuntimeEngineFixture(t)
			fixture.write(t, ".github/workflows/build_release.yml", test.workflow)
			fixture.commit(t)
			_, err := RuntimeEngineSourceSHA256(fixture.root, "HEAD", DefaultRuntimeLock())
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("workflow pin error = %v, want %q", err, test.want)
			}
		})
	}
}

type runtimeEngineFixture struct{ root string }

func newRuntimeEngineFixture(t *testing.T) runtimeEngineFixture {
	t.Helper()
	fixture := runtimeEngineFixture{root: t.TempDir()}
	fixture.git(t, "init", "-q")
	fixture.write(t, DefaultRuntimeLock().Module.Path+"/register_types.cpp", "module")
	fixture.write(t, ".github/scripts/runtime/build_contract.py", "contract")
	fixture.write(t, ".github/workflows/build_release.yml", engineWorkflowFixture(strings.Repeat("1", 40)))
	fixture.commit(t)
	return fixture
}

func engineWorkflowFixture(pin string) string {
	return "jobs:\n  engine:\n    uses: goplus/godot/.github/workflows/release_builds.yml@" + pin + "\n"
}

func (f runtimeEngineFixture) git(t *testing.T, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", f.root}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com", "GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return strings.TrimSpace(string(output))
}

func (f runtimeEngineFixture) write(t *testing.T, name, data string) {
	t.Helper()
	filename := filepath.Join(f.root, name)
	if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filename, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func (f runtimeEngineFixture) commit(t *testing.T) {
	t.Helper()
	f.git(t, "add", ".")
	f.git(t, "commit", "-qm", "fixture")
}

func (f runtimeEngineFixture) digest(t *testing.T, revision string, lock RuntimeLock) string {
	t.Helper()
	value, err := RuntimeEngineSourceSHA256(f.root, revision, lock)
	if err != nil {
		t.Fatal(err)
	}
	return value
}
