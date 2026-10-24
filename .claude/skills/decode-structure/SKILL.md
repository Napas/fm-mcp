---
name: decode-structure
description: Reverse engineer a new structure in FM26 save files (header field, section, record type) and land it as a tested Go parser plus a docs/format/ entry. Use when decoding any unknown bytes in a .fm save or investigating how the game serializes data.
---

Target: $ARGUMENTS

## 1. Get the bytes

- Work on `data/test_save.fm` or a copy in `$TMPDIR`. Never open real saves under `~/Library/Application Support/Sports Interactive/` for writing.
- Decompress once and reuse: `tail -c +27 data/test_save.fm | zstd -d -o "$TMPDIR/payload.bin"` (skip if the dump exists).
- Find the region: `xxd -s <offset> -l 256 "$TMPDIR/payload.bin"`, `grep -boa '<ASCII name>' "$TMPDIR/payload.bin"` for record names / length-prefixed strings.
- Prefer known in-game values as anchors (player names, ages, wages, dates from the save's game state). Search for their encodings (u32 LE, u16 LE, float32) to locate fields.

## 2. Hypothesise from more than one sample

- Check the hypothesis against at least two instances (two records, two players, or two saves). One match is coincidence.
- If bytes alone don't resolve it, inspect the game's serializer with the REA tools (`reverse-engineer-anything` skill) on `game_plugin` (native SI engine) — search strings for the record/type name and follow xrefs to the reader/writer.

## 3. Land it with TDD

1. Write a failing table-driven unit test with a synthetic byte buffer for the layout.
2. Write an integration test asserting real values from `data/test_save.fm`.
3. Implement the parser in the raw parsing layer (no MCP or domain logic). Add sanity caps on counts/lengths.
4. `go test ./...` green.

## 4. Document

Update the structure's file under `docs/format/` (a new file for a new structure, listed in the index table in `docs/format.md`): offsets/sizes table, how it was verified (anchor values, sample count), build version verified on, and move resolved items out of that file's "Open questions". Label anything not confirmed by bytes or code as a hypothesis.
