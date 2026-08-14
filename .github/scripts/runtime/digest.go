package main

import (
	"fmt"
	"os"

	"github.com/goplus/spx/v3/internal/release"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: runtime_digest <pack-source|build-recipe> <revision>")
		os.Exit(2)
	}

	var (
		digest string
		err    error
	)
	switch os.Args[1] {
	case "pack-source":
		digest, err = release.RuntimePackSourceSHA256(".", os.Args[2])
	case "build-recipe":
		digest, err = release.RuntimeBuildRecipeSHA256(".", os.Args[2])
	default:
		fmt.Fprintf(os.Stderr, "unknown runtime digest kind %q; want pack-source or build-recipe\n", os.Args[1])
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(digest)
}
