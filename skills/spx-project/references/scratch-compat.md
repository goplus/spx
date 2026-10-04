# Scratch compatibility for spx project authoring

Use this reference when a request is phrased in Scratch blocks or other Scratch terms.

This file is a translation aid. It helps map Scratch concepts to current `.spx` project code. It does not replace
`xgo-syntax.md`, `project-layout.md`, or `spx-api.md`.

You can think of spx project authoring as a Scratch-style DSL layer on top of XGo classfiles:

- Scratch contributes the stage, sprite, message, clone, costume, backdrop, and monitor mental model
- spx contributes the exact project-side APIs and enums
- XGo classfiles contribute the actual source form, top-level rules, and type system

## How to use this reference

1. Translate the Scratch concept to the nearest current spx authoring shape here.
2. Use `project-layout.md` to decide which `.spx` file owns the behavior.
3. Use `spx-api.md` to confirm the exact event signature, overload, enum, or helper spelling.
4. Use `xgo-syntax.md` to write the final code in valid `.spx` form.

## Non-negotiable translation rules

- Write current `.spx` spellings such as `onStart`, `onMsg`, `setXYpos`, `broadcastAndWait`, and `touchingColor`.
- Do not emit raw Scratch block labels or `Type::Method` style names in handwritten project code.
- Treat Scratch scripts as top-level event registrations plus helper `func` declarations in the owning `.spx` file.
- Treat Scratch custom blocks as normal top-level helper `func` declarations in the owning file.
- Treat Scratch control blocks as ordinary XGo control flow or spx loop helpers, not as a separate block DSL.
- Keep the field declaration block rules from `xgo-syntax.md`. Shared or per-sprite state still belongs in the first
  qualifying top-level `var` declaration.
- Use real XGo types when the intended type is clear. Do not keep everything as dynamically typed Scratch-style data.
- Prefer current project-side spellings over older alternate names.
- Older names such as `lookLike:`, `startScene`, and `whenSceneStarts` should be mentally translated to current
  authoring spellings such as `setCostume`, `setBackdrop`, and `onBackdrop`.

## Mental model

- Scratch stage maps to `main.spx`
- each Scratch sprite maps to one non-`main.spx` file
- stage-wide variables and stage-wide coordination usually belong in `main.spx`
- sprite-local variables and behavior usually belong in that sprite file
- broadcast-based coordination still maps well to stage-plus-sprites project structure
- Scratch monitors still often involve `assets/index.json`, plus `showVar(name)` and `hideVar(name)` when visibility is
  changed at runtime
- a Scratch script stack usually becomes one top-level event registration plus helper functions in the owning file

## Event mapping

- `when green flag clicked` maps to top-level `onStart => { ... }`
- `when this sprite clicked` maps to `onClick => { ... }`
- `when [key] key pressed` maps to `onKey KeySpace, => { ... }` or another `Key...` constant
- `when I receive [message]` maps to `onMsg "message", => { ... }`
- `when backdrop switches to [name]` maps to `onBackdrop "name", => { ... }`
- `when I start as a clone` maps to `onCloned => { ... }`
- `when [sensor] > [value]` has no normal direct spx event. Model it with polling, timers, or another existing API
  instead of inventing a fake event.

Event rules:

- register event handlers at top level
- use `onCloned` only in sprite files
- use a helper `func` when several handlers share the same logic
- if a Scratch task suggests one event family but the current repo needs another, prefer the current spx API over the
  literal Scratch phrasing

## Broadcasts and control flow

- `broadcast [msg]` maps to `broadcast "msg"`
- `broadcast [msg] and wait` maps to `broadcastAndWait "msg"`
- `wait (secs)` maps to `wait secs`
- `wait until <condition>` maps to `waitUntil(condition)` or a normal polling loop
- `forever` usually maps to a normal `for { ... }` loop started from `onStart` or `onCloned`
- `repeat (n)` maps to `repeat(n, func())` or a normal `for` loop
- `repeat until <condition>` maps to `repeatUntil(condition, func())` or a normal loop
- `if` and `if else` map to normal XGo `if` and `if else`
- `stop [all]` maps to `stop(All)`
- `stop [this script]` maps to `stop(ThisScript)`
- `stop [other scripts in sprite]` maps to `stop(OtherScriptsInSprite)`
- `run without screen refresh` or `warp` has no normal handwritten `.spx` equivalent

