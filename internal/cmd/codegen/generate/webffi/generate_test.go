package webffi

import (
	"testing"

	"github.com/goplus/spx/v2/internal/cmd/codegen/gdextensionparser/clang"
	"github.com/goplus/spx/v2/internal/cmd/codegen/generate/common"
	"github.com/stretchr/testify/require"
)

func TestGetJsFuncArgsFlattensGdObj(t *testing.T) {
	function := &clang.TypedefFunction{
		Name: "GDExtensionSpxPhysicsCheckTouchedStageBoundaries",
		ReturnType: clang.PrimativeType{
			Name: "void",
		},
		Arguments: []clang.Argument{
			{
				Name: "obj",
				Type: clang.Type{
					Primative: &clang.PrimativeType{Name: "GdObj"},
				},
			},
			{
				Name: "ret_value",
				Type: clang.Type{
					Primative: &clang.PrimativeType{Name: "int64_t"},
				},
			},
		},
	}

	require.Equal(t, []string{"obj_low", "obj_high"}, getJsFuncArgs(function))
}

func TestGetJsFuncArgsSkipsNativeArrayLenArg(t *testing.T) {
	common.ClearNativeArrayBridgeSpecs()
	t.Cleanup(common.ClearNativeArrayBridgeSpecs)
	common.RegisterNativeArrayBridgeSpec(common.NativeArrayBridgeSpec{
		BaseFunctionName: "GDExtensionSpxSpriteBatchUpdateTransforms",
		BaseArgName:      "buffer",
		DataArgName:      "buffer_data",
		DataArgGoType:    "[]float32",
		DataArgPtrType:   "*float32",
		LenArgName:       "len",
		LenArgGoType:     "int32",
		GoArgType:        "[]float32",
	})

	function := &clang.TypedefFunction{
		Name: "GDExtensionSpxSpriteBatchUpdateTransforms",
		Arguments: []clang.Argument{
			{
				Name: "buffer_data",
				Type: clang.Type{
					Primative: &clang.PrimativeType{Name: "float", IsPointer: true},
				},
			},
			{
				Name: "len",
				Type: clang.Type{
					Primative: &clang.PrimativeType{Name: "int"},
				},
			},
		},
	}

	require.Equal(t, []string{"buffer"}, getJsFuncArgs(function))
}

func TestGetJsFuncBodyUsesLowHighOrderForFlatGdIntArgs(t *testing.T) {
	function := &clang.TypedefFunction{
		Name: "GDExtensionSpxPhysicsCheckTouchedStageBoundaries",
		ReturnType: clang.PrimativeType{
			Name: "void",
		},
		Arguments: []clang.Argument{
			{
				Name: "obj",
				Type: clang.Type{
					Primative: &clang.PrimativeType{Name: "GdObj"},
				},
			},
			{
				Name: "ret_value",
				Type: clang.Type{
					Primative: &clang.PrimativeType{Name: "int64_t"},
				},
			},
		},
	}

	body := getJsFuncBody(function)
	require.Contains(t, body, "Module._gdspx_new_obj(obj_low, obj_high)")
}

func TestGetJsFuncBodyUsesFixedScratchAccessorForFlatReturn(t *testing.T) {
	function := &clang.TypedefFunction{
		Name: "GDExtensionSpxPhysicsCheckTouchedStageBoundaries",
		ReturnType: clang.PrimativeType{
			Name: "void",
		},
		Arguments: []clang.Argument{
			{
				Name: "obj",
				Type: clang.Type{
					Primative: &clang.PrimativeType{Name: "GdObj"},
				},
			},
			{
				Name: "ret_value",
				Type: clang.Type{
					Primative: &clang.PrimativeType{Name: "GdInt"},
				},
			},
		},
	}

	body := getJsFuncBody(function)
	require.Contains(t, body, "this._readGdIntLike(_retValue, this._getGdIntScratch())")
	require.NotContains(t, body, `"_gdIntScratch"`)
}

func TestJsTemplateUsesHeapU32ForFlatReads(t *testing.T) {
	require.Contains(t, jsEngineJsFileText, "var _u32 = Module.HEAPU32;")
	require.Contains(t, jsEngineJsFileText, "scratch.low = _u32[_word];")
	require.Contains(t, jsEngineJsFileText, "scratch.high = _u32[_word + 1];")
	require.NotContains(t, jsEngineJsFileText, "_getGdDataView()")
	require.NotContains(t, jsEngineJsFileText, "getUint32(")
}
