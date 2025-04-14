module github.com/goplus/spx/pkg/gdspx/cmd/codegen

go 1.21.3

require (
	github.com/alecthomas/participle/v2 v2.1.4
	github.com/davecgh/go-spew v1.1.1
	github.com/iancoleman/strcase v0.3.0
	github.com/stretchr/testify v1.10.0
	golang.org/x/exp v0.0.0-20230522175609-2e198f4a06a1
)

require (
	github.com/pmezard/go-difflib v1.0.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace (
	golang.org/x/image => golang.org/x/image v0.0.0-20210628002857-a66eb6448b8d
	golang.org/x/mobile => golang.org/x/mobile v0.0.0-20210902104108-5d9a33257ab5
	golang.org/x/mod => golang.org/x/mod v0.5.1
	golang.org/x/tools => golang.org/x/tools v0.1.8
)
