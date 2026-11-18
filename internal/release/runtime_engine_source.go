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

package release

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// RuntimeEngineSourceSHA256 identifies compiled engine inputs independently of
// runtime version and pack resources. Godot's pinned commit also pins its build
// platform SCons scripts. The reusable workflow implementation is pinned
// separately by the release workflow. The complete native module includes
// headers, generated bindings, SCsub and the SPX SCons profile, not just C++.
// Go/XGo build the command-line tools and game library, not these engine assets.
func RuntimeEngineSourceSHA256(repoRoot, revision string, lock RuntimeLock) (string, error) {
	if err := lock.Validate(); err != nil {
		return "", err
	}
	tree, err := trackedTree(repoRoot, revision, "runtime engine source")
	if err != nil {
		return "", err
	}
	module, err := selectedTreeSHA256(tree, "runtime engine module", func(name string) bool {
		return strings.HasPrefix(name, lock.Module.Path+"/")
	})
	if err != nil {
		return "", err
	}
	contract, err := selectedTreeSHA256(tree, "runtime engine contract", func(name string) bool {
		return name == ".github/scripts/runtime/build_contract.py"
	})
	if err != nil {
		return "", err
	}
	workflow, err := runtimeEngineWorkflow(repoRoot, revision)
	if err != nil {
		return "", err
	}
	// A named projection prevents publication metadata and Go tooling versions
	// from silently becoming engine inputs when the lock schema grows.
	identity := struct {
		Format                                     string
		ABI                                        int
		GodotRepository, GodotCommit, GodotVersion string
		Module, Contract, Workflow                 string
		SCons, EMSDK, AndroidNDK, JDK              string
	}{
		"spx-engine-v1", lock.RuntimeABI,
		lock.Godot.Repository, lock.Godot.Commit, lock.Godot.Version,
		module, contract, workflow,
		lock.Toolchain.SCons, lock.Toolchain.EMSDK, lock.Toolchain.AndroidNDK, lock.Toolchain.JDK,
	}
	data, err := json.Marshal(identity)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:]), nil
}

// The caller's jobs and transport may change without recompiling an engine,
// but its pinned reusable build implementation can change effective SCons args.
var runtimeEngineWorkflowPattern = regexp.MustCompile(`(?m)^\s+uses:\s+([^\s]+/\.github/workflows/release_builds\.yml@[0-9a-f]{40})\s*$`)

func runtimeEngineWorkflow(repoRoot, revision string) (string, error) {
	output, err := exec.Command("git", "-C", repoRoot, "cat-file", "blob", revision+":.github/workflows/build_release.yml").Output()
	if err != nil {
		return "", fmt.Errorf("release: read engine build workflow: %w", err)
	}
	matches := runtimeEngineWorkflowPattern.FindAllSubmatch(output, -1)
	if len(matches) != 1 {
		return "", fmt.Errorf("release: expected one commit-pinned engine build workflow, found %d", len(matches))
	}
	return string(matches[0][1]), nil
}
