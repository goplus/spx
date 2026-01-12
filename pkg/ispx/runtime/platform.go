package runtime

// Platform represents platform-specific operations
type Platform interface {
	// HandleLookupError handles package lookup errors
	HandleLookupError(err error)
}

// defaultPlatform will be set by platform-specific init functions
var defaultPlatform Platform
