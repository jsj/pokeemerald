# Pokéctx Agent Plugin PRD

## Problem Statement

A player can forget their location, completed tasks, and next task in Pokémon Emerald. A long walkthrough does not know the current game state.

A save-file reader only sees the last in-game save. It cannot report live changes unless the player saves the game again.

The player runs Pokémon Emerald on a Mac. The player wants to ask an existing agent about the game from the same Mac or a laptop.

The answers must use observed game state. The agent must not guess the location, inventory, party, or story progress.

## Solution

Build Pokéctx as a portable Agent Plugin. The plugin contains one `$pokectx` skill and one read-only Go command.

The skill maps user questions to named Pokéctx commands. Each command writes JSON to standard output.

The Go command reads live state from an emulator adapter. It can read a local adapter or a configured Mac through SSH.

Pokéctx uses symbols and data from the `pokeemerald` decompilation. It converts raw values into a stable game snapshot.

A reviewed progression model finds completed milestones and the next main-story task. This model also gives the evidence for each result.

The plugin does not contain an AI model. The current agent reads the JSON and answers the player.

The first release reports location, party, key items, badges, story progress, and the next task. It does not require a new in-game save.

A later save-sync module can move a normal Emerald save between the Mac and a phone. This module uses versioned save objects and prevents concurrent play.

## User Stories

