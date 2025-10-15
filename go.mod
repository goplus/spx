module github.com/goplus/spx/v2

go 1.23.0

require (
	github.com/goplus/spbase v0.1.0
	github.com/h2non/filetype v1.1.3
	github.com/petermattis/goid v0.0.0-20250721140440-ea1c0173183e
	github.com/pkg/errors v0.9.1
	golang.org/x/mobile v0.0.0-20220518205345-8578da9835fd
)

require (
	github.com/gorilla/websocket v1.5.1 // indirect
	golang.org/x/image v0.23.0 // indirect
	golang.org/x/net v0.17.0 // indirect
)

replace (
	golang.org/x/image => golang.org/x/image v0.0.0-20210628002857-a66eb6448b8d
	golang.org/x/mobile => golang.org/x/mobile v0.0.0-20210902104108-5d9a33257ab5
	golang.org/x/mod => golang.org/x/mod v0.5.1
	golang.org/x/tools => golang.org/x/tools v0.1.8
)
