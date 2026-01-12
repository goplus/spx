package runtime

import "errors"

// Common errors returned by the runtime package.
var (
	// ErrBuildFailed indicates that the build process failed
	ErrBuildFailed = errors.New("runtime: build failed")

	// ErrNoBuildCache indicates that no build cache is available
	ErrNoBuildCache = errors.New("runtime: no build cache available")

	// ErrInterpFailed indicates that interpreter execution failed
	ErrInterpFailed = errors.New("runtime: interpreter execution failed")

	// ErrInvalidConfig indicates that the configuration is invalid
	ErrInvalidConfig = errors.New("runtime: invalid configuration")
)