1. As a player, I want to see my current map name, so that I know where I am in Hoenn.
2. As a player, I want location results to update while the game is running, so that I do not have to save before asking a question.
3. As a player, I want to see my map coordinates, so that detailed tools can locate me within a map.
4. As a player, I want indoor locations to include their parent city or route when known, so that names such as a building floor have useful geographic context.
5. As a player, I want to see my party species and levels, so that I can quickly recall my current team.
6. As a player, I want to see current and maximum HP for each party Pokémon, so that I know whether the team needs healing.
7. As a player, I want to see the moves of each party Pokémon, so that later tools can use my actual team.
8. As a player, I want to see my badges, so that the bridge can accurately characterize my progression.
9. As a player, I want to see my key items, so that recommendations respect traversal and story prerequisites.
10. As a player, I want to see relevant HMs and traversal capabilities, so that I am not directed through an inaccessible route.
11. As a player, I want to see my current money and selected inventory summaries, so that clients can answer resource questions from facts.
12. As a player, I want Pokéctx to identify completed main-story milestones, so that I can understand how far I have progressed.
13. As a player, I want Pokéctx to recommend the most likely next main-story objective, so that I can resume a game after time away.
14. As a player, I want each recommendation to name its destination, so that I know where to go.
15. As a player, I want each recommendation to explain which observed facts selected it, so that I can trust or challenge the result.
16. As a player, I want recommendations to distinguish required objectives from optional activities, so that side content is not presented as blocking progress.
17. As a player, I want Pokéctx to report when more than one next objective is valid, so that nonlinear sections are not reduced to a false certainty.
18. As a player, I want Pokéctx to report uncertainty explicitly, so that an incomplete progression model does not produce confident misinformation.
19. As a player, I want to request a concise status summary, so that I can orient myself with one command.
20. As a player, I want to request only my location, so that simple questions return minimal output.
21. As a player, I want to request only the next objective, so that I can resume play quickly.
22. As a player, I want to request a complete JSON snapshot, so that other programs and assistants can consume game context.
23. As a player, I want stable field names and a schema version in JSON output, so that clients can detect compatibility changes.
24. As a player, I want machine-readable errors, so that remote clients can distinguish a stopped emulator from malformed game state.
25. As a player, I want a live event stream, so that I can observe map changes, party changes, obtained items, badges, and story progress.
26. As a player, I want duplicate polling results suppressed in the event stream, so that consumers receive meaningful changes rather than noise.
27. As a player, I want events to include timestamps and the resulting semantic values, so that a client can display a useful activity history.
28. As a player, I want the bridge to reconnect after the emulator restarts when possible, so that ordinary play interruptions do not require rebuilding configuration.
29. As a player, I want a clear diagnostic when no supported emulator session is available, so that I know why live context cannot be read.
30. As a player, I want the bridge to reject an unsupported ROM revision, so that incorrect memory layouts do not silently corrupt answers.
31. As a player, I want Pokéctx to identify Pokémon Emerald, so that it does not interpret unrelated memory as game state.
32. As a player, I want to run Pokéctx entirely on the gaming Mac, so that live emulator access stays local.
33. As a player, I want to invoke every read command through SSH, so that I can ask questions from my laptop while playing elsewhere.
34. As a player, I want the standard SSH transport to be sufficient, so that I do not have to expose or secure a custom network port.
35. As a player, I want remote commands to finish quickly and non-interactively, so that they work as building blocks for scripts and assistants.
36. As a player, I want the live reader to be read-only, so that asking a question cannot alter my game, memory, or save data.
37. As a player, I want the objective engine to operate without an internet connection, so that guidance remains available locally.
38. As a player, I want factual game data to come from the decompilation where possible, so that names and identifiers match the running ROM.
39. As a player, I want progression rules to be reviewed and deterministic, so that the same state always produces the same recommendation.
40. As a contributor, I want catalogs to preserve decompilation names, so that I can trace runtime values to source definitions.
41. As a contributor, I want progression rules stored as data rather than scattered control flow, so that the main story can be expanded and reviewed incrementally.
42. As a contributor, I want a progression rule to declare its prerequisites, completion conditions, destination, priority, and explanation, so that behavior is inspectable.
43. As a contributor, I want fixture snapshots for representative story stages, so that objective changes can be tested without launching an emulator.
44. As a contributor, I want runtime adapters hidden behind a small interface, so that a different emulator or instrumented ROM can be added later.
45. As a contributor, I want the normalized snapshot independent of raw memory addresses, so that downstream clients do not change with emulator integration details.
46. As a contributor, I want unknown identifiers preserved numerically and labeled as unknown, so that partial catalog coverage remains diagnosable.
47. As a contributor, I want catalog generation to fail on duplicate or inconsistent identifiers, so that ambiguous data does not reach users.
48. As a contributor, I want recordings of adapter output to be replayable, so that parsing and event behavior can be reproduced in tests.
49. As an assistant developer, I want to consume a compact, deterministic briefing derived from a snapshot, so that a language model receives relevant context without parsing raw game internals.
50. As an assistant developer, I want the raw snapshot alongside the briefing, so that answers can cite exact party, inventory, flag, and location facts.
51. As an assistant developer, I want confidence and evidence attached to objective recommendations, so that the assistant can communicate uncertainty honestly.
52. As a future UI developer, I want the same bridge contract available to a native Mac or iOS client, so that presentation work does not become coupled to emulator memory.
53. As an agent user, I want to invoke `$pokectx`, so that I can ask about the game without a separate application.
54. As an agent user, I want the skill to select named commands, so that the agent uses the smallest required read.
55. As an agent user, I want the plugin to run `doctor` before its first live read, so that connection errors are clear.
56. As a player, I want `doctor` to show the configured target, so that I know which Mac Pokéctx reads.
57. As a player, I want `doctor` to show the emulator connection, so that I know whether mGBA is available.
58. As a player, I want `doctor` to show the detected game and ROM revision, so that I know the decoder is correct.
59. As a player, I want `doctor` to show the time of the last live read, so that I can judge its freshness.
60. As a player, I want `doctor` to show the last save that Pokéctx observed, so that I know the save status.
61. As a player, I want the last-save result to distinguish observation time from game play time, so that timestamps are not misleading.
62. As a player, I want `doctor` to show the source of the save evidence, so that I can distinguish memory data from file data.
63. As a player, I want `doctor` to give one correction for each failure, so that I can restore the connection.
64. As a laptop user, I want one connection setting for my gaming Mac, so that every plugin command can use SSH.
65. As a laptop user, I want local and SSH reads to return the same JSON shape, so that the skill does not need two workflows.
66. As a maintainer, I want the plugin package to contain its manifest, skill, command, and tests, so that installation is portable.
67. As a player, I want to upload a normal Emerald save, so that I can continue the game on another device.
68. As a player, I want to download the newest valid save, so that I can continue from the last completed session.
69. As a player, I want each save version retained for recovery, so that I can restore an earlier valid save.
70. As a player, I want Pokéctx to reject a damaged save, so that cloud sync does not replace a valid copy.
71. As a player, I want Pokéctx to show the device and time for each save version, so that I can select the correct copy.
72. As a player, I want Pokéctx to compare save counters before an upload, so that it can find an older save.
73. As a player, I want Pokéctx to prevent two active play sessions, so that two devices do not replace each other's progress.
74. As a player, I want a clear conflict result, so that Pokéctx never merges two binary saves.
75. As a phone user, I want an import and export workflow, so that my emulator can exchange its sandboxed save.
76. As a phone user, I want Pokéctx to state when an emulator cannot share its save, so that the platform limit is clear.
77. As a player, I want `saves doctor` to show cloud access and the newest save, so that I know whether sync can work.
78. As a player, I want local save commands to work without cloud storage, so that cloud setup is optional.

