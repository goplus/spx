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

package ispx

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strconv"
)

const spxPackagePath = "github.com/goplus/spx/v4"

type generatedStructInfo struct {
	// 这些字段来自 xgo 生成的 Go 结构体。融合器只对能够确认类型的字段下手，
	// 不根据字段名猜测，避免把用户自定义的 At/Set/Contains 方法误优化。
	listFields  map[string]struct{}
	valueFields map[string]struct{}
	// embedded 保存匿名嵌入的本地结构体名，用于递归查找提升后的字段。
	embedded []string
}

type sourceReplacement struct {
	// 替换使用字节偏移，而不是直接修改 AST。这样可以保留生成代码中
	// 不需要重新格式化的部分，只替换已经确认安全的最小表达式范围。
	start int
	end   int
	text  string
}

// fuseGeneratedListCalls 合并 XGo 生成代码中的高频容器组合调用。
//
// 处理发生在 xgobuild 已经把 SPX 源码转换成 Go 之后、ixgo LoadFile 之前：
//
//	SPX:  elements.Contains(particles.At(i))
//	Go:   this.elements.Contains(this.particles.At(i))
//	优化: this.elements.ContainsAt(&this.particles, i)
//
// 这里不是修改用户的 .spx 文件，也不是修改 ixgo 解释器指令，而是对即将
// 交给解释器的 Go 源码做一次保守的局部重写。只匹配能够从结构体声明中
// 确认的 spx.List/spx.Value 字段，避免误改用户的同名类型或方法。
func fuseGeneratedListCalls(source []byte) ([]byte, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "main.go", source, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse generated Go source: %w", err)
	}

	spxName := importedPackageName(file, spxPackagePath, "spx")
	if spxName == "" {
		return source, nil
	}
	structs := collectGeneratedStructs(file, spxName)
	replacements := make([]sourceReplacement, 0)

	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Body == nil {
			continue
		}
		receiverName, receiverType := methodReceiver(function)
		if receiverName == "" || receiverType == "" {
			continue
		}

		// 访问 CallExpr 而不是文本搜索：AST 能区分真正的调用和注释、字符串，
		// 也能准确取得 At 的接收者与索引表达式。
		ast.Inspect(function.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			if replacement, ok := fuseEqualAt(source, fset, call, spxName, receiverName, receiverType, structs); ok {
				replacements = append(replacements, replacement)
				// 外层调用已经整体替换，不能继续访问它的子树，否则会产生
				// 与当前替换范围重叠的 At/Int 等替换。
				return false
			}
			if replacement, ok := fuseContainsAt(source, fset, call, receiverName, receiverType, structs); ok {
				replacements = append(replacements, replacement)
				return false
			}
			if replacement, ok := fuseSetAt(source, fset, call, receiverName, receiverType, structs); ok {
				replacements = append(replacements, replacement)
				return false
			}
			if replacement, ok := fuseTypedAt(source, fset, call, receiverName, receiverType, structs); ok {
				replacements = append(replacements, replacement)
				return false
			}
			return true
		})
	}

	if len(replacements) == 0 {
		return source, nil
	}
	// AST 遍历顺序不保证适合直接拼接；按源码位置排序后再一次性生成新源代码。
	sort.Slice(replacements, func(i, j int) bool { return replacements[i].start < replacements[j].start })

	var output bytes.Buffer
	output.Grow(len(source))
	previousEnd := 0
	for _, replacement := range replacements {
		if replacement.start < previousEnd {
			return nil, fmt.Errorf("overlapping generated call fusion at byte %d", replacement.start)
		}
		output.Write(source[previousEnd:replacement.start])
		output.WriteString(replacement.text)
		previousEnd = replacement.end
	}
	output.Write(source[previousEnd:])
	return output.Bytes(), nil
}

func importedPackageName(file *ast.File, importPath, defaultName string) string {
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil || path != importPath {
			continue
		}
		if spec.Name == nil {
			return defaultName
		}
		if spec.Name.Name == "." || spec.Name.Name == "_" {
			return ""
		}
		return spec.Name.Name
	}
	return ""
}

func collectGeneratedStructs(file *ast.File, spxName string) map[string]generatedStructInfo {
	structs := make(map[string]generatedStructInfo)
	for _, declaration := range file.Decls {
		group, ok := declaration.(*ast.GenDecl)
		if !ok || group.Tok != token.TYPE {
			continue
		}
		for _, specification := range group.Specs {
			typeSpec, ok := specification.(*ast.TypeSpec)
			if !ok {
				continue
			}
			structType, ok := typeSpec.Type.(*ast.StructType)
			if !ok {
				continue
			}
			info := generatedStructInfo{
				listFields:  make(map[string]struct{}),
				valueFields: make(map[string]struct{}),
			}
			for _, field := range structType.Fields.List {
				if isSPXListType(field.Type, spxName) {
					for _, name := range field.Names {
						info.listFields[name.Name] = struct{}{}
					}
					continue
				}
				if isSPXValueType(field.Type, spxName) {
					for _, name := range field.Names {
						info.valueFields[name.Name] = struct{}{}
					}
					continue
				}
				// 匿名嵌入字段可能把 List/Value 字段提升到当前接收者，
				// 记录本地类型名，后续通过 hasListField/hasValueField 递归解析。
				if len(field.Names) == 0 {
					if embedded := localTypeName(field.Type); embedded != "" {
						info.embedded = append(info.embedded, embedded)
					}
				}
			}
			structs[typeSpec.Name.Name] = info
		}
	}
	return structs
}

