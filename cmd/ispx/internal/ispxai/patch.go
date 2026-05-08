package ispxai

import (
	"fmt"

	"github.com/goplus/ixgo"
)

const aiPackagePath = "github.com/goplus/builder/tools/ai"

func RegisterPatch(ctx *ixgo.Context) error {
	if err := ctx.RegisterPatch(aiPackagePath, `
package ai

import . "github.com/goplus/builder/tools/ai"

func XGot_Player_XGox_OnCmd[T any](p *Player, handler func(cmd T) error) {
	var cmd T
	PlayerOnCmd_(p, cmd, handler)
}

func XGot_Player_XGox_OnCmd__0[T any](p *Player, handler func(cmd T) error) {
	XGot_Player_XGox_OnCmd(p, handler)
}
`); err != nil {
		return fmt.Errorf("failed to register ai patch: %w", err)
	}
	return nil
}
