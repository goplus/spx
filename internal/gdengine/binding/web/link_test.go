//go:build js && wasm

package webffi

import (
	_ "embed"
	"reflect"
	"regexp"
	"syscall/js"
	"testing"
)

// Use the generated resolver's actual names so the fixture follows ABI additions.
//
//go:embed ffi.gen.go
var generatedResolver string

func TestLinkOlderHostWithoutSpine(t *testing.T) {
	installLinkHost(t)
	for _, name := range []string{
		"gdspx_sprite_is_spine_supported", "gdspx_sprite_set_spine_skeleton",
		"gdspx_sprite_clear_spine_skeleton", "gdspx_sprite_get_spine_bounds",
		"gdspx_sprite_has_spine_animation",
	} {
		js.Global().Delete(name)
	}
	session, _ := Link()
	defer session.Unlink()
	if API.SpxSpriteIsSpineSupported.Invoke().Bool() {
		t.Fatal("older host reported Spine support")
	}
}

func installLinkHost(t *testing.T) {
	t.Helper()
	names := regexp.MustCompile(`resolveJSFunc\("([^"]+)"\)`).FindAllStringSubmatch(generatedResolver, -1)
	if len(names) != reflect.TypeOf(API).NumField() {
		t.Fatal("host fixture does not match generated API fields")
	}
	previousAPI := API
	API = GDExtensionInterface{}
	stub := js.FuncOf(func(js.Value, []js.Value) any { return nil })
	previous := make(map[string]js.Value, len(names))
	for _, name := range names {
		previous[name[1]] = js.Global().Get(name[1])
		js.Global().Set(name[1], stub)
	}
	t.Cleanup(func() {
		API = previousAPI
		for name, value := range previous {
			js.Global().Set(name, value)
		}
		stub.Release()
	})
	for _, entry := range []struct {
		name   string
		handle *js.Func
	}{
		{"go_wasm_init", &goWasmInitCallbackHandle},
		{"gdspx_dispatch", &callbackDispatcherHandle},
		{"gdspx_on_contact_events", &contactEventsHandle},
	} {
		oldHandle, oldGlobal := *entry.handle, js.Global().Get(entry.name)
		*entry.handle = js.Func{}
		js.Global().Delete(entry.name)
		t.Cleanup(func() {
			if entry.handle.Type() == js.TypeFunction {
				entry.handle.Release()
			}
			*entry.handle = oldHandle
			js.Global().Set(entry.name, oldGlobal)
		})
	}
}
