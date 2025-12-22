package gdext

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/iancoleman/strcase"
)

// Pre-compiled regular expressions for better performance
var (
	// For normalizeParams function
	reSpaceComma = regexp.MustCompile(`\s+,`)
	reSpaceParen = regexp.MustCompile(`\s+\)`)
	reCommaSpace = regexp.MustCompile(`,\s*`)

	// For mergeManagerHeader function
	reClassDefinition = regexp.MustCompile(`class\s+(\w+)\s*:\s*(?:public\s+)?(?:SpxBaseMgr|SpxObjectMgr<\w+>)\s*{`)

	// For generateManagerHeader function
	reMethodVoid   = regexp.MustCompile(`\s*void\s+(\w+)\((.*)\);`)
	reMethodReturn = regexp.MustCompile(`\s*(\w+)\s+(\w+)\((.*)\);`)
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
		if strings.Contains(file, "spx_base_mgr.h") || strings.Contains(file, "spx_object_mgr.h") {
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
				continue
			}
			// Skip inline function definitions (lines with both { and })
			if strings.Contains(line, "{") && strings.Contains(line, "}") {
				continue
			}

			if className == "" {
				match := reClassDefinition.FindStringSubmatch(line)
				if len(match) > 0 {
					className = match[1]
				} else {
					continue
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

// normalizeParams ensures proper spacing in parameter lists
func normalizeParams(params string) string {
	if params == "" {
		return params
	}
	// Remove trailing spaces before commas and closing paren
	params = reSpaceComma.ReplaceAllString(params, ",")
	params = reSpaceParen.ReplaceAllString(params, ")")
	// Ensure single space after commas
	params = reCommaSpace.ReplaceAllString(params, ", ")
	// Trim any leading/trailing whitespace
	return strings.TrimSpace(params)
}

func generateManagerHeader(input string, rawFormat bool) string {
	scanner := bufio.NewScanner(strings.NewReader(input))
	var currentClassName string

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
		if reMethodVoid.MatchString(line) {
			matches := reMethodVoid.FindStringSubmatch(line)
			methodName := strcase.ToCamel(matches[1])
			params := normalizeParams(matches[2])
			builder.WriteString(fmt.Sprintf("typedef void (*GDExtension%s%s)(%s);\n", currentClassName, methodName, params))
		} else if reMethodReturn.MatchString(line) {
			matches := reMethodReturn.FindStringSubmatch(line)
			returnType := matches[1]
			methodName := strcase.ToCamel(matches[2])
			params := normalizeParams(matches[3])
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
		fmt.Println("Error reading string:", err)
	}

	return builder.String()
}
