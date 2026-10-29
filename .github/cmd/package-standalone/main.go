package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/goplus/spx/v4/internal/base/fileutil"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	flags := flag.NewFlagSet("package-standalone", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	binary := flags.String("binary", "", "standalone spx binary")
	output := flags.String("output", "", "output ZIP archive")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || *binary == "" || *output == "" {
		return fmt.Errorf("usage: package-standalone --binary <spx|spx.exe> --output <archive.zip>")
	}
	return packageStandalone(*binary, *output)
}

func packageStandalone(binary, output string) error {
	binary, err := filepath.Abs(binary)
	if err != nil {
		return err
	}
	output, err = filepath.Abs(output)
	if err != nil {
		return err
	}
	name := filepath.Base(binary)
	if name != "spx" && name != "spx.exe" {
		return fmt.Errorf("standalone binary must be named spx or spx.exe: %s", binary)
	}
	info, err := os.Lstat(binary)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() == 0 {
		return fmt.Errorf("standalone binary must be a nonempty regular file: %s", binary)
	}
	if outputInfo, err := os.Lstat(output); err == nil {
		if !outputInfo.Mode().IsRegular() {
			return fmt.Errorf("archive destination must be a regular file: %s", output)
		}
		if os.SameFile(info, outputInfo) {
			return fmt.Errorf("archive would replace its input binary: %s", output)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	return fileutil.WriteNamedZip(output, map[string]string{name: binary})
}
