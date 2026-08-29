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
cp "$rom_path" "$output_app/Contents/Resources/Pokemon Emerald 386.gba"
if [ -f "$repo_root/AppIcon.icns" ]; then
	cp "$repo_root/AppIcon.icns" "$output_app/Contents/Resources/PokemonEmerald.icns"
	/usr/libexec/PlistBuddy -c 'Set :CFBundleIconFile PokemonEmerald.icns' "$output_app/Contents/Info.plist"
fi

launcher_source=$(mktemp "${TMPDIR:-/tmp}/pokemon-emerald-launcher.XXXXXX.c")
trap 'rm -f "$launcher_source"' EXIT HUP INT TERM
cat > "$launcher_source" <<'EOF'
#include <libgen.h>
#include <limits.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/stat.h>
#include <unistd.h>

int main(int argc, char **argv) {
	(void) argc;
	char executable[PATH_MAX];
	if (!realpath(argv[0], executable)) return 1;
	char *macos = dirname(executable);

	char runtime[PATH_MAX];
	char rom[PATH_MAX];
	char saves[PATH_MAX];
	const char *home = getenv("HOME");
	if (!home) return 1;
	snprintf(runtime, sizeof(runtime), "%s/EmeraldRuntime", macos);
	snprintf(rom, sizeof(rom), "%s/../Resources/Pokemon Emerald 386.gba", macos);
	snprintf(saves, sizeof(saves), "%s/Library/Application Support/Pokemon Emerald 386", home);
	mkdir(saves, 0755);

	char save_option[PATH_MAX + 32];
	char state_option[PATH_MAX + 32];
	snprintf(save_option, sizeof(save_option), "savegamePath=%s", saves);
	snprintf(state_option, sizeof(state_option), "savestatePath=%s", saves);
	char *const arguments[] = {
		runtime, "-C", "fullscreen=0", "-C", "displayDriver=0",
		"-C", save_option, "-C", state_option, rom, NULL
	};
	execv(runtime, arguments);
	perror("Pokemon Emerald 386");
	return 1;
}
EOF

xcrun clang -Os "$launcher_source" -o "$output_app/Contents/MacOS/Pokemon Emerald 386"
/usr/libexec/PlistBuddy -c 'Set :CFBundleExecutable Pokemon Emerald 386' "$output_app/Contents/Info.plist"
/usr/libexec/PlistBuddy -c 'Set :CFBundleName Pokemon Emerald 386' "$output_app/Contents/Info.plist"
/usr/libexec/PlistBuddy -c 'Set :CFBundleIdentifier com.zoverflow.pokemon-emerald-386' "$output_app/Contents/Info.plist"
/usr/libexec/PlistBuddy -c 'Delete :CFBundleDocumentTypes' "$output_app/Contents/Info.plist" 2>/dev/null || true

codesign --force --deep --sign - "$output_app"
codesign --verify --deep --strict "$output_app"
printf 'Created %s\n' "$output_app"
