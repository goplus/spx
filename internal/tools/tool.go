package tools

import (
	"strconv"

	"github.com/goplus/spx/internal/math32"
)

func GetVec2(unk interface{}) (*math32.Vector2, bool) {
	return unk.(*math32.Vector2), true
}

func GetFloat(unk interface{}) (float64, bool) {
	switch i := unk.(type) {
	case float32:
		return float64(i), true
	case float64:
		return float64(i), true
	case int64:
		return float64(i), true
	case int32:
		return float64(i), true
	case int16:
		return float64(i), true
	case int8:
		return float64(i), true
	case uint64:
		return float64(i), true
	case uint32:
		return float64(i), true
	case uint16:
		return float64(i), true
	case uint8:
		return float64(i), true
	case int:
		return float64(i), true
	case uint:
		return float64(i), true
	case string:
		f, err := strconv.ParseFloat(i, 64)
		if err != nil {
			return 0, false
		}
		return float64(f), true
	default:
		return 0, false
	}
}

func GetInt(unk interface{}) (int, bool) {
	switch i := unk.(type) {
	case float64:
		return int(i), true
	case float32:
		return int(i), true
	case int64:
		return int(i), true
	case int32:
		return int(i), true
	case int16:
		return int(i), true
	case int8:
		return int(i), true
	case uint64:
		return int(i), true
	case uint32:
		return int(i), true
	case uint16:
		return int(i), true
	case uint8:
		return int(i), true
	case int:
		return int(i), true
	case uint:
		return int(i), true
	case string:
		f, err := strconv.ParseFloat(i, 64)
		if err != nil {
			return 0, false
		}
		return int(f), true
	default:
		return 0, false
	}
}
