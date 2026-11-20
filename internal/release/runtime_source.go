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
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

type gitTree struct {
	repoRoot string
	entries  []gitTreeEntry
}

// record preserves the exact ls-tree bytes included in published digests.
type gitTreeEntry struct {
	path   string
	record []byte
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
	tree, err := readGitTree(repoRoot, p.SPXCommit, "runtime pack source")
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
	tree, err := readGitTree(repoRoot, current.SPXCommit, "runtime source")
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

// Read by object ID so all blobs come from the tree already listed, even if
// the original revision names a branch that has since advanced.
func (tree gitTree) readBlob(path string) ([]byte, error) {
	for _, entry := range tree.entries {
		if entry.path != path {
			continue
		}
		header, _, _ := bytes.Cut(entry.record, []byte{'\t'})
		fields := bytes.Fields(header)
		if len(fields) != 3 || string(fields[1]) != "blob" {
			return nil, fmt.Errorf("release: invalid git blob entry %q", entry.record)
		}
		return gitOutput(tree.repoRoot, "cat-file", "blob", string(fields[2]))
	}
	return nil, fmt.Errorf("release: no Git tree entry for %q", path)
}

func (tree gitTree) digest(label string, include func(string) bool, project func(gitTreeEntry) ([]byte, error)) (string, error) {
	hash := sha256.New()
	count := 0
	for _, entry := range tree.entries {
		if !include(entry.path) {
			continue
		}
		record := entry.record
		if project != nil {
			var err error
			record, err = project(entry)
			if err != nil {
				return "", err
			}
		}
		hash.Write(record)
		hash.Write([]byte{0})
		count++
	}
	if count == 0 {
		return "", fmt.Errorf("release: %s tree contains no tracked inputs", label)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

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

func readGitTree(repoRoot, revision, label string) (gitTree, error) {
	if strings.TrimSpace(repoRoot) == "" {
		return gitTree{}, fmt.Errorf("release: %s repository must not be empty", label)
	}
	if strings.TrimSpace(revision) == "" {
		return gitTree{}, fmt.Errorf("release: %s revision must not be empty", label)
	}
	output, err := gitOutput(repoRoot, "ls-tree", "-r", "-z", "--full-tree", "--end-of-options", revision)
	if err != nil {
		return gitTree{}, fmt.Errorf("release: list %s tree at %q: %w", label, revision, err)
	}
	return parseGitTree(repoRoot, output)
}

func parseGitTree(repoRoot string, data []byte) (gitTree, error) {
	tree := gitTree{repoRoot: repoRoot}
	for len(data) > 0 {
		record, rest, ok := bytes.Cut(data, []byte{0})
		if !ok {
			return gitTree{}, errors.New("release: malformed NUL-delimited git tree output")
		}
		_, path, ok := bytes.Cut(record, []byte{'\t'})
		if !ok || len(path) == 0 {
			return gitTree{}, fmt.Errorf("release: malformed git tree entry %q", record)
		}
		tree.entries = append(tree.entries, gitTreeEntry{path: string(path), record: record})
		data = rest
	}
	return tree, nil
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
