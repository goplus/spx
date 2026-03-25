package main

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type engineExecConfig struct {
	lockDir string
	workdir string
	script  string
	command []string
}

func runEngineExec(args []string) error {
	cfg, err := parseEngineExecArgs(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	repoRoot, err := findRepoRoot()
	if err != nil {
		return err
	}
	return execEngineCommand(cfg, repoRoot)
}

func parseEngineExecArgs(args []string) (engineExecConfig, error) {
	cfg := engineExecConfig{}

	fs := flag.NewFlagSet("engine exec", flag.ContinueOnError)
	fs.SetOutput(osStderr)
	fs.StringVar(&cfg.lockDir, "lock-dir", "", "directory used for engine build locking")
	fs.StringVar(&cfg.workdir, "workdir", "", "working directory for the executed command")
	fs.StringVar(&cfg.script, "script", "", "bash script body executed under the engine build lock")
	fs.Usage = func() {
		fmt.Fprintln(osStderr, "Usage: buildctl engine exec --lock-dir DIR [--workdir DIR] [--script '...'] [-- command args...]")
	}

	if err := fs.Parse(args); err != nil {
		return engineExecConfig{}, err
	}
	if cfg.lockDir == "" {
		fs.Usage()
		return engineExecConfig{}, errors.New("--lock-dir is required")
	}
	cfg.command = append([]string(nil), fs.Args()...)
	if cfg.script != "" && len(cfg.command) != 0 {
		fs.Usage()
		return engineExecConfig{}, errors.New("--script cannot be combined with command arguments")
	}
	if cfg.script == "" && len(cfg.command) == 0 {
		fs.Usage()
		return engineExecConfig{}, errors.New("missing command to execute")
	}
	return cfg, nil
}

func execEngineCommand(cfg engineExecConfig, repoRoot string) error {
	lockDir := cfg.lockDir
	if !filepath.IsAbs(lockDir) {
		lockDir = filepath.Join(repoRoot, lockDir)
	}
	workdir := cfg.workdir
	if workdir == "" {
		workdir = repoRoot
	} else if !filepath.IsAbs(workdir) {
		workdir = filepath.Join(repoRoot, workdir)
	}

	var cmdName string
	var cmdArgs []string
	if cfg.script != "" {
		cmdName = "bash"
		cmdArgs = []string{"-lc", cfg.script}
	} else {
		cmdName = cfg.command[0]
		cmdArgs = cfg.command[1:]
	}

	return withEngineBuildLock(lockDir, func() error {
		return runTrackedEngineCommand(workdir, cmdName, cmdArgs...)
	})
}

func withEngineBuildLock(lockDir string, fn func() error) error {
	if err := acquireEngineBuildLock(lockDir); err != nil {
		return err
	}
	defer releaseEngineBuildLock(lockDir)
	return fn()
}

func acquireEngineBuildLock(lockDir string) error {
	waitLogged := false
	for {
		if err := os.Mkdir(lockDir, 0o755); err == nil {
			pidPath := filepath.Join(lockDir, "pid")
			if err := os.WriteFile(pidPath, []byte(strconv.Itoa(os.Getpid())+"\n"), 0o644); err != nil {
				releaseEngineBuildLock(lockDir)
				return err
			}
			return nil
		} else if !errors.Is(err, fs.ErrExist) {
			return err
		}

		stale, message, err := detectStaleEngineBuildLock(lockDir)
		if err != nil {
			return err
		}
		if stale {
			if message != "" {
				fmt.Fprintln(os.Stdout, message)
			}
			releaseEngineBuildLock(lockDir)
			continue
		}
		if !waitLogged {
			fmt.Fprintf(os.Stdout, "Another build is using %s; waiting for build lock...\n", filepath.Dir(lockDir))
			waitLogged = true
		}
		time.Sleep(time.Second)
	}
}

func detectStaleEngineBuildLock(lockDir string) (bool, string, error) {
	info, err := os.Lstat(lockDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, "", nil
		}
		return false, "", err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return true, fmt.Sprintf("Removing invalid build lock symlink at %s...", lockDir), nil
	}

	pidPath := filepath.Join(lockDir, "pid")
	data, err := os.ReadFile(pidPath)
	if err == nil {
		pidText := strings.TrimSpace(string(data))
		pid, parseErr := strconv.Atoi(pidText)
		if parseErr != nil || pid <= 0 {
			return true, "Removing stale build lock (invalid pid metadata)...", nil
		}
		if !engineProcessExists(pid) {
			return true, fmt.Sprintf("Removing stale build lock (pid %d is dead)...", pid), nil
		}
		return false, "", nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return false, "", err
	}
	if time.Since(info.ModTime()) >= 5*time.Second {
		return true, "Removing stale build lock (missing pid metadata)...", nil
	}
	return false, "", nil
}

func releaseEngineBuildLock(lockDir string) {
	info, err := os.Lstat(lockDir)
	if err != nil {
		return
	}
	if info.Mode()&os.ModeSymlink != 0 {
		_ = os.Remove(lockDir)
		return
	}
	_ = os.RemoveAll(lockDir)
}

func engineProcessExists(pid int) bool {
	return trackedProcessExists(pid)
}

func runTrackedEngineCommand(workdir, name string, args ...string) error {
	return runTrackedEngineCommandWithEnv(workdir, nil, name, args...)
}

func runTrackedEngineCommandWithEnv(workdir string, env map[string]string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = workdir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if len(env) != 0 {
		cmd.Env = envMapToSlice(env)
	}
	configureTrackedCommand(cmd)

	if err := cmd.Start(); err != nil {
		return err
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, trackedSignals()...)
	defer signal.Stop(sigCh)

	select {
	case err := <-done:
		return err
	case <-sigCh:
		terminateTrackedProcess(cmd.Process)
		err := <-done
		if err != nil {
			return err
		}
		return errors.New("engine build interrupted")
	}
}

func terminateTrackedProcess(process *os.Process) {
	terminateTrackedProcessGroup(process)
}
