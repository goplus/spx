package valueutil

func OrDefault[T any](pval *T, defaultValue T) T {
	if pval == nil {
		return defaultValue
	}
	return *pval
}

func SetDefaultIfZero[T comparable](pval *T, defaultValue T) {
	var zero T
	if *pval == zero {
		*pval = defaultValue
	}
}
