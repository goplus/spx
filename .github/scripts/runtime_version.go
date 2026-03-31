package main

import (
	"fmt"

	"github.com/goplus/spx/v2/internal/releasemeta"
)

func main() {
	fmt.Println(releasemeta.DefaultReleaseMeta().Runtime.Version)
}
