# xgo syntax for spx

Use this reference when the task is about XGo syntax inside `.spx` files, especially lambdas, command-style calls,
top-level file rules, and classfile-oriented source structure.

## Contents

- mental model
- top-level source structure
- import placement
- field declaration block
- top-level functions
- top-level statements and entry body
- XGo lambdas and closures
- command-style calls
- practical guidance

## Mental model

- a `.spx` file is still an ordinary XGo source file
- syntax questions should be answered with XGo rules, not by inventing spx-only syntax
- classfile behavior changes how top-level declarations and top-level statements are interpreted, but it does not
  replace the underlying XGo language

## Top-level source structure

Treat a `.spx` file as:

```text
[package]
imports
declarations
top-level statements
```

Practical consequences:

- `import` declarations must stay before all non-import declarations and top-level statements
- the first top-level statement starts the file entry body
- after the first top-level statement, the rest of the file is parsed like ordinary function-body statements
- do not place later `var`, `const`, or `type` declarations after top-level statements unless you intentionally want
  them to be local declarations inside the entry body

## Import placement

Keep imports in normal XGo position:

- `package`, if present
- all `import` declarations
- top-level declarations such as the file state block and helper functions
- top-level executable statements

Good:

```xgo
import "math"

var speed float64

func resetSpeed() {
	speed = 3
}

onStart => {
	speed = math.Max(speed, 3)
}
```

Bad:

```xgo
onStart => {
	wait 1
}

import "math"
```

## Field declaration block

For normal project authoring, the file state is the field declaration block.

The field declaration block is not defined as "whatever top-level `var` appears first in the file".

It is the first top-level `var` declaration that satisfies all of these conditions:

- it appears in the file declaration list, before top-level statements begin
- every preceding top-level declaration, if any, is only an `import`, `const`, or `type` declaration
- it is the first top-level `var` declaration that satisfies those conditions

Practical consequences:

- put the file state `var` block before helper functions and before event registrations
- do not put a top-level `func` before the file state `var` block
- in `main.spx`, that field declaration block becomes shared project state
- in a sprite file, that field declaration block becomes per-sprite state
- any other top-level `var` declaration is not part of the file state
- avoid additional top-level `var` declarations before top-level statements unless you really need package-level values

Good:

```xgo
var (
	score int
	level int
)

func resetGame() {
	score = 0
	level = 1
}

onStart => {
	resetGame()
}
```

This is valid because the `var` declaration appears before any top-level `func`.

Bad:

```xgo
onStart => {
	score = 0
}

var (
	score int
)
```

Bad:

```xgo
func resetGame() {
	score = 0
}

var (
	score int
)
```

In that pattern, the `var` declaration is no longer the field declaration block because a top-level `func` appears
before it. That `var` follows ordinary variable-declaration rules instead of becoming class fields.

Risky:

```xgo
var (
	score int
)

var (
	globalDebug bool
)
```

That second top-level `var` is no longer part of the file state block. In normal project code, avoid patterns that make
state ownership ambiguous.

## Top-level functions

Top-level `func` declarations without explicit receivers are methods of the current file object.

Practical consequences:

- helper functions in `main.spx` can directly use shared state
- helper functions in a sprite file can directly use that sprite's state and methods
- helper functions should usually be declared after the file state `var` block, because putting `func` first changes how
  a later `var` declaration is classified
- `func init()` inside `.spx` is not a package initialization hook
- `func main()` inside `.spx` is not a package entrypoint
- explicit receiver methods are possible in XGo, but they are not the normal style for spx project code

Use plain top-level functions:

```xgo
func resetHP() {
	hp = 3
}
```

Do not introduce receiver boilerplate unless a task explicitly needs it.

## Top-level statements and entry body

Top-level statements become the file entry body.

Practical consequences:

- top-level statements are real executable code
- in `main.spx`, they run as part of the project entry
- in sprite files, they run as part of that sprite's file entry
- prefer helper functions plus event handlers for most gameplay logic
- use top-level statements for small setup code only when that form is clearer than `onStart`

Good small setup:

```xgo
hide

onCloned => {
	show
}
```

Less clear for ongoing behavior:

```xgo
for {
	wait 0.1
	step 10
}
```

Prefer `onStart` or `onCloned` for loops like that.

## XGo lambdas and closures

XGo function literals use `=>`. They are ordinary lambdas with type inference and closure capture.

Common forms:

```xgo
=> doSomething()
x => x * 2
(x, y) => x + y
x => {
	return x * factor
}
```

Rules:

- parameter types are inferred from context
- return types are inferred from context
- lambda parameters do not use explicit type annotations
- lambdas capture surrounding variables like ordinary closures

Closure example:

```xgo
var speed float64

func startMover(multiplier float64) {
	onStart => {
		speed = 2 * multiplier
	}
}
```

In `.spx` files, event handlers are commonly written as lambdas:

```xgo
onStart => {
	resetScore()
}

onMsg "fire", => {
	clone
}

onTimer 0.5, => {
	broadcast "tick"
}
```

Use a named function instead of an inline lambda when:

- the handler is reused
- the callback takes parameters and you want the signature to be explicit
- the body is long enough that inline form hurts readability

## Command-style calls

XGo supports command-style calls without parentheses. spx project code uses this style heavily.

Common examples:

```xgo
wait 1
broadcast "fire"
setXYpos mouseX, mouseY
turnTo Mouse
```

Prefer explicit call syntax when:

- you need a return value
- the expression appears inside a larger expression
- the shorthand form is unclear

Examples:

```xgo
x := mouseX()
y := mouseY()
if touching(Mouse) {
	destroy
}
Camera.SetZoom(1.5)
```

## Practical guidance

- keep imports first
- place the file state block before helper functions and before event registrations
- keep long callbacks in named helper functions
- do not invent new syntactic sugar
- if a user request is fundamentally about language syntax, answer it with XGo rules first and only then apply
  spx-specific file semantics
- remember that event registration forms like `onStart => { ... }` are still ordinary XGo calls plus a lambda
- prefer the simplest ordinary XGo form that matches the current project style
- do not summarize the field declaration rule as merely "the first `var` block". The declarations before that `var`
  matter
