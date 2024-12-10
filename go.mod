module github.com/goplus/spx

go 1.21.3

require (
	github.com/pkg/errors v0.9.1
	github.com/realdream-ai/gdspx v0.0.0-20241206104940-c8b4a8f07f1c
	github.com/realdream-ai/mathf v0.0.0-20241205091834-a85cb2402a16
	golang.org/x/mobile v0.0.0-20220518205345-8578da9835fd
)

require golang.org/x/image v0.23.0 // indirect

replace (
	golang.org/x/image => golang.org/x/image v0.0.0-20210628002857-a66eb6448b8d
	golang.org/x/mobile => golang.org/x/mobile v0.0.0-20210902104108-5d9a33257ab5
	golang.org/x/mod => golang.org/x/mod v0.5.1
	golang.org/x/tools => golang.org/x/tools v0.1.8
)
