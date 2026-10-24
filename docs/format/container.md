# FM26 format: Container, directory and inner files

Part of the format reference. Conventions, dates and the index of all files: [`docs/format.md`](../format.md).

## Container header (26 bytes) — verified 26.2.0 (bytes + code)

Read by the function at `0x3ab3ed4`.

| Offset | Size | Content |
|--------|------|---------|
| 0 | 1 | u8 header kind = `02` (reader rejects anything else) |
| 1 | 1 | u8 byte order: `01` = little-endian; `00` sets the stream's byte-swap flag |
| 2 | 4 | magic `"fmf."` (u32 `0x2e666d66`); the reader also accepts `"afe."` (u32 `0x2e656661`) |
| 6 | 2 | u16 container version; reader accepts 7 or 8; test save = 8 |
| 8 | 1 | u8, `00`; read, then unused on this path |
| 9 | 8 | u64 directory offset, relative to offset 9. Test save: 46 481 093 → directory header at 46 481 102 |
| 17 | 8 | u64 data offset, relative to offset 9. Test save: 17 → data starts at 26. The reader stores `9 + value` as the data start |
| 25 | 1 | u8, `03`; read only when version ≥ 8 (version 7 stores a bool byte instead), and kept in the archive object at `+0x5a`. Meaning unknown |
| 26 | … | data area: inner files, one zstd frame each, back to back |

The reader seeks to `9 + directory offset` and reads a second header there.

## Directory — verified 26.2.0 (bytes + code)

At the directory offset:

| Offset | Size | Content |
|--------|------|---------|
| 0 | 9 | header: `02 01 "fmf." u16 8 00` (same checks as the container header) |
| 9 | … | one zstd frame holding the directory tree |

Version 8 opens a zstd decoder over the rest of the file (`0x3abb5ec`); version 7 uses a different reader (`0x3abaf3c`, not decoded).

The decompressed tree (78 635 bytes in the test save) is one recursive `dir` node:

```
dir   = string name
        u32    file_count
        file   × file_count
        u32    subdir_count
        dir    × subdir_count
file  = string name           e.g. "game_db"
        string extension      ".dat" or ".cmt"
        u64    offset         relative to the data area (container offset 26)
        u64    compressed size
        u64    uncompressed size
        i64    time 1         Unix seconds; -62135596800 (0001-01-01) means unset
        i64    time 2         same as time 1 in every entry of the test save
```

The test save parses to exactly the end of the buffer: root `test_save` with 76 files plus one subdir `rgman` with 1 139 files, giving 1 215 files. That matches the 1 215 zstd frames in the data area, and every offset/size pair matches a frame. Only the 12 `.cmt` files carry real timestamps (1773345046–1773345220 = 2026-03-12, the day the save was written).

`zstd -d` on the whole data area works, because zstd concatenates frames. It then fails on the directory header, and the output is all inner files glued together. Split by frame (or by directory entry) instead.

### Root files (test save)

`game_info`, `memory_pools`, `save_game_summary`, `game_db` (122 MB uncompressed — the main database), `pl_hist_ls`, `non_pl_hist_ls`, `humans`, `news`, `used_player_data`, `transfer_man`, `trade_man`, `shortlist_man`, `manager_manager`, `tc_record_man`, `tc_*_history_ls`, `tactics_man`, `regeneration_manager`, `retirement_manager`, `database_analyser`, `edit_session_man`, `weather_manager`, `fog_of_war`, `injury_manager`, `injury_histories_ls`, `training_man`, `training_camp_man`, `scout_man`, `awol`, `award_man`, `award_*_hist_ls`, `fine_manager`, `contract_man`, `person_record_history_ls`, `person_record_manager`, `official_manager`, `agent_manager`, `dispute`, `finance_manager`, `network_man`, `sim_stats`, `hall_of_fame`, `backroom_advice_manager`, `interaction_manager`, `press_conference_manager`, `job_centre`, `team_talk_man`, `board_takeover_manager`, `financial_fair_play_manager`, `feeder_man`, `captain_system`, `media_man`, `morale_manager`, `matchday_manager`, `play_fixture_manager`, `dynamics_pivotal_moments`, `virtual_manager`, `match_stats_cfg` (all `.dat`), then 12 `.cmt` files: `player_stats_hist_dt`, `tc_cup_history_dt`, `person_db_changes` (45 MB), `tc_league_history_dt`, `award_year_hist_dt`, `tc_manager_history_dt`, `person_record_history_dt`, `starting_club_debt_db`, `comp_history_dt`, `non_pl_hist_dt`, `injury_histories_dt`, `tc_history_dt`.

