package dockercmd

import (
	"os"

	"github.com/goplus/spx/v2/internal/cmd/buildctl/shared"
)

var osStderr = os.Stderr

var errUsage = shared.ErrUsage

func findRepoRoot() (string, error) { return shared.FindRepoRoot() }
func fileExists(path string) bool   { return shared.FileExists(path) }
