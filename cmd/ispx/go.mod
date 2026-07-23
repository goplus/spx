// cmd/ispx is an isolated local Builder AI test harness and intentionally stays on SPX v2.
// Builder consumes the root module's pkg/ispx package instead.
module github.com/goplus/spx/v2/cmd/ispx

go 1.25.0

tool github.com/goplus/ixgo/cmd/qexp

require (
	github.com/goplus/builder/tools/ai v0.0.0-20260507022922-937aacf1cd16
	github.com/goplus/ixgo v1.1.1
	github.com/goplus/mod v0.21.1
	github.com/goplus/spx/v2 v2.0.4
)

require (
	github.com/goplus/gogen v1.23.5 // indirect
	github.com/goplus/reflectx v1.7.2 // indirect
	github.com/goplus/spbase v0.1.0 // indirect
	github.com/goplus/xgo v1.7.5 // indirect
	github.com/petermattis/goid v0.0.0-20250721140440-ea1c0173183e // indirect
	github.com/qiniu/x v1.18.0 // indirect
	github.com/timandy/routine v1.1.6 // indirect
	github.com/visualfc/funcval v0.1.5 // indirect
	github.com/visualfc/gid v0.3.1 // indirect
	github.com/visualfc/xtype v0.3.2 // indirect
	golang.org/x/image v0.23.0 // indirect
	golang.org/x/mobile v0.0.0-20220518205345-8578da9835fd // indirect
	golang.org/x/mod v0.32.0 // indirect
	golang.org/x/tools v0.41.0 // indirect
)
