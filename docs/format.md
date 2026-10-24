# FM26 save file format

All multi-byte integers are little-endian. Strings are a u32 LE length followed by UTF-8 bytes, no NUL terminator.

A `.fm` save is a small archive: a header, one zstd frame per inner file, and a zstd-compressed directory at the end. Verified on `data/test_save.fm` (build 26.2.0) unless marked otherwise.

Code addresses refer to the arm64 slice of `game_plugin` (sha256 `01c785dd…beecd`, installed build 26.3.2). That slice has `__TEXT` vmaddr == file offset.

The reference is split by structure under `docs/format/`. Read the file for a structure before changing its parser; record new findings there (offsets, evidence, build) and add new sections to the table below. Each file ends with its open questions. Quoted section names in the docs and code comments (e.g. "Person table") are the headings listed here.

## Dates — low 9 bits verified

A u16 + u16 year pair (`0x43fe348` writes two raw u16s). Low 9 bits of the first u16 = 1-based day of year; verified across thousands of birth, contract and injury dates (e.g. Maddison born 328/1996 = 1996-11-23). Evidence: `bc 1a e9 07` → 188/2025 = 2025-07-07 (the in-game date). `47 70 ea 07` in `game_info` → 71/2026 = 2026-03-12 (the real save date). The upper 7 bits (13 and 56 in these samples) are unknown.

Unset dates are saved in 1900. Seen: joined dates of 1900-01-01 on loanees and free agents (in game); in the real KŽ save (26.3.2, 2035-12-02), contract ends of 1900-01-02 on 45 665 main contracts and 34 loans (e.g. Stephen Connolly, uid 2002128293, £0 a week), and two main-contract starts of 1900-01-01. `fm` treats every 1900 date as unset; that the whole year is a marker is an inference.

## Files

| File | Covers | Sections |
|------|--------|----------|
| [`container.md`](format/container.md) | archive header, directory tree, inner file header, 26.3.2 differences, parser robustness caps | Container header (26 bytes); Directory; Root files (test save); Inner file header; Robustness; 26.3.2 saves |
| [`small-files.md`](format/small-files.md) | game_info, memory_pools, save_game_summary, humans | game_info.dat; memory_pools.dat; save_game_summary.dat; humans.dat |
| [`fog-of-war.md`](format/fog-of-war.md) | scouting knowledge, baseline, decay | fog_of_war.dat |
| [`injuries.md`](format/injuries.md) | injury_manager.dat queues, INJURY table (player injury items are in players.md) | injury_manager.dat; Injury types (INJURY) |
| [`transfers.md`](format/transfers.md) | TRANSFER records, arranged moves | transfer_man.dat |
| [`tactics.md`](format/tactics.md) | tactics grammar, roles/positions enums, last lineups, formations, shirt numbers | tactics_man.dat; Last lineup |
| [`role-ability.md`](format/role-ability.md) | role attribute lists, role rating, best role, role ability, stars, out-of-possession table | Role ability (game code) |
| [`player-interest.md`](format/player-interest.md) | interest filter, hard gates, just-joined penalty, reputation gap, playing-time value, verdicts | Player interest (game code) |
| [`player-stats.md`](format/player-stats.md) | season stats: totals, 60 detail fields, goalkeeper split, scopes/slots, rollover | rgman/player_stats.dat |
| [`game-db.md`](format/game-db.md) | game_db header, table template, name tables, person table and record tags, trailer flags | game_db.dat; Person name tables; Person table |
| [`players.md`](format/players.md) | player part offsets, reputation level, current injuries, BASIC part, hidden attributes, relationships/favoured clubs, personality, traits, media handling | Player record (tags 2 and 3); Reputation level; Current injuries; BASIC person part; Relationships and favoured clubs; Personality; Player traits; Media handling |
| [`contracts.md`](format/contracts.md) | ACTUAL_PERSON fields before the marker, contract types, FULL header, clauses, bonuses, playing time, actual playing time, squad numbers, loyalty bonus | Contracts (ACTUAL_PERSON part) |
| [`clubs.md`](format/clubs.md) | CLUB/COMP/TEAM tables, club budgets, squad lists | Clubs, teams and competitions; Club budgets (CLUB_FINANCE); Squad lists |
| [`currencies.md`](format/currencies.md) | CURRENCY_RECORD rates, display rule, game preferences (currency, wage period) | Currencies (CURRENCY_RECORD); Display preferences (outside the save) |
| [`nations.md`](format/nations.md) | CONTINENT strengths, NATION tail grammar, reputation factors | Continents, nations and national-team containers |
| [`fixtures.md`](format/fixtures.md) | fix_man.dat lists, FIXTURE base, FIXTURE_RESULT scores | fix_man.dat; FIXTURE; FIXTURE_RESULT |
| [`competitions.md`](format/competitions.md) | competition keys (ACTUAL_COMP_ID), stage names, league tables in comp files | Competition keys (ACTUAL_COMP_ID); Stage names (STAGE_NAME); League tables (LEAGUE_STAGE rows) |
| [`language-db.md`](format/language-db.md) | lang_db.dat container, nation/injury/competition names by uid rank | Language database (names) |
