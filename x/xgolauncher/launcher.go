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

// Package xgolauncher runs a self-contained SPX payload embedded by the XGo
// runtime provider. Run is a library boundary and never changes global cwd,
// calls os.Exit, or downloads data. Exit is the single application boundary.
package xgolauncher

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"github.com/goplus/spx/v3/internal/interpruntime"
	"github.com/goplus/spx/v3/internal/processsupervisor"
	"github.com/goplus/spx/v3/internal/runtimebundle"
	"github.com/goplus/spx/v3/internal/runtimepayload"
)

// ProcessStatus preserves either a normal process exit code or a host signal.
// Signal is zero for a normal exit. Code is ignored when Signal is non-zero.
type ProcessStatus struct {
	Code   int
	Signal int
}

// Success reports whether the process exited normally with status zero.
func (s ProcessStatus) Success() bool { return s.Signal == 0 && s.Code == 0 }

// Run verifies and materializes an embedded payload, creates a fresh session,
// and runs the bundled Engine with applicationArgs. Child termination is
// returned as ProcessStatus; preparation/materialization failures are returned
// as errors and are never printed or converted into an application exit here.
func Run(payload []byte, payloadSHA256, manifestSHA256 string, applicationArgs []string, stdin io.Reader, stdout, stderr io.Writer) (ProcessStatus, error) {
	return RunContext(context.Background(), payload, payloadSHA256, manifestSHA256, applicationArgs, stdin, stdout, stderr)
}

// RunContext is Run with caller-controlled cancellation. Command entry points
// should wrap their complete parse/prepare/run lifecycle with RunCommand so
// cleanup finishes before an intercepted host signal is reproduced.
func RunContext(ctx context.Context, payload []byte, payloadSHA256, manifestSHA256 string, applicationArgs []string, stdin io.Reader, stdout, stderr io.Writer) (ProcessStatus, error) {
	return run(ctx, payload, payloadSHA256, manifestSHA256, applicationArgs, stdin, stdout, stderr, "")
}

func run(ctx context.Context, payload []byte, payloadSHA256, manifestSHA256 string, applicationArgs []string, stdin io.Reader, stdout, stderr io.Writer, cacheRoot string) (ProcessStatus, error) {
	if ctx == nil {
		return ProcessStatus{}, errors.New("xgolauncher: nil context")
	}
	verified, err := runtimepayload.Verify(payload, payloadSHA256, manifestSHA256, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return ProcessStatus{}, err
	}
	if cacheRoot == "" {
		cacheRoot, err = launcherCacheRoot()
		if err != nil {
			return ProcessStatus{}, err
		}
	}

	workDir, err := os.MkdirTemp("", "spx-launcher-")
	if err != nil {
		return ProcessStatus{}, fmt.Errorf("xgolauncher: create launcher work directory: %w", err)
	}
	defer os.RemoveAll(workDir)

	cache := runtimebundle.NewCache(cacheRoot)
	engine, err := materializeComponent(ctx, cache, workDir, runtimebundle.NamespaceEngine, verified.Manifest.Engine.BundleDigest, func(dst io.Writer) error {
		return verified.WriteComponentZIP("engine/", dst)
	})
	if err != nil {
		return ProcessStatus{}, fmt.Errorf("xgolauncher: materialize Engine: %w", err)
	}
	defer engine.Close()
	bridge, err := materializeComponent(ctx, cache, workDir, runtimebundle.NamespaceBridge, verified.Manifest.Bridge.BundleDigest, func(dst io.Writer) error {
		return verified.WriteComponentZIP("bridge/", dst)
	})
	if err != nil {
		return ProcessStatus{}, fmt.Errorf("xgolauncher: materialize bridge: %w", err)
	}
	defer bridge.Close()
	project, err := materializeComponent(ctx, cache, workDir, runtimebundle.NamespaceProject, verified.Manifest.Project.BundleDigest, func(dst io.Writer) error {
		return verified.WriteProjectZIP(dst)
	})
	if err != nil {
		return ProcessStatus{}, fmt.Errorf("xgolauncher: materialize project: %w", err)
	}
	defer project.Close()

	sessionDir, err := os.MkdirTemp("", "spx-session-")
	if err != nil {
		return ProcessStatus{}, fmt.Errorf("xgolauncher: create Engine session: %w", err)
	}
	defer os.RemoveAll(sessionDir)
	roots := interpruntime.Roots{
		ProjectDir: project.Path,
		AssetDir:   filepath.Join(project.Path, filepath.FromSlash(verified.Manifest.Project.PackDirectory)),
		SessionDir: sessionDir,
	}
	bridgePath := filepath.Join(bridge.Path, verified.Manifest.Bridge.File)
	if err := interpruntime.PrepareSession(interpruntime.SessionConfig{Roots: roots, BridgePath: bridgePath}); err != nil {
		return ProcessStatus{}, fmt.Errorf("xgolauncher: prepare Engine session: %w", err)
	}
	command, err := interpruntime.PrepareCommand(ctx, interpruntime.CommandConfig{
		Roots: roots, Executable: filepath.Join(engine.Path, verified.Manifest.Engine.Executable),
		Args: applicationArgs, Env: os.Environ(), Stdin: stdin, Stdout: stdout, Stderr: stderr,
		PathPolicy: interpruntime.RejectPath,
	})
	if err != nil {
		return ProcessStatus{}, fmt.Errorf("xgolauncher: prepare Engine command: %w", err)
	}
	status, err := processsupervisor.Run(ctx, command)
	if err != nil {
		return ProcessStatus{}, fmt.Errorf("xgolauncher: run Engine: %w", err)
	}
	return ProcessStatus{Code: status.Code, Signal: status.Signal}, nil
}

func launcherCacheRoot() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("xgolauncher: resolve user cache directory: %w", err)
	}
	if base == "" {
		return "", fmt.Errorf("xgolauncher: user cache directory is empty")
	}
	if err := os.MkdirAll(base, 0o700); err != nil {
		return "", fmt.Errorf("xgolauncher: prepare user cache directory: %w", err)
	}
	return filepath.Join(base, "spx-runtimebundle"), nil
}

func materializeComponent(ctx context.Context, cache *runtimebundle.Cache, workDir string, namespace runtimebundle.Namespace, expectedDigest string, write func(io.Writer) error) (*runtimebundle.Materialized, error) {
	zipPath := filepath.Join(workDir, string(namespace)+".zip")
	file, err := os.OpenFile(zipPath, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return nil, err
	}
	closeFile := true
	defer func() {
		if closeFile {
			_ = file.Close()
		}
	}()
	if err := write(file); err != nil {
		return nil, err
	}
	if err := file.Sync(); err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	bundle, err := runtimepayload.ComponentBundleReaderAt(file, info.Size(), namespace)
	if err != nil {
		return nil, err
	}
	if bundle.Digest != expectedDigest {
		return nil, fmt.Errorf("embedded %s identity changed after payload verification", namespace)
	}
	if err := file.Close(); err != nil {
		return nil, err
	}
	closeFile = false
	return cache.Materialize(ctx, namespace, zipPath, &bundle)
}
