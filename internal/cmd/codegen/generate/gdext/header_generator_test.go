package gdext

import (
	"strings"
	"testing"

	"github.com/goplus/spx/v2/internal/cmd/codegen/generate/common"
	"github.com/stretchr/testify/require"
)

func TestGenerateManagerHeaderSkipsRawMethods(t *testing.T) {
	input := strings.TrimSpace(`
class SpxSpriteMgr {
public:
	void batch_update_transforms(GdArray buffer);
	void batch_update_transforms_raw(const float *buffer_data, int len);
	GdBool destroy_sprite(GdObj obj);
	void batch_update_visuals_raw(const float *buffer_data, int len);
};
`)

	output := generateManagerHeader(input, false)

	require.Contains(t, output, "GDExtensionSpxSpriteBatchUpdateTransforms")
	require.Contains(t, output, "GDExtensionSpxSpriteDestroySprite")
	require.NotContains(t, output, "GDExtensionSpxSpriteBatchUpdateTransformsRaw")
	require.NotContains(t, output, "GDExtensionSpxSpriteBatchUpdateVisualsRaw")

	spec, ok := common.GetNativeArrayBridgeSpec("GDExtensionSpxSpriteBatchUpdateTransforms")
	require.True(t, ok)
	require.Equal(t, "buffer", spec.BaseArgName)
	require.Equal(t, "[]float32", spec.GoArgType)
	require.EqualValues(t, 2, spec.FastArrayType)
	require.Equal(t, "GDExtensionSpxSpriteBatchUpdateTransformsRaw", spec.RawFunctionName)
}

func TestGenerateManagerHeaderRegistersDirectNativeArrayBridge(t *testing.T) {
	input := strings.TrimSpace(`
class SpxSpriteMgr {
public:
	void batch_update_transforms(const float *buffer_data, int len);
};
`)

	output := generateManagerHeader(input, false)

	require.Contains(t, output, "GDExtensionSpxSpriteBatchUpdateTransforms")
	spec, ok := common.GetNativeArrayBridgeSpec("GDExtensionSpxSpriteBatchUpdateTransforms")
	require.True(t, ok)
	require.Equal(t, "buffer", spec.BaseArgName)
	require.Equal(t, "buffer_data", spec.DataArgName)
	require.Equal(t, "len", spec.LenArgName)
	require.Equal(t, "[]float32", spec.DataArgGoType)
	require.Equal(t, "*float32", spec.DataArgPtrType)
	require.Equal(t, "int32", spec.LenArgGoType)
}
