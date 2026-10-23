module github.com/goplus/spx/v4/internal/cmd/codegen

go 1.26.0

require (
	github.com/alecthomas/participle/v2 v2.1.4
	// The local replace below owns the source; this is only the v4 module floor.
	github.com/goplus/spx/v4 v4.0.0
	github.com/iancoleman/strcase v0.3.0
	github.com/stretchr/testify v1.12.1
)

replace github.com/goplus/spx/v4 => ../../..

require go.yaml.in/yaml/v3 v3.0.5 // indirect
