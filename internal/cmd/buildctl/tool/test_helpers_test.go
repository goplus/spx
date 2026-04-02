package tool

import (
	"testing"

	"github.com/goplus/spx/v2/internal/cmd/buildctl/shared"
)

func mustDefaultRuntimeVersion(t *testing.T) string {
	t.Helper()

	version, err := shared.DefaultRuntimeVersion()
	if err != nil {
		t.Fatal(err)
	}
	return version
}
