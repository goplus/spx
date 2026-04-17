module github.com/goplus/spxdemo

go 1.25.0

require github.com/goplus/spx/v2 v2.0.0-pre.28 //xgo:class

require (
	github.com/goplus/spbase v0.1.0 // indirect
	github.com/petermattis/goid v0.0.0-20250721140440-ea1c0173183e // indirect
	golang.org/x/image v0.23.0 // indirect
	golang.org/x/mobile v0.0.0-20220518205345-8578da9835fd // indirect
)

replace github.com/goplus/spx/v2 => ../..
