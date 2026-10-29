package main

import (
	"fmt"
	"io"
	"os"

	"github.com/goplus/spx/v4/internal/release"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) != 2 {
		fmt.Fprintln(stderr, "usage: runtime-digest <pack-source|build-recipe> <revision>")
		return 2
	}

	var (
		digest string
		err    error
	)
	switch args[0] {
	case "pack-source":
		digest, err = release.RuntimePackSourceSHA256(".", args[1])
	case "build-recipe":
		digest, err = release.RuntimeBuildRecipeSHA256(".", args[1])
	default:
		fmt.Fprintf(stderr, "unknown runtime digest kind %q; want pack-source or build-recipe\n", args[0])
		return 2
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintln(stdout, digest)
	return 0
}
