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
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// Resolve the commit before reading any of its inputs so a moving branch can
// never mix two revisions in the same provenance record.
func runtimeRevision(repoRoot, revision string, lock RuntimeLock) (RuntimeProvenance, error) {
	var p RuntimeProvenance
	if strings.TrimSpace(revision) == "" {
		return p, errors.New("SPX revision must not be empty")
	}
	commit, err := gitObject(repoRoot, revision+"^{commit}")
	if err != nil {
		return p, fmt.Errorf("resolve current SPX commit: %w", err)
	}
	p.SPXCommit = commit
	p.ModuleTree, err = gitObject(repoRoot, commit+":"+lock.Module.Path)
	if err != nil {
		return p, fmt.Errorf("resolve current module tree: %w", err)
	}
	p.GodotCommit, p.Toolchain = lock.Godot.Commit, lock.Toolchain
	return p, nil
}

func gitObject(repoRoot, revision string) (string, error) {
	output, err := gitOutput(repoRoot, "rev-parse", "--verify", "--end-of-options", revision)
	if err != nil {
		return "", fmt.Errorf("resolve Git object %q: %w", revision, err)
	}
	return strings.TrimSpace(string(output)), nil
}

// RuntimeProvenanceAt derives all build identities from one immutable revision.
func RuntimeProvenanceAt(repoRoot, revision string, lock RuntimeLock) (RuntimeProvenance, error) {
	if err := lock.Validate(); err != nil {
		return RuntimeProvenance{}, err
	}
	p, err := runtimeRevision(repoRoot, revision, lock)
	if err != nil {
		return p, err
	}
	tree, err := trackedTree(repoRoot, p.SPXCommit, "runtime pack source")
	if err != nil {
		return p, err
	}
	p.RuntimePackSourceSHA256, err = tree.packSourceSHA256()
	if err != nil {
		return p, err
	}
	p.BuildRecipeSHA256, err = tree.buildRecipeSHA256()
	if err != nil {
		return p, err
	}
	p.EngineSourceSHA256, err = tree.engineSourceSHA256(lock)
	return p, err
}

// ReuseIncompatibility reports why a complete published runtime no longer
// matches this source. An error means its inputs could not be inspected.
func (m RuntimeManifest) ReuseIncompatibility(repoRoot, revision string, lock RuntimeLock) (string, error) {
	if err := m.ValidateForLock(lock); err != nil {
		return err.Error(), nil
	}
	repoRoot = filepath.Clean(repoRoot)
	current, err := runtimeRevision(repoRoot, strings.TrimSpace(revision), lock)
	if err != nil {
		return "", err
	}
	p := m.Provenance
	if p.ModuleTree != current.ModuleTree {
		return fmt.Sprintf("module tree = %s, current runtime input = %s", p.ModuleTree, current.ModuleTree), nil
	}
	tree, err := trackedTree(repoRoot, current.SPXCommit, "runtime source")
	if err != nil {
		return "", err
	}
	// Older complete releases predate the independent engine identity.
	if p.EngineSourceSHA256 != "" {
		digest, err := tree.engineSourceSHA256(lock)
		if err != nil {
			return "", err
		}
		if digest != p.EngineSourceSHA256 {
			return "engine source digest differs from current inputs", nil
		}
	}
	pack, err := tree.packSourceSHA256()
	if err != nil {
		return "", err
	}
	if pack != p.RuntimePackSourceSHA256 {
		return fmt.Sprintf("runtime pack source digest = %s, current runtime input = %s", p.RuntimePackSourceSHA256, pack), nil
	}
	recipe, err := tree.buildRecipeSHA256()
	if err != nil {
		return "", err
	}
	if recipe != p.BuildRecipeSHA256 {
		return fmt.Sprintf("runtime build recipe digest = %s, current runtime input = %s", p.BuildRecipeSHA256, recipe), nil
	}
	return "", nil
}

type runtimeSourceTree struct {
	root, revision string
	entries        []byte
}

func trackedTree(repoRoot, revision, label string) (runtimeSourceTree, error) {
	if strings.TrimSpace(repoRoot) == "" {
		return runtimeSourceTree{}, fmt.Errorf("release: %s repository must not be empty", label)
	}
	if strings.TrimSpace(revision) == "" {
		return runtimeSourceTree{}, fmt.Errorf("release: %s revision must not be empty", label)
	}

	output, err := gitOutput(repoRoot, "ls-tree", "-r", "-z", "--full-tree", revision)
	if err != nil {
		return runtimeSourceTree{}, fmt.Errorf("release: list %s tree at %q: %w", label, revision, err)
	}
	return runtimeSourceTree{repoRoot, revision, output}, nil
}

func gitOutput(root string, args ...string) ([]byte, error) {
	output, err := exec.Command("git", append([]string{"-C", filepath.Clean(root)}, args...)...).Output()
	if exit, ok := err.(*exec.ExitError); ok {
		if detail := strings.TrimSpace(string(exit.Stderr)); detail != "" {
			err = fmt.Errorf("%w: %s", err, detail)
		}
	}
	return output, err
}
