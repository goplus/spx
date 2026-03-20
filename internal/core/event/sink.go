package event

import (
	"math"
	"slices"
)

func NewSink(owner any, handler any, cond ...func(any) bool) Sink {
	sink := Sink{
		Owner:   owner,
		Handler: handler,
	}
	if len(cond) > 0 {
		sink.Cond = cond[0]
	}
	return sink
}

func MatchOwner(owner any) func(any) bool {
	return func(data any) bool {
		return data == owner
	}
}

func MatchOwnerOrNil(owner any) func(any) bool {
	return func(data any) bool {
		return data == nil || data == owner
	}
}

func MatchValue[T comparable](want T) func(any) bool {
	return func(data any) bool {
		got, ok := data.(T)
		return ok && got == want
	}
}

func MatchAnyOf[T comparable](values []T) func(any) bool {
	return func(data any) bool {
		got, ok := data.(T)
		if !ok {
			return false
		}
		return slices.Contains(values, got)
	}
}

func MatchApproxFloat(want, tolerance float64) func(any) bool {
	return func(data any) bool {
		got, ok := data.(float64)
		return ok && math.Abs(got-want) < tolerance
	}
}
