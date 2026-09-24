# spx APIs and events

Use this reference when the task needs exact event signatures, API overloads,
helpers, constants, or uncommon project-side APIs.

## Contents

- how to use this reference
- event and lifecycle selection guide
- event reference
- shared stage APIs
- sprite API reference
- API selection by intent
- common implementation templates
- `List`, `Value`, colors, helpers
- constants and enums
- extra imports
- known caveats

## How to use this reference

Use this file in three passes:

1. Pick the owner file with `project-layout.md`.
2. Pick the right event or API family here.
3. Only then write the code.

If you are not sure which overload exists, read the exact signature list below before writing code.

## Event and lifecycle selection guide

Choose startup timing like this:

- use top-level statements for tiny file-local setup
- use `onStart` for normal startup behavior and loops
- use `func OnLoaded()` in `main.spx` for post-load setup
- use `onCloned` for clone-local initialization and clone-local loops

Choose input events like this:

- use `onKey` when a specific key or key set should trigger behavior
- use `onAnyKey` when the pressed key value itself is the payload
- use `onClick` when click reaction is enough
- use `onSwipe` for swipe-direction input

Choose communication style like this:

- use direct sprite calls when one known sprite should act immediately
- use `broadcast` when multiple listeners may react
- use `broadcastAndWait` when the sender must continue only after listeners finish
- use `onMsg(func(msg MsgName, data any))` when you need the message name and payload
- use `onMsg "name", => { ... }` when only one message name matters and no payload is needed

Choose collision or sensing style like this:

- use `onTouchStart` for touch-start reactions
- use `touching(...)` inside a loop for continuous polling
- use `distanceTo(...)` for proximity decisions
- use `intersectRect`, `intersectCircle`, or `raycast` for scene queries from stage logic

Choose timing like this:

- use `wait` inside loops for repeated behavior
- use `waitNextFrame` for frame-by-frame logic
- use `onTimer` for one-shot delayed behavior
- use `resetTimer()` when later `onTimer` timing should be relative to a new time base

## Event reference

Register events at top level. These are ordinary XGo calls whose callback is usually written as a lambda.

Events available in both `main.spx` and sprite files:

```xgo
onStart => { ... }
onClick => { ... }
onKey KeySpace, => { ... }
onKey [KeyA, KeyD], => { ... }
onMsg "fire", => { ... }
onBackdrop "level2", => { ... }
onSwipe Left, => { ... }
onTimer 0.5, => { ... }
```

Sprite-only events:

```xgo
onCloned => { ... }
onTouchStart "Bullet", => { ... }
onTouchStart ["Enemy", "Boss"], => { ... }
```

For short one-off handlers, inline lambdas are idiomatic. Use a named function when the callback is reused, takes
parameters you want to spell out clearly, or becomes long:

```xgo
func handleAnyKey(key Key) {
	println key
}

onAnyKey handleAnyKey
```

```xgo
func handleMsg(msg MsgName, data any) {
	println msg, data
}

onMsg handleMsg
```

```xgo
func handleBackdrop(name BackdropName) {
	println name
}

onBackdrop handleBackdrop
```

```xgo
func handleClone(data any) {
	println data
}

onCloned handleClone
clone {"hp": 3}
```

```xgo
func handleTouch(other Sprite) {
	other.Destroy()
}

onTouchStart "Enemy", handleTouch
```

Event signatures:

```xgo
onStart(func())
onClick(func())
onAnyKey(func(key Key))
onKey(key, func())
onKey([]Key, func())
onKey([]Key, func(key Key))
onMsg(msg, func())
onMsg(func(msg MsgName, data any))
onBackdrop(name, func())
onBackdrop(func(name BackdropName))
onSwipe(direction, func())
onTimer(seconds, func())
onCloned(func())
onCloned(func(data any))
onTouchStart(spriteName, func())
onTouchStart(spriteName, func(other Sprite))
onTouchStart([]SpriteName, func())
onTouchStart([]SpriteName, func(other Sprite))
```

Rules:

