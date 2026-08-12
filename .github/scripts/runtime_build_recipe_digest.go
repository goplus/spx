package main

import (
	"fmt"
	"os"

	"github.com/goplus/spx/v3/internal/release"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: runtime_build_recipe_digest <revision>")
		os.Exit(2)
	}
	digest, err := release.RuntimeBuildRecipeSHA256(".", os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(digest)
}
