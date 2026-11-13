# SPX skills

[`spx-project`](spx-project/SKILL.md) helps author game projects in `.spx`
files. It covers project structure, events, assets, and Scratch-style behavior,
not engine implementation.

## Install

In Codex, ask the skill installer:

```text
Install the skill at https://github.com/goplus/spx/tree/dev/skills/spx-project
```

Or copy `skills/spx-project` from a checkout into
`${CODEX_HOME:-$HOME/.codex}/skills/spx-project`. Keep its `references/` and
`agents/` directories together. Check for an existing installation before
replacing it. Other clients can use the same folder if they support `SKILL.md`.

Invoke it with, for example:

```text
Use $spx-project to add clone-based enemies to this game.
```

## Version and verification

This skill targets the SPX v4 project model on `dev`. Install from a tag or
commit that contains the skill when you need a reproducible version. It does
not install SPX or upgrade a game's dependencies. Follow the skill's version
check before applying it to an older project.

The skill links to existing tutorials instead of duplicating their code and
assets. From the repository root, check that those projects compile with:

```sh
go test ./pkg/ispx -run '^TestProjectTutorials$'
```

For runtime setup and visual checks, follow the
[repository README](../README.md) and the tutorial instructions in the skill.