- register events at top level
- use only the shared event set in `main.spx`
- use `onCloned` and `onTouchStart` only in sprite files
- do not register `onStart` late from inside other callbacks
- multiple handlers for the same event are allowed
- use `onCloned` for clone initialization
- if you need the pressed key value, use `onAnyKey` or `onKey` with a key list callback that accepts `key Key`
- if you need message payload data, use the catch-all `onMsg(func(msg MsgName, data any))` form
- if you need the touched sprite object, use the `func(other Sprite)` form of `onTouchStart`
- `onTimer 0.5, ...` is a one-shot event at 0.5 seconds since the current timer base
- `resetTimer()` resets that base time

Event selection notes:

- `onStart => { ... }` is the normal place for startup loops
- `onClick => { ... }` works well for simple interactive sprites
- `onKey KeySpace, => { ... }` is the direct choice for one known key
- `onKey [KeyA, KeyD], key => { ... }` is the direct choice when several keys share one handler and the key value
  matters
- `onBackdrop "boss", => { ... }` reacts to one named backdrop
- `onBackdrop(func(name BackdropName))` is the catch-all form when the backdrop name matters
- `onCloned(data => { ... })` is the most explicit clone setup when clone payload data matters

Useful patterns:

```xgo
onMsg func(msg MsgName, data any) {
	if msg == "damage" {
		hp -= data.(int)
	}
}
```

```xgo
onKey [KeyLeft, KeyRight], key => {
	if key == KeyLeft {
		changeXpos -5
	} else {
		changeXpos 5
	}
}
```

## Shared stage APIs

These APIs are stage-owned, but normal sprite project code can also call them directly.

### Reload, messaging, stop

```xgo
reload(index)

broadcast(msg)
broadcast(msg, data)

broadcastAndWait(msg)
broadcastAndWait(msg, data)

stop(kind)

exit()
exit(code)
```

Notes:

- `reload(index)` is commonly used with another index file such as `"ch-2.json"`
- `broadcast(msg, data)` pairs with `onMsg func(msg MsgName, data any)`
- `stop(kind)` uses stop-kind constants like `AllStop`, `ThisSprite`, `ThisScript`
- use `broadcastAndWait` when listeners must finish before the current code continues

### Stage input, time, dialog, variables

```xgo
keyPressed(key) bool
mousePressed() bool
mouseX() float64
mouseY() float64

wait(seconds)
waitNextFrame
timer() float64
resetTimer()

ask(msg)
answer() string

showVar(name)
hideVar(name)
```

Notes:

- `ask(msg)` on stage opens the ask UI without a sprite bubble
- `answer()` returns the latest answer string
- `showVar(name)` and `hideVar(name)` target stage monitors by property name

### Backdrop and stage visuals

```xgo
backdropName() string
backdropIndex() int

setBackdrop(name)
setBackdrop(indexFloat)
setBackdrop(indexInt)
setBackdrop(Prev)
setBackdrop(Next)

setBackdropAndWait(name)
setBackdropAndWait(indexFloat)
setBackdropAndWait(indexInt)
setBackdropAndWait(Prev)
setBackdropAndWait(Next)

setGraphicEffect(kind, value)
changeGraphicEffect(kind, delta)
clearGraphicEffects()

setWindowSize(width, height)
eraseAll()
```

Backdrop selection rules:

- use `setBackdrop(name)` when the target backdrop is known by name
- use `setBackdrop(Next)` or `setBackdrop(Prev)` for simple cycling
- use `setBackdropAndWait(...)` when later code must wait for the switch to complete

### Stage sound

```xgo
play(sound)
play(sound, loop)
playAndWait(sound)
pausePlaying(sound)
resumePlaying(sound)
stopPlaying(sound)

volume() float64
setVolume(value)
changeVolume(delta)

getSoundEffect(kind) float64
setSoundEffect(kind, value)
changeSoundEffect(kind, delta)
clearSoundEffects()

stopAllSounds()
loudness() float64
username() string
```

Notes:

- `loudness()` currently returns `0`
- `username()` currently returns `""`

### Camera

