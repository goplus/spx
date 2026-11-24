# SPX v4 script API reference

Use these forms in `.spx` scripts. Stage fields and methods are accessible
from sprite scripts. Commands in the sprite sections act on the current
instance. Substitute names from the game's sprite, backdrop, costume,
animation, and sound catalogs.

| To look up | Sections |
| --- | --- |
| Stage and shared calls | [Events and input](#events-and-input), [timing and loops](#timing-loops-and-random-numbers), [backdrops and navigation](#backdrops-effects-and-navigation), [sound](#sound) |
| Sprite calls | [Motion](#sprite-motion), [appearance and animation](#sprite-appearance-and-animation), [sensing and clones](#sprite-sensing-and-clones), [physics](#sprite-physics) |
| Other objects and data | [Camera and monitors](#camera-and-monitors), [lists and values](#scratch-lists-and-values), [Builder equivalents](#adapting-builder-scripts), [Builder AI players](gameplay.md#builder-ai-interactions) |

## Events and input

Register handlers at file scope in `main.spx` or a sprite's script.

| Script call or expression | Behavior |
| --- | --- |
| `onStart => {}` | Run when the game starts. A clone has its own `onCloned` event. |
| `onClick => {}`, `onSwipe Left, => {}` | Handle a click or directional swipe on this sprite. A stage handler receives gestures on the background. Swipe directions are `Left`, `Right`, `Up`, and `Down`. |
| `onKey KeySpace, => {}`, `onKey [KeyLeft, KeyRight], key => {}`, `onAnyKey key => {}` | React to a key press. The latter two forms receive the actual key. |
| `keyPressed(KeyA)`, `keyPressed(KeyAny)` | Poll whether a particular key or any key is held, for example during continuous movement. |
| `mouseX`, `mouseY`, `mousePressed` | Read the pointer's world coordinates and whether the left or right button is held. In a sprite, `touching(Mouse)` tests that sprite against the pointer. |
| `broadcast "hit"`, `broadcast "hit", 10`, `broadcastAndWait "hit", 10` | Send a message to stage and sprite listeners. Both commands accept an optional payload. The waiting form resumes after matching handlers finish. |
| `onMsg "hit", => {}`, `onMsg (name, data) => {}` | The named form runs for one message. To consume a payload, use the second form and check the message name inside it. |
| `onCond keyPressed(KeySpace), => {}` | Fire when a condition becomes true. The runtime skips further checks while this handler runs. |
| `timer`, `resetTimer`, `onTimer 5, => {}`, `wait 0.5` | Read elapsed seconds, reset the shared timer, fire at a threshold, or pause the current script for half a second. Resetting the timer re-arms its thresholds. |
| `ask "Your name?"`, `answer` | Wait for text input, then read the game's most recent answer as a string. A visible sprite shows its question in a speech bubble. |

### Key names

| Input | Constants for `onKey` and `keyPressed` |
| --- | --- |
| Letters, digits, function keys | `KeyA` through `KeyZ`, `Key0` through `Key9`, `KeyF1` through `KeyF12` |
| Navigation | `KeyLeft`, `KeyRight`, `KeyUp`, `KeyDown`, `KeyHome`, `KeyEnd`, `KeyPageUp`, `KeyPageDown` |
| Text and editing | `KeySpace`, `KeyEnter`, `KeyTab`, `KeyEscape`, `KeyBackspace`, `KeyDelete`, `KeyInsert` |
| Modifiers, locks, system keys | `KeyShift`, `KeyControl`, `KeyAlt`, `KeyCapsLock`, `KeyNumLock`, `KeyScrollLock`, `KeyMenu`, `KeyPause`, `KeyPrintScreen` |
| Punctuation | `KeyApostrophe`, `KeyBackslash`, `KeyComma`, `KeyEqual`, `KeyExclam`, `KeyGraveAccent`, `KeyLeftBracket`, `KeyMinus`, `KeyPeriod`, `KeyRightBracket`, `KeySemicolon`, `KeySlash` |
| Numeric keypad | `KeyKP0` through `KeyKP9`, `KeyKPDecimal`, `KeyKPDivide`, `KeyKPEnter`, `KeyKPEqual`, `KeyKPMultiply`, `KeyKPSubtract` |

Use `KeyAny` to poll for any held key. `keyFromString` returns `KeyMax`
for an unrecognized name. [SPX key definitions](https://github.com/goplus/spx/blob/dev/input.go)

## Timing, loops, and random numbers

The loop examples use an integer field named `score`.

| Script call or expression | Behavior |
| --- | --- |
| `waitNextFrame`, `deltaTime` | Wait for the next frame, or read the elapsed frame time in seconds to scale movement. |
| `repeat 3, => { score++ }`, `forever => { println score }` | Run the callback a fixed number of times or keep repeating it. |
| `repeatUntil score >= 10, => { score++ }`, `waitUntil keyPressed(KeySpace)` | Recheck the condition before each iteration. Use the waiting form when the script only needs to resume once the condition is true. |
| `rand(1, 6)`, `rand(0.5, 1.5)` | Integer bounds select a whole number including both endpoints. Floating-point bounds select a continuous value. Both overloads return `float64`. |
| `stop ThisScript`, `exit`, `exit 1` | End the current script, exit the game successfully, or exit with the supplied status code. |

## Backdrops, effects, and navigation

| Script call or expression | Behavior |
| --- | --- |
| `backdropName`, `backdropIndex` | Read the stage's current backdrop name or zero-based index. |
| `setBackdrop "Night"`, `setBackdrop 0`, `setBackdrop Next`, `setBackdrop Prev` | Select a backdrop by name or index, or cycle through the stage's backdrops. |
| `onBackdrop "Night", => {}`, `onBackdrop name => {}`, `setBackdropAndWait Next` | Listen for a named backdrop or receive the selected name on every switch. The waiting setter takes the same selectors and waits for matching handlers to finish. |
| `setGraphicEffect ColorEffect, 20`, `changeGraphicEffect GhostEffect, 10`, `clearGraphicEffects` | Set one effect, add to it, or clear all effects on the stage or the calling sprite. Other effects include `FishEyeEffect`, `WhirlEffect`, `PixelateEffect`, `MosaicEffect`, and `BrightnessEffect`. |
| `findPath(0, 0, 100, 80)` | Query a route between two world positions. The result is a flat `[]float64` of alternating X and Y coordinates at grid-cell centers. Move the sprite along those waypoints in script. A blocked route can be partial. |

`findPath` also accepts a fifth argument to show the debug grid and a sixth to toggle jump-point search. Set these on the first query when configuring the pathfinder. The defaults are debug off and jump-point search on.

## Sound

Both stage and sprite scripts can play sounds from the game's sound catalog.

| Script call or expression | Behavior |
| --- | --- |
| `play "bell"`, `play "bell", true`, `playAndWait "bell"` | Start once, start looping, or wait for one playback to end. Starting the same sound file again replaces its current playback, including playback started by another sprite. |
| `pausePlaying "bell"`, `resumePlaying "bell"`, `stopPlaying "bell"`, `stopAllSounds` | Control the current playback of the named sound across the game, or stop every sound. |
| `volume`, `setVolume 60`, `changeVolume -10` | Read, set, or adjust volume in the range 0–100. The stage and each sprite have their own volume. |
| `setSoundEffect SoundPanEffect, -50`, `changeSoundEffect SoundPitchEffect, 10`, `getSoundEffect(SoundPanEffect)`, `clearSoundEffects` | Set, adjust, read, or reset the stage's or sprite's pan and pitch. Negative pan favors the left channel. |

## Sprite motion

SPX coordinates increase to the right and up. Headings are degrees clockwise from up.
Here `Shark` refers to a named sprite in the project.

| Script call or expression | Behavior |
| --- | --- |
| `xpos`, `ypos` | Read this sprite's coordinates. |
| `setXYpos 20, 80`, `setXpos 20`, `setYpos 80` | Place the sprite at absolute coordinates or set just one axis. |
| `changeXYpos 5, -2`, `changeXpos 5`, `changeYpos -2` | Add offsets to the current position. |
| `heading`, `setHeading Right`, `changeHeading 15` | Read, set, or add to the heading immediately. `Up` is 0°, `Right` 90°, `Down` 180°, and `Left` −90°. |
| `step 40`, `step 40, 2`, `step 40, 2, "walk"` | Travel along the current heading. A negative distance moves backward. The optional arguments are a speed multiplier (default 1) and an animation name. |
| `stepTo Shark`, `stepTo Mouse`, `stepTo 20, 80` | Move to the target's current position or specified coordinates. Each form also accepts a speed and then an optional animation, as in `stepTo Shark, 2` or `stepTo 20, 80, 2, "walk"`. |
| `glide Shark, 1`, `glide Mouse, 1`, `glide 20, 80, 0.5` | Move to the destination over the given number of seconds. The handler resumes when the glide finishes. |
| `turn 90`, `turn 90, 2`, `turn 90, 2, "spin"` | Rotate by the given angle. Speed and animation work as for stepping. |
| `turnTo Shark`, `turnTo Mouse`, `turnTo Right` | Face the sprite, pointer, or absolute heading. Each form also accepts a speed and then an optional animation, as in `turnTo Mouse, 2` or `turnTo Right, 2, "spin"`. |
| `bounceOffEdge` | Reflect the heading when the sprite touches a stage edge. |

Stepping and turning use the sprite's configured `step` and `turn` state animations.
If the relevant animation is absent, the position or heading changes immediately.

## Sprite appearance and animation

| Script call or expression | Behavior |
| --- | --- |
| `show`, `hide`, `visible` | Show or hide the sprite, or read its visibility. |
| `size`, `setSize 0.5`, `changeSize 0.1` | Read, set, or increment the scale factor. A factor of 1 draws the costume at its original size. |
| `setRotationStyle Normal`, `setRotationStyle LeftRight`, `setRotationStyle None` | Rotate the costume with the heading, flip it horizontally, or keep its orientation fixed. |
| `setLayer Front`, `setLayer Back`, `setLayer Forward, 1`, `setLayer Backward, 1` | Send the sprite to either end of the drawing order or move it a number of layers. |
| `costumeName`, `costumeIndex`, `setCostume "idle"`, `setCostume 0`, `setCostume Next`, `setCostume Prev` | Read the current costume or switch by asset name, zero-based index, or adjacent costume. |
| `animate "walk"`, `animate "walk", true`, `animateAndWait "clap"`, `stopAnimation "walk"` | Start once, loop, wait for playback, or stop the currently playing animation if its name matches. |
| `say "Ready"`, `think "Hmm"`, `say "Go!", 1`, `think "Hmm", 1` | Display a speech or thought bubble. A positive duration waits that many seconds and clears that bubble. An empty message clears the bubble immediately. |

## Sprite sensing and clones

| Script call or expression | Behavior |
| --- | --- |
| `distanceTo(Shark)`, `distanceTo(Mouse)` | Measure the straight-line distance between the sprite's position and the target's position. |
| `touching(Shark)`, `touching("Shark")` | Test overlap with that specific instance or with any sprite of that name, including its clones. |
| `touching(Mouse)`, `touching(Edge)`, `touching(EdgeBottom)` | Test the pointer, any stage edge, or a selected edge (`EdgeLeft`, `EdgeRight`, `EdgeTop`, `EdgeBottom`). |
| `HSB(50, 100, 100)`, `HSBA(50, 100, 100, 75)` | Build a color from hue, saturation, and brightness. All components use 0–100, including alpha when supplied. |
| `touchingColor(HSB(50, 100, 100))`, `touchingColor(HSB(0, 100, 100), HSB(50, 100, 100))` | Test contact with a scene color, or whether pixels of the sprite's first color touch the scene's second color. |
| `onTouchStart "Shark", => { hide }`, `onTouchStart ["Shark", "Coin"], other => { say other.name }` | Handle the start of an overlap with a matching sprite. The optional callback argument is the other `Sprite` instance. |
| `clone`, `clone 7`, `onCloned => { show }`, `onCloned data => { say data }` | Copy the current sprite and initialize the new instance. An optional value passes to the clone's handler. |
| `isCloned`, `deleteThisClone`, `destroy`, `die` | Test whether this instance is a clone, remove only a clone, remove the current sprite immediately, or play its configured death animation before removal. |

## Sprite physics

Enable physics in the stage config with `"physics": true` and `"autoSetCollisionLayer": false`. Body colliders determine physical contact. Sprite overlap sensing uses costume pixels by default and trigger shapes when `touchByShape` is enabled.

| Script call or expression | Behavior |
| --- | --- |
| `physicsMode`, `setPhysicsMode DynamicPhysics` | Read or select the body mode. Use `DynamicPhysics` for gravity and impulses, `KinematicPhysics` for script-controlled movement with collisions, `StaticPhysics` for a stationary obstacle, and `NoPhysics` for a sprite without physical response. |
| `setVelocity 100, 0`, `vx, vy := velocity` | Set or read horizontal and vertical velocity in SPX units per second. Positive Y is up. |
| `setGravity 980`, `gravity` | Set or read the body's base gravitational acceleration. On a dynamic body the sprite's configured gravity scale and the stage's global gravity multiplier scale that acceleration. |
| `addImpulse 0, 300`, `isOnFloor` | Give a dynamic body an upward impulse, or test whether a body is grounded. On the next physics tick the impulse adds its value divided by mass to the velocity. |
| `setColliderShape(false, RectCollider, [32, 16])`, `setColliderShape(true, CircleCollider, [24])` | Set a 32-by-16 body rectangle or a trigger with radius 24 in sprite-local units. The call returns an error if the parameters do not fit the shape. |
| `setColliderPivot true, 8, 0`, `kind, params := colliderShape(true)` | Offset the trigger from the sprite origin, or read back its shape type and dimensions. Pass `false` for the body collider. |

## Camera and monitors

| Script call or expression | Behavior |
| --- | --- |
| `Camera.follow Dragon`, `Camera.follow "Dragon"` | Follow a sprite instance or find the original by name. Inside a clone, `Camera.follow this` follows that clone. |
| `Camera.setXYpos 0, 0`, `Camera.changeXYpos 10, 0` | Set the camera center or pan by an offset. Panning releases the follow target. Read the center with `Camera.xpos` and `Camera.ypos`. |
| `Camera.setZoom 2`, `Camera.zoom` | Set or read the zoom factor. Values above 1 zoom in. |
| `x, y, w, h := Camera.viewportRect()` | Read the visible rectangle's origin and size. |
| `m := getWidget(Monitor, "monitor-1")` | Get the monitor declared with this name in the stage's `zorder`. Lookup panics for a missing name or mismatched type. |
| `m.getName`, `m.visible`, `m.show`, `m.hide` | Read a widget's name or visibility and show or hide it. |
| `m.xpos`, `m.ypos`, `m.setXYpos 0, 0`, `m.changeXYpos 10, 10` | Read or change position. Use `m.setXpos`, `m.setYpos`, `m.changeXpos`, or `m.changeYpos` with one coordinate. |
| `m.size`, `m.setSize 2`, `m.changeSize 0.1` | Read, set, or increment widget scale. A scale of 1 is the original size. |
| `showVar "score"`, `hideVar "score"` | Toggle an existing monitor by its bound variable. A sprite checks its own monitor first, then the stage's. |

## Scratch lists and values

Import `github.com/goplus/spx/v4/pkg/scratchcompat` for `scratchcompat.List`.
SPX also exposes `List` as a compatibility alias. Declare a field such as
`var items scratchcompat.List` before using the `items` examples. Use the
[Scratch guide](scratch.md) when translating selectors and comparisons.

| Script call or expression | Behavior |
| --- | --- |
| `source := scratchcompat.NewList("a", 2)`, `items.init "a", "b"` | Create a list or initialize an existing list's contents. |
| `items.initFrom &source` | Copy the source elements into separate list storage. |
| `items.append 3`, `items.insert 0, "coin"`, `items.set 0, "gem"` | Append, insert before a zero-based position, or replace an element. |
| `v := items.at(0)`, `items.len`, `items.string` | Read a `Value`, the item count, or the Scratch-style text of the list. An out-of-range lookup returns the zero `Value`. |
| `items.contains("gem")`, `items.indexOf("gem")` | Test membership or find the first matching index. A missing item gives `-1`. |
| `items.delete 0`, `items.clear` | Remove one item or empty the list. |
| `v := newValue(2)`, `v.set 1` | Create a dynamic `Value` or update it. |
| `v.equal("2")`, `v.int`, `v.float`, `v.string`, `v.raw` | Compare using Scratch coercion, convert for arithmetic or display, or read the underlying value. |

## Adapting Builder scripts

For Builder scripts that use `mouseHitItem`, handle clicks on the sprite
with `onClick`. To poll whether a known sprite overlaps the pointer,
use `touching(Mouse)` in that sprite. [Events and input](#events-and-input)

To replace `MovingInfo` and `TurningInfo` in Builder examples, capture
`oldX, oldY, oldDir := xpos, ypos, heading` before the movement. Afterward,
`dx, dy := xpos-oldX, ypos-oldY` gives the displacement. Use
`dir := heading-oldDir` to match the old `TurningInfo.Dir()` calculation.
A heading change from 170° to −170° gives −340° with that calculation.

For additional overloads, run `go doc github.com/goplus/spx/v4.Sprite`
or `go doc github.com/goplus/spx/v4.Game` in the game's module.
The original names and overload families come from the [Builder API catalog](https://github.com/goplus/builder/blob/d378a09697bf8f63e460db60b4cd14c7dbb5a805/gui/src/utils/spx/skills/spx-project/references/apis.md).
