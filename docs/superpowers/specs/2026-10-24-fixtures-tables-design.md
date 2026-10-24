# Fixtures, results and league tables — design

Date: 2026-10-24. Status: approved 2026-10-24.

## Why

The LLM assistant manager has no fixture or table context. It cannot say who the next opponent is, what happened in the last matches, or where the club stands ("3rd, 4 points off Europe"). The assistant rated this the biggest missing capability (backlog, "New data domains"). The manager asked for **all fixtures and full tables, including playoffs**, and chose to reverse engineer the exact competition mapping first (option A) rather than guess it from team divisions.

## Goals

- Every fixture the save holds, played and unplayed, for any team or competition: date, competition and stage, home and away team, and for played matches the score (90 minutes, after extra time, penalties).
- Every league table the save holds, in the game's own order: groups, conferences, league phases and playoff tables included, each labelled with its competition and stage.
- Playoff and knockout rounds labelled the way the game names them ("Playoffs – Semi Final – First Leg", "Champions Path – Playoff", "Quarter Final"), so a knockout bracket can be read from fixtures.
- The manager's next fixture and his league position are one call away (defaults to the human's first team).

## Anti-goals

- **A wrong table or a wrong competition label.** A table the parser cannot validate is left out with a load warning; it is never shown half-parsed or under a guessed stage. Same for fixtures whose competition key does not resolve.
- Computing a table from results when the save stores it. The stored table carries the game's tie-break order and any point deductions; a computed one can disagree.
- Slowing every load noticeably for data most calls never use.

## Non-goals

