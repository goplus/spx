# project layout for spx

Use this reference when the task is about project structure, file roles, state placement, startup timing, clone
patterns, or asset config layout in a spx project.

## Contents

- project tree
- decision order
- core coding model
- where code should go
- startup timing
- `main.spx`
- sprite files
- shared state vs sprite state
- cross-sprite access vs messaging
- naming and calling style
- adding a new sprite
- clone patterns
- resource config patterns
- common task routing
- common mistakes

## Project tree

Typical shape:

```text
main.spx
Player.spx
Enemy.spx
assets/
  index.json
  sprites/
    Player/
      index.json
    Enemy/
      index.json
  sounds/
    Hit/
      index.json
```

Rules:

- `main.spx` is the stage or game script
- every other `.spx` file is a sprite script
- the filename is the sprite name
- `assets/index.json` is the usual project config file
- sprite config usually lives under `assets/sprites/<SpriteName>/index.json`
- sound config usually lives under `assets/sounds/<SoundName>/index.json`
- some existing projects use `res/` instead of `assets/`. Keep whichever root the repo already uses
- the project starts automatically

## Decision order

When implementing a request, decide in this order:

1. Which file owns the requested behavior.
2. Whether the new state is shared state or per-sprite state.
3. Whether the behavior should run from top-level entry code, `onStart`, `OnLoaded`, or another event.
4. Whether the change requires `.spx` code only or also asset config changes.
5. Whether direct sprite access or broadcast messaging is the simpler design.

## Core coding model

- `main.spx` owns shared state and stage-level logic
- each sprite `.spx` owns that sprite's per-instance state and behavior
- the file state must be declared in the field declaration block, which must appear before top-level helper `func`
  declarations
- top-level `func` declarations are methods of the current file object
- top-level executable statements are the file's setup body
- sprite code can directly access:
  - shared state declared in `main.spx`
  - stage APIs like `broadcast`, `wait`, `mouseX`, and `Camera`
  - other sprites by name, such as `Bullet` or `Enemy`

## Where code should go

Put code in `main.spx` for:

- score, level, round, and other shared variables
- stage-wide input and stage-wide event coordination
- backdrop changes, reload flow, and scene-wide messaging
- helper functions used by more than one sprite through shared state or stage behavior
- optional `func OnLoaded()` post-load setup

Put code in a sprite file for:

- local hp, speed, cooldown, facing, and animation state
- movement, aiming, and local input handling
- collisions, hit reactions, and local death behavior
- clone behavior and clone-local loops
- sprite-specific helper functions

## Startup timing

Use top-level statements when:

- a small amount of file setup reads more clearly as entry code
- the setup belongs to the file itself rather than an event reaction

Use `onStart` when:

- the behavior should start with the project or sprite lifecycle
- the code launches a loop, repeated action, or ongoing behavior

Use `func OnLoaded()` in `main.spx` when:

- the project needs a post-load hook
- the setup should happen after load rather than being part of a sprite loop

Use `onCloned` when:

- clone-only state must be initialized
- the clone needs to start its own loop or behavior immediately after cloning

Practical rules:

- top-level statements in `main.spx` still run as part of the project entry
- `onStart` still runs normally after startup
- keep gameplay loops in `onStart` or `onCloned`, not in `OnLoaded`
- use `OnLoaded` for post-load setup, not for long-running movement or collision loops

## `main.spx`

Use `main.spx` for:

- shared variables
- stage-level events
- startup logic
- scene reload logic
- optional project-wide helper functions
- optional `func OnLoaded()` hook for post-load setup

Rules:

- do not add manual `Player Player` or `Enemy Enemy` declarations just to access sprites
- refer to sprite files by name directly when needed
- keep `main.spx` focused on stage concerns and shared state
- use `main.spx` when multiple sprites should react through shared state or stage messages

Example:

```xgo
var (
	score int
)

onKey KeySpace, => {
	broadcast "fire"
}

func OnLoaded() {
	println "loaded"
}
```

## Sprite files

Use sprite `.spx` files for:

- per-sprite state
- input handling for that sprite
- movement and animation
- collisions and interactions
- clone behavior
- helper functions specific to that sprite
- optional local state machines such as `alive`, `cooldown`, `phase`, or `targetX`

Example:

```xgo
var (
	hp int
)

func resetHP() {
	hp = 3
}

onStart => {
	resetHP()
}

onTouchStart "Bullet", => {
	hp--
	if hp <= 0 {
		die
	}
}
```

## Shared state vs sprite state

Variables in `main.spx` are shared project state.

Use `main.spx` state for:

- score
- level
- shared counters
- global flags
- stage-wide mode switches

Variables in a sprite file are that sprite's own fields.

Use sprite state for:

- hp
- local cooldown
- local movement state
- current target
- clone lifetime

Cloned sprites copy sprite fields, so clone-only state must be reset in `onCloned`.

Good clone-safe pattern:

```xgo
var (
	life int
)

onStart => {
	for {
		wait 0.3
		clone
	}
}

onCloned => {
	life = 3
	show
}
```

