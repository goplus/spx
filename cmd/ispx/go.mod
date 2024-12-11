module github.com/goplus/builder/ispx

go 1.21

require (
	github.com/goplus/igop v0.27.1
	github.com/goplus/reflectx v1.2.2
	github.com/goplus/spx v1.0.1-0.20241029011511-845f2c0e2e74
	github.com/hajimehoshi/ebiten/v2 v2.8.0-alpha.3
)

require (
	github.com/ajstarks/svgo v0.0.0-20210927141636-6d70534b1098 // indirect
	github.com/ebitengine/gomobile v0.0.0-20240518074828-e86332849895 // indirect
	github.com/ebitengine/hideconsole v1.0.0 // indirect
	github.com/ebitengine/oto/v3 v3.3.0-alpha.3 // indirect
	github.com/ebitengine/purego v0.8.0-alpha.3 // indirect
	github.com/esimov/stackblur-go v1.0.1-0.20190121110005-00e727e3c7a9 // indirect
	github.com/golang/freetype v0.0.0-20170609003504-e2365dfdc4a0 // indirect
	github.com/gopherjs/gopherjs v0.0.0-20200217142428-fce0ec30dd00 // indirect
	github.com/goplus/canvas v0.1.0 // indirect
	github.com/goplus/gogen v1.15.2 // indirect
	github.com/goplus/gop v1.2.6 // indirect
	github.com/goplus/mod v0.13.10 // indirect
	github.com/hajimehoshi/go-mp3 v0.3.4 // indirect
	github.com/jezek/xgb v1.1.1 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/qiniu/audio v0.2.1 // indirect
	github.com/qiniu/x v1.13.10 // indirect
	github.com/srwiley/oksvg v0.0.0-20210519022825-9fc0c575d5fe // indirect
	github.com/srwiley/rasterx v0.0.0-20210519020934-456a8d69b780 // indirect
	github.com/timandy/routine v1.1.1 // indirect
	github.com/visualfc/funcval v0.1.4 // indirect
	github.com/visualfc/gid v0.1.0 // indirect
	github.com/visualfc/goembed v0.3.2 // indirect
	github.com/visualfc/xtype v0.2.0 // indirect
	golang.org/x/image v0.18.0 // indirect
	golang.org/x/mobile v0.0.0-20220518205345-8578da9835fd // indirect
	golang.org/x/mod v0.19.0 // indirect
	golang.org/x/sync v0.7.0 // indirect
	golang.org/x/sys v0.22.0 // indirect
	golang.org/x/text v0.16.0 // indirect
	golang.org/x/tools v0.23.0 // indirect
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
)

replace (
	github.com/goplus/spx => ../../
	github.com/hajimehoshi/oto => github.com/hajimehoshi/oto v1.0.1
	github.com/srwiley/oksvg => github.com/qiniu/oksvg v0.2.0-no-charset
	golang.org/x/image => golang.org/x/image v0.0.0-20210628002857-a66eb6448b8d
	golang.org/x/mobile => golang.org/x/mobile v0.0.0-20210902104108-5d9a33257ab5
	golang.org/x/mod => golang.org/x/mod v0.5.1
)