func isSPXListType(expression ast.Expr, spxName string) bool {
	return isSPXNamedType(expression, spxName, "List")
}

func isSPXValueType(expression ast.Expr, spxName string) bool {
	return isSPXNamedType(expression, spxName, "Value")
}

func isSPXNamedType(expression ast.Expr, spxName, typeName string) bool {
	selector, ok := expression.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != typeName {
		return false
	}
	packageName, ok := selector.X.(*ast.Ident)
	return ok && packageName.Name == spxName
}

func localTypeName(expression ast.Expr) string {
	if pointer, ok := expression.(*ast.StarExpr); ok {
		expression = pointer.X
	}
	name, _ := expression.(*ast.Ident)
	if name == nil {
		return ""
	}
	return name.Name
}

func methodReceiver(function *ast.FuncDecl) (name, typeName string) {
	if function.Recv == nil || len(function.Recv.List) != 1 || len(function.Recv.List[0].Names) != 1 {
		return "", ""
	}
	return function.Recv.List[0].Names[0].Name, localTypeName(function.Recv.List[0].Type)
}

func hasListField(structs map[string]generatedStructInfo, typeName, fieldName string, visited map[string]bool) bool {
	if visited[typeName] {
		return false
	}
	visited[typeName] = true
	info, ok := structs[typeName]
	if !ok {
		return false
	}
	if _, ok := info.listFields[fieldName]; ok {
		return true
	}
	for _, embedded := range info.embedded {
		if hasListField(structs, embedded, fieldName, visited) {
			return true
		}
	}
	return false
}

func hasValueField(structs map[string]generatedStructInfo, typeName, fieldName string, visited map[string]bool) bool {
	if visited[typeName] {
		return false
	}
	visited[typeName] = true
	info, ok := structs[typeName]
	if !ok {
		return false
	}
	if _, ok := info.valueFields[fieldName]; ok {
		return true
	}
	for _, embedded := range info.embedded {
		if hasValueField(structs, embedded, fieldName, visited) {
			return true
		}
	}
	return false
}

func listField(expression ast.Expr, receiverName, receiverType string, structs map[string]generatedStructInfo) bool {
	selector, ok := expression.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	receiver, ok := selector.X.(*ast.Ident)
	return ok && receiver.Name == receiverName && hasListField(structs, receiverType, selector.Sel.Name, make(map[string]bool))
}

func valueField(expression ast.Expr, receiverName, receiverType string, structs map[string]generatedStructInfo) bool {
	selector, ok := expression.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	receiver, ok := selector.X.(*ast.Ident)
	return ok && receiver.Name == receiverName && hasValueField(structs, receiverType, selector.Sel.Name, make(map[string]bool))
}

func listAtCall(expression ast.Expr, receiverName, receiverType string, structs map[string]generatedStructInfo) (list, index ast.Expr, ok bool) {
	call, ok := expression.(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return nil, nil, false
	}
	method, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || method.Sel.Name != "At" || !listField(method.X, receiverName, receiverType, structs) {
		return nil, nil, false
	}
	return method.X, call.Args[0], true
}

// fuseEqualAt 把 Equal(list.At(i), literal) 合并为 list.EqualAt(i, literal)。
// 第二个参数目前只接受字面量，是为了保持求值顺序：原表达式先求值 At(i)，
// 再求值比较参数；如果比较参数本身可能修改 list，提前放入新方法参数会改变语义。
func fuseEqualAt(source []byte, fset *token.FileSet, call *ast.CallExpr, spxName, receiverName, receiverType string, structs map[string]generatedStructInfo) (sourceReplacement, bool) {
	function, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || function.Sel.Name != "Equal" || len(call.Args) != 2 {
		return sourceReplacement{}, false
	}
	packageName, ok := function.X.(*ast.Ident)
	if !ok || packageName.Name != spxName {
		return sourceReplacement{}, false
	}
	if _, ok := call.Args[1].(*ast.BasicLit); !ok {
		return sourceReplacement{}, false
	}
	list, index, ok := listAtCall(call.Args[0], receiverName, receiverType, structs)
	if !ok {
		return sourceReplacement{}, false
	}
	return replacementForCall(source, fset, call, "%s.EqualAt(%s, %s)", list, index, call.Args[1])
}

