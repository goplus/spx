module github.com/goplus/spx/v3/internal/cmd/codegen

go 1.25.0

require (
	github.com/alecthomas/participle/v2 v2.1.4
	github.com/davecgh/go-spew v1.1.1
	// The local replace below owns the source; this is only the v3 module floor.
	github.com/goplus/spx/v3 v3.0.0
	github.com/iancoleman/strcase v0.3.0
	github.com/stretchr/testify v1.12.0
	golang.org/x/exp v0.0.0-20230522175609-2e198f4a06a1
)

replace github.com/goplus/spx/v3 => ../../..

require gopkg.in/yaml.v3 v3.0.1 // indirect
