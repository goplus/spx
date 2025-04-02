package engine

import (
	"fmt"
	"runtime/debug"
)

func PrintStack() {
	fmt.Printf("%s\n", debug.Stack())
}
