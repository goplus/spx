package enginewrap

func F64Tof32(slice []float64) []float32 {
	if slice == nil {
		return []float32{}
	}
	out := make([]float32, len(slice))
	for i, v := range slice {
		out[i] = float32(v)
	}
	return out
}

func F32Tof64(slice []float32) []float64 {
	if slice == nil {
		return []float64{}
	}
	out := make([]float64, len(slice))
	for i, v := range slice {
		out[i] = float64(v)
	}
	return out
}