```xgo
Camera.ViewportRect() (x, y, w, h)
Camera.SetZoom(scale)
Camera.Zoom() float64
Camera.Xpos() float64
Camera.Ypos() float64
Camera.SetXYpos(x, y)
Camera.ChangeXYpos(dx, dy)
Camera.Follow(sprite)
Camera.Follow(spriteName)
```

Camera selection rules:

- use `Camera.Follow(sprite)` or `Camera.Follow(spriteName)` when one target should drive the camera
- use `Camera.SetXYpos(x, y)` when the camera should jump to a fixed position
- use `Camera.ChangeXYpos(dx, dy)` when input or scripted motion nudges the camera
- use `Camera.SetZoom(scale)` when the task explicitly changes zoom level

### Pathfinding, physics query, debug draw

```xgo
setupPathFinder()
setupPathFinder(xGridSize, yGridSize, xCellSize, yCellSize, withJump, withDebug)

findPath(xFrom, yFrom, xTo, yTo) []float64
findPath(xFrom, yFrom, xTo, yTo, withDebug) []float64
findPath(xFrom, yFrom, xTo, yTo, withDebug, withJump) []float64

intersectRect(x, y, width, height) []Sprite
intersectCircle(x, y, radius) []Sprite

raycast(fromX, fromY, toX, toY) (hit bool, sprite Sprite, hitX float64, hitY float64)
raycast(fromX, fromY, toX, toY, ignoreSprite) (hit bool, sprite Sprite, hitX float64, hitY float64)
raycast(fromX, fromY, toX, toY, ignoreSprites) (hit bool, sprite Sprite, hitX float64, hitY float64)

debugDrawRect(x, y, width, height, color)
debugDrawCircle(x, y, radius, color)
debugDrawLine(fromX, fromY, toX, toY, color)
debugDrawLines([]float64{x1, y1, x2, y2, ...}, color)
```

Notes:

- `findPath` returns a flattened point array like `[x1, y1, x2, y2, ...]`
- `debugDrawLines` also uses a flattened point array
- use `findPath` when the stage or a controller sprite needs waypoint data
- use `raycast` for line-of-sight style checks
- use `intersectRect` or `intersectCircle` for area queries

### Tilemap

```xgo
placeTiles([]float64{x1, y1, x2, y2, ...}, texturePath)
placeTiles([]float64{x1, y1, x2, y2, ...}, texturePath, layerIndex)

placeTile(x, y, texturePath)

eraseTile(x, y)
eraseTile(x, y, layerIndex)

getTile(x, y) string
getTile(x, y, layerIndex) string

loadTilemap(mapDir)
unloadTilemap()
tilemapName() string
```

Tilemap rules:

- use `placeTile` or `placeTiles` when the task edits map tiles at runtime
- use `loadTilemap(mapDir)` when the task explicitly swaps map content
- use `getTile` to inspect current tile state

## Sprite API reference

### Core lifecycle

```xgo
name() string
isCloned() bool

clone
clone(data)

deleteThisClone
destroy
die

deltaTime() float64
timeSinceLevelLoad() float64
```

Notes:

- `deleteThisClone` only affects clones
- `die` plays the `"die"` animation first if the sprite has one, then destroys the sprite
- `clone(data)` pairs with `onCloned func(data any)`
- `timeSinceLevelLoad()` is often useful for spawn patterns or delayed behavior without adding new counters

### Visibility, costume, layer, size

```xgo
show
hide
visible() bool

showVar(name)
hideVar(name)

size() float64
setSize(size)
changeSize(delta)

costumeName() SpriteCostumeName
costumeIndex() int

setCostume(name)
setCostume(indexFloat)
setCostume(indexInt)
setCostume(Prev)
setCostume(Next)

setLayer(Front)
setLayer(Back)
setLayer(Forward, delta)
setLayer(Backward, delta)

setGraphicEffect(kind, value)
changeGraphicEffect(kind, delta)
clearGraphicEffects()
```

Notes:

- in sprite files, `showVar(name)` and `hideVar(name)` first target that sprite's own monitor, then fall back to
  the stage monitor with the same name
