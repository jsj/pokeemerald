---
name: pokectx
description: Use Pokéctx for read-only Pokémon Emerald location, team, bag, progress, next-task, connection, and save questions.
---

# Pokéctx

Use Pokéctx when the user invokes `$pokectx`.
Also use Pokéctx for questions about the connected Pokémon Emerald game.

Resolve `../../scripts/pokectx` from this file.
Run one Pokéctx command in each shell call.
The command writes JSON to standard output.

## Command routing

| User request | Command |
|---|---|
| Connection or save status | `doctor` |
| General game summary | `doctor`, then `status` |
| Current location | `doctor`, then `location` |
| Current Pokémon team | `doctor`, then `team` |
| Bag, badges, and key items | `doctor`, then `bag` |
| Story milestones | `doctor`, then `progress` |
| Next task | `doctor`, then `next` |
| Complete game state | `doctor`, then `snapshot` |
| Live changes | `doctor`, then `watch` |
| Inspect an exported save | `<command> --save /path/to/game.sav` |

Run `doctor` before the first command that needs live state.
Skip `doctor` after a successful result in the same conversation.
Run the smallest set of commands that answers the request.

## Source and failures

- State the answer first.
- Include the observation time and source after the answer.
- Report stale, partial, and unavailable state as a limit.
- If a command fails, report the failure.
- Do not replace missing game state with model memory or a walkthrough.
- Treat all commands as read-only.
