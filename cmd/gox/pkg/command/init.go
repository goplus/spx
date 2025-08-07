package command

import (
	"fmt"
	"os"
	"path/filepath"
)

// Init initializes a new SPX project in the specified directory
func (cmd *CmdTool) Init() error {
	// Use the path from command line arguments, default to current directory
	targetPath := *cmd.Args.Path
	if targetPath == "." {
		var err error
		targetPath, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}
	} else {
		// Convert to absolute path
		var err error
		targetPath, err = filepath.Abs(targetPath)
		if err != nil {
			return fmt.Errorf("failed to resolve target path: %w", err)
		}

		// Create the target directory if it doesn't exist
		if err := os.MkdirAll(targetPath, 0755); err != nil {
			return fmt.Errorf("failed to create target directory: %w", err)
		}
	}

	fmt.Printf("Initializing SPX project in: %s\n", targetPath)

	// Create assets directory
	assetsDir := filepath.Join(targetPath, "assets")
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		return fmt.Errorf("failed to create assets directory: %w", err)
	}

	// Create assets/index.json file
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

	// Create main.spx file
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

	cmd.createDefaultGoMod(targetPath, true)

	fmt.Println("")
	fmt.Println("SPX project initialized successfully!")
	fmt.Println("You can now run 'spx run' to start your project.")

	return nil
}
