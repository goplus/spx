package runtime

// Config holds configuration for SpxRunner
type Config struct {
	// Debug enables debug mode
	Debug bool

	// Logger for runtime logging (optional, uses defaultLogger if nil)
	Logger Logger

	// Platform for platform-specific operations (optional, uses defaultPlatform if nil)
	Platform Platform

	// Plugins to register (optional)
	Plugins []Plugin
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		Debug:    false,
		Logger:   defaultLogger,
		Platform: defaultPlatform,
		Plugins:  []Plugin{},
	}
}
