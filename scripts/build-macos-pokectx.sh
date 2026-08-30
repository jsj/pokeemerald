#!/bin/sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
build_dir="$repo_root/emulator/mgba/build-pokectx-qt5"
output_app=${1:-"$repo_root/build/Pokémon Emerald Pokéctx.app"}
qt5_prefix=${QT5_PREFIX:-/opt/homebrew/opt/qt@5}

test -d "$qt5_prefix"
test -f "$repo_root/pokeemerald.gba"

cmake -S "$repo_root/emulator/mgba" -B "$build_dir" -G Ninja \
	-DCMAKE_BUILD_TYPE=Release \
	-DCMAKE_PREFIX_PATH="$qt5_prefix" \
	-DFORCE_QT_VERSION=5 \
	-DBUILD_QT=ON \
	-DBUILD_SDL=OFF \
	-DUSE_LUA=ON
cmake --build "$build_dir" --target mgba-qt -j8

runtime_app="$build_dir/qt/mGBA.app"
if ! otool -L "$runtime_app/Contents/MacOS/mGBA" | grep -q 'QtCore.framework/Versions/5/QtCore'; then
	printf 'mGBA did not link Qt 5: %s\n' "$runtime_app" >&2
	exit 1
fi

(cd "$repo_root/tools/pokectx" && go build -o bin/pokectx ./cmd/pokectx)
"$repo_root/scripts/package-macos-app.sh" "$runtime_app" "$repo_root/pokeemerald.gba" "$output_app"
