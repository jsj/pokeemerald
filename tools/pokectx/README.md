# Pokéctx

Pokéctx is a read-only Agent Plugin for a live Pokémon Emerald game. The plugin uses a Go command and an mGBA Lua adapter.

Use `pokectx team` to see the Pokémon that you carry. Use `pokectx bag` to see badges and key items.

The old `party` and `inventory` commands remain available as aliases.

## Build

```bash
go build -o bin/pokectx ./cmd/pokectx
go test ./...
```

## Connect mGBA

1. Open the matching Pokémon Emerald ROM in mGBA.
2. Open **Tools > Scripting**.
3. Load `scripts/pokectx.lua`.
4. Run `bin/pokectx doctor`.

The adapter listens only on `127.0.0.1:8765`. It reads game memory and does not write game memory.

### Automatic connection on the Mac mini

Open this app on the Mac mini:

```text
/Volumes/SandDiskTB/Games/Pokémon Emerald Pokéctx.app
```

This app contains `pokectx.lua` in `Contents/Resources/autorun/`. The `jsj/mgba` fork loads Lua files from this directory during startup.

The app uses this separate save directory:

```text
~/Library/Application Support/Pokemon Emerald Pokectx
```

The original Emerald 386 app uses a different save directory. The two apps do not replace each other's saves.

## Use SSH

Install `pokectx` and the Lua adapter on the gaming Mac. Then set these values on the client computer:

```bash
export POKECTX_SSH_HOST=couch-mac
export POKECTX_REMOTE_COMMAND=/path/to/pokectx
```

The plugin uses the existing SSH connection. It does not open a public service.

## Inspect a save file

Pokéctx can read a saved game while mGBA is closed:

```bash
pokectx snapshot --save /path/to/pokeemerald.sav
```

It validates both rotating Emerald save slots, sector signatures, and checksums. It chooses the newest complete slot. Files with mGBA metadata after the 128 KiB flash image are accepted.

## Manage and sync saves

Pokéctx uses this local save by default on macOS:

```text
~/Library/Application Support/Pokemon Emerald Pokectx/Pokemon Emerald.sav
```

Export a validated copy without changing the active save:

```bash
pokectx save export --to ~/Desktop/emerald.sav
```

Import a save:

```bash
pokectx save import --from ~/Downloads/emerald.sav
```

Close mGBA before an import. Pokéctx validates the source and keeps the current save in `.pokectx-backups` before it installs the imported file.

Set these private values in `local.env` to use Cloudflare R2:

```bash
export POKECTX_R2_ENDPOINT=https://ACCOUNT_ID.r2.cloudflarestorage.com
export POKECTX_R2_BUCKET=pokectx-saves
export POKECTX_R2_ACCESS_KEY_ID=...
export POKECTX_R2_SECRET_ACCESS_KEY=...
```

Back up the local save or synchronize it with the newest R2 save:

```bash
pokectx save backup
pokectx save sync
```

Backups are immutable objects. `save sync` compares the Emerald save counter and SHA-256 hash. It stops on a conflict instead of choosing a save silently.

## Test

Run the routine offline tests:

```bash
go test ./...
go vet ./...
```

Run the opt-in public checkpoint test when network access is available:

```bash
./scripts/test-public-save
```

The script downloads one pinned community save into a temporary directory, verifies its SHA-256 checksum, inspects it, checks known progress, and removes it. The save is not committed to this repository.
