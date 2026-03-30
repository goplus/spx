package project

import (
	"fmt"
	"reflect"
)

func ResolveMemberStringEval(target reflect.Value, name string, from int) func() string {
	// Try as field first (fields don't support lowercase aliases in gogen).
	ref := getValueRef(target, name, from)
	if ref.IsValid() {
		return func() string {
			return fmt.Sprint(ref.Interface())
		}
	}

	// Try as method with alias support (lowercase -> uppercase).
	targetForMethod := target
	if target.Kind() != reflect.Ptr && target.CanAddr() {
		targetForMethod = target.Addr()
	}

	aliasName := aliasNameOf(name, true)

	// Try original name first.
	m := targetForMethod.MethodByName(name)
	if m.IsValid() && methodHasAutoProperty(m) {
		return makeAutoPropertyAccessor(m, false)
	}

	// Only try alias if original name didn't find any method.
	if !m.IsValid() && aliasName != name {
		mAlias := targetForMethod.MethodByName(aliasName)
		if mAlias.IsValid() && methodHasAutoProperty(mAlias) {
			return makeAutoPropertyAccessor(mAlias, true)
		}
	}

	return nil
}

func getValueRef(target reflect.Value, name string, from int) reflect.Value {
	if valPtr := findPromotedFieldPtr(target, name, from); valPtr != nil {
		return reflect.ValueOf(valPtr).Elem()
	}
	return reflect.Value{}
}

// aliasNameOf mimics gogen's aliasNameOf logic:
// For methods, lowercase names are mapped to uppercase (e.g., "add" -> "Add").
func aliasNameOf(name string, isMethod bool) string {
	if isMethod && name != "" {
		if c := name[0]; c >= 'a' && c <= 'z' {
			return string(rune(c)+('A'-'a')) + name[1:]
		}
	}
	return ""
}

// methodHasAutoProperty checks if a method value is a valid auto-property getter.
func methodHasAutoProperty(m reflect.Value) bool {
	if !m.IsValid() {
		return false
	}
	mType := m.Type()
	return mType.NumIn() == 0 && mType.NumOut() == 1
}

func makeAutoPropertyAccessor(m reflect.Value, autoProperty bool) func() string {
	return func() string {
		if autoProperty {
			result := m.Call(nil)[0].Interface()
			if fVal, ok := result.(float64); ok {
				return fmt.Sprintf("%.2f", fVal)
			}
			if f32Val, ok := result.(float32); ok {
				return fmt.Sprintf("%.2f", f32Val)
			}
			return fmt.Sprint(result)
		}

		// Keep current behavior for non-auto-property path.
		return fmt.Sprintf("%p", m.Interface())
	}
}
