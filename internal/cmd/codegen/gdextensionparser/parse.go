package gdextensionparser

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/goplus/spx/v2/internal/cmd/codegen/gdextensionparser/clang"
	"github.com/goplus/spx/v2/internal/cmd/codegen/gdextensionparser/preprocessor"
)

func ReadFiles(dir, fileName string) (string, error) {
	var allLines []string
	lines, err := readLines(filepath.Join(dir, fileName))
	if err != nil {
		return "", err
	}
	for _, line := range lines {
		if strings.HasPrefix(line, "#include \"") {
			includePath := strings.ReplaceAll(strings.ReplaceAll(line, "#include \"", ""), "\"", "")
			includeLines, err := readLines(filepath.Join(dir, includePath))
			if err != nil {
				return "", err
			}
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
	return finalStr, nil
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

func findProjectRoot(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		header := filepath.Join(dir, "internal", "gdengine", "binding", "native", "gdextension_spx_codegen_header.h")
		if _, err := os.Stat(header); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("unable to find project root from %s", start)
}

func expandIncludeFiles(projectPath, header, outputName string) (string, error) {
	rootPath, err := findProjectRoot(projectPath)
	if err != nil {
		return "", err
	}
	dirPath := filepath.Join(rootPath, "internal", "gdengine", "binding", "native")
	allStrs, err := ReadFiles(dirPath, header)
	if err != nil {
		return "", err
	}
	tempPath := filepath.Join(dirPath, outputName)
	err = os.WriteFile(tempPath, []byte(allStrs), 0644)
	if err != nil {
		return "", err
	}
	return allStrs, nil
}

func GenerateGDExtensionInterfaceAST(projectPath, astOutputFilename string) (clang.CHeaderFileAST, error) {
	str, err := expandIncludeFiles(projectPath, "gdextension_spx_codegen_header.h", "_temp_output.h")
	if err != nil {
		return clang.CHeaderFileAST{}, err
	}
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
