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
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestHighestTag(t *testing.T) {
	tests := []struct {
		name string
		tags []string
		want string
	}{
		{"empty", nil, ""},
		{"other majors and invalid tags", []string{"v3.99.0", "v5.0.0", "v40.0.0", "4.9.0", "v4", "v4.bad", "v4.01.0", "v4.1.0-01"}, ""},
		{"numeric ordering", []string{"v4.9.9", "v4.10.0", "v4.2.99"}, "v4.10.0"},
		{"prerelease numeric ordering", []string{"v4.2.0-rc.2", "v4.2.0-rc.10", "v4.2.0-beta.99", "v4.1.99"}, "v4.2.0-rc.10"},
		{"release beats prerelease", []string{"v4.2.0-rc.10", "v4.2.0", "v4.2.0-beta"}, "v4.2.0"},
		{"mixed valid and invalid", []string{"v5.99.0", "v4.2.0", "v4.100.invalid", "v4.10.0-rc.1", "v3.999.0"}, "v4.10.0-rc.1"},
		{"build metadata preserves first equal version", []string{"v4.2.0+first", "v4.2.0+second"}, "v4.2.0+first"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := highestTag(tt.tags); got != tt.want {
				t.Fatalf("highestTag(%q) = %q, want %q", tt.tags, got, tt.want)
			}
		})
	}
}

func TestPseudoVersionMain(t *testing.T) {
	t.Run("untagged commit uses UTC and twelve digit revision", func(t *testing.T) {
		git := newVersionRepo(t)
		sha := git("rev-parse", "HEAD")
		// Empty optional targets have the same behavior as omitting the target.
		for _, args := range [][]string{nil, {""}} {
			want := "v4.0.0-20260102010405-" + sha[:12] + "\n"
			if got := runVersionMain(t, args...); got != want {
				t.Fatalf("main(%q) = %q, want %q", args, got, want)
			}
		}
	})
	t.Run("highest exact tag including annotated tags wins", func(t *testing.T) {
		git := newVersionRepo(t)
		for _, tag := range []string{"v4.2.0", "v4.10.0-rc.2", "v5.99.0", "v4.invalid"} {
			git("tag", tag)
		}
		git("tag", "-a", "v4.10.0", "-m", "release")
		if got := runVersionMain(t); got != "v4.10.0\n" {
			t.Fatalf("main() = %q", got)
		}
	})
	t.Run("release ancestor increments patch and ignores unreachable tags", func(t *testing.T) {
		git := newVersionRepo(t)
		git("tag", "v4.2.4")
		git("tag", "v3.99.0")
		git("commit", "--quiet", "--allow-empty", "-m", "target")
		target := git("rev-parse", "HEAD")
		git("commit", "--quiet", "--allow-empty", "-m", "future")
		git("tag", "v4.99.0")
		want := "v4.2.5-0.20260102010405-" + target[:12] + "\n"
		if got := runVersionMain(t, target); got != want {
			t.Fatalf("main(target) = %q, want %q", got, want)
		}
	})
	t.Run("prerelease ancestor gets dotted pseudo suffix", func(t *testing.T) {
		git := newVersionRepo(t)
		git("tag", "v4.5.0-rc.2")
		git("commit", "--quiet", "--allow-empty", "-m", "after prerelease")
		sha := git("rev-parse", "HEAD")
		want := "v4.5.0-rc.2.0.20260102010405-" + sha[:12] + "\n"
		if got := runVersionMain(t); got != want {
			t.Fatalf("main() = %q, want %q", got, want)
		}
	})
}

