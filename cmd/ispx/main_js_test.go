//go:build js && wasm

package main

import (
	"os"
	"strings"
	"syscall/js"
	"testing"

	"github.com/goplus/spx/v4/pkg/ispx"
)

func TestMain(m *testing.M) {
	// Init owns process-wide state, including when tests repeat with -count.
	go main()
	ready := make(chan struct{})
	callback := js.FuncOf(func(js.Value, []js.Value) any {
		close(ready)
		return nil
	})
	// The JS event loop resumes only when no Go goroutine is runnable, after
	// main has completed synchronous initialization and reached select {}.
	js.Global().Call("setTimeout", callback, 0)
	<-ready
	callback.Release()
	os.Exit(m.Run())
}

func TestBaseWebInterpreterBuildsAndRebuildsSPX(t *testing.T) {
	for _, source := range []string{
		"onStart => { println(\"hello\") }\n",
		"onStart => { println(\"rebuilt\") }\n",
	} {
		if err := ispx.Build(map[string][]byte{"main.spx": []byte(source)}); err != nil {
			t.Fatalf("build a game with the base interpreter: %v", err)
		}
	}
}

func TestBaseWebInterpreterHasNoAI(t *testing.T) {
	err := ispx.Build(map[string][]byte{"main.spx": []byte("var player ai.Player\n")})
	if err == nil || !strings.Contains(err.Error(), "undefined: ai") {
		t.Fatalf("build with optional AI = %v, want unresolved ai identifier", err)
	}
	// Recover from the rejected game and execute a script without starting Godot.
	if err := ispx.Build(map[string][]byte{"main.xgo": []byte("func main() {}\n")}); err != nil {
		t.Fatalf("rebuild after rejecting AI: %v", err)
	}
	if code, err := ispx.Run(); code != 0 || err != nil {
		t.Fatalf("run after rejecting AI = %d, %v", code, err)
	}
}