- Tables of past seasons. The save keeps only current stage tables (verified: Ligue 1's file holds one 18-row table on 2035-12-02). Past results stay available as fixtures.
- Match details beyond the score: goalscorers, cards, lineups of other matches, statistics. Fixture events exist (16-byte items) but are not decoded.
- Kick-off times, TV picks, rescheduling history.
- Competition rules (promotion and relegation places, tie-break rules, qualification zones). The game holds them in the comp files' rules TLV; not decoded.
- Names of in-game-created competitions (uid ≥ 2 000 000 000) and stages, which the language database lacks.

## What the manager decided

- Scope: all fixtures and full tables, including playoffs.
- Option A: resolve a fixture's competition exactly from the save's own key table, not by inference.
- Sub-competition tables are labelled by matching their stats to fixture results (see "Tables"), not by decoding the whole comp-file layout.

## Decoded data (evidence)

All of this is verified on both saves (test 26.2.0, 2025-07-07; KŽ 26.3.2, 2035-12-02) unless marked. Addresses are in the arm64 slice of `game_plugin` 26.3.2. Scratch scripts: the session scratchpad (`acid.py`, `fixwalk.py`, `ltable.py`); the builder re-derives the parsers with tests, the scripts are not authoritative. Record each structure in `docs/format/` as part of the work (below).

### Fixtures — `rgman/fix_man.dat`

- Layout: header `u32, u32, u16, u32`; a first list, then `u32 n` more lists. A list is `u16 key, u8 present`; when present, `u32, u32`, then items `i32 index` (−1 ends the list), `u8 flag` (1 = played, with a FIXTURE_RESULT before the base; 0 = base only), then `u32 count` equal to the items read. Decoded on both saves to the end of the file.
- FIXTURE base (63 bytes, version byte `0x1c`): competition key ref (`+0x20`), stadium, home and away team (TEAM index), match date (date pair), season `year` (season start), referee, `round` byte (`+0x3b`) and unknown fields (`fixwalk.py` names them).
- FIXTURE_RESULT: version, `u32`, home score bytes, away score bytes (`[0]` 90 minutes, `[1]` after extra time, `[2]` penalties; `0xff` = not played), attendance (`u32`, inference from values like 62 500), then `u32 n` × 16-byte events, then the base.
- Counts: KŽ 138 714 fixtures (seasons 2033 to 2035), test 19 239. The KŽ file is 31.5 MB decompressed.
- Code: FIXTURE vtable `0x54e6c20`, load `0x3e5c748`, save `0x3e5c0a8`.

### Competition keys — ACTUAL_COMP_ID table in `game_db.dat`

- A fixture's competition ref is the index of an ACTUAL_COMP_ID record (writer `0x33618e8`, reader `0x3362394`; storage `[0x5548760]+0x88`; created by `0x3363ba8`).
- Table: after the previous table's 9-byte end marker, `u32 count, u8 0`, then 33-byte records: `u8 tag 0, i32 A, i32 B, i32 C, i32 D, i32 E`, database record (`u32 index, u32 id, u32 id`, id = index + 1). Exactly one table in each save parses this way (KŽ 7 753 records, test 3 360).
- A = COMP index (competition). B = stage, C = sub-stage or group, D = leg or replay; all three are STAGE_NAME indices, −1 = none. E = a second COMP index for a promotion/relegation playoff between two divisions (Ligue 1 v Ligue 2, Eredivisie v Eerste Divisie), −1 = none.
- Examples (KŽ): 267 = Premier League; 268 = National League Cup / Group Stage / Group A; 5618 = Championship / Playoffs / Semi Final / First Leg; 5724 = Championship / Playoffs / Final; 3967 = Champions League / Champions Path / First Qualifying Round / First Leg; 4574 = French Cup / – / Fifth Round.
- Checks: test save, all 56 league-like fixture groups of 2025 resolve to the teams' division; KŽ 124 of 153 do, and the other 29 resolve to the correct sub-stage, cup or qualifier. All 2 073 keys KŽ fixtures use are in range.

### Stage names

- Save: STAGE_NAME table in `game_db.dat`, 285 records (memory pool `STAGE_NAME` 285 in both saves): `u32 count, u8 0`, then `u8 tag 0, u8 0`, database record. The uids equal the language table's uids in the same ascending order, so a stage index is the uid's rank (the nation rule).
- Names: language database file `8_23.dat` (English): `u32 count`, then `u32 uid, string name, u8, string, u8, string, u8, u8`; 285 records, parses to the end. Uid 1 "Replay", 3 "First Leg", 20 "Final", 22 "Playoff Final", 76 "Group A", 228 "Playoffs", 2000223075 "League Phase".

### League tables — `rgman/comp_<uid>.dat`

- A LEAGUE_STAGE (vtable `0x54e9900`, load `0x404291c`) holds two u8-counted lists of table rows (`+0xc0` and `+0x138`), back to back. The first is in table order; the second holds the same rows in another order (meaning unknown). Row loader `0x4044190`, row base `0x4046740`, stat record `0x4077f90`.
- Ref: `u8, i32` (the `u8` was 0 in every row seen; meaning unknown).
- Stat record (17 bytes): ref team (−1 = this row's team), `u8 played, u8 (equal to played in every row seen), u8 won, u8 drawn, u8 lost, u8 shoot-out losses`, `u16 goals for, u16 goals against, i16 points`. Played = won + drawn + lost + shoot-out losses in every table that passes the other checks.
- Points are stored, not derived. In most leagues they equal 3 × won + drawn. In group stages where drawn matches go to penalties (KŽ: EFL Trophy, Scottish League Cup, Leagues Cup, National League Cup; 13 tables), they fit 3 × won + 2 × drawn + 1 × shoot-out losses, so "drawn" there counts shoot-out wins (inference from the fit). Swedish Second Division Norrland's second-stage tables fit neither (carried-over points, guess). Validation must therefore never check points.
- Row base: ref team; `u8 position` (0-based, equals the row index in the first list) and 5 more `u8` (previous positions or highs and lows, guess); `u8`; 4 × `u8`; 3 × `u8`; `i32`; 5 stat records (overall, home, away, overall again, one more; meanings after the first three are guesses); `u8 flag`, when set `u8 n` + n × 2 stat records (head-to-head against each opponent); `u8, u8, u8, u16, 7 × u8`. A nested row base follows a `u8 1` flag only when the base is read as nested.
- Row (after the base): `i16 n` + n × `u8` (position after each round, `0xff` = not yet; 34 for Ligue 1), `i8 m` + m × (`i32, u16, u8, u8`), `u8 flag` (when 1 a sub-object, `0x4077bf0`, not decoded; never set in the tables seen), 10 × `u8` (form: the last ten results, most recent first, 3 = win, 2 = draw, 1 = loss; verified on all 18 Ligue 1 rows of KŽ against their fixtures), `u8 flag` (1 = a nested row follows).
- Stage name: the `i16` 17 bytes before the first list's count is the stage's STAGE_NAME index (−1 = none). It matches the stage of the table's fixtures for 369 of 374 KŽ tables; the 5 others are tables whose teams also met in a later knockout round, which confuses the check, not the field.
- Coverage by a validated scan (row count 1–64, every row parses, position = index, overall = home + away, played = won + drawn + lost + shoot-out losses; 2 rows minimum to avoid one-row false hits): test save 238 tables against 240 LEAGUE_STAGE objects (memory pools); KŽ 603 against 645. For 235 and 583 of them the partner list follows directly with the same teams. The gap is not explained yet (stages without rows, stages of other classes, `SUB_LEAGUE_STAGE` and `VIRTUAL_LEAGUE_STAGE`, guess).
- The gap does not hit live leagues. Every current-season round-robin fixture group of 6+ teams (all pairs scheduled) has its teams covered by found tables: KŽ 216 of 217 (the miss: South American U20 Championship, 6 teams, 15 fixtures), test 74 of 74. MLS is two 15-team conference tables; no 30-team overall table is stored.
- **Sub-competitions have no file of their own.** Their tables sit in the parent competition's file: Liga MX Opening and Closing Stage in Mexican First Division's file, English U18 Division 1 North and South in English U18 Division 1's, Spanish U19 Division 1 Groups 1–7 in Spanish U19 Division 1's. These tables carry no stage name (−1) and no COMP index within 200 bytes before them. Opening and Closing hold the same teams, so team sets alone cannot tell them apart.
- Example (KŽ, Ligue 1, 2035-12-02): 1st team 685, 15 played, 10-3-2, 28–12, 33 pts; 2nd FC Metz (652), 7-6-2, 34–22, 27 pts; home 4-2-1, away 3-4-1.
- Comp files total 60 MB decompressed in KŽ (1 252 files), 15 MB in the test save.

## Behaviour

Tool names, inputs and output shapes are the builder's call within these rules; the builder follows the existing tools' conventions (`team_index`, `limit`, warnings, server instructions).

### Fixtures

- One tool lists fixtures. Filters: team (default the human's first team), competition, date range, played / unplayed, and a limit. Default view answers "what are my next matches and my last results": upcoming fixtures from the game date and recent results.
- Each fixture: date, competition name, stage label (B, C and D joined, e.g. "Playoffs – Semi Final – First Leg"), home and away team name and `team_index`, and for played matches the 90-minute score, plus extra-time and penalty scores when set. For a two-division playoff, both competitions are named.
- Results are public: fog of war does not hide them.

### Tables

- One tool returns tables. Input: a competition, or a team (default the human's first team, whose current league table is returned). A competition with several stage tables returns all of them, each with its stage name.
- Each row: position (1-based), team name and `team_index`, played, won, drawn, lost, goals for, against, difference, points (as stored), and home and away splits. Shoot-out losses are shown only when non-zero, with the label made clear in the tool description. Form (last ten results, most recent first) is included; values other than 1–3 (no result yet) are left out of it.
- A table's label is its competition plus its stored stage name: "Group A", not "Group Stage – Group A". It is not completed from fixtures (builder ruling; the stored name is the deepest level, as the 369 matches show). Tables without a stored name are labelled by their competition.
- **Sub-competition tables (manager's decision, 2026-10-24: match by results).** A table found in a parent competition's file is labelled with the sub-competition (an ACTUAL_COMP_ID key's COMP) whose played current-season fixtures add up exactly to every row's played, won, drawn, lost, goals for and against. Exactly one key must match; otherwise the table keeps the parent competition's name. Before any match is played, tables with the same teams (Opening and Closing) cannot be matched and keep the parent's name. The response says which way a label was found.
- When a requested competition has no table, the tool says so and names the warning; it never computes one.
- Playoff and knockout stages without a table are served through fixtures by stage label. Playoff tables (Jupiler Playoffs, Playoff Table, mini leagues) are ordinary stage tables.

### Finding competitions

- The assistant needs competition names to pass. The builder decides how: a search tool, names accepted by the fixture and table tools, or both. Names come from the language database (already loaded for team divisions).

## Constraints

- Layering stays as in `CLAUDE.md`: `savefile` parses (fix_man.dat, ACTUAL_COMP_ID, STAGE_NAME, comp-file tables, `8_23.dat`), `fm` builds domain models, `mcpserver` holds no parsing.
- Saves and game files are read only. Real saves are never touched; experiments use copies in `$TMPDIR`.
- Load time: decompressing all comp files and the 31.5 MB fixture file on every load is a real cost on the corporate-managed Mac (`CLAUDE.md`, "Gotchas"). The builder measures it on the KŽ copy and loads lazily (on first use) if it adds more than a few seconds.
- Robustness: every count and length read from the save is capped (`docs/format/container.md`, "Robustness"); table location must be bounded like the nations readers (`internal/savefile/nations.go`).
- Format drift: the test save is 26.2.0, the real save 26.3.2. Both must pass.
- Unresolved data degrades with a load warning, never a failed load (as the other readers do).

## Left to the builder

- How tables are located in a comp file: a full comp-file walk (deterministic) or the validated scan described above. Either reports its count against the LEAGUE_STAGE pool as a warning; the gap does not block shipping, since live leagues are covered.
- Tool names, inputs, output field names and defaults; whether `get_last_lineup` gains the match date from the fixture list.
- Eager or lazy loading. When lazy, the fixture and table tools return their own warnings; `game_overview` need not show them before first use (builder ruling).
- The PR breakdown (shown before code, per `CLAUDE.md`).

## Docs to update

- `docs/format/fixtures.md` (new): `fix_man.dat`, FIXTURE, FIXTURE_RESULT.
- `docs/format/competitions.md` (new): ACTUAL_COMP_ID, STAGE_NAME, LEAGUE_STAGE table rows; index entries in `docs/format.md`.
- `docs/format/language-db.md`: stage names (`8_23.dat`).
- `docs/format/clubs.md`: the COMP paragraph points to the new file.
- `docs/backlog.md`: move "Fixtures, results, league tables" out of the open list; record what stays open (second row list, fixture events, the LEAGUE_STAGE gap).

## Acceptance

- Test save: Premier League table has 20 rows, all with 0 played, in the game's pre-season order; Tottenham's first league fixture of 2025/26 is listed with the right opponent and date (check against FM's 2025/26 fixture list in the save).
- KŽ copy: the Ligue 1 table matches the decoded example (685 first on 33 pts, Metz second on 27, 34–22); Metz's next fixtures and last results match the game (screenshot to be taken by the manager); the Championship's 2034 playoff semi-finals and final are listed with their stage labels.
- No fixture with an unresolvable competition key and no table that fails validation is served; both are counted in load warnings.
