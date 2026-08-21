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

package launchpack

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

const generatedLauncherTemplate = `package main

import (
	"context"
	_ "embed"
	"fmt"
	"os"

	"github.com/goplus/spx/v3/x/xgolauncher"
)

//go:embed payload.spxpkg
var payload []byte

const payloadSHA256 = %s
const manifestSHA256 = %s

func main() {
	status, err := xgolauncher.RunCommand(context.Background(), func(ctx context.Context) (xgolauncher.ProcessStatus, error) {
		return xgolauncher.RunContext(ctx, xgolauncher.Config{
			Payload: payload, PayloadSHA256: payloadSHA256, ManifestSHA256: manifestSHA256,
			Args: os.Args[1:], Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr,
		})
	})
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "xgolauncher: %%v\n", err)
		status = xgolauncher.ProcessStatus{Code: 1}
	}
	xgolauncher.Exit(status)
}
`

func renderGeneratedLauncher(payloadDigest, manifestDigest string) []byte {
	return []byte(fmt.Sprintf(generatedLauncherTemplate, strconv.Quote(payloadDigest), strconv.Quote(manifestDigest)))
}

type payloadBuilder func(workDir string, dst io.Writer) (payloadDigest, manifestDigest string, err error)

func compileLauncher(ctx context.Context, cfg Config, streams IO, buildPayload payloadBuilder) error {
	if err := cfg.validateGraphInputs(); err != nil {
		return err
	}
	if info, err := os.Lstat(cfg.Output); err == nil {
		return fmt.Errorf("launchpack: staging output %q already exists with mode %s", cfg.Output, info.Mode())
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("launchpack: inspect staging output %q: %w", cfg.Output, err)
	}
	workDir, err := os.MkdirTemp("", "spx-launchpack-build-")
	if err != nil {
		return fmt.Errorf("launchpack: create launcher work directory: %w", err)
	}
	keepWork := hasBuildFlag(cfg.BuildFlags, "work")
	if keepWork {
		if streams.Stderr != nil {
			_, _ = fmt.Fprintf(streams.Stderr, "SPXWORK=%s\n", workDir)
		}
	} else {
		defer os.RemoveAll(workDir)
	}
	payloadPath := filepath.Join(workDir, "payload.spxpkg")
	mainPath := filepath.Join(workDir, "main.go")
	if buildPayload == nil {
		return fmt.Errorf("launchpack: nil payload builder")
	}
	payloadFile, err := os.OpenFile(payloadPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("launchpack: create generated payload: %w", err)
	}
	payloadDigest, manifestDigest, buildErr := buildPayload(workDir, payloadFile)
	if buildErr == nil {
		buildErr = payloadFile.Sync()
	}
	if closeErr := payloadFile.Close(); buildErr == nil {
		buildErr = closeErr
	}
	if buildErr != nil {
		return fmt.Errorf("launchpack: write generated payload: %w", buildErr)
	}
	if err := os.WriteFile(mainPath, renderGeneratedLauncher(payloadDigest, manifestDigest), 0o600); err != nil {
		return fmt.Errorf("launchpack: write generated launcher: %w", err)
	}

	args := append([]string{"build"}, cfg.GraphFlags...)
	args = append(args, normalizedGoBuildFlags(cfg.BuildFlags)...)
	args = append(args, "-buildmode=exe", "-o", cfg.Output, mainPath)
	command := exec.CommandContext(ctx, cfg.GoCommand, args...)
	command.Dir = cfg.WorkDir
	command.Env = hostGoEnv(cfg, streams.Env)
	command.Stdin = streams.Stdin
	command.Stdout = streams.Stdout
	command.Stderr = streams.Stderr
	if err := cfg.verifyGraph(ctx, "before launcher build"); err != nil {
		return err
	}
	if err := command.Run(); err != nil {
		return fmt.Errorf("launchpack: build generated launcher: %w", err)
	}
	if err := cfg.verifyGraph(ctx, "after launcher build"); err != nil {
		return err
	}
	if err := validateHostExecutable(cfg.Output); err != nil {
		return err
	}
	if runtime.GOOS == "darwin" {
		if err := signDarwinLauncher(ctx, cfg.Output, streams); err != nil {
			return err
		}
	}
	return validateHostExecutable(cfg.Output)
}

func normalizedGoBuildFlags(flags []string) []string {
	result := make([]string, 0, len(flags))
	for _, flag := range flags {
		name, value, hasValue := strings.Cut(strings.TrimPrefix(flag, "-"), "=")
		switch name {
		case "v", "x", "work", "trimpath":
			if !hasValue || value == "true" {
				result = append(result, "-"+name)
			}
		case "buildvcs":
			result = append(result, "-buildvcs="+value)
		}
	}
	return result
}

func validateHostExecutable(name string) error {
	info, err := os.Lstat(name)
	if err != nil {
		return fmt.Errorf("launchpack: inspect launcher output %q: %w", name, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() == 0 {
		return fmt.Errorf("launchpack: launcher output %q is not a non-empty regular non-symlink file", name)
	}
	file, err := os.Open(name)
	if err != nil {
		return fmt.Errorf("launchpack: open launcher output: %w", err)
	}
	defer file.Close()
	var magic [4]byte
	if _, err := file.Read(magic[:]); err != nil {
		return fmt.Errorf("launchpack: read launcher output header: %w", err)
	}
	valid := false
	switch runtime.GOOS {
	case "darwin":
		valid = bytes.Equal(magic[:], []byte{0xcf, 0xfa, 0xed, 0xfe}) || bytes.Equal(magic[:], []byte{0xfe, 0xed, 0xfa, 0xcf}) || bytes.Equal(magic[:], []byte{0xca, 0xfe, 0xba, 0xbe})
	case "linux":
		valid = bytes.Equal(magic[:], []byte{0x7f, 'E', 'L', 'F'})
	case "windows":
		valid = magic[0] == 'M' && magic[1] == 'Z'
	}
	if !valid {
		return fmt.Errorf("launchpack: launcher output %q is not a host %s executable", name, runtime.GOOS)
	}
	return nil
}

func signDarwinLauncher(ctx context.Context, output string, streams IO) error {
	sign := exec.CommandContext(ctx, "/usr/bin/codesign", "--force", "--sign", "-", output)
	sign.Stdout, sign.Stderr = streams.Stdout, streams.Stderr
	if err := sign.Run(); err != nil {
		return fmt.Errorf("launchpack: ad-hoc sign launcher: %w", err)
	}
	verify := exec.CommandContext(ctx, "/usr/bin/codesign", "--verify", "--strict", output)
	verify.Stdout, verify.Stderr = streams.Stdout, streams.Stderr
	if err := verify.Run(); err != nil {
		return fmt.Errorf("launchpack: verify launcher signature: %w", err)
	}
	return nil
}