- use `show` or `hide` for sprite visibility
- use `setLayer(Front)` or `setLayer(Back)` for coarse ordering
- use `setLayer(Forward, delta)` or `setLayer(Backward, delta)` for relative layer movement

### Motion and transform

```xgo
xpos() float64
ypos() float64
heading() Direction

setXYpos(x, y)
changeXYpos(dx, dy)
setXpos(x)
changeXpos(dx)
setYpos(y)
changeYpos(dy)

move(stepFloat)
move(stepInt)

step(step)
step(step, speed)
step(step, speed, animation)

stepTo(sprite)
stepTo(spriteName)
stepTo(Mouse)
stepTo(x, y)
stepTo(sprite, speed)
stepTo(spriteName, speed)
stepTo(Mouse, speed)
stepTo(x, y, speed)
stepTo(sprite, speed, animation)
stepTo(spriteName, speed, animation)
stepTo(Mouse, speed, animation)
stepTo(x, y, speed, animation)

glide(sprite, secs)
glide(spriteName, secs)
glide(Mouse, secs)
glide(Random, secs)
glide(x, y, secs)

setHeading(dir)
changeHeading(dir)

turn(dir)
turn(dir, speed)
turn(dir, speed, animation)

turnTo(sprite)
turnTo(spriteName)
turnTo(dir)
turnTo(Mouse)
turnTo(sprite, speed)
turnTo(spriteName, speed)
turnTo(dir, speed)
turnTo(Mouse, speed)
turnTo(sprite, speed, animation)
turnTo(spriteName, speed, animation)
turnTo(dir, speed, animation)
turnTo(Mouse, speed, animation)

setRotationStyle(Normal)
setRotationStyle(LeftRight)
setRotationStyle(None)

bounceOffEdge()
```

Notes:

- `move` is a direct move along the current heading
- `step` is the more flexible movement helper with optional speed and animation
- for `stepTo` and `turnTo`, the special target used in project code is normally `Mouse`
- `glide(Random, secs)` is the practical use of the `Pos` overload

Motion selection rules:

- use `move` for simple heading-based motion
- use `step` for repeated movement that may later gain speed or animation arguments
- use `stepTo` to move toward a target position or target object
- use `glide` for time-based interpolation to a destination
- use `turn` to rotate relative to the current heading
- use `turnTo` to face a target direction, sprite, or mouse position
- use `bounceOffEdge()` when the sprite should reflect from stage edges

Typical choices:

```xgo
step 8
step 8, 0.2, "walk"
turnTo Mouse
glide 100, 50, 0.4
```

### Animation

```xgo
animate(name)
animate(name, loop)
animateAndWait(name)
stopAnimation(name)
```

Animation rules:

- use `animate(name)` for fire-and-forget animation
- use `animate(name, true)` for looping animation
- use `animateAndWait(name)` when later code should wait for the animation to finish
- use `stopAnimation(name)` to end one named animation explicitly

### Sensing

```xgo
touching(sprite) bool
touching(spriteName) bool
touching(Mouse) bool
touching(Edge) bool
touching(EdgeTop) bool
touching(EdgeBottom) bool
touching(EdgeLeft) bool
touching(EdgeRight) bool

touchingColor(color) bool

distanceTo(sprite) float64
distanceTo(spriteName) float64
distanceTo(Mouse) float64
distanceTo(Random) float64
```

Sensing rules:

- use `touching(spriteName)` for direct collision-style checks in loops
- use `touching(Mouse)` for pointer overlap
- use `touching(Edge...)` for stage boundary checks
- use `touchingColor(color)` when the task is color-based rather than sprite-based
- use `distanceTo(...)` when the task is proximity-based rather than overlap-based

### Dialog

```xgo
say(msg)
say(msg, secs)

think(msg)
think(msg, secs)

ask(msg)

quote(message)
quote(message, secs)
quote(message, description)
quote(message, description, secs)
```

Notes:

