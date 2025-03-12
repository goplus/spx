package util

import (
	_ "embed"
	"log"
	"os"
	"os/exec"
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

func RunGoplus(envVars []string, args ...string) error {
	return RunCommandWithEnv(envVars, "gop", args...)
}

func RunGolang(envVars []string, args ...string) error {
	return RunCommandWithEnv(envVars, "go", args...)
}
