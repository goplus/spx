# SPX skills

[spx-project](spx-project/SKILL.md) helps write XGo scripts and configure
assets for SPX v4 games. Codex and Claude Code can use it directly in this
checkout. To use it in another game, copy the complete `skills/spx-project`
folder into one of the directories below.

| Agent | Relative to the game root | All local projects | Invoke |
| --- | --- | --- | --- |
| Codex | `.agents/skills/` | `~/.agents/skills/` | `$spx-project` |
| Claude Code | `.claude/skills/` | `~/.claude/skills/` | `/spx-project` |

Other agents that support the same format can use the folder in their own
skills directory. [Agent Skills specification](https://agentskills.io/specification)
