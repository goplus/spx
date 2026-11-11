// Spine native smoke fixture uses a self-authored rectangular skeleton.
package main

import (
	"fmt"
	"os"
	"time"

	spx "github.com/goplus/spx/v4"
)

type Game struct {
	spx.Game
	Quad Quad
}
type Quad struct {
	spx.SpriteImpl
	*Game
}

func (q *Quad) Main() {
	q.OnStart(func() {
		fmt.Println("SPX_SPINE_NATIVE_STARTED")
		start := time.Now()
		q.AnimateAndWait("walk")
		if time.Since(start) < 200*time.Millisecond {
			panic("Spine animation returned before its duration")
		}
		fmt.Println("SPX_SPINE_NATIVE_OK")
		spx.Exit__1(0)
	})
}
func main() {
	// Godot consumes this option before initializing the extension. The SPX
	// script flag parser does not yet recognize it; keep that unrelated CLI
	// limitation separate from this PCK resource/animation smoke.
	args := os.Args[:1]
	for i := 1; i < len(os.Args); i++ {
		if os.Args[i] == "--main-pack" {
			i++
			continue
		}
		args = append(args, os.Args[i])
	}
	os.Args = args
	spx.XGot_Game_Main(&Game{}, &Quad{})
}
