package main

import (
	"errors"
	"fmt"
)

var errUsage = errors.New("usage")

func run(args []string) error {
	if len(args) == 0 {
		printRootUsage()
		return errUsage
	}

	switch args[0] {
	case "env":
		return runEnv(args[1:])
	case "prepare":
		return runPrepare(args[1:])
	case "tool":
		return runTool(args[1:])
	case "engine":
		return runEngine(args[1:])
	case "runtime":
		return runRuntime(args[1:])
	case "docker":
		return runDocker(args[1:])
	case "workflow":
		return runWorkflow(args[1:])
	case "help", "-h", "--help":
		printRootUsage()
		return nil
	default:
		printRootUsage()
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func printRootUsage() {
	fmt.Fprintln(osStderr, "Usage: buildctl <command> [options]")
	fmt.Fprintln(osStderr)
	fmt.Fprintln(osStderr, "Commands:")
	fmt.Fprintln(osStderr, "  env        Print shared build environment values")
	fmt.Fprintln(osStderr, "  prepare    Install tools and prepare runtime/web assets")
	fmt.Fprintln(osStderr, "  tool       Install build tooling")
	fmt.Fprintln(osStderr, "  engine     Download and build engine assets")
	fmt.Fprintln(osStderr, "  runtime    Export runtime artifacts")
	fmt.Fprintln(osStderr, "  docker     Run legacy container-based build workflows")
	fmt.Fprintln(osStderr, "  workflow   Run higher-level local build workflows")
}
