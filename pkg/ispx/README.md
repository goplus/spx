# ispx - Interpreted SPX Runtime

`ispx` is an independent Go module that provides runtime support for interpreting SPX programs using the ixgo interpreter.

## Module Structure

```
pkg/ispx/
├── go.mod              # Independent module with ixgo dependency
├── api.go              # Public API for launching SPX runtime
├── runtime/            # SPX runtime implementation
│   ├── config.go             # Runtime configuration
│   ├── logger.go             # Logging interface
│   ├── platform.go           # Platform abstraction
│   ├── platform_native.go    # Native platform support
│   ├── platform_wasm.go      # WASM/JavaScript platform support
│   └── runner.go             # SPX runner implementation
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
- **Clean API**: Simple, user-friendly API for launching SPX runtime

## Dependencies

- `github.com/goplus/ixgo` - Go interpreter runtime
- `github.com/goplus/mod` - Go+ module file support
- `github.com/goplus/reflectx` - Reflection utilities with icall support
- `github.com/goplus/spx/v2` - Main SPX framework (via replace directive)

## Usage

### Basic Usage

```go
import "github.com/goplus/spx/v2/pkg/ispx"

func main() {
    // Launch with default configuration
    ispx.Launch(nil)
}
```

### Custom Configuration

```go
import "github.com/goplus/spx/v2/pkg/ispx"

func main() {
    cfg := &ispx.Config{
        Debug: true,
        Plugins: []ispx.Plugin{
            {Name: "myPlugin", Plugin: myPluginInstance},
        },
    }
    ispx.Launch(cfg)
}
```

### With Custom Plugins

```go
import (
    "github.com/goplus/spx/v2/pkg/ispx"
    "github.com/goplus/spx/v2/pkg/ispx/plugin"
    "github.com/goplus/ixgo"
)

type MyPlugin struct{}

func (p *MyPlugin) RegisterJS() {}
func (p *MyPlugin) RegisterPatch(ctx *ixgo.Context) error { 
    // Register custom patches
    return nil 
}
func (p *MyPlugin) Init() {
    // Initialize plugin
}

func main() {
    cfg := &ispx.Config{
        Debug: true,
        Plugins: []ispx.Plugin{
            {
                Name:   "myplugin",
                Plugin: &MyPlugin{},
            },
        },
    }
    ispx.Launch(cfg)
}
```

## API Reference

### ispx.Launch(cfg *Config)

Starts the SPX runtime with the given configuration.

**Parameters:**
- `cfg`: Configuration for the runtime. If `nil`, `DefaultConfig()` will be used.

### ispx.Config

Configuration structure for the SPX runtime:

```go
type Config struct {
    // Debug enables debug mode
    Debug bool
    
    // Logger for runtime logging (optional, uses default logger if nil)
    Logger runtime.Logger
    
    // Platform for platform-specific operations (optional, uses default platform if nil)
    Platform runtime.Platform
    
    // Plugins to register (optional)
    Plugins []Plugin
}
```

### ispx.Plugin

Plugin structure:

```go
type Plugin struct {
    Name   string
    Plugin plugin.Plugin
}
```

### ispx.DefaultConfig()

Returns the default configuration with sensible defaults.

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

## Platform Support

### Native Platform

On native platforms (desktop), the runtime:
- Reads SPX project files from the file system
- Builds and runs the interpreter
- Outputs logs to stdout/stderr

### WASM Platform

On WASM platforms (browser), the runtime:
- Registers JavaScript interfaces for file access
- Provides callbacks for build/run operations
- Integrates with browser APIs

## License

This module follows the same license as the parent SPX project.
