package skeleton

type EBoneNames int

const (
	hip EBoneNames = iota
	abdomen
	chest
	neck
	head
	shoulder_l
	arm_l
	hand_l
	thigh_l
	calf_l
	foot_l
	toe_l
	shoulder_r
	arm_r
	hand_r
	thigh_r
	calf_r
	foot_r
	toe_r
	lastBone
)

func (e EBoneNames) String() string {
	switch e {
	case hip:
		return "hip"
	case abdomen:
		return "abdomen"
	case chest:
		return "chest"
	case neck:
		return "neck"
	case head:
		return "head"
	case shoulder_l:
		return "shoulder_l"
	case arm_l:
		return "arm_l"
	case hand_l:
		return "hand_l"
	case thigh_l:
		return "thigh_l"
	case calf_l:
		return "calf_l"
	case foot_l:
		return "foot_l"
	case toe_l:
		return "toe_l"
	case shoulder_r:
		return "shoulder_r"
	case arm_r:
		return "arm_r"
	case hand_r:
		return "hand_r"
	case thigh_r:
		return "thigh_r"
	case calf_r:
		return "calf_r"
	case foot_r:
		return "foot_r"
	case toe_r:
		return "toe_r"
	default:
		return "unknown"
	}
}
