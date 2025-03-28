package gdext

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/iancoleman/strcase"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func generateSpxExtHeader(dir, outputFile string, isRawFormat bool) {
	mergedStr := mergeManagerHeader(dir)
	mergedHeaderFuncStr := generateManagerHeader(mergedStr, isRawFormat)
	finalHeader := strings.Replace(gdSpxExtH, "###MANAGER_FUNC_DEFINE", mergedHeaderFuncStr, -1)
	// Write the final header file
	f, err := os.Create(outputFile)
	if err != nil {
		panic(err)
	}
	f.Write([]byte(finalHeader))
	f.Close()
}

func mergeManagerHeader(dir string) string {
	files, err := filepath.Glob(filepath.Join(dir, "spx*mgr.h"))
	if err != nil {
		fmt.Println("Error finding files:", err)
		return ""
	}

	var builder strings.Builder
	builder.WriteString("#include \"gdextension_interface.h\"\n")
	builder.WriteString("#include \"gdextension_spx_mgr_pre_define.h\"\n")

	for _, file := range files {
		if strings.Contains(file, "spx_base_mgr.h") {
			continue
		}

		f, err := os.Open(file)
		if err != nil {
			fmt.Println("Error opening file:", err)
			continue
		}
		defer f.Close()

		var buffer bytes.Buffer
		scanner := bufio.NewScanner(f)
		className := ""
		inPublicSection := false

		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if strings.HasPrefix(line, "//") || strings.HasPrefix(line, "/*") || strings.HasPrefix(line, "*") || strings.HasPrefix(line, "*/") {
				continue
			}
			if strings.HasPrefix(line, "};") {
				break
			}

			if className == "" {
				re := regexp.MustCompile(`class\s+(\w+)\s*:\s*SpxBaseMgr\s*{`)
				match := re.FindStringSubmatch(line)
				if len(match) > 0 {
					className = match[1]
				}
			}

			if strings.HasPrefix(line, "public:") {
				inPublicSection = true
				buffer.Reset()
				buffer.WriteString("public:\n")
				continue
			}

			if inPublicSection {
				buffer.WriteString("\t" + line + "\n")
			}
		}

		if className != "" {
			builder.WriteString(fmt.Sprintf("class %s {\n", className))
			builder.WriteString(buffer.String())
			builder.WriteString("\n};\n\n")
		}

		if err := scanner.Err(); err != nil {
			fmt.Println("Error reading file:", err)
		}
	}

	return builder.String()
}

func generateManagerHeader(input string, rawFormat bool) string {
	scanner := bufio.NewScanner(strings.NewReader(input))
	var currentClassName string
	methodRegex := regexp.MustCompile(`\s*void\s+(\w+)\((.*)\);`)
	returnRegex := regexp.MustCompile(`\s*(\w+)\s+(\w+)\((.*)\);`)

	var builder strings.Builder

	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "class") {
			parts := strings.Fields(line)
			currentClassName = parts[1]
			currentClassName = currentClassName[:len(currentClassName)-3]
			builder.WriteString("// " + currentClassName + "\n")
			continue
		}
		if methodRegex.MatchString(line) {
			matches := methodRegex.FindStringSubmatch(line)
			methodName := strcase.ToCamel(matches[1])
			params := matches[2]
			builder.WriteString(fmt.Sprintf("typedef void (*GDExtension%s%s)(%s);\n", currentClassName, methodName, params))
		} else if returnRegex.MatchString(line) {
			matches := returnRegex.FindStringSubmatch(line)
			returnType := matches[1]
			methodName := strcase.ToCamel(matches[2])
			params := matches[3]
			if rawFormat {
				builder.WriteString(fmt.Sprintf("typedef %s (*GDExtension%s%s)(%s);\n", returnType, currentClassName, methodName, params))
			} else {
				if len(params) > 0 {
					returnType = ", " + returnType
				}
				builder.WriteString(fmt.Sprintf("typedef void (*GDExtension%s%s)(%s%s* ret_value);\n", currentClassName, methodName, params, returnType))
			}
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Println("读取字符串时出错:", err)
	}

	return builder.String()
}