## Implementation Decisions

- Pokéctx will use the portable Agent Plugin shape proven by Vibetrade.
- The plugin package will contain the manifest, the `$pokectx` skill, the Go command, fixtures, and plugin tests.
- The skill will contain an explicit routing table from user requests to named commands.
- The skill will run one Pokéctx command in each shell call.
- The skill will run `doctor` before the first command that needs live game state in a conversation.
- The skill will skip a second `doctor` call after a successful result in the same conversation.
- The Go command will write JSON to standard output. It will write logs and diagnostics to standard error.
- The Go command will use named read-only commands. It will not expose a generic memory-read command to the agent.
- The initial command set will include `doctor`, `status`, `location`, `party`, `inventory`, `progress`, `next`, `snapshot`, and `watch`.
- `doctor` will make sure that each part of the read path works.
- `doctor` will report the plugin version, command version, target mode, SSH target, adapter state, emulator state, game identity, and ROM revision.
- `doctor` will report the last live-read time, snapshot age, and last observed save.
- The last-save record will include the observation time, in-game play time, save counter, evidence source, and save-file modification time when available.
- Pokémon Emerald does not store a reliable wall-clock save time. Pokéctx will label its observation time as `last_save_seen_at`.
- The core bridge and command-line interface will use Go. The release will contain one executable for each supported Mac architecture.
- Pokéctx will stay independent from the ROM build. The first release will not change the matching Emerald ROM.
- The first live integration will use a small mGBA adapter. A Lua script or supported debug interface can provide this adapter.
- The adapter will read and transport raw state. It will not contain progression rules.
- The runtime adapter is a deep module with a small contract. It identifies the game, reports health, and returns one coherent raw observation.
- The adapter will contain all emulator-specific memory addresses and callbacks.
- All runtime access is read-only. Pokéctx will not write emulator memory, manipulate controls, create save states, or mutate the `.sav` file.
- Live emulator memory is the primary source. A live query does not require a normal in-game save.
- The adapter will observe save changes for `doctor`. A later save-file adapter can provide offline snapshots.
- The normalized snapshot is a deep, versioned domain model. It does not expose mGBA details or raw addresses.
- The snapshot includes the player, location, party, inventory, badges, event flags, vars, source, and freshness.
- A snapshot represents one observation. The adapter will reject state that combines values from different frames.
- Pokéctx will generate symbolic game data from the decompilation where practical.
- The catalogs will include maps, map groups, flags, vars, items, species, and moves.
- The catalog module maps numeric values to canonical names and user labels. It preserves unknown numeric values for diagnostics.
- Generated data must be reproducible. Generation will reject duplicate identifiers, unresolved aliases, and invalid relationships.
- The progression engine is a deep, pure module. It accepts a snapshot and a versioned rule set.
- The progression result includes milestones, available tasks, blocked tasks, a primary task, alternatives, evidence, and confidence.
- Progression rules will use declarative data. Each rule defines its prerequisites, completion state, destination, priority, type, and evidence.
- The initial progression model will cover the main Emerald story through the Champion milestone.
- Optional rules will exist only when their absence causes incorrect main-story advice.
- Results will be deterministic. A language model will not select tasks or replace observed state inside Pokéctx.
- The engine will preserve all valid tasks. The primary task will not hide alternatives.
- If no rule matches, the engine will return an unknown or low-confidence result.
- The event module compares snapshots. It reports map, party, level, item, badge, save, and story changes.
- Event output will use newline-delimited JSON and include schema version, event type, timestamp, relevant before/after values, and the observation identifier.
- JSON is the stable plugin contract. The agent converts JSON into a short answer for the player.
- Commands will use separate exit statuses for each failure class.
- The classes include emulator, ROM, adapter, observation, connection, and internal failures.
- JSON results will include structured failure data.
- The default target mode is local. An SSH target can run the same command on the gaming Mac.
- Pokéctx will not open a public network port. SSH will provide remote transport and authentication.
- The connection setting will contain the target mode, SSH host, remote command path, and adapter selection.
- The bridge will make sure that the ROM revision is supported before it decodes live state.
- Bun, TypeScript, Swift, and an MCP server are not runtime requirements.
- The main modules are the plugin skill, CLI, target transport, runtime adapter, snapshot, catalog, progression engine, event module, and output module.
- Save sync is a separate module. Live context does not depend on save sync.
- The first save-sync provider will use an R2 bucket through its S3-compatible API.
- R2 fits the Go command and supports clients outside the Apple platform.
- An R2 access token will not appear in an agent prompt or command argument.
- The Mac will store its scoped credential in Keychain. A phone client will use short-lived signed requests from a small broker.
- A personal iCloud Drive adapter can be an experiment when both emulators support file import and export.
- CloudKit is not the first provider. It requires Apple entitlements and an application container that the plugin does not own.
- CloudKit becomes a valid provider if Pokéctx later includes a native Apple application or emulator integration.
- The cloud object will contain an unchanged `.sav` file and a separate manifest.
- The manifest will include the game identity, ROM revision, save counter, play time, checksum, size, source device, and upload time.
- Every upload will create an immutable version. Pokéctx will update a small pointer to the newest accepted version.
- Pokéctx will make sure that all Emerald save sectors and checksums are valid before an upload or restore.
- Pokéctx will not merge save files. Emerald save data is binary state and has no safe field-level merge.
- A session lease will mark one device as the active writer. A second device must stop or take over the expired lease.
- A client will download the selected save before play. It will upload only after a completed in-game save and a safe emulator pause or stop.
- The save commands will include `saves doctor`, `saves list`, `saves pull`, `saves push`, and `saves restore`.
- A pull will preserve the current local save as a recovery version before it replaces the emulator copy.
- The iPhone emulator must support Files, Shortcuts, a document picker, shared storage, or a Pokéctx integration.
- The plugin cannot read or write the private container of an unrelated iPhone emulator.

