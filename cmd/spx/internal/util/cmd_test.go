package util

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCommandHelperProcess(t *testing.T) {
	mode := os.Getenv("SPX_UTIL_TEST_HELPER")
	if mode == "" {
		return
	}
	if mode == "fail" {
		fmt.Fprintln(os.Stderr, "controlled command failure")
		os.Exit(7)
	}
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := json.NewEncoder(os.Stdout).Encode([]string{dir, os.Getenv("SPX_UTIL_TEST_VALUE"), os.Args[len(os.Args)-1]}); err != nil {
		t.Fatal(err)
	}
	os.Exit(0)
}

func TestOutputCommandAppliesDirectoryEnvironmentAndArguments(t *testing.T) {
	t.Setenv("SPX_UTIL_TEST_VALUE", "inherited")
	dir := t.TempDir()
	argument := "literal spaces; $HOME"
	for _, override := range []bool{false, true} {
		t.Run(fmt.Sprintf("override=%v", override), func(t *testing.T) {
			options := CommandOptions{Env: []string{"SPX_UTIL_TEST_HELPER=output"}}
			wantDir, err := os.Getwd()
			if err != nil {
				t.Fatal(err)
			}
			wantValue := "inherited"
			if override {
				options.Dir = dir
				options.Env = append(options.Env, "SPX_UTIL_TEST_VALUE=overridden")
				wantDir, wantValue = dir, "overridden"
			}
			output, err := OutputCommand(options, os.Args[0], "-test.run=^TestCommandHelperProcess$", "--", argument)
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			if err := json.Unmarshal(output, &got); err != nil {
				t.Fatalf("decode command output %q: %v", output, err)
			}
			// Getwd resolves the /var -> /private/var alias used by macOS temporary directories.
			wantDir, err = filepath.EvalSymlinks(wantDir)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != 3 || got[0] != wantDir || got[1] != wantValue || got[2] != argument {
				t.Fatalf("command observations = %q, want [%q %q %q]", got, wantDir, wantValue, argument)
			}
		})
	}
}

func TestCommandExecutionAndFailures(t *testing.T) {
	t.Setenv("SPX_UTIL_TEST_HELPER", "output")
	args := []string{"-test.run=^TestCommandHelperProcess$", "--", "argument"}
	for name, run := range map[string]func() error{
		"exec":             func() error { return ExecCommand(CommandOptions{}, os.Args[0], args...) },
		"run":              func() error { return RunCommand(nil, "", os.Args[0], args...) },
		"run in directory": func() error { return RunCommandInDir(t.TempDir(), os.Args[0], args...) },
	} {
		t.Run(name, func(t *testing.T) {
			if err := run(); err != nil {
				t.Fatal(err)
			}
		})
	}
	for name, run := range map[string]func() error{
		"exec": func() error {
			return ExecCommand(CommandOptions{Env: []string{"SPX_UTIL_TEST_HELPER=fail"}}, os.Args[0], args...)
		},
		"output": func() error {
			_, err := OutputCommand(CommandOptions{Env: []string{"SPX_UTIL_TEST_HELPER=fail"}}, os.Args[0], args...)
			return err
		},
	} {
		t.Run(name+" failure", func(t *testing.T) {
			var exit *exec.ExitError
			if err := run(); !errors.As(err, &exit) || exit.ExitCode() != 7 {
				t.Fatalf("command error = %v, want exit 7", err)
			}
		})
	}
	if _, err := OutputCommand(CommandOptions{}, filepath.Join(t.TempDir(), "missing")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing executable error = %v", err)
	}
}

func TestRunCommandFailureExits(t *testing.T) {
	if os.Getenv("SPX_UTIL_TEST_FATAL") == "1" {
		RunCommand(nil, "", os.Getenv("SPX_UTIL_TEST_MISSING"))
		t.Fatal("failed command returned instead of exiting")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestRunCommandFailureExits$")
	command.Env = append(os.Environ(), "SPX_UTIL_TEST_FATAL=1", "SPX_UTIL_TEST_MISSING="+filepath.Join(t.TempDir(), "missing-command"))
	if dir := flag.Lookup("test.gocoverdir"); dir != nil && dir.Value.String() != "" {
		command.Args = append(command.Args, "-test.gocoverdir="+dir.Value.String())
		command.Env = append(command.Env, "GOCOVERDIR="+dir.Value.String())
	}
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	var exit *exec.ExitError
	if err := command.Run(); !errors.As(err, &exit) || exit.ExitCode() != 1 {
		t.Fatalf("failed command exit = %v, stderr=%s", err, &stderr)
	}
	if stdout.Len() != 0 || !strings.Contains(stderr.String(), "missing-command failed:") || !strings.Contains(stderr.String(), "[ERROR] [SPX]") {
		t.Fatalf("failed command output: stdout=%q stderr=%q", &stdout, &stderr)
	}
}
