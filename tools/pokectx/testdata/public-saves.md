# Public save test sources

Public saves are optional integration fixtures. Routine tests do not download files.

- Project Pokémon Base Pokémon Save Files: <https://projectpokemon.org/home/files/file/2128-base-pok%C3%A9mon-save-files-pkhex-compatible/>
- Community checkpoint saves: <https://github.com/Bl1ndBeholder/pokemon-saves>
- GameFAQs Pokémon Emerald saves: <https://gamefaqs.gamespot.com/gba/921905-pokemon-emerald-version/saves>

Verified integration fixture (not committed):

- URL: <https://raw.githubusercontent.com/Bl1ndBeholder/pokemon-saves/main/emerald/011020251345.sav>
- SHA-256: `e88d19392f23c0df2b4d6a817d9aa244defd6271189aad4aacb87272f9e199b2`
- Size: 131,088 bytes (128 KiB flash plus a 16-byte emulator trailer)
- Expected: player DIXIE, Lavaridge Pokémon Center, five party members, three badges, save counter 41

Before a fixture enters this repository, record its source URL, SHA-256 checksum, language, expected game code, and expected progress.
