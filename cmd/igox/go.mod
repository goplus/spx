module github.com/goplus/spx/v2/cmd/igox

go 1.24.0

tool github.com/goplus/ixgo/cmd/qexp

require (
	github.com/goplus/builder/tools/ai v0.0.0-20250522033218-53c368333ac2
	github.com/goplus/ixgo v0.52.0
	github.com/goplus/mod v0.17.1
	github.com/goplus/reflectx v1.4.2
	github.com/goplus/spx/v2 v2.0.0-00010101000000-000000000000
)

require (
	github.com/gopherjs/gopherjs v0.0.0-20200217142428-fce0ec30dd00 // indirect
	github.com/goplus/gogen v1.19.0 // indirect
	github.com/goplus/xgo v1.5.0 // indirect
	github.com/h2non/filetype v1.1.3 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/qiniu/x v1.15.1 // indirect
	github.com/realdream-ai/mathf v0.0.0-20250513071532-e55e1277a8c5 // indirect
	github.com/timandy/routine v1.1.5 // indirect
	github.com/visualfc/funcval v0.1.4 // indirect
	github.com/visualfc/gid v0.3.0 // indirect
	github.com/visualfc/goembed v0.3.2 // indirect
	github.com/visualfc/xtype v0.2.0 // indirect
	golang.org/x/image v0.23.0 // indirect
	golang.org/x/mobile v0.0.0-20220518205345-8578da9835fd // indirect
	golang.org/x/mod v0.23.0 // indirect
	golang.org/x/tools v0.30.0 // indirect
)

replace (
	github.com/goplus/spx/v2 => ../../
	golang.org/x/image => golang.org/x/image v0.0.0-20210628002857-a66eb6448b8d
	golang.org/x/mobile => golang.org/x/mobile v0.0.0-20210902104108-5d9a33257ab5
)
