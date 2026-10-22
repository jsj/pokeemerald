# Pokémon Emerald

This is a decompilation of Pokémon Emerald.

This fork builds a modified Emerald ROM with the Pokectx bridge:

* **pokeemerald.gba** `sha1: b72f9abddc3f272efddc5abd4d4de1e50358cbd4`

To set up the repository, see [INSTALL.md](INSTALL.md).

## Build the macOS Pokéctx app

Install Homebrew `qt@5`, CMake, Ninja, and Go. Build the matching ROM first. Then run:

```bash
./scripts/build-macos-pokectx.sh
```

The command creates `build/Pokémon Emerald Pokéctx.app`. This build uses Qt 5.15 because Qt 6.11 shows a black game surface on macOS 27 beta.

For contacts and other pret projects, see [pret.github.io](https://pret.github.io/).
