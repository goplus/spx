package base

import (
	"flag"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// SkipSwitches skips all switches and returns non-switch arguments.
func SkipSwitches(args []string, f *flag.FlagSet) []string {
	out := make([]string, 0, len(args))
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			if f.Lookup(arg[1:]) == nil { // flag not found
				continue
			}
		}
		out = append(out, arg)
	}
	return out
}

// GetBuildDir Get build directory from arguments
func GetBuildDir(args []string) (dir string, recursive bool) {
	if len(args) == 0 {
		args = []string{"."}
	}
	dir = args[0]
	if strings.HasSuffix(dir, "/...") {
		dir = dir[:len(dir)-4]
		recursive = true
	}
	if fi, err := os.Stat(dir); err == nil {
		if fi.IsDir() {
			return
		}
		return filepath.Dir(dir), recursive
	}
	return
}

// RunGoCmd executes `go` command tools.
func RunGoCmd(dir string, op string, args ...string) {
	cmd := exec.Command("go", append([]string{op}, args...)...)
	cmd.Dir = dir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	err := cmd.Run()
	if err != nil {
		switch e := err.(type) {
		case *exec.ExitError:
			os.Exit(e.ExitCode())
		default:
			log.Fatalln("RunGoCmd failed:", err)
		}
	}
}

func RunBashCmd(dir string, args string) {
	cmd := exec.Command("bash", args)
	cmd.Dir = dir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	err := cmd.Run()
	if err != nil {
		switch e := err.(type) {
		case *exec.ExitError:
			os.Exit(e.ExitCode())
		default:
			log.Fatalln("RunGoCmd failed:", err)
		}
	}
}

func CpDir(dst, src string) error {
	walk := func(srcPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dst, srcPath[len(src):])
		if info.IsDir() {
			return os.MkdirAll(dstPath, 0755)
		}
		return Cp(dstPath, srcPath)
	}
	return filepath.Walk(src, walk)
}

func Cp(dst, src string) error {
	sf, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sf.Close()
	fi, err := sf.Stat()
	if err != nil {
		return err
	}
	tmpDst := dst + ".tmp"
	df, err := os.Create(tmpDst)
	if err != nil {
		return err
	}
	defer df.Close()

	if runtime.GOOS != "windows" {
		if err := df.Chmod(fi.Mode()); err != nil {
			return err
		}
	}
	_, err = io.Copy(df, sf)
	if err != nil {
		return err
	}
	if err := df.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpDst, dst); err != nil {
		return err
	}
	// Ensure the destination has the same mtime as the source.
	return os.Chtimes(dst, fi.ModTime(), fi.ModTime())
}

func WriteDataFiles(data map[string]string, base string) error {
	for name, body := range data {
		dst := filepath.Join(base, name)
		err := os.MkdirAll(filepath.Dir(dst), 0755)
		if err != nil {
			return err
		}
		b := []byte(body)

		// (We really mean 0755 on the next line; some of these files
		// are executable, and there's no harm in making them all so.)
		if err := ioutil.WriteFile(dst, b, 0755); err != nil {
			return err
		}
	}
	return nil
}

func Run(dir, name string, arg ...string) error {
	cmd := exec.Command(name, arg...)
	cmd.Dir = dir
	cmd.Env = os.Environ()
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}