## Testing Decisions

- Good tests will measure behavior through stable module contracts. They will not depend on private functions or internal data structures.
- Tests will cover snapshots, recommendations, events, output schemas, failures, and CLI behavior.
- Snapshot tests will use valid, incomplete, unknown, and inconsistent observations.
- The catalog generator and resolver will be tested against representative decompilation constants and map metadata, including duplicate detection and unknown identifier behavior.
- Progression tests will use table-driven fixture snapshots.
- Fixtures will cover story milestones, boundary states, nonlinear choices, contradictory inputs, and states without a match.
- Every initial main-story objective will have at least one fixture where it is recommended and one where its completion condition prevents it from being recommended.
- Objective tests will measure the identifier, type, destination, evidence, alternatives, and confidence.
- Tests will measure exact prose only when prose is part of the output contract.
- The event differ will be tested using recorded snapshot sequences for map transitions, party changes, level gains, item changes, badges, story flags, reconnects, and unchanged polling cycles.
- The runtime adapter will be tested primarily with recorded or fake emulator observations so that decoding failures are reproducible without a live emulator.
- A small mGBA suite will measure attachment, ROM identity, live reads, map transitions, absence, and restart behavior.
- CLI contract tests will execute the built command and assert standard output, standard error, exit status, JSON schema version, and non-interactive operation suitable for SSH.
- Plugin tests will make sure that the manifest is valid and the skill routes requests to the correct named commands.
- `doctor` tests will cover local success, SSH success, stale state, no emulator, unsupported ROM, adapter failure, and missing save evidence.
- Last-save tests will distinguish `last_save_seen_at`, in-game play time, save counter, and save-file modification time.
- SSH tests will make sure that local and remote targets return the same JSON contract.
- Save-format tests will use valid, damaged, incomplete, and older Emerald save fixtures.
- Save-sync tests will cover immutable uploads, newest-version updates, rollback, expired leases, active leases, and conflicts.
- Provider tests will use an R2-compatible fake. Live R2 tests will require explicit credentials.
- Restore tests will make sure that Pokéctx preserves the replaced local save.
- An end-to-end smoke test will observe a location and one controlled state change in a supported Emerald session.
- The smoke test will make sure that Pokéctx does not write game memory.
- Continuous integration will examine generated catalogs and progression rules independently from the ROM build.
- The repository does not contain an application-level Go test suite.
- The existing reproducible ROM build is the closest prior example.
- Go tests will use table-driven tests and golden fixtures. The ROM build will remain an independent regression test.

