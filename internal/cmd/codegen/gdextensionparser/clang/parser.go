/*
 * Copyright (c) 2021 The XGo Authors (xgo.dev). All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// Package clang parses the C declarations used by SPX bindings.
package clang

import (
	"fmt"
	"strings"

	"github.com/alecthomas/participle/v2"
	"github.com/alecthomas/participle/v2/lexer"
)

type CHeaderFileAST struct {
	Expr []Expr `parser:" @@* " json:",omitempty"`
}

type Expr struct {
	Comment  string           `parser:"   @Comment " json:",omitempty"`
	Enum     *TypedefEnum     `parser:" | @@ ';'   " json:",omitempty"`
	Alias    *TypedefAlias    `parser:" | @@ ';'   " json:",omitempty"`
	Function *TypedefFunction `parser:" | @@ ';'   " json:",omitempty"`
	Struct   *TypedefStruct   `parser:" | @@ ';'   " json:",omitempty"`
}

type TypedefEnum struct {
	Values []EnumValue `parser:" 'typedef' 'enum' '{' ( @@ ( ',' Comment? @@ Comment? )* ','? Comment? )? '}' " json:",omitempty"`
	Name   *string     `parser:" @Ident                                                                       " json:",omitempty"`
}

type EnumValue struct {
	Name          string  `parser:" @Ident                     " json:",omitempty"`
	IntValue      *int    `parser:" ( '=' ( @Int               " json:",omitempty"`
	ConstRefValue *string `parser:"              | @Ident ) )? " json:",omitempty"`
}

type TypedefAlias struct {
	Type PrimitiveType `parser:" 'typedef' @@ "`
	Name string        `parser:" @Ident       " json:",omitempty"`
}

type TypedefFunction struct {
	ReturnType PrimitiveType `parser:" 'typedef' @@                "`
	Name       string        `parser:" '(' '*' @Ident ')'          " json:",omitempty"`
	Arguments  []Argument    `parser:" '(' ( @@ ( ',' @@ )* )? ')' " json:",omitempty"`
}

type TypedefStruct struct {
	Fields []StructField `parser:" 'typedef' 'struct' '{' @@* '}' " json:",omitempty"`
	Name   string        `parser:" @Ident                         " json:",omitempty"`
}

type StructField struct {
	Variable *StructVariable `parser:" ( @@       " json:",omitempty"`
	Function *StructFunction `parser:" | @@ ) ';' " json:",omitempty"`
}

type StructVariable struct {
	Type PrimitiveType `parser:" @@     "`
	Name string        `parser:" @Ident " json:",omitempty"`
}

type FunctionType struct {
	ReturnType PrimitiveType `parser:" @@                          "`
	Name       string        `parser:" '(' '*' @Ident ')'          " json:",omitempty"`
	Arguments  []Argument    `parser:" '(' ( @@ ( ',' @@ )* )? ')' " json:",omitempty"`
}

// PrimitiveType describes a named C type with optional const and pointer qualifiers.
type PrimitiveType struct {
	IsConst   bool   `parser:" @'const'? " json:",omitempty"`
	Name      string `parser:" @Ident    " json:",omitempty"`
	IsPointer bool   `parser:" @'*'?     " json:",omitempty"`
}

type Type struct {
	Function  *FunctionType  `parser:" ( @@   " json:",omitempty"`
	Primitive *PrimitiveType `parser:" | @@ ) " json:",omitempty"`
}

type StructFunction struct {
	ReturnType PrimitiveType `parser:" @@                     "`
	Name       string        `parser:" '(' '*' @Ident ')'     " json:",omitempty"`
	Arguments  []Argument    `parser:" '(' @@ ( ',' @@ )* ')' " json:",omitempty"`
	Comment    string        `parser:" @Comment?              " json:",omitempty"`
}

// Argument represents a C parameter, including function-pointer parameters.
type Argument struct {
	Type Type   `parser:" @@                               "`
	Name string `parser:" ( @Ident | '(' '*' @Ident ')' )? " json:",omitempty"`
}

func (a CHeaderFileAST) FindVariantEnumType() *TypedefEnum {
	for _, e := range a.Expr {
		if e.Enum != nil && e.Enum.Name != nil && *e.Enum.Name == "GDExtensionVariantType" {
			return e.Enum
		}
	}
	return nil
}

func (a CHeaderFileAST) CollectFunctions() []TypedefFunction {
	// Included headers may repeat typedefs, such as GDExtensionClassGetPropertyList.
	distinct := map[string]struct{}{}
	fns := make([]TypedefFunction, 0, len(a.Expr))

	for _, e := range a.Expr {
		if e.Function != nil {
			if _, ok := distinct[e.Function.Name]; !ok {
				fns = append(fns, *e.Function)
				distinct[e.Function.Name] = struct{}{}
			}
		}
	}
	return fns
}

// filterFunctions preserves declaration order and the deduplication in CollectFunctions.
func (a CHeaderFileAST) filterFunctions(include func(TypedefFunction) bool) []TypedefFunction {
	functions := a.CollectFunctions()
	filtered := make([]TypedefFunction, 0, len(functions))
	for _, function := range functions {
		if include(function) {
			filtered = append(filtered, function)
		}
	}
	return filtered
}

func isInterfaceFunction(function TypedefFunction) bool {
	return strings.HasPrefix(function.Name, "GDExtensionSpx") &&
		!strings.HasPrefix(function.Name, "GDExtensionSpxCallback") &&
		!strings.HasPrefix(function.Name, "GDExtensionSpxGlobal")
}

func (a CHeaderFileAST) CollectFunctionsOfClass(className string) []TypedefFunction {
	return a.filterFunctions(func(function TypedefFunction) bool {
		return isInterfaceFunction(function) && strings.HasPrefix(function.Name, "GDExtensionSpx"+className)
	})
}

func (a CHeaderFileAST) CollectGDExtensionManagerFunctions(managerName string, managerNames ManagerNames) []TypedefFunction {
	return a.filterFunctions(func(function TypedefFunction) bool {
		// Global functions remain available when explicitly requesting their manager.
		return strings.HasPrefix(function.Name, "GDExtensionSpx") &&
			!strings.HasPrefix(function.Name, "GDExtensionSpxCallback") &&
			managerNames.resolveASCII(function.Name) == managerName
	})
}

func (a CHeaderFileAST) CollectGDExtensionInterfaceFunctions() []TypedefFunction {
	return a.filterFunctions(isInterfaceFunction)
}

func (a CHeaderFileAST) CollectGDExtensionISpriteFunctions() []TypedefFunction {
	return a.CollectFunctionsOfClass("Sprite")
}

func (a CHeaderFileAST) CollectGDExtensionICallbackFunctions() []TypedefFunction {
	return a.filterFunctions(func(function TypedefFunction) bool {
		return strings.HasPrefix(function.Name, "GDExtensionSpxCallback")
	})
}

func (a CHeaderFileAST) CollectNonGDExtensionInterfaceFunctions() []TypedefFunction {
	return a.filterFunctions(func(function TypedefFunction) bool {
		return !strings.HasPrefix(function.Name, "GDExtensionSpx")
	})
}

func (a CHeaderFileAST) CollectStructs() []TypedefStruct {
	structs := make([]TypedefStruct, 0, len(a.Expr))
	for _, e := range a.Expr {
		if e.Struct != nil {
			structs = append(structs, *e.Struct)
		}
	}
	return structs
}

func (a CHeaderFileAST) CollectAliases() []TypedefAlias {
	aliases := make([]TypedefAlias, 0, len(a.Expr))
	for _, e := range a.Expr {
		if e.Alias != nil {
			aliases = append(aliases, *e.Alias)
		}
	}
	return aliases
}

func (a CHeaderFileAST) CollectEnums() []TypedefEnum {
	enums := make([]TypedefEnum, 0, len(a.Expr))
	for _, e := range a.Expr {
		if e.Enum != nil {
			enums = append(enums, *e.Enum)
		}
	}
	return enums
}

func (t TypedefStruct) CollectFunctions() []StructFunction {
	fns := make([]StructFunction, 0, len(t.Fields))
	for _, f := range t.Fields {
		if f.Function != nil {
			fns = append(fns, *f.Function)
		}
	}
	return fns
}

func (t FunctionType) CStyleString() string {
	sb := strings.Builder{}
	sb.WriteString(t.ReturnType.CStyleString())
	sb.WriteString("(*")
	sb.WriteString(t.Name)
	sb.WriteString(")(")
	for i := 0; i < len(t.Arguments); i++ {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(t.Arguments[i].Type.CStyleString())
	}
	sb.WriteString(")")
	return sb.String()
}

func (t PrimitiveType) CStyleString() string {
	sb := strings.Builder{}

	if t.IsConst {
		sb.WriteString("const ")
	}

	sb.WriteString(t.Name)

	if t.IsPointer {
		sb.WriteString(" * ")
	}

	return sb.String()
}

func (t Type) CStyleString() string {
	if t.Primitive != nil {
		return t.Primitive.CStyleString()
	} else if t.Function != nil {
		return t.Function.CStyleString()
	}

	panic("unhandled type")
}

func (a Argument) IsPinnable() bool {
	switch {
	case a.Type.Function != nil:
		return false
	case a.Type.Primitive != nil:
		switch a.Type.Primitive.Name {
		case "char":
			return false
		default:
			return a.Type.Primitive.IsPointer
		}
	}

	return false
}

func (a Argument) CStylePtrString(i int) string {
	if a.Type.Function != nil {
		return a.Type.CStyleString()
	}

	name := a.ResolvedName(i)
	typeName := strings.TrimSpace(a.Type.CStyleString())
	return typeName + " *" + name
}

func (a Argument) ResolvedPtrName(i int) string {
	if a.Type.Function != nil && a.Type.Function.Name != "" {
		return a.Type.Function.Name
	}
	return "*" + a.ResolvedName(i)
}

func (a Argument) CStyleString(i int) string {
	if a.Type.Function != nil {
		return a.Type.CStyleString()
	}

	name := a.ResolvedName(i)
	return joinCTypeAndName(a.Type.CStyleString(), name)
}

func (a Argument) ResolvedName(i int) string {
	if a.Type.Function != nil && a.Type.Function.Name != "" {
		return a.Type.Function.Name
	}

	if a.Name != "" {
		return a.Name
	}
	return fmt.Sprintf("arg_%d", i)
}

func ParseCString(s string) (CHeaderFileAST, error) {
	var headerFileLexer = lexer.MustStateful(lexer.Rules{
		"Root": {
			{Name: `Typedef`, Pattern: `typedef`},
			{Name: `Struct`, Pattern: `struct`},
			{Name: `{`, Pattern: `{`},
			{Name: `}`, Pattern: `}`},
			{Name: `;`, Pattern: `;`},
			{Name: `,`, Pattern: `,`},
			{Name: `"`, Pattern: `"`},
			{Name: `(`, Pattern: `\(`},
			{Name: `)`, Pattern: `\)`},
			{Name: `*`, Pattern: `\*`},
			{Name: `=`, Pattern: `=`},
			{Name: `Const`, Pattern: `const`},
			{Name: `Ident`, Pattern: `[a-zA-Z_][a-zA-Z0-9_]*`},
			{Name: `Int`, Pattern: `[+-]?\d+`},
			{Name: `Comment`, Pattern: `[ \t\r\n]*(\/\/[^\n]*)|(\/\*(.|[\r\n])*?\*\/)[ \t\r\n]*`},
			{Name: `Whitespace`, Pattern: `[ \t\r\n]+`},
		},
	})

	parser, err := participle.Build[CHeaderFileAST](
		participle.Lexer(headerFileLexer),
		participle.UseLookahead(20),
		participle.Elide("Whitespace", "Comment"),
	)

	if err != nil {
		return CHeaderFileAST{}, err
	}

	ast, err := parser.ParseString("", s)

	if err != nil {
		return CHeaderFileAST{}, err
	}

	return *ast, nil
}

func joinCTypeAndName(typeName, name string) string {
	typeName = strings.TrimSpace(typeName)
	if strings.HasSuffix(typeName, "*") {
		return typeName + name
	}
	return typeName + " " + name
}
