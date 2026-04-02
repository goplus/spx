package util

import (
	"os"
	"os/exec"

	spxlog "github.com/goplus/spx/v2/internal/log"
)

type CommandOptions struct {
	Env []string
	Dir string
}

func RunCommandInDir(dir string, name string, args ...string) error {
	return RunCommand(nil, dir, name, args...)
}

func RunCommandWithEnv(envVars []string, name string, args ...string) error {
	return RunCommand(envVars, "", name, args...)
}

func RunCommand(envVars []string, dir string, name string, args ...string) error {
	err := ExecCommand(CommandOptions{Env: envVars, Dir: dir}, name, args...)
	if err != nil {
		spxlog.Fatalf("command %s failed: %v", name, err)
	}
	return err
}

func ExecCommand(options CommandOptions, name string, args ...string) error {
	execCmd := exec.Command(name, args...)
	applyCommandOptions(execCmd, options)
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr
	return execCmd.Run()
}

func OutputCommand(options CommandOptions, name string, args ...string) ([]byte, error) {
	execCmd := exec.Command(name, args...)
	applyCommandOptions(execCmd, options)
	return execCmd.Output()
}

func applyCommandOptions(execCmd *exec.Cmd, options CommandOptions) {
	if len(options.Env) > 0 {
		execCmd.Env = append(os.Environ(), options.Env...)
	}
	if options.Dir != "" {
		execCmd.Dir = options.Dir
	}
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
