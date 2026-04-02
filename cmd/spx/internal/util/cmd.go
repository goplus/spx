package util

import (
	"os"
	"os/exec"

	spxlog "github.com/goplus/spx/v2/internal/log"
)

// RunCommandInDir runs a command in dir.
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
		spxlog.Fatalf("command %s failed: %v", name, err)
	}
	return err
}

func RunXGo(envVars []string, args ...string) error {
	return RunCommandWithEnv(envVars, "xgo", args...)
}

func RunGolang(envVars []string, args ...string) error {
	return RunCommandWithEnv(envVars, "go", args...)
}

// RunTinyGo runs tinygo.
func RunTinyGo(envVars []string, args ...string) error {
	return RunCommandWithEnv(envVars, "tinygo", args...)
}
