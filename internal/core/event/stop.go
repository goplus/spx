package event

type StopKind int

const (
	AllStop              StopKind = -3
	AllOtherScripts      StopKind = -100
	AllSprites           StopKind = -101
	ThisSprite           StopKind = -102
	ThisScript           StopKind = -103
	OtherScriptsInSprite StopKind = -104
)

type StopFilter func(obj any, isCurrent bool) bool

func ResolveStop(kind StopKind, owner any, isSprite func(any) bool, isGame func(any) bool) (StopFilter, bool) {
	switch kind {
	case AllSprites:
		return func(obj any, isCurrent bool) bool {
			return isSprite(obj)
		}, false
	case ThisSprite:
		return func(obj any, isCurrent bool) bool {
			return obj == owner
		}, false
	case OtherScriptsInSprite:
		return func(obj any, isCurrent bool) bool {
			return obj == owner && !isCurrent
		}, false
	case AllOtherScripts:
		return func(obj any, isCurrent bool) bool {
			return !isCurrent && (isSprite(obj) || isGame(obj))
		}, false
	case AllStop:
		return func(obj any, isCurrent bool) bool {
			return isSprite(obj) || isGame(obj)
		}, true
	case ThisScript:
		return nil, true
	default:
		return nil, false
	}
}
