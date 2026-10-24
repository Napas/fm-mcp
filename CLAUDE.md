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
- Real saves: `~/Library/Application Support/Sports Interactive/Football Manager 26/cloud/games/*.fm` — Steam Cloud synced, ~400 MB, non-ASCII filenames (`Ž`). Never write, move or touch these; a bad write syncs to the cloud and destroys a career. Copy to `$TMPDIR` before experimenting.
- Game: `~/Library/Application Support/Steam/steamapps/common/Football Manager 26/fm.app`. Unity IL2CPP front end (`Contents/Frameworks/GameAssembly.dylib` + `global-metadata.dat`); the native SI engine is `Contents/PlugIns/game_plugin.bundle/Contents/MacOS/game_plugin`. Bundled database under `game_plugin.bundle/Contents/Resources/shared/data/database/db/`.

## Gotchas

- Lint with `golangci-lint run`; `.golangci.yml` uses the v2 schema (`version: "2"`, `formatters:` section), not v1.
- `go get` / `go mod download` fail inside the Bash sandbox with an x509 `OSStatus -26276` error; rerun with the sandbox disabled.
- Close zstd decoders (`Decoder.Close()`); they hold goroutines.
- Corporate MDM endpoint security slows every file op. Decompress a save once to `$TMPDIR` and reuse the dump rather than re-decompressing 400 MB per experiment.