## Cross-sprite access vs messaging

Use direct sprite access when:

- the target sprite is known
- the interaction is local and immediate
- direct reads or method calls are clearer than message passing

Example:

```xgo
Bullet.clone
Red.SetXYpos(0, 0)
if touching("Enemy") {
	destroy
}
```

Use broadcast messaging when:

- multiple listeners should react
- stage logic should coordinate multiple sprites
- the sender should not know concrete listeners
- the message is a gameplay event such as `"fire"`, `"hit"`, `"game-over"`, or `"next-wave"`

Rule of thumb:

- use direct access for one known sprite
- use `broadcast` for one-to-many coordination
- use `broadcastAndWait` when the sender must wait until listeners finish their work

## Naming and calling style

- prefer the lowercase project-code spellings already used in `.spx` files:
  - `setXYpos`
  - `broadcastAndWait`
  - `animateAndWait`
  - `changeSoundEffect`
- exported Go-style names also exist behind the scenes, but normal `.spx` project code should prefer the lowercase
  spellings
- some existing repos mix lowercase spellings and Go-style spellings. Follow the current file's style unless the task
  asks for cleanup
- no-argument calls are often written without parentheses when used as statements:

```xgo
waitNextFrame
show
hide
destroy
clone
```

- use ordinary call syntax when you need a returned value:

```xgo
xpos()
ypos()
mouseX()
mouseY()
timer()
answer()
```

- if the shorthand form is unclear, explicit method calls are fine:

```xgo
this.SetXYpos(10, 20)
Camera.SetZoom(1.5)
Enemy.SetXYpos(0, 0)
```

## Adding a new sprite

When a task requires a new sprite:

1. Create `<SpriteName>.spx`.
2. Put the sprite's state in the field declaration block, before any top-level helper `func`.
3. Add top-level event handlers and helper functions.
4. Add or update `assets/sprites/<SpriteName>/index.json`.
5. Keep the sprite filename and sprite config directory name aligned.
6. Do not add manual sprite field declarations in `main.spx` just to make the new sprite visible.

## Clone patterns

Use clones for:

- bullets
- particles
- repeated enemies or pickups
- short-lived effects with mostly identical behavior

Rules:

- initialize clone-only state in `onCloned`
- call `show` in `onCloned` if the clone should become visible immediately
- do not assume clones start with clean runtime state
- call `destroy` or `deleteThisClone` when the clone is done
- keep emitter logic in the owner sprite and movement or lifetime logic in the cloned sprite

Bullet-style pattern:

```xgo
onStart => {
	for {
		wait 0.1
		Bullet.clone
	}
}
```

```xgo
onCloned => {
	setXYpos Player.xpos, Player.ypos
	show
	for {
		waitNextFrame
		step 10
		if touching(Edge) {
			destroy
		}
	}
}
```

## Resource config patterns

Project config commonly contains:

- `map`
- `backdrops`
- `zorder`
- optional `camera`

Sprite config commonly contains:

- `costumes` or `costumeSet`
- `fAnimations`, `mAnimations`, `tAnimations`
- `heading`
- `size`
- `visible`
- `x`
- `y`
- `rotationStyle`

Stage monitor example:

```json
{
  "type": "monitor",
  "target": "",
  "val": "getVar:score",
  "label": "score",
  "visible": true
}
```

Practical config rules:

- add a stage monitor when shared state such as score should be visible on screen
- add or update sprite config when a task changes costume, size, heading, visibility, or animation metadata
- keep asset paths and directory names aligned with the project's existing layout
- if a repo already uses a custom config shape, extend that shape consistently instead of replacing it with a new style

Camera follow config example:

```json
{
  "camera": {
    "on": "Player"
  }
}
```

## Common task routing

Use this table of thumb when choosing files:

- add score or level counter: `main.spx` and maybe `assets/index.json`
- add player movement or aim: player sprite file
- add player firing cooldown: player sprite file
- add bullet motion and lifetime: bullet sprite file
- add enemy hp and death: enemy sprite file
- add stage restart on game over: usually `main.spx`
- add backdrop change on phase switch: usually `main.spx`
- add camera follow target in config: `assets/index.json`
- add exact collision or trigger shape: the sprite's config or sprite file, depending on the request

## Common mistakes

- putting shared state into one sprite file and then trying to use it as project state
- putting clone-local initialization in `onStart` instead of `onCloned`
- registering events inside other handlers instead of at top level
- moving the first `var` block below top-level event code
- adding manual sprite field declarations in `main.spx`
- choosing `broadcast` when a direct sprite call would be simpler
- choosing direct sprite access when the request really describes one-to-many coordination
- putting shared score or level state inside a sprite file
- putting per-enemy hp in `main.spx`
- forgetting to reset clone-only state in `onCloned`
- writing `var` blocks after top-level event statements
- mixing `assets/` and `res/` in the same project without a task-driven reason
- editing asset config paths without keeping filenames and sprite names aligned
