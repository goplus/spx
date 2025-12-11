//go:build !js

package launcher

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/goplus/spx/v2/cmd/igox/memfs"
	"github.com/goplus/spx/v2/cmd/igox/plugin"
	goxfs "github.com/goplus/spx/v2/fs"

	"github.com/goplus/ixgo"
	"github.com/goplus/ixgo/xgobuild"
)

func Run(plugins ...Plugin) {
	for _, info := range plugins {
		plugin.GetPluginManager().RegisterPlugin(info.Name, info.Plugin)
	}

	// register FFI for worker mode
	spxEngineRegisterFFI()
	projDir, err := filepath.Abs("..")
	if err != nil {
		logger.Error("failed to get absolute path", "error", err)
		return
	}
	if err := defaultRunner.build(projDir); err != nil {
		logger.Error("failed to build project", "error", err)
		return
	}
	if result := defaultRunner.run(); result != nil {
		if err, ok := result.(error); ok {
			logger.Error("failed to run project", "error", err)
			return
		}
	}
	// Unlike the web wasm mode, there is no need to block the main process here
}

// handleLookupError handles package lookup errors for PC platform.
func handleLookupError(err error) {
	fmt.Println("[ispxpc] Error:", err.Error())
}

// build builds SPX project from a directory path.
func (r *SpxRunner) build(projectPath string) error {
	if r.entry != nil && r.entry.interp != nil {
		r.Release()
	}

	// Read all files from directory into memory
	filesMap, err := readDirToMap(projectPath)
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
	}

	fs := memfs.NewMemFs(filesMap)
	goxfs.RegisterSchema("", func(path string) (goxfs.Dir, error) {
		return fs.Chroot(path)
	})

	ctx := r.ctx
	source, err := xgobuild.BuildFSDir(ctx, fs, "")
	if err != nil {
		return fmt.Errorf("failed to build XGo source: %w", err)
	}

	pkg, err := ctx.LoadFile("main.go", source)
	if err != nil {
		return fmt.Errorf("failed to load XGo source: %w", err)
	}

	interp, err := ctx.NewInterp(pkg)
	if err != nil {
		return fmt.Errorf("failed to create interp: %w", err)
	}

	if r.debug {
		capacity, allocate, available := ixgo.IcallStat()
		fmt.Printf("Icall Capacity: %d, Allocate: %d, Available: %d\n", capacity, allocate, available)
	}

	r.entry = &interpCacheEntry{
		interp: interp,
		closer: func() error { return fs.Close() },
	}
	return nil
}

// RunInterp executes the cached interpreter.
func (r *SpxRunner) run() any {
	return r.RunInterp(func(msg string) {
		fmt.Println("[ispxpc] Error:", msg)
	})
}

// readDirToMap reads all files from a directory into a map.
func readDirToMap(dirPath string) (map[string][]byte, error) {
	filesMap := make(map[string][]byte)

	err := readDirRecursive(dirPath, "", filesMap)
	if err != nil {
		return nil, err
	}

	return filesMap, nil
}

// readDirRecursive recursively reads files from a directory.
func readDirRecursive(basePath, relativePath string, filesMap map[string][]byte) error {
	currentPath := basePath
	if relativePath != "" {
		currentPath = filepath.Join(basePath, relativePath)
	}

	entries, err := os.ReadDir(currentPath)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		name := entry.Name()
		entryRelPath := name
		if relativePath != "" {
			entryRelPath = relativePath + "/" + name
		}

		if entry.IsDir() {
			if err := readDirRecursive(basePath, entryRelPath, filesMap); err != nil {
				return err
			}
		} else {
			fullPath := basePath + "/" + entryRelPath
			data, err := os.ReadFile(fullPath)
			if err != nil {
				return err
			}
			filesMap[entryRelPath] = data
		}
	}

	return nil
}
