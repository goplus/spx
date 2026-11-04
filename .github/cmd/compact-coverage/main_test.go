package main

import (
	"bytes"
	"errors"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/tools/cover"
)

func TestCompactProfilesMergeCountersAndRetainUncoveredBlocks(t *testing.T) {
	for _, mode := range []string{"atomic", "count", "set"} {
		t.Run(mode, func(t *testing.T) {
			first, second, wantCount := "2", "3", "5"
			if mode == "set" {
				first, second, wantCount = "1", "1", "1"
			}
			input := "mode: " + mode + "\n" +
				"example.com/z.go:9.2,11.3 2 0\n" +
				"example.com/a file.go:4.1,5.2 1 " + first + "\n" +
				"example.com/a file.go:1.1,2.2 3 0\n" +
				"example.com/a file.go:4.1,5.2 1 " + second + "\n"
			want := "mode: " + mode + "\n" +
				"example.com/a file.go:1.1,2.2 3 0\n" +
				"example.com/a file.go:4.1,5.2 1 " + wantCount + "\n" +
				"example.com/z.go:9.2,11.3 2 0\n"
			name := filepath.Join(t.TempDir(), "coverage with spaces.out")
			if err := os.WriteFile(name, []byte(input), 0o640); err != nil {
				t.Fatal(err)
			}
			original, err := os.Stat(name)
			if err != nil {
				t.Fatal(err)
			}
			for range 2 {
				var stderr bytes.Buffer
				if code := run([]string{name}, &stderr); code != 0 || stderr.Len() != 0 {
					t.Fatalf("run = %d, %q", code, stderr.String())
				}
				got, err := os.ReadFile(name)
				if err != nil || string(got) != want {
					t.Fatalf("profile = %q, %v; want %q", got, err, want)
				}
				info, err := os.Stat(name)
				if err != nil {
					t.Fatal(err)
				}
				if info.Mode().Perm() != original.Mode().Perm() {
					t.Fatalf("permissions = %v; want %v", info.Mode().Perm(), original.Mode().Perm())
				}
				entries, err := os.ReadDir(filepath.Dir(name))
				if err != nil || len(entries) != 1 {
					t.Fatalf("temporary output was left behind: %v, %v", entries, err)
				}
			}
		})
	}
}

func TestCompactHeaderOnlyProfiles(t *testing.T) {
	var names []string
	var expected []string
	for _, mode := range []string{"atomic", "count", "set"} {
		for _, ending := range []string{"\r\n", "\n", ""} {
			name := filepath.Join(t.TempDir(), "coverage.out")
			if err := os.WriteFile(name, []byte("mode: "+mode+ending), 0o644); err != nil {
				t.Fatal(err)
			}
			names = append(names, name)
			expected = append(expected, "mode: "+mode+"\n")
		}
	}
	var stderr bytes.Buffer
	if code := run(names, &stderr); code != 0 || stderr.Len() != 0 {
		t.Fatalf("run = %d, %q", code, stderr.String())
	}
	for i, want := range expected {
		got, err := os.ReadFile(names[i])
		if err != nil || string(got) != want {
			t.Fatalf("profile = %q, %v", got, err)
		}
	}
}