Control-flow rules:

- for simple gameplay loops, normal XGo `for` is usually clearer than forcing everything through loop helpers
- for small Scratch-style rewrites, `repeat`, `repeatUntil`, `waitUntil`, and `forever` can preserve the intent well
- `broadcastAndWait` is the direct equivalent when the sender must block until listeners finish

## Motion and targeting

- `move (10) steps` usually maps to `step 10`
- clockwise and counterclockwise turn blocks usually map to `turn angle` with the appropriate sign
- `point in direction (90)` maps to `setHeading 90`
- `point towards [sprite or mouse-pointer]` maps to `turnTo "Sprite"` or `turnTo Mouse`
- `go to x: (x) y: (y)` maps to `setXYpos x, y`
- `change x by` and `change y by` map to `changeXpos dx` and `changeYpos dy`
- `set x to` and `set y to` map to `setXpos x` and `setYpos y`
- `go to [mouse-pointer]` maps to `stepTo Mouse`
- `go to [random position]` usually becomes explicit random coordinates such as
  `setXYpos rand(minX, maxX), rand(minY, maxY)` in authored project code
- `glide (secs) to x: (x) y: (y)` maps to `glide x, y, secs`
- `glide (secs) to [mouse-pointer]` maps to `glide Mouse, secs`
- `if on edge, bounce` maps to `bounceOffEdge`
- `set rotation style [left-right]` maps to `setRotationStyle(LeftRight)`
- `set rotation style [don't rotate]` maps to `setRotationStyle(None)`
- `set rotation style [all around]` maps to `setRotationStyle(Normal)`

Targeting and motion notes:

- in current project code, common special targets include `Mouse`, `Edge`, and `Random` where the API family supports
  them
- `step` is the normal project-side spelling that most directly matches the common Scratch "move steps" intent
- `turnTo` means face a target or direction immediately. `setHeading` is the direct setter for heading
- keep the local project's calling style. If the repo uses command-style calls such as `setXYpos x, y`, follow that

## Looks, costume, backdrop, layer, and size

- `show` maps to `show`
- `hide` maps to `hide`
- `say [msg]` maps to `say msg`
- `say [msg] for (2) seconds` maps to `say msg, 2`
- `think [msg]` maps to `think msg`
- `think [msg] for (2) seconds` maps to `think msg, 2`
- `switch costume to [name]` maps to `setCostume "name"`
- `next costume` maps to `setCostume Next`
- `previous costume` maps to `setCostume Prev`
- costume reporters map to `costumeIndex()` and `costumeName()`
- `switch backdrop to [name]` maps to `setBackdrop "name"`
- `switch backdrop to [name] and wait` maps to `setBackdropAndWait "name"`
- `next backdrop` maps to `setBackdrop Next`
- `previous backdrop` maps to `setBackdrop Prev`
- backdrop reporters map to `backdropIndex()` and `backdropName()`
- `go to front layer` maps to `setLayer(Front)`
- `go to back layer` maps to `setLayer(Back)`
- `go forward (n) layers` maps to `setLayer(Forward, n)`
- `go backward (n) layers` maps to `setLayer(Backward, n)`
- `change [graphic effect] by` maps to `changeGraphicEffect(kind, delta)`
- `set [graphic effect] to` maps to `setGraphicEffect(kind, value)`
- `clear graphic effects` maps to `clearGraphicEffects()`
- `change size by` maps to `changeSize delta`
- `set size to (100)%` maps to `setSize 100`
- `size` maps to `size()`

Looks and size notes:

- in handwritten current `.spx` authoring, size uses the same percentage-like values that current spx project code
  already uses. Do not rewrite `100` to `1` in normal project code
- use current effect enums such as `ColorEffect`, `BrightnessEffect`, `GhostEffect`, `FishEyeEffect`, `WhirlEffect`,
  `PixelateEffect`, and `MosaicEffect`
