package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
			for range 2 {
				var stderr bytes.Buffer
				if code := run([]string{name}, &stderr); code != 0 || stderr.Len() != 0 {
					t.Fatalf("run = %d, %q", code, stderr.String())
				}
				got, err := os.ReadFile(name)
				if err != nil || string(got) != want {
					t.Fatalf("profile = %q, %v; want %q", got, err, want)
				}
			}
		})
	}
}

func TestCompactHeaderOnlyProfiles(t *testing.T) {
	var names []string
	for _, mode := range []string{"atomic", "count", "set"} {
		name := filepath.Join(t.TempDir(), "coverage.out")
		if err := os.WriteFile(name, []byte("mode: "+mode+"\r\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		names = append(names, name)
	}
	var stderr bytes.Buffer
	if code := run(names, &stderr); code != 0 || stderr.Len() != 0 {
		t.Fatalf("run = %d, %q", code, stderr.String())
	}
	for i, mode := range []string{"atomic", "count", "set"} {
		got, err := os.ReadFile(names[i])
		if err != nil || string(got) != "mode: "+mode+"\n" {
			t.Fatalf("profile = %q, %v", got, err)
		}
	}
}

func TestCompactInvalidProfilesRemainUntouched(t *testing.T) {
	for _, input := range []string{
		"",
		"mode: unknown\n",
		"mode: atomic\nmalformed\n",
		"mode: count\na.go:1.1,2.2 1 3\na.go:1.1,2.2 2 4\n",
	} {
		t.Run(input, func(t *testing.T) {
			dir := t.TempDir()
			name := filepath.Join(dir, "coverage.out")
			if err := os.WriteFile(name, []byte(input), 0o644); err != nil {
				t.Fatal(err)
			}
			var stderr bytes.Buffer
			if code := run([]string{name}, &stderr); code != 1 || !strings.Contains(stderr.String(), name) {
				t.Fatalf("run = %d, %q", code, stderr.String())
			}
			got, err := os.ReadFile(name)
			if err != nil || string(got) != input {
				t.Fatalf("invalid input changed: %q, %v", got, err)
			}
			entries, err := os.ReadDir(dir)
			if err != nil || len(entries) != 1 {
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