- sprite `ask(msg)` shows the sprite bubble and the ask UI
- `quote("")` clears the quote bubble
- use `say` for speech bubbles
- use `think` for thought bubbles
- use `quote` for quote-style presentation when the task explicitly wants that UI

### Pen

```xgo
penDown
penUp
stamp

setPenColor(color)
setPenColor(kind, value)
changePenColor(kind, delta)

setPenSize(size)
changePenSize(delta)
```

Pen rules:

- use `penDown` before moving if the sprite should draw
- use `stamp` to stamp the current sprite image
- use `setPenColor(color)` when a concrete color value is already known
- use `setPenColor(kind, value)` or `changePenColor(kind, delta)` when adjusting hue, saturation, brightness, or
  transparency components

### Sprite sound

```xgo
play(sound)
play(sound, loop)
playAndWait(sound)
pausePlaying(sound)
resumePlaying(sound)
stopPlaying(sound)

volume() float64
setVolume(value)
changeVolume(delta)

getSoundEffect(kind) float64
setSoundEffect(kind, value)
changeSoundEffect(kind, delta)
```

Sound rules:

- use sprite sound APIs when the sound belongs to that sprite's behavior
- use stage sound APIs when the sound is project-wide or not owned by a specific sprite

### Sprite physics

```xgo
setPhysicsMode(mode)
physicsMode() PhysicsMode

velocity() (velocityX, velocityY)
setVelocity(velocityX, velocityY)

gravity() float64
setGravity(gravity)

addImpulse(impulseX, impulseY)
isOnFloor() bool

setColliderShape(isTrigger, colliderType, params) error
colliderShape(isTrigger) (ColliderShapeType, []float64)

setColliderPivot(isTrigger, offsetX, offsetY)
colliderPivot(isTrigger) (offsetX, offsetY)

setCollisionLayer(layer)
setCollisionMask(mask)
setCollisionEnabled(enabled)
collisionLayer() int64
collisionMask() int64
collisionEnabled() bool

setTriggerEnabled(enabled)
setTriggerLayer(layer)
setTriggerMask(mask)
triggerLayer() int64
triggerMask() int64
triggerEnabled() bool
```

Physics notes:

- `isTrigger == false` means the normal collision body
- `isTrigger == true` means the trigger shape
- collider parameters:
  - `RectCollider`: `[]float64{width, height}`
  - `CircleCollider`: `[]float64{radius}`
  - `CapsuleCollider`: `[]float64{radius, height}`
  - `PolygonCollider`: `[]float64{x1, y1, x2, y2, ...}`
- polygon vertices should be provided in counterclockwise order
- common examples:

```xgo
setColliderShape false, RectCollider, []float64{32, 48}
setColliderShape true, CircleCollider, []float64{16}
setColliderShape false, PolygonCollider, []float64{0, -16, -16, 16, 16, 16}
```

Physics selection rules:

- use `setPhysicsMode(DynamicPhysics)` for physically simulated bodies
- use `setPhysicsMode(KinematicPhysics)` for script-driven motion with collision support
- use `setPhysicsMode(StaticPhysics)` for non-moving colliders
- use `setColliderShape(false, ...)` for the solid collision body
- use `setColliderShape(true, ...)` for a trigger shape
- use collision layer and mask APIs when selective collision routing matters
- use trigger layer and mask APIs when selective trigger routing matters
- use `isOnFloor()` for platforming-style grounded checks

Common physics setup:

```xgo
onStart => {
	setPhysicsMode KinematicPhysics
	setColliderShape false, RectCollider, []float64{32, 48}
}
```

```xgo
onStart => {
	setPhysicsMode DynamicPhysics
	setColliderShape false, CircleCollider, []float64{16}
	setTriggerEnabled true
	setColliderShape true, CircleCollider, []float64{32}
}
```

## API selection by intent

### Movement and facing

- use `changeXpos`, `changeYpos`, or `changeXYpos` for direct axis updates
- use `move` when heading already determines motion
- use `step` for common scripted movement
- use `stepTo` when movement should chase a target
- use `glide` for timed movement
- use `turnTo` when facing matters more than immediate movement