- `hide all sprites` and `say nothing` do not have normal one-to-one project-side APIs. Re-express the behavior with
  current spx features instead of inventing a helper

## Sound, ask, and answer

- `start sound [name]` maps to `play "name"`
- `play sound [name] until done` maps to `playAndWait "name"`
- `stop all sounds` maps to `stopAllSounds()`
- `volume` maps to `volume()`
- `set volume to` maps to `setVolume value`
- `change volume by` maps to `changeVolume delta`
- `set [sound effect] to` maps to `setSoundEffect(kind, value)`
- `change [sound effect] by` maps to `changeSoundEffect(kind, delta)`
- `clear sound effects` maps to `clearSoundEffects()`
- `ask [question] and wait` maps to `ask "question"`
- `answer` maps to `answer`

Sound notes:

- current sound-effect enums are `SoundPanEffect` and `SoundPitchEffect`
- do not assume Scratch music-extension blocks such as tempo, drum, or instrument exist in normal project authoring
- use sprite sound APIs when the sound belongs to that sprite, and stage sound APIs when the sound is project-wide

## Sensing, input, and property reporters

- `mouse x` and `mouse y` map to `mouseX()` and `mouseY()`
- `mouse down?` maps to `mousePressed()`
- `key [space] pressed?` maps to `keyPressed(KeySpace)`
- a dynamic key name string maps to `keyPressed(KeyFromString(name))`
- `touching [sprite or edge or mouse-pointer]?` maps to `touching("Sprite")`, `touching(Edge)`, or `touching(Mouse)`
- `touching color [color]?` maps to `touchingColor(color)`
- `distance to [sprite or mouse-pointer]` maps to `distanceTo("Sprite")` or `distanceTo(Mouse)`
- `timer` maps to `timer()`
- `reset timer` maps to `resetTimer()`
- `username` maps to `username()`
- reporters such as `x position`, `y position`, `direction`, `costume #`, `costume name`, `backdrop #`, and
  `backdrop name` usually map to current spx getters such as `xpos()`, `ypos()`, `heading()`, `costumeIndex()`,
  `costumeName()`, `backdropIndex()`, and `backdropName()`

Reporter and sensing rules:

- if the target sprite is known, prefer direct access in the local project style, for example `Player.xpos()` or the
  equivalent shorthand already used by the repo
- if a Scratch task is phrased as "of [target]" property access, translate the property name to current spx spellings
  before writing code
- use `onKey` when the task is event-driven and `keyPressed(...)` when the task is polling inside a loop

Sensing notes:

- current `touching` behavior follows Scratch-like alpha-aware overlap semantics
- for color-based sensing, use a real `Color` value. `HSB` and `HSBA` use Scratch-like `0..100` ranges
- when a Scratch block gives a color number or color picker value, translate it to a `Color` before calling
  `touchingColor`, pen APIs, or other color-based helpers

## Operators, text, and math

- arithmetic operators such as `+`, `-`, `*`, `/`, `>`, `<`, `=`, `and`, `or`, and `not` usually map directly to
  normal XGo operators
- `pick random (from) to (to)` maps to `rand(from, to)`
- `round (value)` maps to `iround(value)`
- `join [a] [b]` usually maps to `a + b`
- `length of [text]` maps to `len(text)`
- `letter (n) of [text]` usually maps to `text[n-1]`
- `(a) mod (b)` maps to `mod(a, b)`

Operator notes:

- Scratch positions are 1-based. XGo indexing is 0-based. Subtract `1` when translating letter or list-position blocks
- most Scratch operator blocks become ordinary expressions, not API calls
- if a task depends on Scratch-specific math semantics rather than simple arithmetic, prefer explicit code over assumed
  implicit conversions
- if a task depends on degree-based Scratch trig behavior, spell that conversion out explicitly in `.spx` code

## Colors, pen, and effects

