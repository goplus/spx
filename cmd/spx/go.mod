module github.com/goplus/spx/cmd/spx

go 1.23.1

require (
	github.com/otiai10/copy v1.14.0
	godot-ext/gdspx/cmd/gdspx v0.0.0
)

require (
	github.com/goplus/spx v0.0.0-00010101000000-000000000000
	godot-ext/gdspx v0.0.0 // indirect
)

require (
	github.com/pkg/errors v0.9.1 // indirect
	golang.org/x/image v0.18.0 // indirect
	golang.org/x/mobile v0.0.0-20220518205345-8578da9835fd // indirect
	golang.org/x/sync v0.3.0 // indirect
	golang.org/x/sys v0.0.0-20220715151400-c0bba94af5f8 // indirect
)

replace github.com/goplus/spx => ../../

replace godot-ext/gdspx/cmd/gdspx => github.com/realdream-ai/gdspx/cmd/gdspx v0.0.0-20241021082413-21077ba5d509

replace godot-ext/gdspx => github.com/realdream-ai/gdspx v0.0.0-20241021082413-21077ba5d509
