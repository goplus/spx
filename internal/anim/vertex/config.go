package vertex

import (
	"github.com/goplus/spx/internal/anim/common"
)

type AnimPrefab struct {
	Names     []string   `json:"names"`
	Vertices  []float64  `json:"vertices"`
	Uvs       []float64  `json:"uv"`
	Triangles [][]uint16 `json:"triangles"`
}

type animCfg struct {
	FrameCount       int       `json:"frame_count"`
	RenderOrders     [][]int   `json:"render_orders"`
	AnimFramesVertex []float64 `json:"anim_frames_vertex"`
}

type AnimClip struct {
	Name   string `json:"Name"`
	Config common.AnimClipConfig
	Data   animCfg
}
