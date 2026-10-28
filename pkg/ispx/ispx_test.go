/*
 * Copyright (c) 2021 The XGo Authors (xgo.dev). All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package ispx

import (
	"errors"
	"go/types"
	"io"
	"os"
	"runtime"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/goplus/ixgo"
	"github.com/goplus/ixgo/xgobuild"
	spxfs "github.com/goplus/spx/v4/fs"
	"github.com/goplus/spx/v4/fs/asset"
)

type interpreterImportFailure struct {
	types.Importer
	err error
}

func TestInterpreterInit(t *testing.T) {
	for _, name := range []string{"default", "custom", "missing lookup"} {
		t.Run(name, func(t *testing.T) {
			isolateInterpreter(t)
			custom := name == "custom"
			var ctx *ixgo.Context
			var options []InitOption
			if name != "default" {
				ctx = ixgo.NewContext(ixgo.SupportMultipleInterp | xgobuild.StaticLoad)
				ctx.Lookup = nil
			}
			if custom {
				ctx.Lookup = func(root, path string) (string, bool) { return "custom-lookup", false }
				options = append(options, WithSSAOptimization())
			}
			if err := Init(ctx, options...); err != nil {
				t.Fatal(err)
			}
			if ixgoCtx == nil || ixgoCtx.Lookup == nil || optimizeSSA != custom {
				t.Fatalf("initialization state: context=%v, optimize=%v", ixgoCtx, optimizeSSA)
			}
			if custom {
				if ixgoCtx != ctx {
					t.Fatal("Init replaced the supplied context")
				}
				if dir, ok := ctx.Lookup("root", "pkg"); dir != "custom-lookup" || ok {
					t.Fatalf("custom lookup = %q, %v", dir, ok)
				}
			} else if name == "missing lookup" && runtime.GOOS != "js" {
				if dir, ok := ixgoCtx.Lookup("root", "missing"); dir != "" || ok {
					t.Fatalf("default lookup = %q, %v", dir, ok)
				}
			}
			if ixgoCtx.SourcePackage("github.com/goplus/spx/v4@patch") == nil {
				t.Fatal("Init did not register the generic widget patch")
			}
			requireInterpreterPanic(t, "ispx: already initialized", func() { _ = Init(nil) })
		})
	}
}

func TestConfigureFilesystemRoots(t *testing.T) {
	isolateInterpreter(t)
	for _, configure := range []struct {
		name string
		call func(string, string) error
	}{
		{"strict", ConfigureFilesystemRoots},
		{"legacy", ConfigureLegacyFilesystemRoots},
	} {
		t.Run(configure.name, func(t *testing.T) {
			if err := configure.call("", ""); err == nil || !strings.Contains(err.Error(), "project root is empty") {
				t.Fatalf("empty roots = %v, want engine validation error", err)
			}
			ixgoCtx = &ixgo.Context{}
			defer func() { ixgoCtx = nil }()
			if err := configure.call("/unused/project", "/unused/assets"); err == nil || !strings.Contains(err.Error(), "before Init") {
				t.Fatalf("initialized roots = %v, want lifecycle rejection", err)
			}
		})
	}
}

func TestInterpreterLifecycleGuards(t *testing.T) {
	isolateInterpreter(t)
	requireInterpreterPanic(t, "ispx: not initialized", func() { _ = Build(nil) })
	requireInterpreterPanic(t, "ispx: not built", func() { _, _ = Run() })
	if err := Shutdown(); err != nil {
		t.Fatalf("idle Shutdown() = %v", err)
	}
}

func TestInterpreterBuildMissingFunctionBody(t *testing.T) {
	isolateInterpreter(t)
	if err := Init(nil); err != nil {
		t.Fatal(err)
	}
	err := Build(map[string][]byte{"main.xgo": []byte("func missing()\nfunc main() { missing() }\n")})
	if err == nil || !strings.Contains(err.Error(), "failed to create interp") || !strings.Contains(err.Error(), "missing function body") {
		t.Fatalf("Build() = %v, want missing function body error", err)
	}
	if ixgoInterp != nil {
		t.Fatal("failed Build published an interpreter")
	}
}

func TestInterpreterBuildLoadError(t *testing.T) {
	isolateInterpreter(t)
	if err := Init(nil); err != nil {
		t.Fatal(err)
	}
	failure := errors.New("interpreter import unavailable")
	ixgoCtx.Importer = interpreterImportFailure{ixgoCtx.Importer, failure}
	err := Build(map[string][]byte{"main.xgo": []byte("import \"fmt\"\nfunc main() { fmt.Println(42) }\n")})
	if err == nil || !strings.Contains(err.Error(), "failed to load XGo source") || !strings.Contains(err.Error(), failure.Error()) {
		t.Fatalf("Build() = %v, want interpreter load error", err)
	}
	if ixgoInterp != nil {
		t.Fatal("failed LoadFile published an interpreter")
	}
}

func TestInterpreterBuildRunAndRebuild(t *testing.T) {
	for _, optimized := range []bool{false, true} {
		name := "default"
		if optimized {
			name = "optimized"
		}
		t.Run(name, func(t *testing.T) {
			isolateInterpreter(t)
			var options []InitOption
			if optimized {
				options = append(options, WithSSAOptimization())
			}
			if err := Init(nil, options...); err != nil {
				t.Fatal(err)
			}
			files := fstest.MapFS{
				"main.xgo":           {Data: []byte("var result int\nfunc main() { result = 6 * 7 }\nfunc Result() int { return result }\n")},
				"assets/message.txt": {Data: []byte("first")},
			}
			assertResource := func(want string) {
				t.Helper()
				dir, err := spxfs.Open("assets")
				if err != nil {
					t.Fatal(err)
				}
				defer dir.Close()
				file, err := dir.Open("message.txt")
				if err != nil {
					t.Fatal(err)
				}
				defer file.Close()
				data, err := io.ReadAll(file)
				if err != nil || string(data) != want {
					t.Fatalf("resource = %q, %v, want %q", data, err, want)
				}
			}
			if err := BuildFS(files); err != nil {
				t.Fatal(err)
			}
			assertResource("first")
			// Resource reads are lazy: BuildFS borrows the supplied filesystem.
			files["assets/message.txt"].Data = []byte("updated")
			assertResource("updated")
			if code, err := Run(); code != 0 || err != nil {
				t.Fatalf("Run() = %d, %v", code, err)
			}
			if result, err := ixgoInterp.RunFunc("Result"); result != 42 || err != nil || runDone != nil {
				t.Fatalf("completed run: result=%v, error=%v, done=%v", result, err, runDone)
			}
			if err := Build(map[string][]byte{"main.xgo": []byte("this is not valid code")}); err == nil || !strings.Contains(err.Error(), "failed to build XGo source") {
				t.Fatalf("invalid rebuild = %v", err)
			}
			if ixgoInterp != nil {
				t.Fatal("failed rebuild retained the released interpreter")
			}
			assertResource("updated")
			if err := Build(map[string][]byte{
				"main.xgo":           []byte("var result int\nfunc main() { result = 7 }\nfunc Result() int { return result }\n"),
				"assets/message.txt": []byte("second"),
			}); err != nil {
				t.Fatal(err)
			}
			assertResource("second")
			if code, err := Run(); code != 0 || err != nil {
				t.Fatalf("rebuilt Run() = %d, %v", code, err)
			}
			if result, err := ixgoInterp.RunFunc("Result"); result != 7 || err != nil {
				t.Fatalf("rebuilt result = %v, %v", result, err)
			}
		})
	}
}

func TestReadmeTutorialExamplesMatchSource(t *testing.T) {
	readme, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Fatal(err)
	}
	// Ignore presentation indentation, but preserve the source text on each line.
	normalize := func(source string) string {
		lines := strings.Split(strings.TrimSpace(source), "\n")
		for i := range lines {
			lines[i] = strings.TrimSpace(lines[i])
		}
		return strings.Join(lines, "\n")
	}
	examples := make(map[string]bool)
	for _, block := range strings.Split(string(readme), "```coffee\n")[1:] {
		code, _, ok := strings.Cut(block, "\n```")
		if !ok {
			t.Fatal("unclosed README code block")
		}
		examples[normalize(code)] = true
	}
	// These examples are presented as complete source files in the README.
	for _, path := range []string{
		"02-Dragon/Dragon.spx",
		"03-Clone/main.spx",
		"03-Clone/Arrow.spx",
		"04-Bullet/MyAircraft.spx",
		"04-Bullet/Bullet.spx",
	} {
		t.Run(path, func(t *testing.T) {
			source, err := os.ReadFile("../../tutorial/" + path)
			if err != nil {
				t.Fatal(err)
			}
			if !examples[normalize(string(source))] {
				t.Errorf("README must include the complete current source of tutorial/%s", path)
			}
		})
	}
}

func TestBuildAutoClosureConditions(t *testing.T) {
	ctx := ixgo.NewContext(xgobuild.StaticLoad)
	for _, pkg := range defaultPackagesToImport {
		if _, err := ctx.Loader.Import(pkg); err != nil {
			t.Fatalf("import %q: %v", pkg, err)
		}
	}

	source, err := xgobuild.BuildFSDir(ctx, newXGoParserFS(fstest.MapFS{
		"main.spx": {Data: []byte(`n := 0
repeatUntil n > 0, => {
	n++
}
waitUntil n > 1
`)},
		"SpEvent.spx": {Data: []byte(`var score int
onCond score >= 3, => {
	score++
}
`)},
	}), ".")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(source), "spx.RepeatUntil(func() bool") {
		t.Fatalf("repeatUntil condition was not compiled as a closure:\n%s", source)
	}
	if !strings.Contains(string(source), "spx.WaitUntil(func() bool") {
		t.Fatalf("waitUntil condition was not compiled as a closure:\n%s", source)
	}
	if !strings.Contains(string(source), ".OnCond(func() bool") {
		t.Fatalf("onCond condition was not compiled as a closure:\n%s", source)
	}
}

func (i interpreterImportFailure) Import(path string) (*types.Package, error) {
	if path == "fmt" {
		return nil, i.err
	}
	return i.Importer.Import(path)
}

// Lifecycle tests own the package singleton and release every interpreter they
// build. None may run in parallel with another interpreter lifecycle test.
func isolateInterpreter(t *testing.T) {
	t.Helper()
	mu.Lock()
	previousCtx, previousInterp, previousDone, previousOptimize := ixgoCtx, ixgoInterp, runDone, optimizeSSA
	ixgoCtx, ixgoInterp, runDone, optimizeSSA = nil, nil, nil, false
	mu.Unlock()
	t.Cleanup(func() {
		mu.Lock()
		defer mu.Unlock()
		if runDone != nil {
			t.Error("test left an interpreter running")
		}
		if ixgoInterp != nil {
			ixgoInterp.UnsafeRelease()
		}
		ixgoCtx, ixgoInterp, runDone, optimizeSSA = previousCtx, previousInterp, previousDone, previousOptimize
		spxfs.RegisterSchema("", asset.Open)
	})
}

func requireInterpreterPanic(t *testing.T, want string, fn func()) {
	t.Helper()
	defer func() {
		if got := recover(); got != want {
			t.Errorf("panic = %v, want %q", got, want)
		}
	}()
	fn()
}