func TestCompactInvalidProfilesStopBeforeLaterInputs(t *testing.T) {
	for _, input := range []string{
		"",
		"mode: unknown\n",
		"mode: atomic\nmalformed\n",
		"mode: count\na.go:1.1,2.2 1 3\na.go:1.1,2.2 2 4\n",
	} {
		t.Run(input, func(t *testing.T) {
			dir := t.TempDir()
			name := filepath.Join(dir, "coverage.out")
			later := filepath.Join(dir, "later.out")
			// A valid later profile would change if run did not stop on the error.
			unchanged := map[string]string{name: input, later: "mode: atomic\r\n"}
			for path, content := range unchanged {
				if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			var stderr bytes.Buffer
			if code := run([]string{name, later}, &stderr); code != 1 || !strings.Contains(stderr.String(), name) {
				t.Fatalf("run = %d, %q", code, stderr.String())
			}
			for path, content := range unchanged {
				got, err := os.ReadFile(path)
				if err != nil || string(got) != content {
					t.Fatalf("%s changed: %q, %v", path, got, err)
				}
			}
			entries, err := os.ReadDir(dir)
			if err != nil || len(entries) != len(unchanged) {
				t.Fatalf("temporary output was left behind: %v, %v", entries, err)
			}
		})
	}
}

func TestCompactReportsUsageAndMissingProfiles(t *testing.T) {
	var stderr bytes.Buffer
	if code := run(nil, &stderr); code != 2 || !strings.Contains(stderr.String(), "usage:") {
		t.Fatalf("usage = %d, %q", code, stderr.String())
	}
	stderr.Reset()
	name := filepath.Join(t.TempDir(), "missing.out")
	if code := run([]string{name}, &stderr); code != 1 || !strings.Contains(stderr.String(), name) {
		t.Fatalf("missing profile = %d, %q", code, stderr.String())
	}
}

func TestCompactInputFailuresPreserveProfile(t *testing.T) {
	for _, failure := range []string{"stat", "read", "create output"} {
		t.Run(failure, func(t *testing.T) {
			dir := t.TempDir()
			name := filepath.Join(dir, "coverage.out")
			const original = "mode: atomic\r\n"
			if err := os.WriteFile(name, []byte(original), 0o640); err != nil {
				t.Fatal(err)
			}
			inputName, outputName := name, name
			if failure == "read" {
				inputName = dir
			}
			if failure == "create output" {
				// A file in the destination parent path prevents temporary creation.
				outputName = filepath.Join(name, "coverage.out")
			}
			input, err := os.Open(inputName)
			if err != nil {
				t.Fatal(err)
			}
			defer input.Close()
			if failure == "stat" {
				if err := input.Close(); err != nil {
					t.Fatal(err)
				}
			}
			if err := compactOpenedProfile(outputName, input); err == nil {
				t.Fatal("accepted failing input or destination")
			}
			got, err := os.ReadFile(name)
			if err != nil || string(got) != original {
				t.Fatalf("original profile = %q, %v; want %q", got, err, original)
			}
			entries, err := os.ReadDir(dir)
			if err != nil || len(entries) != 1 {
				t.Fatalf("temporary output was left behind: %v, %v", entries, err)
			}
		})
	}
}

type closeErrorFile struct {
	*os.File
	err error
}

func (f closeErrorFile) Close() error {
	if err := f.File.Close(); err != nil {
		return err
	}
	return f.err
}

func TestCompactOutputFailuresPreserveProfileAndCleanTemporaryFile(t *testing.T) {
	for _, failure := range []string{"chmod", "flush", "buffered write", "close", "rename"} {
		t.Run(failure, func(t *testing.T) {
			dir := t.TempDir()
			name := filepath.Join(dir, "coverage.out")
			const original = "mode: atomic\r\n"
			if err := os.WriteFile(name, []byte(original), 0o640); err != nil {
				t.Fatal(err)
			}
			output, err := os.CreateTemp(dir, ".coverage-*")
			if err != nil {
				t.Fatal(err)
			}
			defer output.Close()
			var profiles []*cover.Profile
			if failure == "chmod" || failure == "flush" || failure == "buffered write" {
				if err := output.Close(); err != nil {
					t.Fatal(err)
				}
			}
			if failure == "flush" || failure == "buffered write" {
				// Chmod succeeds on a read-only descriptor, but writes fail.
				output, err = os.Open(output.Name())
				if err != nil {
					t.Fatal(err)
				}
				defer output.Close()
			}
			if failure == "buffered write" {
				// Exceed bufio's buffer so Flush must retain an earlier write error.
				profiles = []*cover.Profile{{
					FileName: strings.Repeat("x", 8192) + ".go",
					Blocks:   []cover.ProfileBlock{{StartLine: 1, StartCol: 1, EndLine: 2, EndCol: 1, NumStmt: 1}},
				}}
			}
			destination := name
			if failure == "rename" {
				destination = filepath.Join(dir, "blocked")
				if err := os.Mkdir(destination, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			var replacementOutput profileOutput = output
			var closeErr error
			if failure == "close" {
				// A delayed I/O error can surface after writes have succeeded.
				closeErr = errors.New("delayed output close failure")
				replacementOutput = closeErrorFile{File: output, err: closeErr}
			}
			err = replaceProfile(destination, replacementOutput, 0o640, "mode: atomic", profiles)
			if err == nil {
				t.Fatal("accepted failing output or destination")
			}
			if closeErr != nil && !errors.Is(err, closeErr) {
				t.Fatalf("close error = %v; want %v", err, closeErr)
			}
			got, err := os.ReadFile(name)
			if err != nil || string(got) != original {
				t.Fatalf("original profile = %q, %v; want %q", got, err, original)
			}
			if _, err := os.Stat(output.Name()); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("temporary output was left behind: %v", err)
			}
			if failure == "rename" {
				if info, err := os.Stat(destination); err != nil || !info.IsDir() {
					t.Fatalf("failed rename changed the destination directory: %v, %v", info, err)
				}
			}
		})
	}
}

func TestCompactMainExitStatus(t *testing.T) {
	const helper = "SPX_TEST_COMPACT_COVERAGE_MAIN"
	if mode := os.Getenv(helper); mode != "" {
		os.Args = []string{"compact-coverage"}
		if mode != "usage" {
			os.Args = append(os.Args, mode+".out")
		}
		main()
		return
	}
	for _, tt := range []struct {
		name    string
		code    int
		message string
	}{
		{"usage", 2, "usage: compact-coverage"},
		{"missing", 1, "missing.out"},
		{"success", 0, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			name := filepath.Join(dir, "success.out")
			if err := os.WriteFile(name, []byte("mode: count\r\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestCompactMainExitStatus$")
			cmd.Dir = dir
			cmd.Env = append(os.Environ(), helper+"="+tt.name)
			// Include the subprocess's main coverage in the parent test profile.
			if coverageDir := flag.Lookup("test.gocoverdir"); coverageDir != nil && coverageDir.Value.String() != "" {
				cmd.Env = append(cmd.Env, "GOCOVERDIR="+coverageDir.Value.String())
			}
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			stdout, err := cmd.Output()
			code := 0
			var exit *exec.ExitError
			if errors.As(err, &exit) {
				code = exit.ExitCode()
			} else if err != nil {
				t.Fatal(err)
			}
			if code != tt.code || len(stdout) != 0 || !strings.Contains(stderr.String(), tt.message) {
				t.Fatalf("main = %d, stdout = %q, stderr = %q; want exit %d and %q", code, stdout, stderr.String(), tt.code, tt.message)
			}
			if tt.name == "success" {
				if stderr.Len() != 0 {
					t.Fatalf("successful main wrote stderr: %q", stderr.String())
				}
				got, err := os.ReadFile(name)
				if err != nil || string(got) != "mode: count\n" {
					t.Fatalf("profile = %q, %v", got, err)
				}
			}
		})
	}
}