`rgman/` (rule group manager) holds `rgman`, `comp_archive`, `rule_group`, about 1 100 `comp_<id>` files (one per competition) and `player_stats`, `player_stats_hist_ls`, `official_stats`, `extended_comp_records_ls`.

## Inner file header — verified 26.2.0 (bytes, all 1 215 files)

| Offset | Size | Content |
|--------|------|---------|
| 0 | 1 | u8 header kind = `03` |
| 1 | 1 | u8 byte order = `01` |
| 2 | 4 | tag = the file extension reversed: `"tad."` for `.dat` (1 203 files), `"tmc."` for `.cmt` (12 files) |
| 6 | 2 | u16 file format version, per file type. Examples: `game_info` 46, `memory_pools` 0, `save_game_summary` 29, `game_db` 4000, `rgman` 1637 |
| 8 | … | file body |

The tag never appears as an immediate or literal in `game_plugin`. Hypothesis: it is built from the `".dat"` / `".cmt"` extension strings at runtime.

## Robustness

Saves are untrusted input. Header offsets are capped at 2^48; the compressed and decompressed directory are each capped at 64 MB; inner files are stream-decoded up to the size the directory claims (a zstd frame header's content size is never trusted) with the zstd window capped at 64 MB; buffers for inner files and tables are sized by the bytes actually present, never by a count or size read from the file; pool-count sums are checked for overflow. `tactics_man.dat` bodies over 32 MB are rejected before decompression; the walk caps team entries at 64 per human and every other list at 1 024 items and at the bytes left (by each item's smallest size); the fallback last-lineup scan tries at most 64 candidate places, validates a candidate's records before building anything, and parses each preset tactic once. The NATION pool count must be at most 1 000 (251 in every save seen). The CONTINENT, NATION and national-team container readers each try at most 64 candidate places; one try scans at most 1 MB (CONTINENT, NATION) or 16 MB (the men's containers), the nation tail scan bounds each counted list at 1 000 items, and a container record is searched for at most 1 MB. Fuzz targets (`internal/savefile/fuzz_test.go`, `tactics_fuzz_test.go`) cover the container, directory, player record, table, nation, fog-of-war, player-stats, transfer and tactics parsers. The `transfer_man.dat` scan rejects a body with more than one record candidate per 30 bytes. Table splitting (`splitTable`: persons, COMP, CONTINENT, NATION) scans for trailers within a budget of 8 × (body length − table start) + 64 KB and fails with a corrupt-table error beyond it; a crafted body whose records only have loose trailers made the look-ahead rescan the rest of the body per record (0.67, 2.8 and 11.1 s on 1, 2 and 4 MB). A record-size cap does not work: the human manager's person record is 21.6 MB (verified, bytes: record 158 530, tag 9, in all three 26.3.2 saves), and the largest COMP record is 175 KB (2035-12-02). The real invariant is about 3 × the table's own length (look-ahead of records k and k+1, then the strict search of record k). Measured scanned / (body length − table start) on the test save and the 2035-12-02, 12-09 and 12-14 saves: person table 2.96 (26.3.2 saves) and 3.03 (test save, worst), CONTINENT 2.79–2.92, COMP 0.08–0.20, NATION 0.31–1.04, so the margin is at least 2.6 ×. A player record that does not decode is skipped and reported as a load warning; the load fails only if most records fail. `savefile.VerifiedDatabaseVersion` lists the checked database versions (3727, 3733).

## 26.3.2 saves

Four real 26.3.2 saves (game dates 2035-09-20 to 2035-11-29, ~410 MB, 1 708 inner files) keep container version 8 and every layout above; `game_db.dat` is 351 MB with database version 3733 and ~140 000 players. Differences: extra root files `*.scm` and `*.apm` (not decoded) and person tags 0 and 19 (not mapped). Generated players can carry common-name refs beyond every name table (up to 646 431), which the parser leaves unresolved.

## Open questions

- Byte at container offset 25 (`03`) and container offset 8; upper 7 bits of the date u16.
- Whether `.fmf` files (tactics, exported teams) use the same container; the `"afe."` magic is a second accepted kind.
