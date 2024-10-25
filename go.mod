module github.com/goplus/spx

go 1.22.3

require godot-ext/gdspx v0.0.0

replace godot-ext/gdspx => github.com/realdream-ai/gdspx v0.0.0-20241024132745-d779ec70144d

require (
	github.com/pkg/errors v0.9.1
	golang.org/x/image v0.18.0
	golang.org/x/mobile v0.0.0-20220518205345-8578da9835fd
)

replace (
	golang.org/x/image => golang.org/x/image v0.0.0-20210628002857-a66eb6448b8d
	golang.org/x/mobile => golang.org/x/mobile v0.0.0-20210902104108-5d9a33257ab5
	golang.org/x/mod => golang.org/x/mod v0.5.1
	golang.org/x/tools => golang.org/x/tools v0.1.8
)
