package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/goplus/spx/v2/internal/cmd/buildctl/shared"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		if !errors.Is(err, shared.ErrUsage) {
			fmt.Fprintln(os.Stderr, "Error:", err)
		}
		os.Exit(1)
	}
}
