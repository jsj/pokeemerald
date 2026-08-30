# Pokémon Emerald

This is a decompilation of Pokémon Emerald.

It builds the following ROM:

* [**pokeemerald.gba**](https://datomatic.no-intro.org/index.php?page=show_record&s=23&n=1961) `sha1: f3ae088181bf583e55daf962a92bb46f4f1d07b7`

To set up the repository, see [INSTALL.md](INSTALL.md).

## Build the macOS Pokéctx app

Install Homebrew `qt@5`, CMake, Ninja, and Go. Build the matching ROM first. Then run:

```bash
./scripts/build-macos-pokectx.sh
```

The command creates `build/Pokémon Emerald Pokéctx.app`. This build uses Qt 5.15 because Qt 6.11 shows a black game surface on macOS 27 beta.

For contacts and other pret projects, see [pret.github.io](https://pret.github.io/).