### Communication and coordination

- use direct sprite calls for one known target
- use `broadcast` for one-to-many event fanout
- use `broadcastAndWait` for synchronized multi-listener flows
- use shared state in `main.spx` for project-wide counters and flags

### Sensing and hit detection

- use `onTouchStart` for hit-on-contact events
- use `touching(...)` for continuous overlap polling
- use `distanceTo(...)` for chase or flee thresholds
- use `raycast(...)` for line-of-sight or straight-shot checks
- use `intersectRect` or `intersectCircle` for area scans

### Scene and stage control

- use `setBackdrop` or `setBackdropAndWait` for backdrop transitions
- use `reload(index)` for scene reload or chapter switch tasks
- use `showVar` or `hideVar` for monitor visibility
- use `ask` and `answer` for simple input prompts

### Camera

- use `Camera.Follow(...)` for tracking a sprite
- use `Camera.SetXYpos(...)` for fixed camera jumps
- use `Camera.ChangeXYpos(...)` for input-driven pans
- use `Camera.SetZoom(...)` for zoom changes

### Physics and pathfinding

- use physics mode and collider APIs when the task explicitly involves bodies, grounded checks, or triggers
- use `findPath(...)` when the task asks for route computation
- use `setupPathFinder(...)` before `findPath(...)` when the project has not already done so

## Common implementation templates

### Player fires bullet clones

Emitter sprite:

```xgo
var cooldown float64

onStart => {
	for {
		waitNextFrame
		if cooldown > 0 {
			cooldown -= deltaTime()
		}
	}
}

onKey KeySpace, => {
	if cooldown <= 0 {
		Bullet.clone
		cooldown = 0.15
	}
}
```

Bullet sprite:

```xgo
onCloned => {
	setXYpos Player.xpos(), Player.ypos()
	turnTo Player.heading()
	show
	for {
		waitNextFrame
		step 12
		if touching(Edge) {
			destroy
		}
	}
}
```

### Enemy hp and bullet hit

```xgo
var hp int

onCloned => {
	hp = 3
	show
}

onTouchStart "Bullet", => {
	hp--
	if hp <= 0 {
		die
	}
}
```

### Broadcast with payload data

Sender:

```xgo
broadcast "damage", 2
```

Receiver:

```xgo
onMsg func(msg MsgName, data any) {
	if msg == "damage" {
		hp -= data.(int)
	}
}
```

### Repeating timed behavior

Use a loop plus `wait`:

```xgo
onStart => {
	for {
		wait 0.5
		clone
	}
}
```

Use `onTimer` only for one-shot delay:

```xgo
onTimer 2.0, => {
	broadcast "phase-2"
}
```

### Reload stage or switch chapter

```xgo
func gameOver() {
	reload "game-over.json"
}
```

```xgo
onMsg "restart", => {
	reload "index.json"
}
```

### Camera follow

```xgo
func OnLoaded() {
	Camera.Follow "Player"
	Camera.SetZoom(1.2)
}
```

### Simple kinematic collider

```xgo
onStart => {
	setPhysicsMode KinematicPhysics
	setColliderShape false, RectCollider, []float64{32, 48}
}
```

### Trigger area around a sprite

```xgo
onStart => {
	setPhysicsMode KinematicPhysics
	setColliderShape false, RectCollider, []float64{32, 48}
	setColliderShape true, CircleCollider, []float64{80}
	setTriggerEnabled true
}
```

## `List`, `Value`, colors, helpers

### `Value`

```xgo
v := NewValue(123)

v.Equal(other) bool
v.String() string
v.Int() int
v.Float() float64
v.Set(value)
```

### `List`

```xgo
l := NewList(1, 2, 3)

l.Init(values...)
l.InitFrom(&other)
l.Len() int
l.String() string
l.Contains(value) bool
l.Append(value)
l.Set(index, value)
l.Insert(index, value)
l.Delete(index)
l.At(index) Value
l.IndexOf(value) Pos
l.Clear()
```

Index helpers:

- `Invalid`
- `Last`
- `All`
- `Random`

