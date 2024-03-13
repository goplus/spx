module github.com/goplus/spx

go 1.16

require (
	github.com/ajstarks/svgo v0.0.0-20210927141636-6d70534b1098
	github.com/goplus/canvas v0.1.0
	github.com/hajimehoshi/ebiten/v2 v2.6.6
	github.com/jezek/xgb v1.1.1 // indirect
	github.com/pkg/errors v0.9.1
	github.com/qiniu/audio v0.2.1
	github.com/qiniu/x v1.13.9
	github.com/srwiley/oksvg v0.0.0-20210519022825-9fc0c575d5fe
	github.com/srwiley/rasterx v0.0.0-20210519020934-456a8d69b780
	golang.org/x/image v0.14.0
	golang.org/x/mobile v0.0.0-20231127183840-76ac6878050a
	golang.org/x/sys v0.15.0 // indirect
)

replace (
	github.com/hajimehoshi/oto => github.com/hajimehoshi/oto v1.0.1
	github.com/srwiley/oksvg => github.com/qiniu/oksvg v0.2.0-no-charset
	golang.org/x/image => golang.org/x/image v0.0.0-20210628002857-a66eb6448b8d
	golang.org/x/mobile => golang.org/x/mobile v0.0.0-20210902104108-5d9a33257ab5
	golang.org/x/mod => golang.org/x/mod v0.5.1
	golang.org/x/tools => golang.org/x/tools v0.1.8
)
