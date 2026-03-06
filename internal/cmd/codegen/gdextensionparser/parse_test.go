package gdextensionparser

import (
	"os"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/require"
)

func TestGenerateGDExtensionInterfaceAST(t *testing.T) {
	projectPath, err := os.Getwd()
	require.NoError(t, err)
	f, err := GenerateGDExtensionInterfaceAST(projectPath, "")
	require.NoError(t, err)
	spew.Dump(f)
}
