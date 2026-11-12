# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Goal

Reverse engineer Football Manager 26 `.fm` save files, expose the decoded data through an MCP server, and build an LLM "assistant manager" on top of it.

## Stack

- Go. MCP via `github.com/modelcontextprotocol/go-sdk`; zstd via `github.com/klauspost/compress/zstd`.
- Layering: raw binary parsing → domain models → MCP handlers. MCP handlers contain no parsing logic; parsers know nothing about MCP.
- Integration tests run against `data/test_save.fm`; unit tests use synthetic byte buffers.

## Save format

@docs/format.md

- The index above is all that loads. Before working on a structure, read its file under `docs/format/` (grep `docs/format/` for a quoted section name).
- Record every decoded structure in its `docs/format/*.md` file with offsets, evidence and the FM build it was verified on; add new files or sections to the index in `docs/format.md`. The format can drift between builds (test save is 26.2.0, installed game is 26.3.2).
- Use the `/decode-structure` skill when decoding a new structure.

## Files outside the repo — READ ONLY

- Test save: `data/test_save.fm` (gitignored; copy from `~/www/my/fm/data/test_save.fm`). FM 26.2.0, Tottenham, game date 2025-07-07, ~46 MB.
- FM27 test save: `data/test_save_27.fm` (gitignored). FM 27.0.0, Kauno Žalgiris, ~49 MB. A copy of the KŽ save taken 2026-11-04 from `~/Library/Application Support/Sports Interactive/Football Manager 27/games/`; that save changes as the game is played, so keep the copy.
- Second FM27 test save: `data/test_save_27b.fm` (gitignored). The same career copied again on 2026-11-04 (21:53 save), game date 2026-01-22, after KŽ's first match (against FK Banga). It is the only FM27 sample with a manager's tactic and played last lineups.
- Transfer status pair: `data/test_save_27_list_before.fm` and `data/test_save_27_list_after.fm` (gitignored). KŽ saves copied on 2026-11-06 before and after setting statuses in game: Ibrahim Gambo Transfer Listed, Elijus Trumpa Listed for Loan, Fedor Černych Unavailable for loan.
- Interest test save: `data/test_save_27_interest.fm` (gitignored). The KŽ save of 2026-11-06 20:34 (game date 2026-02-11), copied 2026-11-07, just after the user's agent-chat approaches to 21 players (outcomes in `docs/superpowers/specs/2026-11-07-interest-field-observations/field-observations.md`).
- Team instructions test save: `data/test_save_27_instructions.fm` (gitignored). The KŽ save of 2026-11-07 20:40 (game date 2026-05-04), whose tactic has team instructions set in game (screenshots in the "27.0.0 team instructions" section of `docs/format/tactics.md`).
- Player instructions test save: `data/test_save_27_player_instructions.fm` (gitignored). The KŽ save of 2026-11-07 22:25 (game date 2026-05-09), with player instructions set in game on Švedkauskas (GK) and Krekovič (AML) (listed in the "27.0.0 player instructions" section of `docs/format/tactics.md`).
- Homegrown test save: `data/test_save_27_homegrown.fm` (gitignored). The KŽ save of 2026-11-09 01:25 (game date 2026-07-16), copied 2026-11-09, with in-game homegrown rows of 10 players (table in the "Relationships and favoured clubs" section of `docs/format/players.md`).
- Shortlist test save: `data/test_save_27_shortlist.fm` (gitignored). The KŽ save of 2026-11-11 14:39 (game date 2026-11-25), with two shortlists: (Default) of 12 players and "interested" of 7 (listed in the "27.0.0 shortlists" section of `docs/format/scouting.md`).
- Match stats test save: `data/test_save_27_matches.fm` (gitignored). The KŽ save of 2026-11-11 21:50, with FK TransINVEST 1–1 KŽ (match id 12438, 2026-11-08) checked against in-game match report and player stats screenshots ("Match object" in `docs/format/matches.md`).
- False trailer test save: `data/test_save_27_trailer.fm` (gitignored). The KŽ save of 2026-11-12 00:44 (game date 2027-01-22). Person record 65 535 holds a false trailer for record 65 536 ("Person table" in `docs/format/game-db.md`).
- Real saves: `~/Library/Application Support/Sports Interactive/Football Manager 26/cloud/games/*.fm` and `.../Football Manager 27/games/*.fm` (FM26 ones Steam Cloud synced), ~400 MB, non-ASCII filenames (`Ž`). Never write, move or touch these; a bad write syncs to the cloud and destroys a career. Copy to `$TMPDIR` before experimenting.
- Game: `~/Library/Application Support/Steam/steamapps/common/Football Manager 26/fm.app`. Unity IL2CPP front end (`Contents/Frameworks/GameAssembly.dylib` + `global-metadata.dat`); the native SI engine is `Contents/PlugIns/game_plugin.bundle/Contents/MacOS/game_plugin`. Bundled database under `game_plugin.bundle/Contents/Resources/shared/data/database/db/`.

## Gotchas

- Lint with `golangci-lint run`; `.golangci.yml` uses the v2 schema (`version: "2"`, `formatters:` section), not v1.
- `go get` / `go mod download` fail inside the Bash sandbox with an x509 `OSStatus -26276` error; rerun with the sandbox disabled.
- Close zstd decoders (`Decoder.Close()`); they hold goroutines.
- Corporate MDM endpoint security slows every file op. Decompress a save once to `$TMPDIR` and reuse the dump rather than re-decompressing 400 MB per experiment.
