module github.com/goplus/spx

go 1.16

require (
	github.com/ajstarks/svgo v0.0.0-20210927141636-6d70534b1098
	github.com/golang/freetype v0.0.0-20170609003504-e2365dfdc4a0
	github.com/goplus/gop v1.0.12
	github.com/hajimehoshi/ebiten/v2 v2.2.0
	github.com/pkg/errors v0.9.1
	github.com/qiniu/audio v0.2.0
	github.com/qiniu/x v1.11.5
	github.com/srwiley/oksvg v0.1.2-no-charset
	github.com/srwiley/rasterx v0.0.0-20210519020934-456a8d69b780
	golang.org/x/image v0.0.0-20210628002857-a66eb6448b8d
)

replace (
	github.com/hajimehoshi/oto => github.com/hajimehoshi/oto v1.0.1
	github.com/srwiley/oksvg => github.com/qiniu/oksvg v0.1.2-no-charset
)
