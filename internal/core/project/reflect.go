package project

import (
	"reflect"
	"strings"
	"unsafe"
)

type FieldAllocConfig struct {
	IsPointerSpriteType        func(reflect.Type) bool
	ResolveInterfaceSpriteType func(fieldName string) (reflect.Type, bool)
}

func FieldPtrOrAlloc(v reflect.Value, fieldIndex int, cfg FieldAllocConfig) (name string, val any) {
	tFld := v.Type().Field(fieldIndex)
	vFld := v.Field(fieldIndex)
	typ := tFld.Type
	word := unsafe.Pointer(vFld.Addr().Pointer())
	ret := reflect.NewAt(typ, word).Interface()

	if vFld.Kind() == reflect.Pointer && cfg.IsPointerSpriteType != nil && cfg.IsPointerSpriteType(typ) {
		obj := reflect.New(typ.Elem())
		reflect.ValueOf(ret).Elem().Set(obj)
		ret = obj.Interface()
	}

	if vFld.Kind() == reflect.Interface && cfg.ResolveInterfaceSpriteType != nil {
		if typ2, ok := cfg.ResolveInterfaceSpriteType(tFld.Name); ok {
			obj := reflect.New(typ2)
			reflect.ValueOf(ret).Elem().Set(obj)
			ret = obj.Interface()
		}
	}
	return tFld.Name, ret
}

func FindFieldPtr(v reflect.Value, name string, from int) any {
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	t := v.Type()
	for i, n := from, v.NumField(); i < n; i++ {
		tFld := t.Field(i)
		if tFld.Name == name {
			word := unsafe.Pointer(v.Field(i).Addr().Pointer())
			return reflect.NewAt(tFld.Type, word).Interface()
		}
	}
	return nil
}

// embeddedStructs returns the dereferenced struct values of all exported
// anonymous fields of v. Nil pointers and nil interfaces are skipped.
func embeddedStructs(v reflect.Value) []reflect.Value {
	t := v.Type()
	var out []reflect.Value
	for i := range v.NumField() {
		tFld := t.Field(i)
		if !tFld.Anonymous || !tFld.IsExported() {
			continue
		}
		f := v.Field(i)
		if f.Kind() == reflect.Pointer {
			if f.IsNil() {
				continue
			}
			f = f.Elem()
		}
		if f.Kind() == reflect.Interface {
			if f.IsNil() {
				continue
			}
			f = f.Elem()
		}
		if f.Kind() != reflect.Struct {
			continue
		}
		out = append(out, f)
	}
	return out
}

// findPromotedFieldPtr approximates Go's embedding promotion rules: direct
// fields at the current level are checked first (respecting `from`), then
// exported anonymous (embedded) fields are searched using BFS so that the
// shallowest match always wins. Anonymous fields before `from` at the top
// level are still included in the BFS (e.g. *Game at index 1 when from=2 for
// sprites).
//
// Access boundary: only exported anonymous fields are recursed into. This
// relies on the assumption that gogen-generated user types always use exported
// (uppercase) type names, so an unexported anonymous field always indicates a
// framework-internal type rather than a user-defined one.
//
// Ambiguity: if the same field name appears at equal depth in multiple
// branches, the first match in declaration order is returned rather than
// raising an error (unlike the Go compiler which rejects such access as
// ambiguous).
func findPromotedFieldPtr(v reflect.Value, name string, from int) any {
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return nil
	}

	// First pass: direct fields at this level (respecting `from`).
	if result := FindFieldPtr(v, name, from); result != nil {
		return result
	}

	// Second pass: BFS through exported anonymous fields.
	// `from` only restricts direct field access at the top level; all
	// embedded fields are searched starting from index 0.
	for queue := embeddedStructs(v); len(queue) > 0; {
		var next []reflect.Value
		var found any
		for _, ev := range queue {
			if result := FindFieldPtr(ev, name, 0); result != nil && found == nil {
				found = result
			}
			next = append(next, embeddedStructs(ev)...)
		}
		if found != nil {
			return found
		}
		queue = next
	}
	return nil
}

func FindFieldRefCaseInsensitive(v reflect.Value, name string, from int) any {
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	t := v.Type()
	nameLower := strings.ToLower(name)
	for i, n := from, v.NumField(); i < n; i++ {
		tFld := t.Field(i)
		if strings.ToLower(tFld.Name) == nameLower {
			word := unsafe.Pointer(v.Field(i).Addr().Pointer())
			return reflect.NewAt(tFld.Type, word).Interface()
		}
	}
	return nil
}

func FindObjectPtr(v reflect.Value, name string, from int) any {
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	t := v.Type()
	for i, n := from, v.NumField(); i < n; i++ {
		tFld := t.Field(i)
		if tFld.Name != name {
			continue
		}

		typ := tFld.Type
		vFld := v.Field(i)
		if vFld.Kind() == reflect.Pointer {
			word := unsafe.Pointer(vFld.Pointer())
			return reflect.NewAt(typ.Elem(), word).Interface()
		}
		if vFld.Kind() == reflect.Interface {
			word := unsafe.Pointer(vFld.Addr().Pointer())
			return reflect.NewAt(tFld.Type, word).Elem().Interface()
		}
		word := unsafe.Pointer(vFld.Addr().Pointer())
		return reflect.NewAt(typ, word).Interface()
	}
	return nil
}
