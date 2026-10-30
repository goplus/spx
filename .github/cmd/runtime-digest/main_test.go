package main

import (
	"bytes"
	"encoding/hex"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimeDigestUsesSelectedRevisionAndInputs(t *testing.T) {
	git := newDigestRepo(t)
	const recipe = ".github/scripts/runtime/build_contract.py"
	writeDigestInput(t, "go.mod", "module example.com/runtime\n")
	writeDigestInput(t, recipe, "# initial build recipe\n")
	git("add", ".")
	git("commit", "--quiet", "-m", "initial inputs")
	initial := git("rev-parse", "HEAD")
	packDigest := readDigest(t, "pack-source", initial)
	recipeDigest := readDigest(t, "build-recipe", initial)
	if packDigest == recipeDigest {
		t.Fatal("pack source and build recipe selected the same inputs")
	}

	writeDigestInput(t, "go.mod", "module example.com/updated-runtime\n")
	git("add", "go.mod")
	git("commit", "--quiet", "-m", "change pack input")
	updatedPackDigest := readDigest(t, "pack-source", "HEAD")
	if updatedPackDigest == packDigest {
		t.Fatal("changing a pack input did not change the pack digest")
	}
	if got := readDigest(t, "build-recipe", "HEAD"); got != recipeDigest {
		t.Fatal("changing a pack input changed the build recipe digest")
	}
	if got := readDigest(t, "pack-source", initial); got != packDigest {
		t.Fatal("digest used HEAD instead of the requested revision")
	}

	writeDigestInput(t, recipe, "# updated build recipe\n")
	if got := readDigest(t, "build-recipe", "HEAD"); got != recipeDigest {
		t.Fatal("an uncommitted input changed the build recipe digest")
	}
	git("add", recipe)
	git("commit", "--quiet", "-m", "change build recipe")
	if got := readDigest(t, "build-recipe", "HEAD"); got == recipeDigest {
		t.Fatal("changing the recipe did not change the build recipe digest")
	}
	if got := readDigest(t, "pack-source", "HEAD"); got != updatedPackDigest {
		t.Fatal("changing the recipe changed the pack digest")
	}
}

func TestRuntimeDigestRejectsInvalidArguments(t *testing.T) {
	for _, args := range [][]string{nil, {"pack-source"}, {"pack-source", "HEAD", "extra"}, {"unknown", "HEAD"}} {
		var stdout, stderr bytes.Buffer
		if code := run(args, &stdout, &stderr); code != 2 {
			t.Fatalf("run(%q) exit code = %d, want 2", args, code)
		}
		want := "usage: runtime-digest <pack-source|build-recipe> <revision>\n"
		if len(args) == 2 {
			want = "unknown runtime digest kind \"unknown\"; want pack-source or build-recipe\n"
		}
		if stdout.Len() != 0 || stderr.String() != want {
			t.Fatalf("run(%q): stdout = %q, stderr = %q; want only %q on stderr", args, stdout.String(), stderr.String(), want)
		}
	}
}

func TestRuntimeDigestReportsRevisionErrors(t *testing.T) {
	newDigestRepo(t)
	for _, kind := range []string{"pack-source", "build-recipe"} {
		for _, revision := range []string{"", "missing-revision"} {
			var stdout, stderr bytes.Buffer
			if code := run([]string{kind, revision}, &stdout, &stderr); code != 1 {
				t.Fatalf("run(%q, %q) exit code = %d, want 1", kind, revision, code)
			}
			want := `tree at "missing-revision"`
			if revision == "" {
				want = "revision must not be empty"
			}
			if stdout.Len() != 0 || !strings.Contains(stderr.String(), want) {
				t.Fatalf("run(%q, %q): stdout = %q, stderr = %q; want only an error containing %q", kind, revision, stdout.String(), stderr.String(), want)
			}
		}
	}
}

func readDigest(t *testing.T, kind, revision string) string {
	t.Helper()
	var stdout, stderr bytes.Buffer
	if code := run([]string{kind, revision}, &stdout, &stderr); code != 0 || stderr.Len() != 0 {
		t.Fatalf("run(%q, %q): exit code = %d, stderr = %q", kind, revision, code, stderr.String())
	}
	output := stdout.String()
	digest := strings.TrimSuffix(output, "\n")
	decoded, err := hex.DecodeString(digest)
	if err != nil || len(decoded) != 32 || output != digest+"\n" {
		t.Fatalf("digest output = %q; want SHA-256 followed by one newline", output)
	}
	return digest
}

func writeDigestInput(t *testing.T, name, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func newDigestRepo(t *testing.T) func(...string) string {
	t.Helper()
	t.Chdir(t.TempDir())
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
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
	git("config", "user.name", "Digest Test")
	git("config", "user.email", "digest@example.invalid")
	git("config", "commit.gpgsign", "false")
	return git
}

func TestRuntimeDigestMainExitStatus(t *testing.T) {
	const helper = "SPX_TEST_DIGEST_MAIN_ERROR"
	if mode := os.Getenv(helper); mode != "" {
		os.Args = []string{"runtime-digest"}
		if mode == "repository" {
			os.Args = append(os.Args, "pack-source", "HEAD")
		}
		main()
		return
	}
	for _, tt := range []struct {
		name    string
		code    int
		message string
	}{
		{"usage", 2, "usage: runtime-digest"},
		{"repository", 1, "git"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestRuntimeDigestMainExitStatus$")
			cmd.Dir = t.TempDir()
			cmd.Env = append(os.Environ(), helper+"="+tt.name)
			output, err := cmd.Output()
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != tt.code || len(output) != 0 {
				t.Fatalf("main = %v, stdout = %q; want exit %d without digest", err, output, tt.code)
			}
			if !strings.Contains(string(exit.Stderr), tt.message) {
				t.Fatalf("stderr = %q; want %q", exit.Stderr, tt.message)
			}
		})
	}
}
