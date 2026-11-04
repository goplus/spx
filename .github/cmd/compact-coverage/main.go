package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/tools/cover"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stderr))
}

func run(args []string, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: compact-coverage <profile>...")
		return 2
	}
	for _, name := range args {
		if err := compactProfile(name); err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", name, err)
			return 1
		}
	}
	return 0
}

func compactProfile(name string) error {
	input, err := os.Open(name)
	if err != nil {
		return err
	}
	defer input.Close()
	return compactOpenedProfile(name, input)
}

func compactOpenedProfile(name string, input *os.File) error {
	info, err := input.Stat()
	if err != nil {
		return err
	}
	reader := bufio.NewReader(input)
	header, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return err
	}
	header = strings.TrimSuffix(strings.TrimSuffix(header, "\n"), "\r")
	switch header {
	case "mode: set", "mode: count", "mode: atomic":
	default:
		return fmt.Errorf("invalid coverage mode %q", header)
	}
	// Go's parser sorts and merges blocks, summing count/atomic counters and
	// ORing set counters. Retain the header separately for profiles with no blocks.
	profiles, err := cover.ParseProfilesFromReader(io.MultiReader(strings.NewReader(header+"\n"), reader))
	if err != nil {
		return err
	}
	if err := input.Close(); err != nil {
		return err
	}

	output, err := os.CreateTemp(filepath.Dir(name), ".coverage-*")
	if err != nil {
		return err
	}
	return replaceProfile(name, output, info.Mode().Perm(), header, profiles)
}

type profileOutput interface {
	io.WriteCloser
	Name() string
	Chmod(os.FileMode) error
}

func replaceProfile(name string, output profileOutput, mode os.FileMode, header string, profiles []*cover.Profile) error {
	defer os.Remove(output.Name())
	defer output.Close()
	if err := output.Chmod(mode); err != nil {
		return err
	}
	writer := bufio.NewWriter(output)
	// Buffered writes retain the first error; Flush checks it before replacement.
	fmt.Fprintln(writer, header)
	for _, profile := range profiles {
		for _, block := range profile.Blocks {
			fmt.Fprintf(writer, "%s:%d.%d,%d.%d %d %d\n", profile.FileName,
				block.StartLine, block.StartCol, block.EndLine, block.EndCol, block.NumStmt, block.Count)
		}
	}
	if err := writer.Flush(); err != nil {
		return err
	}
	if err := output.Close(); err != nil {
		return err
	}
	return os.Rename(output.Name(), name)
}
