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

package command

import (
	"fmt"
	"os"
	"path/filepath"
)

// Init creates a project in the target path.
func (cmd *CmdTool) Init() error {
	targetPath := *cmd.Args.Path
	if targetPath == "." {
		var err error
		targetPath, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}
	} else {
		var err error
		targetPath, err = filepath.Abs(targetPath)
		if err != nil {
			return fmt.Errorf("failed to resolve target path: %w", err)
		}

		if err := os.MkdirAll(targetPath, 0755); err != nil {
			return fmt.Errorf("failed to create target directory: %w", err)
		}
	}

	logInfof("Initializing SPX project in: %s", targetPath)

	assetsDir := filepath.Join(targetPath, "assets")
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		return fmt.Errorf("failed to create assets directory: %w", err)
	}

	indexJsonPath := filepath.Join(assetsDir, "index.json")
	indexJsonContent := `
{
	"map":	{
		"width":480,
		"height":360
	}
}`
	if err := os.WriteFile(indexJsonPath, []byte(indexJsonContent), 0644); err != nil {
		return fmt.Errorf("failed to create assets/index.json: %w", err)
	}

	mainSpxPath := filepath.Join(targetPath, "main.spx")
	mainSpxContent := `// SPX Project Main File
// This is the entry point for your SPX project

onStart => {
	println("Hello, SPX!")
	println("Project started successfully!")
}
`
	if err := os.WriteFile(mainSpxPath, []byte(mainSpxContent), 0644); err != nil {
		return fmt.Errorf("failed to create main.spx: %w", err)
	}

	if err := cmd.createDefaultGoMod(targetPath, true); err != nil {
		return fmt.Errorf("failed to create go.mod: %w", err)
	}

	logInfof("Initialized SPX project successfully")
	logInfof("Run 'spx run' to start your project")

	return nil
}
