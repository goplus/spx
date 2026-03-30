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

// findPromotedFieldPtr is like FindFieldPtr but also recurses into anonymous
// (embedded) fields following Go's embedding promotion rules:
// current-level fields are checked first (respecting `from`), then embedded
// fields are searched. Embedded fields starting before `from` (e.g. *Game at
// index 1 when from=2 for sprites) are still recursed into.
func findPromotedFieldPtr(v reflect.Value, name string, from int) any {
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	t := v.Type()
	n := v.NumField()
	// First pass: direct fields at this level (respecting `from`).
	if result := FindFieldPtr(v, name, from); result != nil {
		return result
	}
	// Second pass: recurse into all anonymous fields (start from 0 to cover
	// embedded fields that appear before `from`).
	for i := range n {
		if !t.Field(i).Anonymous {
			continue
		}
		embedded := v.Field(i)
		if embedded.Kind() == reflect.Pointer && embedded.IsNil() {
			continue
		}
		if result := findPromotedFieldPtr(embedded, name, 0); result != nil {
			return result
		}
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
