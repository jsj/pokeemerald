#!/bin/sh
set -eu

if [ "$#" -ne 3 ]; then
	printf 'usage: %s /path/to/mGBA.app /path/to/game.gba /path/to/output.app\n' "$0" >&2
	exit 64
fi

runtime_app=$1
rom_path=$2
output_app=$3
repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)

test -x "$runtime_app/Contents/MacOS/mGBA"
test -f "$rom_path"
runtime_version=$($runtime_app/Contents/MacOS/mGBA --version 2>&1 || true)
if ! otool -L "$runtime_app/Contents/MacOS/mGBA" | grep -q 'QtCore.framework/Versions/5/QtCore'; then
	printf 'mGBA must use Qt 5 on macOS 27: %s\n' "$runtime_version" >&2
	exit 65
fi
case "$output_app" in
	*.app) ;;
	*) printf 'output must end in .app: %s\n' "$output_app" >&2; exit 64 ;;
esac
if [ -e "$output_app" ]; then
	printf 'refusing to replace existing app: %s\n' "$output_app" >&2
	exit 73
fi

ditto "$runtime_app" "$output_app"
mv "$output_app/Contents/MacOS/mGBA" "$output_app/Contents/MacOS/EmeraldRuntime"
cp "$rom_path" "$output_app/Contents/Resources/Pokemon Emerald.gba"
mkdir -p "$output_app/Contents/Resources/autorun"
cp "$repo_root/tools/pokectx/scripts/pokectx.lua" "$output_app/Contents/Resources/autorun/pokectx.lua"
mailbox_address=$(awk '/gPokectxCheatMailbox$/ { print $1; exit }' "$repo_root/pokeemerald.map")
case "$mailbox_address" in
	0x02??????) ;;
	*) printf 'invalid Pokéctx cheat mailbox address: %s\n' "$mailbox_address" >&2; exit 65 ;;
esac
sed -i '' "s/cheatMailbox = 0x[0-9A-Fa-f]*/cheatMailbox = $mailbox_address/" \
	"$output_app/Contents/Resources/autorun/pokectx.lua"
if [ -f "$repo_root/AppIcon.icns" ]; then
	cp "$repo_root/AppIcon.icns" "$output_app/Contents/Resources/PokemonEmerald.icns"
	/usr/libexec/PlistBuddy -c 'Set :CFBundleIconFile PokemonEmerald.icns' "$output_app/Contents/Info.plist"
fi

xcrun clang -Os -fobjc-arc -framework Foundation \
	"$repo_root/emulator/mgba/tools/macos-bundled-game-launcher.m" \
	-o "$output_app/Contents/MacOS/PokemonEmeraldLauncher"
/usr/libexec/PlistBuddy -c 'Set :CFBundleExecutable PokemonEmeraldLauncher' "$output_app/Contents/Info.plist"
/usr/libexec/PlistBuddy -c 'Set :CFBundleName Pokémon Emerald Pokéctx' "$output_app/Contents/Info.plist"
/usr/libexec/PlistBuddy -c 'Set :CFBundleDisplayName Pokémon Emerald Pokéctx' "$output_app/Contents/Info.plist" 2>/dev/null || \
	/usr/libexec/PlistBuddy -c 'Add :CFBundleDisplayName string Pokémon Emerald Pokéctx' "$output_app/Contents/Info.plist"
/usr/libexec/PlistBuddy -c 'Set :CFBundleIdentifier com.zoverflow.pokemon-emerald-pokectx' "$output_app/Contents/Info.plist"
/usr/libexec/PlistBuddy -c 'Add :MGBARuntimeName string EmeraldRuntime' "$output_app/Contents/Info.plist"
/usr/libexec/PlistBuddy -c 'Add :MGBABundledROM string Pokemon Emerald.gba' "$output_app/Contents/Info.plist"
/usr/libexec/PlistBuddy -c 'Add :MGBASaveDirectory string Pokemon Emerald Pokectx' "$output_app/Contents/Info.plist"
/usr/libexec/PlistBuddy -c 'Delete :CFBundleDocumentTypes' "$output_app/Contents/Info.plist" 2>/dev/null || true

codesign --force --deep --sign - "$output_app"
codesign --verify --deep --strict "$output_app"
printf 'Created %s\n' "$output_app"
