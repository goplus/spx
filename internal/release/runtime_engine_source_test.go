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
	"strings"
	"testing"
)

func TestRuntimeEngineSourceIdentity(t *testing.T) {
	repo := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com", "GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, output)
		}
	}
	write := func(name, data string) {
		t.Helper()
		filename := filepath.Join(repo, name)
		if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filename, []byte(data), 0644); err != nil {
			t.Fatal(err)
		}
	}
	commit := func() { git("add", "."); git("commit", "-qm", "fixture") }
	lock := DefaultRuntimeLock()
	git("init", "-q")
	write(lock.Module.Path+"/register_types.cpp", "module")
	write(".github/scripts/runtime/build_contract.py", "contract")
	write(".github/workflows/build_release.yml", "jobs:\n  engine:\n    uses: goplus/godot/.github/workflows/release_builds.yml@"+strings.Repeat("1", 40)+"\n")
	commit()
	digest := func(lock RuntimeLock) string {
		t.Helper()
		value, err := RuntimeEngineSourceSHA256(repo, "HEAD", lock)
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	baseline := digest(lock)
	for _, name := range []string{"monitor.go", "cmd/spx/internal/command/env.go", "cmd/spx/internal/command/dispatch.go", "cmd/spx/internal/logutil/logutil.go", "cmd/spx/template/project/main.tscn", ".github/scripts/runtime/build_pack.sh"} {
		write(name, "independent")
		commit()
		if digest(lock) != baseline {
			t.Fatalf("%s invalidated engine", name)
		}
	}
	for _, mutate := range []func(*RuntimeLock){
		func(l *RuntimeLock) { l.RuntimeVersion = "9.9.9" },
		func(l *RuntimeLock) { l.Toolchain.Go = "1.99.0" },
		func(l *RuntimeLock) { l.Toolchain.XGo = "9.0.0" },
		func(l *RuntimeLock) { l.Godot.Ref = "other-ref" },
	} {
		changed := lock
		mutate(&changed)
		if digest(changed) != baseline {
			t.Fatal("publication/tooling metadata invalidated engine")
		}
	}
	for _, mutate := range []func(*RuntimeLock){
		func(l *RuntimeLock) { l.RuntimeABI++ },
		func(l *RuntimeLock) { l.Godot.Commit = strings.Repeat("1", 40) },
		func(l *RuntimeLock) { l.Toolchain.SCons = "9.0.0" },
		func(l *RuntimeLock) { l.Toolchain.EMSDK = "9.0.0" },
		func(l *RuntimeLock) { l.Toolchain.AndroidNDK = "99.0.0" },
		func(l *RuntimeLock) { l.Toolchain.JDK = "21" },
	} {
		changed := lock
		mutate(&changed)
		if digest(changed) == baseline {
			t.Fatal("engine lock change did not invalidate engine")
		}
	}
	for _, name := range []string{"register_types.cpp", "register_types.h", "SCsub", "spx_scons_profile.json", "generated/binding.inc"} {
		write(lock.Module.Path+"/"+name, "changed")
		commit()
		next := digest(lock)
		if next == baseline {
			t.Fatalf("native input %s did not invalidate engine", name)
		}
		baseline = next
	}
	write(".github/workflows/build_release.yml", "jobs:\n  engine:\n    name: Renamed job\n    uses: goplus/godot/.github/workflows/release_builds.yml@"+strings.Repeat("1", 40)+"\n")
	commit()
	if digest(lock) != baseline {
		t.Fatal("workflow presentation invalidated engine")
	}
	write(".github/workflows/build_release.yml", "jobs:\n  engine:\n    uses: goplus/godot/.github/workflows/release_builds.yml@"+strings.Repeat("2", 40)+"\n")
	commit()
	if digest(lock) == baseline {
		t.Fatal("workflow implementation did not invalidate engine")
	}
	baseline = digest(lock)

	write(".github/scripts/runtime/build_contract.py", "changed contract")
	commit()
	if digest(lock) == baseline {
		t.Fatal("SCons contract change did not invalidate engine")
	}
	if _, err := RuntimeEngineSourceSHA256(repo, "missing", lock); err == nil {
		t.Fatal("missing revision accepted")
	}
	git("rm", ".github/scripts/runtime/build_contract.py")
	commit()
	if _, err := RuntimeEngineSourceSHA256(repo, "HEAD", lock); err == nil {
		t.Fatal("missing build contract accepted")
	}
	write(".github/scripts/runtime/build_contract.py", "contract")
	commit()
	invalid := lock
	invalid.Module.Path = "../outside"
	if _, err := RuntimeEngineSourceSHA256(repo, "HEAD", invalid); err == nil {
		t.Fatal("invalid lock accepted")
	}

	git("rm", "-r", lock.Module.Path)
	commit()
	if _, err := RuntimeEngineSourceSHA256(repo, "HEAD", lock); err == nil {
		t.Fatal("missing module accepted")
	}
}
