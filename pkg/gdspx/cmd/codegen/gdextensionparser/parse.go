package gdextensionparser

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/goplus/spx/pkg/gdspx/cmd/codegen/gdextensionparser/clang"
	"github.com/goplus/spx/pkg/gdspx/cmd/codegen/gdextensionparser/preprocessor"
)

func ReadFiles(dir, fileName string) string {
	var allLines []string
	lines, _ := readLines(filepath.Join(dir, fileName))
	for _, line := range lines {
		if strings.HasPrefix(line, "#include \"") {
			includePath := strings.ReplaceAll(strings.ReplaceAll(line, "#include \"", ""), "\"", "")
			includeLines, _ := readLines(filepath.Join(dir, includePath))
			for _, inLine := range includeLines {
				if !strings.HasPrefix(inLine, "#include \"") {
					allLines = append(allLines, inLine)
				}
			}
		} else {
			allLines = append(allLines, line)
		}
	}

	var sb strings.Builder
	for _, line := range allLines {
		// hack to remove a specific char
		if strings.Contains(line, "/*******") {
			continue
		}
		sb.WriteString(line + "\n")
	}
	finalStr := sb.String()
	finalStr = strings.ReplaceAll(finalStr, "\r", "")
	return finalStr
}

func readLines(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}
func expandIncludeFiles(projectPath, header, outputName string) (string, error) {
	dirPath := filepath.Join(projectPath, "../../internal/ffi/")
	allStrs := ReadFiles(dirPath, header)
	tempPath := filepath.Join(dirPath, outputName)
	ioutil.WriteFile(tempPath, []byte(allStrs), 0644)
	return allStrs, nil
}

func GenerateGDExtensionInterfaceAST(projectPath, astOutputFilename string) (clang.CHeaderFileAST, error) {
	str, _ := expandIncludeFiles(projectPath, "gdextension_spx_codegen_header.h", "_temp_output.h")
	return generateGDExtensionInterfaceAST(str, projectPath, astOutputFilename)
}

func generateGDExtensionInterfaceAST(b, projectPath, astOutputFilename string) (clang.CHeaderFileAST, error) {
	preprocFile, err := preprocessor.ParsePreprocessorString((string)(b))
	if err != nil {
		return clang.CHeaderFileAST{}, fmt.Errorf("error preprocessing %s: %w", projectPath, err)
	}

	preprocText := preprocFile.Eval(false)
	ast, err := clang.ParseCString(preprocText)
	if err != nil {
		return clang.CHeaderFileAST{}, fmt.Errorf("error parsing %s: %w", projectPath, err)
	}

	// write the AST out to a file as JSON for debugging
	if astOutputFilename != "" {
		b, err := json.Marshal(ast)
		if err != nil {
			panic(err)
		}
		f, err := os.Create(astOutputFilename)
		if err != nil {
			panic(err)
		}
		defer f.Close()
		w := bufio.NewWriter(f)
		w.Write(b)
		w.Flush()
	}

	return ast, nil
}