## Out of Scope

- Building or hosting an AI agent.
- A remote MCP server or HTTP API.
- Prompt design, model selection, API keys, or conversational memory.
- A Swift macOS application, menu-bar application, iPhone application, or iPad application.
- A Bun/TypeScript assistant client.
- General account systems and public sharing of save files.
- Automatic access to the private container of an unrelated iPhone emulator.
- Concurrent play on two devices or field-level save merging.
- Writing emulator memory, automating controls, cheating, editing inventory, or changing story flags.
- Requiring modifications to the matching Pokémon Emerald ROM.
- Full save-file parsing in the first release.
- Support for emulators other than the selected initial mGBA integration.
- Support for other Pokémon games, ROM hacks, randomized ROMs, or unsupported Emerald revisions.
- Complete optional-content, Battle Frontier, contest, Pokédex-completion, breeding, competitive battling, or postgame guidance.
- Turn-by-turn battle coaching, team optimization, encounter routing, or automated walkthrough execution.
- A graphical map or pathfinding system.
- Scraping community websites at runtime.
- Redistributing copyrighted walkthrough prose or game assets.

## Further Notes

- The decompilation supplies machine-readable game data. This data includes save blocks, locations, flags, vars, species, moves, items, scripts, and encounters.
- Pokéctx will use decompilation data instead of copied numeric offsets and names.
- Community walkthroughs can inform the order and meaning of tasks.
- Decompilation scripts will supply progression facts when possible.
- Contributors must examine external material for accuracy and licensing. Pokéctx will store original facts and guidance, not copied prose.
- The progression model joins observed flags, vars, inventory, and location to a clear next task. This model is the principal product asset.
- Memory layouts can vary with ROM revision and build configuration. ROM identification and fail-closed decoding are product requirements, not optional diagnostics.
- Normal in-game saves remain valuable for recovery and future offline snapshots, but they are not part of the live interaction loop.
- SSH stays outside the plugin authentication model. The user secures SSH with the Mac connection, keys, and network controls.
- The Vibetrade plugin supplies the command model. Pokéctx uses the same manifest, skill routing, named reads, `doctor`, JSON, fixtures, and tests.
- This document uses pragmatic Simplified Technical English. Full ASD-STE100 compliance requires the official ASD-STE100 dictionary.
