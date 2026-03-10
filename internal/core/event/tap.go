package event

// If0 wraps a zero-argument callback and only invokes it when enabled returns true.
// A nil call becomes a no-op, and a nil enabled function leaves call unchanged.
func If0(enabled func() bool, call func()) func() {
	if call == nil {
		return func() {}
	}
	if enabled == nil {
		return call
	}
	return func() {
		if enabled() {
			call()
		}
	}
}

// If1 wraps a one-argument callback and only invokes it when enabled returns true.
// A nil call becomes a no-op, and a nil enabled function leaves call unchanged.
func If1[T any](enabled func() bool, call func(T)) func(T) {
	if call == nil {
		return func(T) {}
	}
	if enabled == nil {
		return call
	}
	return func(v T) {
		if enabled() {
			call(v)
		}
	}
}

// If2 wraps a two-argument callback and only invokes it when enabled returns true.
// A nil call becomes a no-op, and a nil enabled function leaves call unchanged.
func If2[A, B any](enabled func() bool, call func(A, B)) func(A, B) {
	if call == nil {
		return func(A, B) {}
	}
	if enabled == nil {
		return call
	}
	return func(a A, b B) {
		if enabled() {
			call(a, b)
		}
	}
}

// Ignore1 converts a zero-argument callback into a one-argument callback that ignores its input.
// A nil call becomes a no-op.
func Ignore1[T any](call func()) func(T) {
	if call == nil {
		return func(T) {}
	}
	return func(T) {
		call()
	}
}

// Ignore2 converts a zero-argument callback into a two-argument callback that ignores its inputs.
// A nil call becomes a no-op.
func Ignore2[A, B any](call func()) func(A, B) {
	if call == nil {
		return func(A, B) {}
	}
	return func(A, B) {
		call()
	}
}

// Tap0 runs tap before call.
// If tap is nil, the original call is returned. If call is nil, only tap runs.
func Tap0(call func(), tap func()) func() {
	if tap == nil {
		return call
	}
	if call == nil {
		return func() {
			tap()
		}
	}
	return func() {
		tap()
		call()
	}
}

// Tap1 runs tap before call for a one-argument callback.
// If tap is nil, the original call is returned. If call is nil, only tap runs.
func Tap1[T any](call func(T), tap func(T)) func(T) {
	if tap == nil {
		return call
	}
	if call == nil {
		return func(v T) {
			tap(v)
		}
	}
	return func(v T) {
		tap(v)
		call(v)
	}
}

// Tap2 runs tap before call for a two-argument callback.
// If tap is nil, the original call is returned. If call is nil, only tap runs.
func Tap2[A, B any](call func(A, B), tap func(A, B)) func(A, B) {
	if tap == nil {
		return call
	}
	if call == nil {
		return func(a A, b B) {
			tap(a, b)
		}
	}
	return func(a A, b B) {
		tap(a, b)
		call(a, b)
	}
}

// TapVoid1 runs tap with the incoming value before invoking the zero-argument call.
// A nil tap still preserves call, and a nil call still preserves tap.
func TapVoid1[T any](call func(), tap func(T)) func(T) {
	if tap == nil {
		return func(T) {
			if call != nil {
				call()
			}
		}
	}
	return func(v T) {
		tap(v)
		if call != nil {
			call()
		}
	}
}

// TapVoid2 runs tap with the incoming values before invoking the zero-argument call.
// A nil tap still preserves call, and a nil call still preserves tap.
func TapVoid2[A, B any](call func(), tap func(A, B)) func(A, B) {
	if tap == nil {
		return func(A, B) {
			if call != nil {
				call()
			}
		}
	}
	return func(a A, b B) {
		tap(a, b)
		if call != nil {
			call()
		}
	}
}
