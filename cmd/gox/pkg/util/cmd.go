package util

import (
	_ "embed"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
)

// Helper function to run a command
func RunCommandInDir(dir string, name string, args ...string) error {
	return RunCommand(nil, dir, name, args...)
}
func RunCommandWithEnv(envVars []string, name string, args ...string) error {
	return RunCommand(envVars, "", name, args...)
}

func RunCommand(envVars []string, dir string, name string, args ...string) error {
	cmd := exec.Command(name, args...)

	// 打印命令行
	fmt.Printf("Running command: %s %s\n", name, strings.Join(args, " "))
	if dir != "" {
		fmt.Printf("In directory: %s\n", dir)
	}

	if envVars != nil {
		cmd.Env = append(os.Environ(), envVars...)
	}
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		log.Fatalf("Command %s failed: %v", name, err)
	}
	return err
}

func RunGoModTidy() error {
	return RunCommandWithEnv(nil, "go", "mod", "tidy")
}

func RunXGo(envVars []string, args ...string) error {
	return RunCommandWithEnv(envVars, "xgo", args...)
}

func RunGolang(envVars []string, args ...string) error {
	return RunCommandWithEnv(envVars, "go", args...)
}

// RunTinyGo runs tinygo command with given environment variables and arguments
func RunTinyGo(envVars []string, args ...string) error {
	return RunCommandWithEnv(envVars, "tinygo", args...)
}
