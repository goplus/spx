package ispx

import (
	_ "github.com/goplus/spx/v2/pkg/ispx/embedpkg"
	_ "github.com/goplus/spx/v2/pkg/ispx/pkg/github.com/goplus/spx/v2"
	_ "github.com/goplus/spx/v2/pkg/ispx/pkg/github.com/goplus/spx/v2/pkg/gdspx/pkg/engine"
	"github.com/goplus/spx/v2/pkg/ispx/plugin"
	"github.com/goplus/spx/v2/pkg/ispx/runtime"
)

// Config holds configuration for SPX runtime
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

// Plugin represents a plugin with name and implementation
type Plugin struct {
	Name   string
	Plugin plugin.Plugin
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	rtConfig := runtime.DefaultConfig()
	return &Config{
		Debug:    rtConfig.Debug,
		Logger:   rtConfig.Logger,
		Platform: rtConfig.Platform,
		Plugins:  []Plugin{},
	}
}

// Launch starts the SPX runtime with the given configuration.
//
// It calls the platform-specific runtime.Run function to initialize and
// start the SPX interpreter.
//
// Parameters:
//   - cfg: Configuration for the runtime. If nil, DefaultConfig() will be used.
//
// Usage:
//
//	// Launch with default config
//	ispx.Launch(nil)
//
//	// Launch with custom config
//	cfg := &ispx.Config{
//		Debug: true,
//		Plugins: []ispx.Plugin{
//			{Name: "myPlugin", Plugin: myPluginInstance},
//		},
//	}
//	ispx.Launch(cfg)
func Launch(cfg *Config) {
	var rtConfig *runtime.Config
	if cfg == nil {
		rtConfig = runtime.DefaultConfig()
	} else {
		// Convert ispx.Config to runtime.Config
		rtConfig = &runtime.Config{
			Debug:    cfg.Debug,
			Logger:   cfg.Logger,
			Platform: cfg.Platform,
			Plugins:  make([]runtime.Plugin, len(cfg.Plugins)),
		}
		for i, p := range cfg.Plugins {
			rtConfig.Plugins[i] = runtime.Plugin{
				Name:   p.Name,
				Plugin: p.Plugin,
			}
		}
	}
	runtime.Launch(rtConfig)
}
