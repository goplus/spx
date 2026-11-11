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
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// CommandConfig describes a prepared Engine child. Env is the complete base
// environment, rather than an overlay; root variables are replaced before the
// command is returned.
type CommandConfig struct {
	Roots      Roots
	Executable string
	Args       []string
	Env        []string
	Stdin      io.Reader
	Stdout     io.Writer
	Stderr     io.Writer
	PathPolicy PathPolicy
}

// PathPolicy controls how pre-existing Engine --path options are handled.
// RejectPath is the safe default. ReplacePath supports the legacy spx adapter.
type PathPolicy uint8

const (
	RejectPath PathPolicy = iota
	ReplacePath
)

// PrepareCommand returns an Engine command rooted in SessionDir without global
// side effects. Executable validation is fail-fast; os/exec reopens it in Start.
func PrepareCommand(ctx context.Context, cfg CommandConfig) (*exec.Cmd, error) {
	if ctx == nil {
		return nil, fmt.Errorf("interpsession: nil context")
	}
	if err := cfg.Roots.Validate(); err != nil {
		return nil, err
	}
	if err := validateRegularFile("Executable", cfg.Executable); err != nil {
		return nil, err
	}
	env, err := cfg.Roots.Environment(cfg.Env)
	if err != nil {
		return nil, err
	}

	args, err := engineArgs(cfg.Args, cfg.Roots.SessionDir, cfg.PathPolicy)
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, cfg.Executable, args...)
	cmd.Dir = cfg.Roots.SessionDir
	cmd.Env = env
	cmd.Stdin = cfg.Stdin
	cmd.Stdout = cfg.Stdout
	cmd.Stderr = cfg.Stderr
	return cmd, nil
}

// engineArgs makes SessionDir authoritative. RejectPath fails closed; the
// explicit ReplacePath compatibility policy removes both accepted spellings.
func engineArgs(input []string, sessionDir string, policy PathPolicy) ([]string, error) {
	if policy != RejectPath && policy != ReplacePath {
		return nil, fmt.Errorf("interpsession: unknown Engine path policy %d", policy)
	}
	args := make([]string, 0, len(input)+3)
	for i := 0; i < len(input); i++ {
		if input[i] == "--path" || strings.HasPrefix(input[i], "--path=") {
			if policy == RejectPath {
				return nil, fmt.Errorf("interpsession: Engine --path is reserved for SessionDir")
			}
			if input[i] == "--path" {
				if i+1 < len(input) {
					i++
				}
			}
			continue
		}
		args = append(args, input[i])
	}
	args = append(args, "--path", sessionDir, "--no-header")
	return args, nil
}