func TestPseudoVersionRejectsTagQueryErrors(t *testing.T) {
	const helperEnv = "SPX_TEST_PSEUDO_VERSION_TAG_TARGET"
	if target := os.Getenv(helperEnv); target != "" {
		os.Args = []string{"npm-version", target}
		main()
		return
	}
	for _, query := range []string{"--points-at", "--merged"} {
		t.Run(query, func(t *testing.T) {
			git := newVersionRepo(t)
			target := "HEAD"
			if query == "--points-at" {
				// An invalid ref is a tag-query failure, not an untagged commit.
				if err := os.WriteFile(".git/refs/tags/v4.1.0", []byte(strings.Repeat("b", 40)+"\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			} else {
				// rev-parse and --points-at accept a blob, but --merged must fail.
				if err := os.WriteFile("blob", []byte("not a commit"), 0o600); err != nil {
					t.Fatal(err)
				}
				target = git("hash-object", "-w", "blob")
			}
			cmd := exec.Command(os.Args[0], "-test.run=^TestPseudoVersionRejectsTagQueryErrors$")
			cmd.Env = append(os.Environ(), helperEnv+"="+target)
			output, err := cmd.Output()
			exit, ok := err.(*exec.ExitError)
			if !ok || exit.ExitCode() != 1 || len(output) != 0 {
				t.Fatalf("tag-query failure: error = %v, version = %q; want exit 1 and no version", err, output)
			}
			if !strings.Contains(string(exit.Stderr), "git tag "+query) {
				t.Fatalf("tag-query diagnostic = %q; want failed %s command", exit.Stderr, query)
			}
		})
	}
}

func TestPseudoVersionInvalidTarget(t *testing.T) {
	const helperEnv = "SPX_TEST_PSEUDO_VERSION_INVALID_TARGET"
	if os.Getenv(helperEnv) == "1" {
		os.Args = []string{"npm-version", "refs/heads/nonexistent-version-target"}
		main()
		return
	}
	newVersionRepo(t)
	cmd := exec.Command(os.Args[0], "-test.run=^TestPseudoVersionInvalidTarget$")
	cmd.Env = append(os.Environ(), helperEnv+"=1")
	out, err := cmd.CombinedOutput()
	exit, ok := err.(*exec.ExitError)
	if !ok || exit.ExitCode() != 1 {
		t.Fatalf("invalid target: error = %v, output = %q; want exit status 1", err, out)
	}
	if !strings.Contains(string(out), "git rev-parse refs/heads/nonexistent-version-target:") {
		t.Fatalf("invalid target diagnostic = %q; want failed Git command", out)
	}
}

// Each scenario uses an isolated repository, so developer tags and Git settings
// cannot change the release version being checked.
func newVersionRepo(t *testing.T) func(...string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is required for pseudo-version integration tests")
	}
	t.Chdir(t.TempDir())
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_AUTHOR_NAME", "Version Test")
	t.Setenv("GIT_AUTHOR_EMAIL", "version@example.invalid")
	t.Setenv("GIT_COMMITTER_NAME", "Version Test")
	t.Setenv("GIT_COMMITTER_EMAIL", "version@example.invalid")
	t.Setenv("GIT_AUTHOR_DATE", "2026-01-02T03:04:05+02:00")
	t.Setenv("GIT_COMMITTER_DATE", "2026-01-02T03:04:05+02:00")
	for _, key := range []string{"GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_COMMON_DIR", "GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES"} {
		if value, ok := os.LookupEnv(key); ok {
			t.Cleanup(func() { _ = os.Setenv(key, value) })
			if err := os.Unsetenv(key); err != nil {
				t.Fatal(err)
			}
		}
	}
	git := func(args ...string) string {
		t.Helper()
		out, err := exec.Command("git", args...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "--quiet")
	git("config", "commit.gpgsign", "false")
	git("config", "tag.gpgsign", "false")
	git("commit", "--quiet", "--allow-empty", "-m", "initial")
	return git
}

func runVersionMain(t *testing.T, args ...string) string {
	t.Helper()
	oldArgs, oldStdout := os.Args, os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		os.Args, os.Stdout = oldArgs, oldStdout
		_ = reader.Close()
		_ = writer.Close()
	})
	os.Args = append([]string{"npm-version"}, args...)
	os.Stdout = writer
	main()
	os.Args, os.Stdout = oldArgs, oldStdout
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	out, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}
