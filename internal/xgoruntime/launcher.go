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

package xgoruntime

import (
	"bytes"
	"context"
	"debug/buildinfo"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	runtimedebug "runtime/debug"
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
		return xgolauncher.RunContext(ctx, payload, payloadSHA256, manifestSHA256, os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
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

type listedPackage struct {
	ImportPath string        `json:"ImportPath"`
	Name       string        `json:"Name"`
	Module     *listedModule `json:"Module"`
}

type listedModule struct {
	Path    string        `json:"Path"`
	Version string        `json:"Version"`
	Dir     string        `json:"Dir"`
	GoMod   string        `json:"GoMod"`
	Main    bool          `json:"Main"`
	Replace *listedModule `json:"Replace"`
}

func verifyLauncherPackage(ctx context.Context, cfg Config, baseEnv []string) error {
	return verifyPackageOrigin(ctx, cfg, baseEnv, "github.com/goplus/spx/v3/x/xgolauncher", "launcher")
}

func verifyProviderPackage(ctx context.Context, cfg Config, baseEnv []string) error {
	return verifyPackageOrigin(ctx, cfg, baseEnv, cfg.ProviderPackage, "provider")
}

func verifyPackageOrigin(ctx context.Context, cfg Config, baseEnv []string, importPath, label string) error {
	if err := cfg.validateGraphInputs(); err != nil {
		return err
	}
	for _, flag := range cfg.GraphFlags {
		if flag == "-mod=vendor" {
			return fmt.Errorf("xgoruntime: runtime provider does not support vendor mode; use -mod=readonly or -mod=mod")
		}
	}
	args := append([]string{"list"}, cfg.GraphFlags...)
	args = append(args, "-json", importPath)
	command := exec.CommandContext(ctx, cfg.GoCommand, args...)
	command.Dir = cfg.GraphWorkDir
	command.Env = hostGoEnv(cfg, baseEnv)
	output, err := command.Output()
	if err != nil {
		return fmt.Errorf("xgoruntime: validate %s package: %w", label, err)
	}
	var listed listedPackage
	if err := json.Unmarshal(output, &listed); err != nil {
		return fmt.Errorf("xgoruntime: decode %s package identity: %w", label, err)
	}
	if listed.ImportPath != importPath || listed.Module == nil {
		return fmt.Errorf("xgoruntime: %s package resolved to unexpected identity %q", label, listed.ImportPath)
	}
	got, err := normalizeListedOrigin(listed.Module)
	if err != nil {
		return fmt.Errorf("xgoruntime: invalid %s package origin: %w", label, err)
	}
	if !sameModuleOrigin(got, cfg.ProviderOrigin) {
		return fmt.Errorf("xgoruntime: %s package origin does not match resolved provider provenance", label)
	}
	return nil
}

func normalizeListedOrigin(module *listedModule) (ModuleOrigin, error) {
	if module == nil {
		return ModuleOrigin{}, fmt.Errorf("missing module")
	}
	origin := ModuleOrigin{
		Selected: ModuleRef{Path: module.Path, Version: module.Version},
		Main:     module.Main,
	}
	if module.Replace == nil {
		origin.Selected.Dir, origin.Selected.GoMod = module.Dir, module.GoMod
	} else {
		replacement := ModuleRef{
			Path: module.Replace.Path, Version: module.Replace.Version,
			Dir: module.Replace.Dir, GoMod: module.Replace.GoMod,
		}
		if replacement.Version == "" {
			replacement.Path = replacement.Dir
		}
		origin.Replace = &replacement
	}
	if err := origin.Validate(); err != nil {
		return ModuleOrigin{}, err
	}
	return origin, nil
}

func sameModuleOrigin(first, second ModuleOrigin) bool {
	if first.Main != second.Main || first.Selected != second.Selected {
		return false
	}
	if first.Replace == nil || second.Replace == nil {
		return first.Replace == nil && second.Replace == nil
	}
	return *first.Replace == *second.Replace
}

type payloadBuilder func(workDir string, dst io.Writer) (payloadDigest, manifestDigest string, err error)

func compileLauncher(ctx context.Context, cfg Config, streams IO, buildPayload payloadBuilder) error {
	if err := cfg.validateGraphInputs(); err != nil {
		return err
	}
	if info, err := os.Lstat(cfg.Output); err == nil {
		return fmt.Errorf("xgoruntime: staging output %q already exists with mode %s", cfg.Output, info.Mode())
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("xgoruntime: inspect staging output %q: %w", cfg.Output, err)
	}
	workDir, err := os.MkdirTemp("", "spx-provider-build-")
	if err != nil {
		return fmt.Errorf("xgoruntime: create launcher work directory: %w", err)
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
		return fmt.Errorf("xgoruntime: nil payload builder")
	}
	payloadFile, err := os.OpenFile(payloadPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("xgoruntime: create generated payload: %w", err)
	}
	payloadDigest, manifestDigest, buildErr := buildPayload(workDir, payloadFile)
	if buildErr == nil {
		buildErr = payloadFile.Sync()
	}
	if closeErr := payloadFile.Close(); buildErr == nil {
		buildErr = closeErr
	}
	if buildErr != nil {
		return fmt.Errorf("xgoruntime: write generated payload: %w", buildErr)
	}
	if err := os.WriteFile(mainPath, renderGeneratedLauncher(payloadDigest, manifestDigest), 0o600); err != nil {
		return fmt.Errorf("xgoruntime: write generated launcher: %w", err)
	}

	args := append([]string{"build"}, cfg.GraphFlags...)
	args = append(args, normalizedGoBuildFlags(cfg.BuildFlags)...)
	args = append(args, "-buildmode=exe", "-o", cfg.Output, mainPath)
	command := exec.CommandContext(ctx, cfg.GoCommand, args...)
	command.Dir = cfg.GraphWorkDir
	command.Env = hostGoEnv(cfg, streams.Env)
	command.Stdin = streams.Stdin
	command.Stdout = streams.Stdout
	command.Stderr = streams.Stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("xgoruntime: build generated launcher: %w", err)
	}
	if err := validateHostExecutable(cfg.Output); err != nil {
		return err
	}
	if err := verifyBuiltProviderOrigin(ctx, cfg.Output, cfg.ProviderOrigin, cfg, streams.Env); err != nil {
		return fmt.Errorf("xgoruntime: verify generated launcher provenance: %w", err)
	}
	if runtime.GOOS == "darwin" {
		if err := signDarwinLauncher(ctx, cfg.Output, streams); err != nil {
			return err
		}
	}
	return validateHostExecutable(cfg.Output)
}

func verifyBuiltProviderOrigin(ctx context.Context, name string, want ModuleOrigin, cfg Config, baseEnv []string) error {
	info, err := buildinfo.ReadFile(name)
	if err != nil {
		return err
	}
	replacementPath, err := effectiveLocalReplacementPath(ctx, cfg, baseEnv, want)
	if err != nil {
		return err
	}
	return verifyBuildInfoOrigin(info, want, replacementPath)
}

func effectiveLocalReplacementPath(ctx context.Context, cfg Config, baseEnv []string, want ModuleOrigin) (string, error) {
	if want.Replace == nil || want.Replace.Version != "" {
		return "", nil
	}
	if err := cfg.validateGraphInputs(); err != nil {
		return "", err
	}
	args := append([]string{"list"}, cfg.GraphFlags...)
	args = append(args, "-m", "-json", want.Selected.Path)
	command := exec.CommandContext(ctx, cfg.GoCommand, args...)
	command.Dir = cfg.GraphWorkDir
	command.Env = hostGoEnv(cfg, baseEnv)
	output, err := command.Output()
	if err != nil {
		return "", fmt.Errorf("xgoruntime: resolve effective module for build provenance: %w", err)
	}
	var listed listedModule
	if err := json.Unmarshal(output, &listed); err != nil {
		return "", fmt.Errorf("xgoruntime: decode effective module for build provenance: %w", err)
	}
	got, err := normalizeListedOrigin(&listed)
	if err != nil {
		return "", fmt.Errorf("xgoruntime: invalid effective module for build provenance: %w", err)
	}
	if !sameModuleOrigin(got, want) {
		return "", fmt.Errorf("xgoruntime: effective module does not match resolved provider provenance")
	}
	if listed.Replace == nil || listed.Replace.Version != "" || listed.Replace.Path == "" {
		return "", fmt.Errorf("xgoruntime: effective module is missing its local replacement path")
	}
	return listed.Replace.Path, nil
}

func verifyBuildInfoOrigin(info *runtimedebug.BuildInfo, want ModuleOrigin, effectiveReplacementPath string) error {
	if info == nil {
		return fmt.Errorf("missing Go build info")
	}
	var got *runtimedebug.Module
	if info.Main.Path == want.Selected.Path {
		got = &info.Main
	} else {
		for _, dependency := range info.Deps {
			if dependency != nil && dependency.Path == want.Selected.Path {
				got = dependency
				break
			}
		}
	}
	if got == nil {
		return fmt.Errorf("built artifact does not contain module %q", want.Selected.Path)
	}
	if want.Main {
		if got.Version != "" && got.Version != "(devel)" {
			return fmt.Errorf("main module build version is %q", got.Version)
		}
	} else if got.Version != want.Selected.Version {
		return fmt.Errorf("selected module version is %q, want %q", got.Version, want.Selected.Version)
	}
	if want.Replace == nil {
		if got.Replace != nil {
			return fmt.Errorf("built module has unexpected replacement %s@%s", got.Replace.Path, got.Replace.Version)
		}
		return nil
	}
	if got.Replace == nil {
		return fmt.Errorf("built module is missing its resolved replacement")
	}
	if want.Replace.Version != "" {
		if got.Replace.Path != want.Replace.Path || got.Replace.Version != want.Replace.Version {
			return fmt.Errorf("built module replacement is %s@%s, want %s@%s", got.Replace.Path, got.Replace.Version, want.Replace.Path, want.Replace.Version)
		}
		return nil
	}
	if !isLocalReplacementVersion(got.Replace.Version) {
		return fmt.Errorf("built local replacement unexpectedly has version %q", got.Replace.Version)
	}
	if filepath.IsAbs(got.Replace.Path) {
		if !sameExistingPath(got.Replace.Path, want.Replace.Dir) {
			return fmt.Errorf("built local replacement %q does not match %q", got.Replace.Path, want.Replace.Dir)
		}
		return nil
	}
	if got.Replace.Path != effectiveReplacementPath {
		return fmt.Errorf("built relative local replacement is %q, want effective graph path %q", got.Replace.Path, effectiveReplacementPath)
	}
	return nil
}

func isLocalReplacementVersion(version string) bool {
	return version == "" || version == "(devel)"
}

func sameExistingPath(first, second string) bool {
	firstInfo, firstErr := os.Stat(first)
	secondInfo, secondErr := os.Stat(second)
	return firstErr == nil && secondErr == nil && os.SameFile(firstInfo, secondInfo)
}

func normalizedGoBuildFlags(flags []string) []string {
	result := make([]string, 0, len(flags))
	for _, flag := range flags {
		name, value, _ := strings.Cut(strings.TrimPrefix(flag, "-"), "=")
		switch name {
		case "v", "x", "work", "trimpath":
			if value == "true" {
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
		return fmt.Errorf("xgoruntime: inspect launcher output %q: %w", name, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() == 0 {
		return fmt.Errorf("xgoruntime: launcher output %q is not a non-empty regular non-symlink file", name)
	}
	file, err := os.Open(name)
	if err != nil {
		return fmt.Errorf("xgoruntime: open launcher output: %w", err)
	}
	defer file.Close()
	var magic [4]byte
	if _, err := file.Read(magic[:]); err != nil {
		return fmt.Errorf("xgoruntime: read launcher output header: %w", err)
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
		return fmt.Errorf("xgoruntime: launcher output %q is not a host %s executable", name, runtime.GOOS)
	}
	return nil
}

func signDarwinLauncher(ctx context.Context, output string, streams IO) error {
	sign := exec.CommandContext(ctx, "/usr/bin/codesign", "--force", "--sign", "-", output)
	sign.Stdout, sign.Stderr = streams.Stdout, streams.Stderr
	if err := sign.Run(); err != nil {
		return fmt.Errorf("xgoruntime: ad-hoc sign launcher: %w", err)
	}
	verify := exec.CommandContext(ctx, "/usr/bin/codesign", "--verify", "--strict", output)
	verify.Stdout, verify.Stderr = streams.Stdout, streams.Stderr
	if err := verify.Run(); err != nil {
		return fmt.Errorf("xgoruntime: verify launcher signature: %w", err)
	}
	return nil
}
