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

package interpsession

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestPrepareCommandUsesExplicitRootsWithoutGlobalMutation(t *testing.T) {
	roots := testRoots(t)
	executable := filepath.Join(t.TempDir(), "engine")
	if err := os.WriteFile(executable, []byte("engine"), 0o755); err != nil {
		t.Fatal(err)
	}

	before, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	beforeEnv := os.Environ()
	stdin := strings.NewReader("input")
	var stdout, stderr bytes.Buffer
	cmd, err := PrepareCommand(context.Background(), CommandConfig{
		Roots:      roots,
		Executable: executable,
		Args:       []string{"--headless", "--path=/ignored", "--path", "/also-ignored"},
		Env:        []string{"KEEP=value", ProjectDirEnv + "=/ignored"},
		PathPolicy: ReplacePath,
		Stdin:      stdin,
		Stdout:     &stdout,
		Stderr:     &stderr,
	})
	if err != nil {
		t.Fatal(err)
	}
	after, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if after != before {
		t.Fatalf("working directory changed from %q to %q", before, after)
	}
	if afterEnv := os.Environ(); !reflect.DeepEqual(afterEnv, beforeEnv) {
		t.Fatal("process environment changed")
	}
	if cmd.Stdin != stdin || cmd.Stdout != &stdout || cmd.Stderr != &stderr {
		t.Fatal("command did not retain the configured input and output streams")
	}
	if cmd.Dir != roots.SessionDir {
		t.Fatalf("command Dir = %q, want %q", cmd.Dir, roots.SessionDir)
	}
	wantArgs := []string{executable, "--headless", "--path", roots.SessionDir, "--no-header"}
	if !reflect.DeepEqual(cmd.Args, wantArgs) {
		t.Fatalf("command Args = %#v, want %#v", cmd.Args, wantArgs)
	}
	wantEnv := []string{
		"KEEP=value",
		ProjectDirEnv + "=" + roots.ProjectDir,
		AssetDirEnv + "=" + roots.AssetDir,
		SessionDirEnv + "=" + roots.SessionDir,
	}
	if !reflect.DeepEqual(cmd.Env, wantEnv) {
		t.Fatalf("command Env = %#v, want %#v", cmd.Env, wantEnv)
	}
}

func TestPrepareCommandRejectsEnginePathByDefault(t *testing.T) {
	roots := testRoots(t)
	executable := filepath.Join(t.TempDir(), "engine")
	if err := os.WriteFile(executable, []byte("engine"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"--path", "/ignored"}, {"--path=/ignored"}} {
		if _, err := PrepareCommand(context.Background(), CommandConfig{
			Roots:      roots,
			Executable: executable,
			Args:       args,
		}); err == nil {
			t.Fatalf("PrepareCommand(%q) accepted reserved --path", args)
		}
	}
}

func TestPrepareCommandRejectsInvalidInputs(t *testing.T) {
	roots := testRoots(t)
	executable := filepath.Join(t.TempDir(), "engine")
	if err := os.WriteFile(executable, []byte("engine"), 0o755); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name         string
		ctx          context.Context
		executable   string
		policy       PathPolicy
		invalidRoots bool
		want         string
	}{
		{name: "nil context", executable: executable, ctx: nil, want: "nil context"},
		{name: "invalid roots", executable: executable, ctx: context.Background(), invalidRoots: true, want: "ProjectDir is empty"},
		{name: "missing executable", executable: filepath.Join(t.TempDir(), "missing"), ctx: context.Background(), want: "lstat Executable"},
		{name: "directory executable", executable: t.TempDir(), ctx: context.Background(), want: "not a regular"},
		{name: "unknown path policy", executable: executable, ctx: context.Background(), policy: PathPolicy(99), want: "unknown Engine path policy"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := roots
			if tt.invalidRoots {
				input = Roots{}
			}
			cmd, err := PrepareCommand(tt.ctx, CommandConfig{
				Roots: input, Executable: tt.executable, PathPolicy: tt.policy,
			})
			if cmd != nil || err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("PrepareCommand() error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestEngineArgsReplacesTrailingPathOption(t *testing.T) {
	args, err := engineArgs([]string{"--headless", "--path"}, "/session", ReplacePath)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"--headless", "--path", "/session", "--no-header"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("engineArgs() = %#v, want %#v", args, want)
	}
	if _, err := engineArgs(nil, "/session", PathPolicy(99)); err == nil || !strings.Contains(err.Error(), "unknown Engine path policy") {
		t.Fatalf("unknown path policy error = %v", err)
	}
}
