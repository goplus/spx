package vertex

import (
	"github.com/goplus/spx/internal/anim/common"
)

type AnimClip struct {
	common.AnimClip
	Data animCfg
}

type AnimMesh struct {
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
