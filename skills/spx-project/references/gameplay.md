# Common SPX gameplay calls

## Input and movement

In a sprite file, this handler turns the sprite toward the pointer and moves
it 32 units along that heading when the player presses Space.

```spx
onKey KeySpace, => {
    turnTo Mouse
    step 32
}
```

- `turn 90` rotates relative to the current heading. `turnTo Right` selects
  an absolute heading. `step` and `turnTo` play the sprite's configured
  `step` and `turn` animations when available. Without a matching animation,
  the movement or turn is immediate. Use `setHeading Right` and
  `setXYpos 0, 0` to set the initial heading and spawn position directly.
- For continuous steering, read `keyPressed(KeyRight)` in a loop that calls
  `waitNextFrame`. Scale the displacement by elapsed seconds, as in
  `changeXpos 150 * deltaTime`. [Steer and score](project.md#steer-and-score)
- Use `onClick => { setCostume Next }` on a sprite to change its appearance
  when clicked. To keep the camera centered on that sprite as it moves, call
  `Camera.follow this` from its startup handler.

## Scene changes and playback

Use names from the game's asset indexes. Costumes and animations belong to
the sprite, sounds come from the game's sound catalog, and backdrops belong
to the stage.

| When the next action should happen | Script call |
| --- | --- |
| As soon as a sound starts | `play "chomp"` |
| After the sound finishes | `playAndWait "chomp"` |
| After a sprite's animation finishes | `animateAndWait "clap"` |
| After switching the stage and finishing its matching `onBackdrop` handlers | `setBackdropAndWait Next` |

For instance, the Dragon tutorial plays `"chomp"` to completion before
moving the sprite back. [Dragon.spx](https://github.com/goplus/spx/blob/dev/tutorial/02-Dragon/Dragon.spx)

## Physics and contact

To make a sprite jump under gravity, set both `"physics": true` and
`"autoSetCollisionLayer": false` in the stage config. Set
`"physicsMode": "dynamic"` in the jumping sprite's config and
`"physicsMode": "static"` in the floor sprite's config. Give both sprites
body colliders and set each mask to include the other sprite's layer. For
example, use `"collisionLayer": 1` and `"collisionMask": 1` on each.

```spx
onKey KeySpace, => {
    if isOnFloor {
        addImpulse 0, 300
    }
}
```

An impulse changes a dynamic body's velocity by impulse divided by mass on
the next physics tick. Positive Y points up. Use `isOnFloor` for grounded
state and `onTouchStart` for the start of an overlap, such as collecting a
coin. [Sprite sensing](scratch.md#gameplay)

## Builder AI interactions

For an XBuilder Web game that uses `ai.Player`, the AI-enabled interpreter
provides the `ai` package and the host provides the interaction service.
In a guide sprite, register a command that displays a hint. Its exported
fields define the arguments the AI can supply.

```spx
type GiveHint struct {
    Text string
}

var guide ai.Player

guide.setRole "Trail guide"
guide.onCmd GiveHint, cmd => {
    say cmd.Text
    return ai.Break
}
guide.onErr err => { println err }

onClick => {
    guide.think "Give me a hint", {"place": "forest"}
}
```

`think` accepts a message and optional game context, and waits for the
interaction to finish. A command can return `nil` to allow another turn,
`ai.Break` to end the interaction, or an error to tell the AI why the
command failed. `onErr` handles errors that end the interaction.
[Builder AI interaction guide](https://github.com/goplus/builder/blob/dev/gui/src/utils/spx/skills/spx-project/references/ai-interaction.md) · [SPX Web interpreter profiles](https://github.com/goplus/spx/blob/dev/cmd/ispx/README.md)