- Scratch-style color thinking usually maps best to `HSB(h, s, b)` or `HSBA(h, s, b, a)` in project code
- `set pen color to [color]` maps to `setPenColor(color)`
- `set pen [hue|saturation|brightness|transparency] to` maps to `setPenColor(kind, value)`
- `change pen [hue|saturation|brightness|transparency] by` maps to `changePenColor(kind, delta)`
- `set pen size to` maps to `setPenSize size`
- `change pen size by` maps to `changePenSize delta`
- `pen down` maps to `penDown`
- `pen up` maps to `penUp`
- `stamp` maps to `stamp`

Pen notes:

- current pen component enums are `PenHue`, `PenSaturation`, `PenBrightness`, and `PenTransparency`
- prefer the current pen API documented by the repo over older spellings such as `setPenHue` or `changePenShade`
- if the task is about a named graphical effect rather than pen state, use `setGraphicEffect` or
  `changeGraphicEffect`, not pen APIs

## Variables, lists, monitors, and indexing

- `set [var] to` usually maps to normal assignment in the owning `.spx` file
- `change [var] by` usually maps to `+=`
- Scratch stage variables usually become shared state in `main.spx`
- Scratch sprite-only variables usually become sprite-local state in that sprite file
- `show variable [name]` and `hide variable [name]` map to `showVar(name)` and `hideVar(name)`
- Scratch list state usually maps to a `List`
- `add [x] to [list]` maps to `myList.Append(x)`
- `delete (n) of [list]` maps to `myList.Delete(n-1)`
- `delete all of [list]` maps to `myList.Delete(All)` or `myList.Clear()`
- `insert [x] at (n) of [list]` maps to `myList.Insert(n-1, x)`
- `replace item (n) of [list] with [x]` maps to `myList.Set(n-1, x)`
- `item (n) of [list]` maps to `myList.At(n-1)`
- `length of [list]` maps to `myList.Len()`
- `[list] contains [x]?` maps to `myList.Contains(x)`

Data and indexing notes:

- Scratch positions are 1-based. XGo indexing and current list APIs are 0-based. Translate positions carefully
- `List.At(i)` returns `Value`, not an untyped Scratch cell. Convert or compare it intentionally
- persistent monitor configuration still usually belongs in `assets/index.json`
- use current XGo types for scalar state instead of emulating Scratch's implicit dynamic conversions everywhere

## Custom blocks and helpers

- Scratch custom block definitions usually map to top-level helper `func` declarations in the owning `.spx` file
- Scratch custom block calls map to ordinary function calls
- if several Scratch hats or scripts share the same steps, pull that logic into a helper `func`
- do not add explicit receivers for normal `.spx` helper functions

## Unsupported or not-one-to-one areas

Treat these as redesign points, not as an invitation to guess a fake matching API:

- `when [sensor] > [value]`
- `color [a] is touching [b]`
- drag mode blocks
- most video sensing blocks
- many music-extension blocks such as tempo, drum, and instrument
- `hide all sprites`
- `say nothing`
- generic property reporters when the target is not known and no current documented spx helper covers the need
- any Scratch extension block that is not already represented in the current spx docs

When you hit one of these:

- restate the behavior in terms of current spx APIs
- use normal XGo code when that is the clearest rewrite
- consult current repo docs or tests instead of assuming a one-to-one translation exists

## Translation examples

Scratch intent:

- when green flag clicked
- forever
- move 5 steps
- if on edge, bounce

Normal `.spx` shape:

```xgo
onStart => {
	for {
		step 5
		bounceOffEdge
		waitNextFrame
	}
}
```

Scratch intent:

- when I receive `fire`
- create clone of `Bullet`
- when I start as a clone, move from the player and destroy at the edge

Normal `.spx` shape:

```xgo
onMsg "fire", => {
	Bullet.clone
}

onCloned => {
	setXYpos Player.xpos(), Player.ypos()
	show
	for {
		step 12
		if touching(Edge) {
			destroy
		}
		waitNextFrame
	}
}
```

Scratch intent:

- if key `(costume name)` pressed

Normal `.spx` shape:

```xgo
if keyPressed(KeyFromString(costumeName())) {
	say "pressed"
}
```

Scratch intent:

- set item 2 of `scores` to 50

Normal `.spx` shape:

```xgo
scores.Set(2-1, 50)
```
