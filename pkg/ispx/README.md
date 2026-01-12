# ispx - Interpreted SPX Runtime

`ispx` is an independent Go module that provides runtime support for interpreting SPX (Scratch for Go+) programs using the ixgo interpreter.

## Module Structure

```
pkg/ispx/
├── go.mod              # Independent module with ixgo dependency
├── runtime/            # SPX runtime implementation
│   ├── runtime_common.go     # Common runtime functionality
│   ├── runtime_js.go         # WASM/JavaScript platform support
│   └── runtime_nojs.go       # Native platform support
├── plugin/             # Plugin system for extending functionality
│   └── plugin.go
└── memfs/              # In-memory file system implementation
    └── memfs.go
```

## Features

- **Independent Module**: Standalone module with clear dependency management
- **Multi-Platform Support**: Supports both native (desktop) and WASM (browser) platforms
- **Plugin System**: Extensible plugin architecture for custom functionality
- **In-Memory File System**: Efficient file system abstraction for SPX projects
- **ixgo Integration**: Uses the ixgo interpreter for running Go+ code

## Dependencies

- `github.com/goplus/ixgo` - Go interpreter runtime
- `github.com/goplus/mod` - Go+ module file support
- `github.com/goplus/reflectx` - Reflection utilities with icall support
- `github.com/goplus/spx/v2` - Main SPX framework (via replace directive)

## Usage

### Native Platform

```go
import "github.com/goplus/spx/v2/pkg/ispx/runtime"

func main() {
    runtime.Run() // Run SPX project from current directory
}
```

### WASM Platform

```go
import "github.com/goplus/spx/v2/pkg/ispx/runtime"

func main() {
    runtime.Run() // Registers WASM interfaces and blocks
}
```

### Custom Plugins

```go
import (
    "github.com/goplus/spx/v2/pkg/ispx/runtime"
    "github.com/goplus/spx/v2/pkg/ispx/plugin"
)

type MyPlugin struct{}

func (p *MyPlugin) RegisterJS() {}
func (p *MyPlugin) RegisterPatch(ctx *ixgo.Context) error { return nil }
func (p *MyPlugin) Init() {}

func main() {
    runtime.Run(runtime.Plugin{
        Name:   "myplugin",
        Plugin: &MyPlugin{},
    })
}
```

## Architecture

### Runtime

The runtime is responsible for:
- Initializing the ixgo context
- Registering SPX project configuration
- Building SPX source files into executable code
- Running the interpreter
- Managing platform-specific behavior (native vs WASM)

### Plugin System

Plugins can extend the runtime with:
- JavaScript bindings (for WASM platform)
- Code patches for runtime behavior modification
- Initialization hooks

### MemFS

The in-memory file system provides:
- Fast file access without disk I/O
- Cross-platform compatibility
- Support for chroot operations
- Standard library fs.FS interface compatibility

## License

This module follows the same license as the parent SPX project.
