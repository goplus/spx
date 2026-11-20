# Projects, assets, and examples

## Start a game

Create a game in an empty directory with `spx init --path ./my-game`. Keep
its own `go.mod` and the `//xgo:class` marker on its SPX requirement. To use
a local SPX checkout, point the module's `replace` directive at it.

Tutorials inherit the repository's Go module. To start from one, copy its
scripts and assets from the matching SPX version into the scaffold.

- Alternate dialogue with broadcasts. [Weather](https://github.com/goplus/spx/tree/dev/tutorial/01-Weather)
- Update a score monitor when sprites touch. [Dragon](https://github.com/goplus/spx/tree/dev/tutorial/02-Dragon)
- Give each clone an ID so an undo message removes the newest one. [Clone](https://github.com/goplus/spx/tree/dev/tutorial/03-Clone)

## Connect sprites and assets

Match each sprite's name across its script, assets, and the stage's `zorder`.
For example, the Clone tutorial uses `Calf.spx`, `assets/sprites/Calf/`,
and the stage entry `"Calf"`.

Check `assets/index_pack.json` before editing a loose `index.json`. Packed
stage fields override matching fields. Each packed sprite or sound entry
replaces that child's whole index. The packed `fonts` field, when present,
defines the entire catalog.

Resolve file paths from the owning config's directory, using `/` and exact
filename case. Both `assets/sprites/Hero/index.json` and the packed
`sprites.Hero` entry can refer to `assets/shared/hero.png` as
`../../shared/hero.png`. Keep assets inside the resource root and font faces
inside their family's directory. [Project asset guide](https://github.com/goplus/spx/blob/dev/docs/en/dev/project_assets.md)

## Run and verify

From the SPX checkout, `make setup` installs the CLI and host runtime.
`make setup-web MODE=normal` prepares assets for standard Web preview.

```sh
spx runnative --path ./my-game  # Build and run on desktop
spx runweb --path ./my-game     # Preview on the Web
```

Exercise the changed interaction in the target runtime to check asset loading and behavior.
To compile the three reference tutorials without launching Godot, run
`go test ./pkg/ispx -run '^TestProjectTutorials$'` from the checkout root.

## Steer and score

Start from a copy of `tutorial/02-Dragon`. Replace `Dragon.spx` with the
script below. Hold Left or Right to move Dragon horizontally. Touching Shark
earns one point.

```spx
var (
    score int
)

onStart => {
    setXYpos -140, Shark.ypos
    for {
        waitNextFrame
        dx := 150 * deltaTime
        if keyPressed(KeyRight) { changeXpos dx }
        if keyPressed(KeyLeft) { changeXpos -dx }
    }
}

onTouchStart "Shark", => {
    score++
    play "chomp"
}
```

The existing stage monitor reads `"val": "getVar:score"` from
`"target": "Dragon"`, so declare `score` in `Dragon.spx`. The sound name
`"chomp"` comes from the tutorial's sound catalog.

Run the copy and steer into Shark. Stop while the costumes overlap and check
that the score stays at one. Move clear, then touch Shark again for a second
point. `onTouchStart` fires on each new contact. Incrementing the score inside
`if touching("Shark")` in the movement loop would award a point every frame
the costumes overlap.
