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
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/goplus/spx/v4/internal/scaffold"
)

const (
	runtimeExtensionFile = "runtime.gdextension"
	projectExtensionFile = "gdspx.gdextension"
	extensionListFile    = "extension_list.cfg"
	bridgeDirectory      = "lib"
)

// SessionConfig describes the files needed to load the interpreter bridge in
// an Engine session. BridgePath is copied by base name into Roots.SessionDir.
type SessionConfig struct {
	Roots      Roots
	BridgePath string
}

// PrepareSession creates the session scaffold without changing global cwd or
// environment.
func PrepareSession(cfg SessionConfig) error {
	if err := validateAbsoluteCleanPath("ProjectDir", cfg.Roots.ProjectDir); err != nil {
		return err
	}
	if err := validateAbsoluteCleanPath("AssetDir", cfg.Roots.AssetDir); err != nil {
		return err
	}
	if err := validateAbsoluteCleanPath("SessionDir", cfg.Roots.SessionDir); err != nil {
		return err
	}
	for _, item := range []struct {
		name          string
		path          string
		rejectSymlink bool
	}{
		{name: "ProjectDir", path: cfg.Roots.ProjectDir},
		{name: "AssetDir", path: cfg.Roots.AssetDir, rejectSymlink: true},
	} {
		if err := validateDirectory(item.name, item.path, item.rejectSymlink); err != nil {
			return err
		}
	}
	bridge, bridgeInfo, err := openPinnedRegularFile("BridgePath", cfg.BridgePath)
	if err != nil {
		return err
	}
	defer bridge.Close()

	if err := ensureDirectoryNoSymlink("SessionDir", cfg.Roots.SessionDir, 0o700); err != nil {
		return err
	}
	if err := cfg.Roots.Validate(); err != nil {
		return err
	}

	session, err := openPinnedRoot("SessionDir", cfg.Roots.SessionDir)
	if err != nil {
		return err
	}
	defer session.Close()

	bridgeName := filepath.Base(cfg.BridgePath)
	if bridgeName == "." || bridgeName == string(filepath.Separator) || bridgeName == runtimeExtensionFile || bridgeName == ".godot" {
		return fmt.Errorf("interpsession: unsafe bridge base name %q", bridgeName)
	}
	if relative, relErr := filepath.Rel(cfg.Roots.SessionDir, cfg.BridgePath); relErr == nil &&
		(relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)))) {
		return fmt.Errorf("interpsession: BridgePath must be outside SessionDir")
	}
	bridgeRoot, err := ensurePinnedSubdirectory(session, bridgeDirectory, 0o700)
	if err != nil {
		return err
	}
	defer bridgeRoot.Close()
	if err := replaceRootFile(bridgeRoot, bridgeName, bridge, 0o700); err != nil {
		return fmt.Errorf("interpsession: copy bridge %q: %w", cfg.BridgePath, err)
	}
	if copied, err := bridge.Seek(0, io.SeekCurrent); err != nil {
		return fmt.Errorf("interpsession: inspect copied BridgePath %q: %w", cfg.BridgePath, err)
	} else if copied != bridgeInfo.Size() {
		return fmt.Errorf("interpsession: BridgePath %q changed size while it was copied", cfg.BridgePath)
	}
	if after, err := bridge.Stat(); err != nil {
		return fmt.Errorf("interpsession: re-stat BridgePath %q: %w", cfg.BridgePath, err)
	} else if !os.SameFile(bridgeInfo, after) || bridgeInfo.Mode() != after.Mode() || bridgeInfo.Size() != after.Size() || !bridgeInfo.ModTime().Equal(after.ModTime()) {
		return fmt.Errorf("interpsession: BridgePath %q changed while it was copied", cfg.BridgePath)
	}
	if err := replaceRootFile(session, runtimeExtensionFile, bytes.NewReader([]byte(scaffold.SessionRuntimeGDExtension())), 0o600); err != nil {
		return fmt.Errorf("interpsession: write %s: %w", runtimeExtensionFile, err)
	}
	if err := replaceRootFile(session, projectExtensionFile, bytes.NewReader([]byte(scaffold.ProjectGDExtension())), 0o600); err != nil {
		return fmt.Errorf("interpsession: write %s: %w", projectExtensionFile, err)
	}
	if err := verifyPinnedSubdirectory(session, bridgeDirectory, bridgeRoot); err != nil {
		return err
	}
	projectData, err := ensurePinnedSubdirectory(session, ".godot", 0o700)
	if err != nil {
		return err
	}
	defer projectData.Close()
	if err := replaceRootFile(projectData, extensionListFile, bytes.NewReader([]byte(scaffold.SessionExtensionList())), 0o600); err != nil {
		return fmt.Errorf("interpsession: write %s: %w", extensionListFile, err)
	}
	if err := verifyPinnedSubdirectory(session, ".godot", projectData); err != nil {
		return err
	}
	if err := verifyPinnedRootPath("SessionDir", cfg.Roots.SessionDir, session); err != nil {
		return err
	}
	return nil
}