Notes:

- `l.Delete(All)` clears the whole list
- `l.At(i)` returns `Value`
- `Last` and `Random` are especially useful with list APIs

### Colors and numeric helpers

```xgo
HSB(h, s, b) Color
HSBA(h, s, b, a) Color

rand(intFrom, intTo) float64
rand(floatFrom, floatTo) float64

iround(v) int
KeyFromString(name) Key
```

Notes:

- integer `rand` is inclusive on both ends
- `HSB` and `HSBA` use Scratch-like `0..100` ranges

### Loop helpers

```xgo
repeat(loopCount, func())
repeatUntil(condition, func())
waitUntil(condition)
forever(func())
```

Advanced scheduler helpers:

```xgo
Sched()
SchedNow()
```

`Sched` and `SchedNow` are advanced helpers. Normal project code usually does not need them.

## Constants and enums

### Keys

- `Key0` through `Key9`
- `KeyA` through `KeyZ`
- arrows and common controls such as `KeyUp`, `KeyDown`, `KeyLeft`, `KeyRight`, `KeySpace`, `KeyEnter`,
  `KeyEscape`, `KeyShift`, `KeyControl`, `KeyAlt`
- function keys such as `KeyF1` through `KeyF12`
- `KeyAny`
- `KeyMax`

### Stage and sprite targeting

- `Mouse`
- `Edge`
- `EdgeTop`
- `EdgeBottom`
- `EdgeLeft`
- `EdgeRight`
- `Up`
- `Down`
- `Left`
- `Right`

### Costume and layer helpers

- `Prev`
- `Next`
- `Front`
- `Back`
- `Forward`
- `Backward`

### Stop kinds

- `AllStop`
- `AllOtherScripts`
- `AllSprites`
- `ThisSprite`
- `ThisScript`
- `OtherScriptsInSprite`

### Graphics

- `Normal`
- `LeftRight`
- `None`

- `ColorEffect`
- `FishEyeEffect`
- `WhirlEffect`
- `PixelateEffect`
- `MosaicEffect`
- `BrightnessEffect`
- `GhostEffect`

### Pen and sound enums

- `PenHue`
- `PenSaturation`
- `PenBrightness`
- `PenTransparency`

- `SoundPanEffect`
- `SoundPitchEffect`

### Physics enums

- `NoPhysics`
- `KinematicPhysics`
- `DynamicPhysics`
- `StaticPhysics`

- `RectCollider`
- `CircleCollider`
- `CapsuleCollider`
- `PolygonCollider`

## Extra imports

Most gameplay code should use normal `.spx` project code directly and needs no framework import.

Only add imports when you need normal Go packages like:

- `fmt`
- `math`
- `strings`

If a task explicitly needs advanced coroutine control, it can import `github.com/goplus/spx/v2/pkg/spx` for helpers
such as:

- `Go`
- `Execute`
- `ExecuteNative`
- `Wait`
- `WaitNextFrame`
- `IsInCoroutine`
- `IsAbortThreadError`

Do not import the main spx framework package just to access normal gameplay APIs. Those are already available in normal
`.spx` project code.

## Known caveats

- `SetDebug` and debug flags such as `DbgFlagLoad`, `DbgFlagInstr`, `DbgFlagEvent`, `DbgFlagPerf`, and `DbgFlagAll`
  exist for runtime debugging. Use them only when a task explicitly needs debug instrumentation.
- `GetWidget` and `Widget` exist for UI/widget lookup, but the current repository does not provide a stable project-side
  example of their normal `.spx` spelling. If a task explicitly needs widgets, inspect that project's existing code
  first instead of guessing the calling pattern.
- `loudness()` currently returns `0`.
- `username()` currently returns `""`.
- `onStart` should be registered at top level. Late registration is ignored.
- `onTimer` is one-shot, not a repeating timer.
- sprite `showVar(name)` and `hideVar(name)` first target the sprite monitor, then fall back to the stage monitor.
- keep using normal project events and helpers. Do not switch to engine lifecycle callbacks for ordinary project tasks.
