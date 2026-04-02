package projecttemplate

import _ "embed"

//go:embed go.mod.template
var goMod string

// GoMod returns the shared go.mod template used by project creation flows.
func GoMod() string {
	return goMod
}
