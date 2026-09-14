package common

import (
	"fmt"

	"github.com/goplus/spx/v3/internal/cmd/codegen/gdextensionparser/clang"
)

// Parameter records the public name and ABI positions once for every renderer.
type Parameter struct {
	Argument    clang.Argument
	Name        string
	Index       int
	Buffer      *ArrayBuffer
	LengthIndex int
	IsLength    bool
	GoType      string
}

// Scalars passed by value across the native and Web ABIs.
var directScalarTypes = map[string]string{
	"int": "int32", "int32_t": "int32", "float": "float32", "uint8_t": "byte",
}

// Parameters returns a read-only view in ABI order, including hidden lengths.
func (c *GenerationContext) Parameters(function *clang.TypedefFunction) []Parameter {
	if params, ok := c.parameters[function.Name]; ok {
		return params
	}
	params, _ := c.prepareParameters(function)
	return params
}

func (c *GenerationContext) Parameter(function *clang.TypedefFunction, name string) Parameter {
	if function == nil {
		return Parameter{}
	}
	if params, ok := c.parameterNames[function.Name]; ok {
		return params[name]
	}
	_, params := c.prepareParameters(function)
	return params[name]
}

func (p Parameter) LocalName(prefix string) string { return fmt.Sprintf("%s%d", prefix, p.Index) }

func (p Parameter) LengthName(prefix string) string {
	return fmt.Sprintf("%s%d", prefix, p.LengthIndex)
}

func (p Parameter) DirectScalar() bool {
	primitive := p.Argument.Type.Primative
	return primitive != nil && !primitive.IsPointer && directScalarTypes[primitive.Name] != ""
}

func (p Parameter) MustGoType(functionName string) string {
	if p.GoType != "" {
		return p.GoType
	}
	panic(fmt.Sprintf("no Go mapping for C type %q in function %s", MustPrimitiveTypeName(p.Argument, functionName), functionName))
}

func (p Parameter) GdxType(functionName string) string {
	switch goType := p.MustGoType(functionName); goType {
	case "Object", "Array":
		return "gdx." + goType
	default:
		return goType
	}
}

func (c *GenerationContext) prepareParameters(function *clang.TypedefFunction) ([]Parameter, map[string]Parameter) {
	args := c.EffectiveArguments(function)
	indices := make(map[string]int, len(args))
	for i, arg := range args {
		indices[arg.Name] = i
	}
	buffers := make(map[string]ArrayBuffer)
	lengths := make(map[string]bool)
	for _, buffer := range c.arrayBridges[function.Name].Buffers {
		buffers[buffer.Data.Name] = buffer
		if buffer.Length.Name != "" {
			lengths[buffer.Length.Name] = true
		}
	}
	params := make([]Parameter, 0, len(args))
	names := make(map[string]Parameter, len(args))
	publicNames := make(map[string]bool)
	for i, arg := range args {
		param := Parameter{Argument: arg, Name: arg.Name, Index: i, LengthIndex: -1, IsLength: lengths[arg.Name]}
		if buffer, ok := buffers[arg.Name]; ok {
			param.Buffer, param.Name = &buffer, buffer.ArgName()
			if buffer.Length.Name != "" {
				index, exists := indices[buffer.Length.Name]
				if !exists {
					panic("missing array length parameter in " + function.Name)
				}
				param.LengthIndex = index
			}
		}
		if !param.IsLength && param.Name != "" {
			if publicNames[param.Name] {
				panic("duplicate public parameter in " + function.Name + ": " + param.Name)
			}
			publicNames[param.Name] = true
		}
		if param.Buffer != nil {
			param.GoType = param.Buffer.GoType()
		} else if arg.Type.Primative != nil {
			param.GoType = c.cppType2Go[arg.Type.Primative.Name]
		}
		params = append(params, param)
		names[arg.Name] = param
	}
	return params, names
}