// fuseContainsAt 把 target.Contains(source.At(i)) 合并为
// target.ContainsAt(&source, i)。原调用需要先跨越 At 的解释器调用边界，
// 再跨越 Contains 的边界；合并后由一个 direct call 完成两步操作。
func fuseContainsAt(source []byte, fset *token.FileSet, call *ast.CallExpr, receiverName, receiverType string, structs map[string]generatedStructInfo) (sourceReplacement, bool) {
	method, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || method.Sel.Name != "Contains" || len(call.Args) != 1 || !listField(method.X, receiverName, receiverType, structs) {
		return sourceReplacement{}, false
	}
	list, index, ok := listAtCall(call.Args[0], receiverName, receiverType, structs)
	if !ok {
		return sourceReplacement{}, false
	}
	return replacementForCall(source, fset, call, "%s.ContainsAt(&%s, %s)", method.X, list, index)
}

// fuseSetAt 覆盖两种写入组合：
//
//	value.Set(list.At(i))       -> value.SetAt(&list, i)
//	list.Set(dst, list.At(src)) -> list.SetAt(dst, &list, src)
//
// SetAt 的实现必须先读取 source，再写入目标；这样即使 source 与目标是同一
// 个 List，也和原来“先执行 At，再进入 Set”的求值顺序一致。
func fuseSetAt(source []byte, fset *token.FileSet, call *ast.CallExpr, receiverName, receiverType string, structs map[string]generatedStructInfo) (sourceReplacement, bool) {
	method, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || method.Sel.Name != "Set" {
		return sourceReplacement{}, false
	}
	if listField(method.X, receiverName, receiverType, structs) {
		if len(call.Args) != 2 {
			return sourceReplacement{}, false
		}
		list, index, ok := listAtCall(call.Args[1], receiverName, receiverType, structs)
		if !ok {
			return sourceReplacement{}, false
		}
		return replacementForCall(source, fset, call, "%s.SetAt(%s, &%s, %s)", method.X, call.Args[0], list, index)
	}
	if valueField(method.X, receiverName, receiverType, structs) {
		if len(call.Args) != 1 {
			return sourceReplacement{}, false
		}
		list, index, ok := listAtCall(call.Args[0], receiverName, receiverType, structs)
		if !ok {
			return sourceReplacement{}, false
		}
		return replacementForCall(source, fset, call, "%s.SetAt(&%s, %s)", method.X, list, index)
	}
	return sourceReplacement{}, false
}

// fuseTypedAt 把 list.At(i).Int/Float/String 合并成一次 direct call。
// 这类访问原本会先生成 Value，再调用 Value 的转换方法；IntAt 等方法直接
// 读取 List 内部元素，省掉中间 Value 和一次方法边界。
func fuseTypedAt(source []byte, fset *token.FileSet, call *ast.CallExpr, receiverName, receiverType string, structs map[string]generatedStructInfo) (sourceReplacement, bool) {
	method, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || len(call.Args) != 0 {
		return sourceReplacement{}, false
	}
	methodName := method.Sel.Name
	switch methodName {
	case "Int":
		methodName = "IntAt"
	case "Float":
		methodName = "FloatAt"
	case "String":
		methodName = "StringAt"
	default:
		return sourceReplacement{}, false
	}
	list, index, ok := listAtCall(method.X, receiverName, receiverType, structs)
	if !ok {
		return sourceReplacement{}, false
	}
	start, end, ok := sourceRange(fset, call)
	if !ok {
		return sourceReplacement{}, false
	}
	listStart, listEnd, ok := sourceRange(fset, list)
	if !ok {
		return sourceReplacement{}, false
	}
	indexStart, indexEnd, ok := sourceRange(fset, index)
	if !ok {
		return sourceReplacement{}, false
	}
	return sourceReplacement{
		start: start,
		end:   end,
		text:  fmt.Sprintf("%s.%s(%s)", source[listStart:listEnd], methodName, source[indexStart:indexEnd]),
	}, true
}

func replacementForCall(source []byte, fset *token.FileSet, call *ast.CallExpr, format string, expressions ...ast.Expr) (sourceReplacement, bool) {
	start, end, ok := sourceRange(fset, call)
	if !ok {
		return sourceReplacement{}, false
	}
	// any 在这里仅表示“可以保存任意 AST 表达式对应的字符串”，不是运行时
	// 的动态值；真正进入生成代码的仍然是 source 中的原始表达式文本。
	arguments := make([]any, len(expressions))
	for index, expression := range expressions {
		expressionStart, expressionEnd, ok := sourceRange(fset, expression)
		if !ok {
			return sourceReplacement{}, false
		}
		arguments[index] = string(source[expressionStart:expressionEnd])
	}
	return sourceReplacement{start: start, end: end, text: fmt.Sprintf(format, arguments...)}, true
}

func sourceRange(fset *token.FileSet, node ast.Node) (start, end int, ok bool) {
	startPosition := fset.PositionFor(node.Pos(), false)
	endPosition := fset.PositionFor(node.End(), false)
	if startPosition.Offset < 0 || endPosition.Offset < startPosition.Offset {
		return 0, 0, false
	}
	return startPosition.Offset, endPosition.Offset, true
}
