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

// Package xgoruntime contains the private SPX implementation behind the
// provider-neutral runtimeprotocol boundary.
package xgoruntime

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/goplus/mod/runtimeprotocol"
	"github.com/goplus/mod/xgomod"
	"github.com/goplus/spx/v3/internal/projectassets"
	"github.com/goplus/spx/v3/internal/projectpolicy"
)

const ProtocolV1 = runtimeprotocol.PreambleV1

type Action = runtimeprotocol.Action

const (
	ActionRun   = runtimeprotocol.ActionRun
	ActionBuild = runtimeprotocol.ActionBuild
)

// ModuleRef and ModuleOrigin are the graph identities produced by XGo. Keep
// the provider boundary on the shared xgomod types so graph validation and
// source identity cannot drift between the dispatcher and provider.
type ModuleRef = xgomod.ModuleRef
type ModuleOrigin = xgomod.ResolvedModule

type ProjectSnapshot struct {
	Extension     string
	FullExtension string
	PackDirectory string
	PackIndexFile string
}

// Config is SPX's domain view of a shared runtimeprotocol.Request. Keeping the
// conversion here prevents transport parsing from acquiring SPX policy while
// the rest of the provider can use domain-oriented field names.
type Config struct {
	Action          Action
	ProjectDir      string
	ProjectFile     string
	ModuleRoot      string
	ProviderPackage string
	ProviderOrigin  ModuleOrigin
	Declaration     xgomod.FileIdentity
	Project         ProjectSnapshot
	GoCommand       string
	GraphWorkDir    string
	GoWork          string
	GraphFlags      []string
	BuildFlags      []string
	Output          string
	FinalOutput     string
	ApplicationArgs []string
}

// Parse delegates the complete argv contract to the provider-neutral codec,
// then verifies the live identities and SPX project assets it references.
func Parse(args []string) (Config, error) {
	request, err := runtimeprotocol.Parse(args)
	if err != nil {
		return Config{}, err
	}
	cfg := Config{
		Action:          request.Action,
		ProjectDir:      request.Project.Dir,
		ProjectFile:     request.Project.File,
		ModuleRoot:      request.Project.ModuleRoot,
		ProviderPackage: request.ProviderPackage,
		ProviderOrigin:  request.ProviderOrigin,
		Declaration:     request.Declaration,
		Project: ProjectSnapshot{
			Extension:     request.Project.Extension,
			FullExtension: request.Project.FullExtension,
		},
		GoCommand:       request.Graph.GoCommand,
		GraphWorkDir:    request.Graph.WorkDir,
		GoWork:          request.Graph.GoWork,
		GraphFlags:      append([]string(nil), request.Graph.Flags...),
		BuildFlags:      append([]string(nil), request.BuildFlags...),
		ApplicationArgs: append([]string(nil), request.ApplicationArgs...),
	}
	if request.Project.Pack != nil {
		cfg.Project.PackDirectory = request.Project.Pack.Directory
		cfg.Project.PackIndexFile = request.Project.Pack.IndexFile
	}
	if request.Output != nil {
		cfg.Output = request.Output.Staging
		cfg.FinalOutput = request.Output.Final
	}
	if err := cfg.validateLive(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// validateLive is the provider half of the identity contract. The shared
// codec intentionally performs no I/O; SPX pins canonical filesystem objects
// and resolves its project asset graph before executing untrusted argv.
func (p Config) validateLive() error {
	for _, item := range []struct {
		name      string
		value     string
		directory bool
	}{
		{name: "project-dir", value: p.ProjectDir, directory: true},
		{name: "project-file", value: p.ProjectFile},
		{name: "module-root", value: p.ModuleRoot, directory: true},
		{name: "declaration-file", value: p.Declaration.Path},
	} {
		if err := validateCanonicalExistingPath(item.name, item.value, item.directory); err != nil {
			return err
		}
	}
	if err := p.ProviderOrigin.Validate(); err != nil {
		return fmt.Errorf("runtime provider origin: %w", err)
	}
	effective := p.ProviderOrigin.Effective()
	if !pathWithin(effective.Dir, p.Declaration.Path) {
		return fmt.Errorf("runtime provider declaring gox.mod must be within the effective module dir")
	}
	if err := p.validateGraphInputs(); err != nil {
		return err
	}
	if p.Project.PackDirectory != "" {
		if err := projectpolicy.ValidatePortableConfig(p.ProjectDir); err != nil {
			return fmt.Errorf("runtime provider: %w", err)
		}
		packRoot := filepath.Join(p.ProjectDir, filepath.FromSlash(p.Project.PackDirectory))
		if err := validateCanonicalExistingPath("pack-dir", packRoot, true); err != nil {
			return err
		}
		if _, err := projectassets.Resolve(projectassets.Config{
			ProjectDir: p.ProjectDir,
			PackDir:    p.Project.PackDirectory,
			PackIndex:  p.Project.PackIndexFile,
		}); err != nil {
			return fmt.Errorf("runtime provider project assets: %w", err)
		}
	}
	return nil
}

func (p Config) validateGraphInputs() error {
	if err := validateCanonicalExistingPath("go-command", p.GoCommand, false); err != nil {
		return err
	}
	if err := validateCanonicalExistingPath("graph-work-dir", p.GraphWorkDir, true); err != nil {
		return err
	}
	if p.GoWork != "off" {
		if err := validateCanonicalExistingPath("go-work", p.GoWork, false); err != nil {
			return err
		}
	}
	for _, flag := range p.GraphFlags {
		name, value, ok := strings.Cut(strings.TrimPrefix(flag, "-"), "=")
		if !ok {
			return fmt.Errorf("runtime provider graph flag is not canonical: %q", flag)
		}
		switch name {
		case "overlay":
			return fmt.Errorf("runtime provider does not support -overlay because the project snapshot uses physical filesystem contents")
		case "modfile":
			if err := validateCanonicalExistingPath("graph-flag-"+name, value, false); err != nil {
				return err
			}
		}
	}
	return nil
}

// validateCanonicalExistingPath verifies XGo's promise that an
// identity-bearing path was canonicalized. It remains a fail-fast check; each
// later consumer must still use no-follow handles and revalidate identity at
// the point of use.
func validateCanonicalExistingPath(name, value string, directory bool) error {
	before, err := os.Lstat(value)
	if err != nil {
		return fmt.Errorf("runtime provider path --%s cannot be inspected: %w", name, err)
	}
	if before.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("runtime provider path --%s must not be a symlink: %q", name, value)
	}
	resolved, err := filepath.EvalSymlinks(value)
	if err != nil {
		return fmt.Errorf("runtime provider path --%s cannot be canonicalized: %w", name, err)
	}
	resolved = filepath.Clean(resolved)
	if resolved != value {
		return fmt.Errorf("runtime provider path --%s is not canonical: %q resolves to %q", name, value, resolved)
	}
	if directory {
		if !before.IsDir() {
			return fmt.Errorf("runtime provider path --%s is not a directory: %q", name, value)
		}
	} else if !before.Mode().IsRegular() {
		return fmt.Errorf("runtime provider path --%s is not a regular file: %q", name, value)
	}
	if name == "go-command" && runtime.GOOS != "windows" && before.Mode().Perm()&0o111 == 0 {
		return fmt.Errorf("runtime provider path --go-command is not executable: %q", value)
	}
	return nil
}

func pathWithin(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}
