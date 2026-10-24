# Fixtures and League Tables Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Two new MCP tools: `get_fixtures`, which lists any team's or competition's fixtures and results with full stage labels, and `get_table`, which returns the stored league tables (groups, conferences and playoff tables included), each correctly labelled.

**Architecture:** `savefile` reads five structures: ACTUAL_COMP_ID and STAGE_NAME from `game_db.dat`, `8_23.dat` from the language database, `rgman/fix_man.dat`, and the table rows in `rgman/comp_<uid>.dat`. `fm` resolves keys to labels at load, and keeps sorted fixtures and labelled tables in `Game`. It labels sub-competition tables by matching them against fixtures. `mcpserver` adds two thin handlers.

**Tech Stack:** Go, `github.com/modelcontextprotocol/go-sdk`, `github.com/klauspost/compress/zstd`.

**Spec:** `docs/superpowers/specs/2026-10-24-fixtures-tables-design.md`. The Python prototypes are in the session scratchpad (`fixwalk.py`, `acid.py`, `ltable.py`, `$TMPDIR/subcomp2.py`, `$TMPDIR/unplayed.py`). They are evidence only. The spec and this plan are authoritative.

## Global Constraints

- **No git:** the repo has no commits, so run no git command that writes. A task is "done" when its tests pass and lint is clean.
- **Build and test:** `GOCACHE=$TMPDIR/gocache go test ./...`.
- **Real-save tests:** add `FM_REAL_SAVE=$PWD/data/kz-2035-12-02.fm` (game date 2035-12-02). They skip when the variable is unset.
- **Lint:** `GOCACHE=$TMPDIR/gocache GOLANGCI_LINT_CACHE=$TMPDIR/lintcache golangci-lint run --allow-parallel-runners` must print `0 issues.`
- **TDD:** write the failing test first, in every task.
- **Read only:**
  - Never write, move or touch anything under `~/Library/Application Support/Sports Interactive/`.
  - Saves and game files are only ever read.
- **Layering:** `savefile` parses and knows nothing about MCP; `fm` builds the models; `mcpserver` holds no parsing.
- **Caps:** cap every count and length read from the save (`docs/format/container.md`, "Robustness"). Bound every scan by a candidate limit, as `internal/savefile/nations.go` does.
- **Degrade, don't fail:** a structure that cannot be read becomes a load warning (`Info.Warnings`), never a failed `Load`.
- **Loading is eager:**
  - Measured on the KŽ copy, decompressing `fix_man.dat` (31.5 MB) and all 1 238 comp files (60 MB) takes 80 ms; `game_db.dat` alone takes 325 ms.
  - The spec's lazy-loading condition ("more than a few seconds") is therefore not met.
  - Warnings go in `game_overview` like every other reader's.
  - Re-check the whole-`Load` time in Task 8. If fixtures and tables together add more than 2 s, stop and report.
- **Fog of war hides nothing here:** fixtures, results and tables are public.
- **Never serve unvalidated data:**
  - No table that fails validation.
  - No fixture whose competition key does not resolve.
  - Both are counted in warnings.
- **Docs:** record every decoded structure in `docs/format/` with offsets, evidence and the build it was verified on, and add index rows in `docs/format.md`. Use the `/decode-structure` skill.
- **Comments:** describe current behaviour only.
- **Clean-up:** delete `.tmpprobe/` (a throwaway probe) before the first review agent runs.

## Review Focus

1. **Stage-name field holds garbage.** The `i16` 17 bytes before a table can be out of range: 2025, −24320 and −248 were seen in KŽ.
   - Expected: any value other than −1 or 0..stage count−1 means "no stage name". It is never a lookup panic or a wrong name.
   - Test in Task 6.
2. **The partner row list is mistaken for a second table.** Each table is followed by the same rows in another order, and that list validates on its own: 407 raw hits against 238 tables in the test save.
   - Expected: a list that directly follows a table and has the same team set is consumed, not returned.
   - Test in Task 6.
3. **Pre-season tables** (test save; all played = 0).
   - Expected: no results matching. Labels come from team sets.
   - Opening and Closing hold the same teams, so their match is ambiguous and both keep the parent competition's name.
   - Test in Task 9.
4. **A competition name matches several competitions, or none** (e.g. "premier league" matches English, Scottish and others).
   - Expected: an exact (folded) name match wins.
   - Otherwise the error lists up to 10 candidates with their `competition_index`; it never picks one silently.
   - Test in Task 4.
5. **A team with no stored table** (cup-only, or the South American U20 Championship).
   - Expected: `get_table` says no table is stored, names the table warning when there is one, and never computes a table.
   - Test in Task 7.

---

## PR breakdown (advisory on this profile; no git)

| PR | Tasks | What | Est. diff | Depends on |
|---|---|---|---|---|
| 1 | 1, 2 | Competition keys, stage names and the `8_23.dat` reader, resolved to labels in `fm`; docs | ~450 | independent |
| 2 | 3, 4, 5 | Fixtures end to end: `fix_man.dat` reader, `fm` fixtures, `get_fixtures`; docs | ~750 | 1 |
| 3 | 6, 7, 8 | Tables end to end: comp-file reader, `fm` tables with file and stage labels, `get_table`; docs | ~800 | 1 |
| 4 | 9 | Sub-competition labels by matching results and team sets; acceptance; backlog | ~350 | 2, 3 |

PR 1 is a shared primitive: fixtures and tables both need keys and stage names, so it lands alone. PRs 2 and 3 exceed the 500-line soft limit; I am setting the size rule aside for them. Splitting either into a reader and its tool would leave the reader with no caller, and each already has its own fuzz target and real-save tests. PR 3 depends only on PR 1, so PRs 2 and 3 can be built in parallel.

---

### Task 1: Competition keys and stage names in `game_db.dat`, and stage names in the language database

**Files:**
- Create: `internal/savefile/competitions.go`, `internal/savefile/competitions_test.go`
- Modify:
  - `internal/savefile/gamedb.go` (`GameDB` fields, `ReadGameDB`)
  - `internal/savefile/langdb.go` (`Stages`)
  - `internal/savefile/fuzz_test.go`
  - `internal/savefile/integration_test.go`
  - `internal/savefile/langdb_test.go`
- Docs:
  - `docs/format/competitions.md` (new): sections "Competition keys (ACTUAL_COMP_ID)" and "Stage names (STAGE_NAME)"
  - `docs/format/language-db.md`: `8_23.dat`
  - `docs/format/clubs.md`: the COMP paragraph points to the new file
  - `docs/format.md`: an index row

**Interfaces:**
- Produces:
  - `type CompetitionKey struct { Comp, Stage, Group, Leg, SecondComp int32 }`. The fields are A–E. Comp and SecondComp are COMP indices, and SecondComp is −1 = none. Stage, Group and Leg are STAGE_NAME indices, −1 = none.
  - `GameDB.CompetitionKeys []CompetitionKey`, indexed by key index, which is a fixture's competition ref. `GameDB.CompetitionKeysErr error` holds why it is empty.
  - `GameDB.StageUIDs []uint32`, indexed by stage index. `GameDB.StagesErr error` holds why it is empty.
  - `GameDB.LeagueStageCount uint32`: the `LEAGUE_STAGE` memory pool (240 test, 645 KŽ).
  - `func (l *LangDB) Stages() ([]LangName, error)`, which reads `8_23.dat`.
  - `const langStagesFile = "8_23.dat"`.

- [ ] **Step 1: Write the failing tests.**
  - **`TestReadCompetitionKeys`:** a synthetic body holding a `tableEnd` marker + `u32 2, u8 0`, then two 33-byte records (`u8 0`, five `i32`, then `u32 index, u32 index+1, u32 index+1`) and a trailing `tableEnd`.
    - It returns `[{267,-1,-1,-1,-1},{8,236,17,2,-1}]`.
    - The same body with the second record's id ≠ index+1 returns `ErrNotFound`.
    - A count above `maxCompetitionKeys` (1 << 20) is never parsed.
  - **`TestReadStageNames`:** a synthetic STAGE_NAME table (`u32 3, u8 0`, records `u8 0, u8 0, u32 index, u32 uid, u32 uid`, then `tableEnd`) with count 3 returns the uids `[1, 3, 238]`.
  - **`TestLangStages`:** a synthetic `8_23.dat`. Records are `u32 uid, string name, u8, string, u8, string, u8, u8`. Uid 238 "Playoffs" round-trips.
  - **`TestTestSaveCompetitionKeys`** (test save):
    - `len(CompetitionKeys) == 3360`; `len(StageUIDs) == 285`; `LeagueStageCount == 240`.
    - Every non-negative Comp and SecondComp is `< len(CompetitionUIDs)`.
    - Every non-negative Stage, Group and Leg is `< 285`.
  - **`TestRealSaveCompetitionKeys`** (`FM_REAL_SAVE`):
    - `len == 7753`.
    - Key 5618 == `{8, 236, 17, 2, -1}`; key 5724 == `{8, 236, 19, -1, -1}`.
    - `StageUIDs[236] == 238`, `[17] == 18`, `[2] == 3`, `[275] == 2000223075`.
  - **`FuzzReadCompetitionKeys`:** must not panic or allocate beyond the caps.

- [ ] **Step 2: Run** `go test ./internal/savefile -run 'CompetitionKeys|StageNames|LangStages'`. Expected: FAIL (undefined).

- [ ] **Step 3: Implement.**
  - **`readCompetitionKeys(body []byte) ([]CompetitionKey, error)`:** the table has no memory pool, so locate it by its first record.
    1. Search for the bytes `00 00 00 00 01 00 00 00 01 00 00 00` (record 0's database record).
    2. Take `p = hit − 26` as the head.
    3. Require `afterTableEnd(body, p)`, `body[p+4] == 0` and `body[p+5] == 0`.
    4. Parse `count` records strictly (tag 0, index == i, both ids == i+1), followed by `tableEnd`.
    5. Accept exactly one parsing table; try at most `maxNationCandidates` heads.
  - **`readStageNames(body []byte, count uint32) ([]uint32, error)`:** use `readTable` with tag 0 and the 14-byte record. The count is the `STAGE_NAME` pool.
  - **Wiring:** in `ReadGameDB`, set the new fields; errors go to `*Err`, never failing `ReadGameDB`.
  - **`Stages`:** follows `Injuries`' pattern.

- [ ] **Step 4: Run** `go test ./internal/savefile` with and without `FM_REAL_SAVE`. Expected: PASS.

- [ ] **Step 5: Docs.** Write the two `competitions.md` sections from the spec's "Competition keys" and "Stage names", including the examples above and the stage-index rule (stage uid = `StageUIDs[index]`). Add the `language-db.md` entry and the `clubs.md` pointer. Run lint.

### Task 2: Labels in `fm`

**Files:**
- Create: `internal/fm/competitions.go`, `internal/fm/competitions_test.go`
- Modify:
  - `internal/fm/load.go` (`langNames.stages`, `langTables.Stages`, `buildWorld`, warnings)
  - `internal/fm/world.go` (`World` fields; `Team.divisionIndex`)
  - `internal/fm/load_test.go`

**Interfaces:**
- Consumes: Task 1.
- Produces:
  - `World.Stages map[int64]string`: stage index → name. The save's uid is looked up in the language table by uid, not by rank.
  - `World.Keys []savefile.CompetitionKey`.
  - `Team.divisionIndex int64` (unexported, `json:"-"`): the TEAM record's division COMP index, −1 when none.
  - `type competitionLabel struct { Index int64; Name string; Stage string; SecondIndex int64; Second string }`.
  - `func (w World) keyLabel(key int32) (competitionLabel, bool)`. It is false when the key is out of range or its Comp is not a COMP index.
    - Stage joins the names of Stage, Group and Leg that are set with " – " (U+2013 with spaces), e.g. "Playoffs – Semi Final – First Leg".
    - An unknown stage index is skipped.
  - `func (w World) stageName(i int64) string`: "" for −1 or unknown.
  - New load warnings:
    - `"competition keys unavailable, so fixtures cannot be read: <err>"`
    - `"stage names unavailable, so fixture and table stages are unlabelled: <err>"`

- [ ] **Step 1: Write the failing tests.**
  - **`TestKeyLabel`:** a World with competitions `{8: "Sky Bet Championship"}`, stages `{236: "Playoffs", 17: "Semi Final", 2: "First Leg"}` and key `{8,236,17,2,-1}`.
    - The key gives `Name "Sky Bet Championship"`, `Stage "Playoffs – Semi Final – First Leg"` and `SecondIndex -1`.
    - Key `{8,-1,19,-1,-1}` with stage 19 unknown gives `Stage ""`.
    - Key index −1 and key index `len` give false.
    - A two-division key `{11,-1,-1,-1,12}` gives both names.
  - **`TestLoadStageNames`** (test save with the game's database dir, skipping when the game is not installed, as the existing language tests do): `World.Stages[236] == "Playoffs"`.

- [ ] **Step 2: Run** the tests. Expected: FAIL.

- [ ] **Step 3: Implement** the interfaces above. `readLang` also reads `Stages()`. Stage names are mapped by uid through `db.StageUIDs`, so a stage index without a uid has no name.

- [ ] **Step 4: Run** `go test ./internal/fm` and lint. Expected: PASS, `0 issues.`

### Task 3: `rgman/fix_man.dat` reader

**Files:**
- Create: `internal/savefile/fixtures.go`, `internal/savefile/fixtures_test.go`
- Modify: `internal/savefile/fuzz_test.go`, `internal/savefile/integration_test.go`
- Docs: `docs/format/fixtures.md` (new), with sections "fix_man.dat", "FIXTURE" and "FIXTURE_RESULT"; an index row in `docs/format.md`

**Interfaces:**
- Produces:
  - `type Fixture struct { Key int32; Home, Away int32; Date Date; Season uint16; Round uint8; Result *FixtureResult }`
    - Key is an ACTUAL_COMP_ID index; Home and Away are TEAM indices.
    - Season is the season's start year; 0 for friendlies.
    - Round is the base byte at `+0x3b`; 255 for friendlies.
  - `type FixtureResult struct { Home, Away [4]uint8 }`. Byte 0 is 90 minutes, 1 after extra time, 2 penalties, 3 the aggregate after a second leg; `0xff` = none.
  - `func ReadFixtures(a *Archive) ([]Fixture, error)` and `func parseFixtures(body []byte) ([]Fixture, error)`.

**Grammar** (verified by `fixwalk.py` on both saves):
- **Header:** `u32, u32, u16, u32`. Then 1 list, `u32 n`, then n more lists.
- **List:** `u16 season key, u8 present`. When present: `u32, u32`, then items until `i32 −1`.
  - Item: `i32 index, u8 flag`. Flag 1 means a FIXTURE_RESULT followed by the base. Flag 0 means the base only. Any other flag is corrupt.
  - The list ends with a `u32` equal to its item count.
- **Base** (63 bytes):

  ```
  u8 version, u32 key, u32 stadium, u16, u8, u32 home, u8, u8, u8, u32 away, u8, u8,
  date, date, u32, u8, u16 season, u32, u32 referee, u8, u8, u8 round, u8, u16, 5 bytes, u16, u8, u8
  ```

  - The first date is the match date. Verified: Tottenham v Burnley on day 228 of 2025 = 2025-08-16, the real 2025/26 opening fixture.
  - The second date's meaning is unknown.
- **Result:** `u8 version, u32, 4 bytes home, 4 bytes away, u32 attendance (guess), u32, u8, u8, u8, u8, u32 n + n × 16-byte events`.
- **Ignored data:**
  - Bytes after the last list (5 993 in the test save) are not decoded and are ignored.
  - Caps: lists ≤ 64, items per list ≤ 1 << 20, events ≤ 1 024.

- [ ] **Step 1: Write the failing tests.**
  - **`TestParseFixtures`:** a synthetic file with one list (season key 2025) and two items, one played (home `[2,0xff,0xff,0xff]`, away `[1,0xff,0xff,0xff]`, 1 event) and one unplayed.
    - It returns both, with Key, Home, Away, Date, Season and Result decoded.
    - A wrong count `u32` returns `ErrCorrupt`; flag 2 returns `ErrCorrupt`.
  - **`TestTestSaveFixtures`:**
    - 19 239 fixtures.
    - The fixture with Key 232 (Premier League), Season 2025 and Round 0 involving team 547 is home 547 v away 442, dated 2025-08-16.
  - **`TestRealSaveFixtures`** (`FM_REAL_SAVE`):
    - 138 714 fixtures.
    - The Season 2033 fixture with Key 5619 and Home 520 has home `[0,0,4,1]` and away `[0,0,1,1]`: 0–0 after extra time, 4–1 on penalties, 1–1 on aggregate.
    - Every Key is `< 7753`.
  - **`FuzzParseFixtures`.**

- [ ] **Step 2: Run** the tests. Expected: FAIL.
- [ ] **Step 3: Implement** with `reader`. The first failure aborts with the offset in the error.
- [ ] **Step 4: Run** the tests, the fuzz target for 30 s and lint. Expected: PASS.
- [ ] **Step 5: Docs.** Write `fixtures.md` from the spec's "Fixtures" plus the byte 3 = aggregate finding.
  - Evidence (KŽ): Leeds 0–2 Fulham on 2035-05-18 carries aggregate 0–4 after Fulham's 2–0 first-leg win.
  - Code addresses are from the spec.

### Task 4: Fixtures in `fm`

**Files:**
- Create: `internal/fm/fixtures.go`, `internal/fm/fixtures_test.go`
- Modify:
  - `internal/fm/load.go` (read fixtures, warnings)
  - `internal/fm/game.go` (`Game.fixtures`, `PlayerExtras.Fixtures`)

**Interfaces:**
- Consumes: Task 2 (`keyLabel`); Task 3 (`ReadFixtures`).
- Produces:
  - **`Fixture`:**

    ```go
    type Fixture struct {
        Date             time.Time `json:"date"`
        Season           int       `json:"season,omitempty"`
        CompetitionIndex int64     `json:"competition_index"`
        Competition      string    `json:"competition,omitempty"`
        Stage            string    `json:"stage,omitempty"`
        SecondCompetitionIndex *int64 `json:"second_competition_index,omitempty"`
        SecondCompetition      string `json:"second_competition,omitempty"`
        HomeTeamIndex    int64     `json:"home_team_index"`
        HomeTeam         string    `json:"home_team,omitempty"`
        AwayTeamIndex    int64     `json:"away_team_index"`
        AwayTeam         string    `json:"away_team,omitempty"`
        Score            *Score    `json:"score,omitempty"` // nil until played
        key              int32     // ACTUAL_COMP_ID index, for Task 9
        result           *savefile.FixtureResult
    }
    ```

  - **`Score`:** `type Score struct { Home, Away int; ExtraTime, Penalties, Aggregate *[2]int }`. JSON names: `home`, `away`, `after_extra_time`, `penalties`, `aggregate`. A part is omitted when its bytes are `0xff`.
  - **`FixtureFilter`:** `type FixtureFilter struct { Team *int64; Competition *int64; From, To time.Time; Status string; Limit int }`. Status is `"played"`, `"upcoming"` or `""`.
  - **`FixtureList`:** `type FixtureList struct { Results []Fixture; Upcoming []Fixture; TotalResults, TotalUpcoming int }`. JSON names: `results`, `upcoming`, `total_results`, `total_upcoming`.
  - **`func (v View) Fixtures(f FixtureFilter) (FixtureList, error)`.**
    - Results are fixtures with a score, most recent first. Upcoming are the rest, soonest first. Each list is cut to Limit (default 10, max `maxLimit`); totals count before the cut.
    - **No filter:** with neither Team nor Competition set, Team defaults to the manager's team. If that is unknown, it returns `ErrInvalidFilter` saying so and asking for team_index.
    - **Team:** must exist (`checkTeam`).
    - **Competition:** a fixture matches when its Comp or SecondComp equals it.
    - **Dates:** From and To are inclusive.
  - **`func (v View) FindCompetition(name string) (int64, error)`.**
    - It folds both sides with `normalize`. Exactly one exact name match wins.
    - Otherwise every competition whose name contains each query word as a word prefix is a candidate (`nameWords`). One candidate wins.
    - With none or several, it returns `ErrInvalidFilter` whose message lists up to 10 `"<name> (competition_index N)"`.
  - **Warnings:**
    - `"fixtures unavailable: <err>"`
    - `"%d fixtures left out because their competition key does not resolve"` (only when > 0)

- [ ] **Step 1: Write the failing tests.**
  - **`TestFixturesDefaultView`:** a synthetic Game with the manager's team 1, game date 2025-08-20, three played and two upcoming fixtures, and Limit 2.
    - Results hold the two latest played, newest first; Upcoming holds both upcoming, soonest first.
    - `TotalResults` is 3.
  - **`TestFixturesCompetitionWithoutTeam`:** returns every team's fixtures in that competition, including a two-division playoff via SecondComp.
  - **`TestFixtureScoreParts`:** bytes `[0,0,4,1]` / `[0,0,1,1]` give `Score{0,0, ExtraTime:&[0 0], Penalties:&[4 1], Aggregate:&[1 1]}`; `[2,0xff,0xff,0xff]` gives no parts.
  - **`TestFindCompetition`:**
    - "sky bet championship" gives index 8.
    - "premier league" with "Premier League" and "Scottish Premier League" present gives the exact match.
    - "league" with three matches gives an error containing `competition_index` and every candidate name.
    - "zzz" gives `ErrInvalidFilter`.
  - **`TestUnresolvedKeysWarned`:** a fixture with key 99 999 is left out, and the warning says 1.
  - **`TestTestSaveTottenhamFirstLeague`:**
    - The test save, with Team 547 and Competition set to Tottenham's division (`divisionIndex`) via the filter.
    - The first Upcoming entry is 2025-08-16, home "Tottenham Hotspur", away "Burnley", no Score.
  - **`TestRealSaveMetzFixtures`** (`FM_REAL_SAVE`, Team 652, Limit 3):
    - **Results:** 2035-12-02 Olympique de Marseille 1–3 FC Metz; 2035-11-29 Molde FK 1–6 FC Metz, with Stage "League Phase" and Competition "UEFA Europa League"; 2035-11-25 FC Metz 5–1 Havre AC.
    - **Upcoming:** 2035-12-09 FC Metz v Stade de Reims; 2035-12-13 FC Metz v Víkingur R.; 2036-01-06 ESTAC Troyes v FC Metz.
  - **`TestRealSaveChampionshipPlayoffs`** (`FM_REAL_SAVE`; Competition = "Sky Bet Championship", From 2035-05-01, To 2035-05-31, Status "played"):
    - Five results.
    - 2035-05-13 Crystal Palace 4–1 Blackburn Rovers, "Playoffs – Semi Final – First Leg".
    - 2035-05-19 Blackburn Rovers 0–1 Crystal Palace, "Playoffs – Semi Final – Second Leg", Aggregate `[1 5]`.
    - 2035-05-28 Crystal Palace 2–1 Fulham, "Playoffs – Final".

- [ ] **Step 2: Run** the tests. Expected: FAIL.

- [ ] **Step 3: Implement.**
  - **At load:** convert the raw fixtures through `keyLabel` and team names, sorted by date then by home team index.
  - **Memory:** 138 714 fixtures are about 14 MB; keep the resolved strings shared, so labels are interned per key.

- [ ] **Step 4: Run** `go test ./internal/fm` (with `FM_REAL_SAVE`) and lint. Expected: PASS. The Metz values were read from bytes only; the manager confirms them against an in-game screenshot after Task 9.

### Task 5: `get_fixtures` tool

**Files:**
- Modify: `internal/mcpserver/server.go`, `internal/mcpserver/server_test.go`

**Interfaces:**
- Consumes: `View.Fixtures` and `View.FindCompetition` from Task 4.
- **Input:** `fixturesIn`:
  - `team_index *int64`
  - `competition string`
  - `competition_index *int64`
  - `from string` and `to string` (`YYYY-MM-DD`; a bad date returns an error that names the field)
  - `status string` (`played` / `upcoming`)
  - `limit int`
- **Output:** `fixturesOut{ saveNotes; fm.FixtureList }`.
- **Description** (exact copy): "Fixtures and results for a team or a competition. Without team_index or a competition, the human manager's first team. results are played matches, most recent first; upcoming are the rest, soonest first; each list holds up to limit (default 10, max 200), totals count all. Each fixture has the date, competition, stage (e.g. \"Playoffs – Semi Final – First Leg\"), home and away team with team_index, and for played matches the score after 90 minutes plus after_extra_time, penalties and aggregate when they apply. competition takes a name (an exact name wins; otherwise the error lists matches with competition_index). Fog of war does not hide fixtures."
- **Server instructions:** add one paragraph covering:
  - get_fixtures and get_table exist;
  - stage labels;
  - where competition names come from: fixtures, a team's division, or the error's list.
- **`get_last_lineup`:** its description and instructions lose "the next fixture is not known". Point to get_fixtures instead.

- [ ] **Step 1: Write the failing tests:** `TestGetFixturesDefaultsToManager`, `TestGetFixturesBadDate` and `TestGetFixturesCompetitionName`, in the style of the existing handler tests.
- [ ] **Step 2: Run.** Expected: FAIL.
- [ ] **Step 3: Implement** the handler (no logic beyond parsing inputs) and register the tool.
- [ ] **Step 4: Run** `go test ./...` and lint. Expected: PASS, `0 issues.`

### Task 6: League tables in comp files

**Files:**
- Create: `internal/savefile/tables.go`, `internal/savefile/tables_test.go`
- Modify: `internal/savefile/fuzz_test.go`, `internal/savefile/integration_test.go`
- Docs: `docs/format/competitions.md`, section "League tables (LEAGUE_STAGE rows)", from the spec's "League tables" (rows, stat record, form, stage-name field, coverage, the partner list)

**Interfaces:**
- Produces:
  - **`LeagueTable`:** `type LeagueTable struct { CompetitionUID uint32; Stage int16; Rows []TableRow }`. CompetitionUID comes from the file name. Stage is −1 when it is unset or out of range.
  - **`TableRow`:** `type TableRow struct { Team int32; Position uint8; Overall, Home, Away TableStats; Form [10]uint8 }`. Position is 0-based. Form is most recent first: 3 W, 2 D, 1 L, other values = none.
  - **`TableStats`:** `type TableStats struct { Played, Won, Drawn, Lost, ShootoutLosses uint8; GoalsFor, GoalsAgainst uint16; Points int16 }`.
  - **`func ReadLeagueTables(a *Archive, stageCount int) ([]LeagueTable, error)`:** reads every `rgman/comp_<digits>.dat`.
  - **`func parseLeagueTables(body []byte, uid uint32, stageCount int) []LeagueTable`.**

**Algorithm** (a validated scan, chosen over a full comp-file walk because live leagues are covered; see the spec's coverage):
- **Candidates:**
  - Find each `00 ff ff ff ff` (the first stat record's ref with team −1).
  - The candidate count byte is at `o = hit − 24`.
  - Require `body[o]` in 1..64, `body[o+1] == 0` and `body[o+6] == 0` (position 0).
- **Parse** the list with the row grammar of the spec's "League tables". Inside it:
  - head-to-head n ≤ 64;
  - round history n ≤ 255;
  - the `i8 m` list ≤ 64;
  - a nested row chain ≤ 8 deep;
  - a sub-object flag of 1 rejects the candidate.
- **Accept** when there are 2 or more rows and, for every row i:
  - team ≥ 0;
  - position == i;
  - overall == home + away for played, won, drawn, lost, goals for and goals against;
  - played == won + drawn + lost + shoot-out losses.
- **Never** check points.
- **Stage:** the `i16` at `o − 17`. Keep it only when it is −1 or in 0..stageCount−1; otherwise −1.
- **Partner list:** after an accepted table, if a list parses at its end with the same team set, skip past it. Resume the scan after the last consumed byte.
- **Caps:** at most 4 096 candidates parsed per file; the rest are skipped and counted.
- **Return value:** `ReadLeagueTables` returns the tables and an error only when no comp file could be read.

- [ ] **Step 1: Write the failing tests.**
  - **`TestParseLeagueTable`:** a synthetic body, built by a test encoder that writes rows in the documented grammar: a 17-byte prefix with stage 74, a 3-row table, then its partner list in another order.
    - It returns one table with Stage 74, positions 0..2, stats and form.
  - **`TestParseLeagueTableRejects`:**
    - position ≠ index;
    - overall ≠ home + away;
    - played ≠ W+D+L+X;
    - a one-row table;
    - a sub-object flag of 1.
  - **`TestParseLeagueTableBadStage`:** stage bytes 2025 with a stage count of 285 give Stage −1.
  - **`TestTestSaveLeagueTables`:**
    - 238 tables.
    - The table in Premier League's file has 20 rows, all with Played 0.
  - **`TestRealSaveLigue1`** (`FM_REAL_SAVE`):
    - 603 tables.
    - In the Ligue 1 file, row 0 is team 685: 15 played, 10-3-2, 28–12, 33 points.
    - Row 1 is team 652: 7-6-2, 34–22, 27 points; home 4-2-1, away 3-4-1.
    - Row 0's first seven Form values equal the W/D/L (3/2/1) of team 685's seven latest played Ligue 1 fixtures (`ReadFixtures`, keys whose Comp is Ligue 1's index).
  - **`FuzzParseLeagueTables`.**

- [ ] **Step 2: Run** the tests. Expected: FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** the tests, the fuzz target for 30 s and lint. Time `ReadLeagueTables` on the KŽ copy in the test log; expected well under 1 s.

### Task 7: Tables in `fm`

**Files:**
- Create: `internal/fm/tables.go`, `internal/fm/tables_test.go`
- Modify: `internal/fm/load.go`, `internal/fm/game.go`

**Interfaces:**
- Consumes: Task 2 (`World.Stages`, `divisionIndex`); Task 6.
- Produces:
  - **`Table`:**

    ```go
    type Table struct {
        CompetitionIndex int64      `json:"competition_index"`
        Competition      string     `json:"competition,omitempty"`
        Stage            string     `json:"stage,omitempty"`
        LabelSource      string     `json:"label_source"` // "file" or "results" (Task 9)
        Rows             []TableRow `json:"rows"`
    }
    ```

  - **`TableRow`:** `type TableRow struct { Position int; TeamIndex int64; Team string; Played, Won, Drawn, Lost, GoalsFor, GoalsAgainst, GoalDifference, Points int; ShootoutLosses int; Home, Away Split; Form string }`.
    - JSON names: `position`, `team_index`, `team`, `played`, `won`, `drawn`, `lost`, `goals_for`, `goals_against`, `goal_difference`, `points`, `shootout_losses` (omitempty), `home`, `away`, `form` (omitempty).
    - Position is 1-based.
    - Form is "WDLWW…", most recent first; values other than 1–3 are dropped.
  - **`Split`:** played, won, drawn, lost, goals_for and goals_against.
  - **`func (v View) Tables(team *int64, competition *int64) ([]Table, error)`:**
    - **Competition set:** every table whose CompetitionIndex is it, in file order.
    - **Team set:** the tables containing the team whose CompetitionIndex is the team's `divisionIndex`; if there are none, every table containing the team.
    - **Neither:** the manager's team.
    - **No match:** `ErrNoTable`, whose message says no table is stored for it. When tables failed to load, the message appends the table warning. It never computes a table.
  - **`var ErrNoTable = errors.New("fm: no stored table")`.**
  - **Label:**
    - the file's COMP (uid → index via `db.CompetitionUIDs`), plus `World.stageName(Stage)`;
    - `LabelSource "file"`;
    - a file uid with no COMP index drops the table into the warning count.
  - **Warnings:**
    - `"league tables unavailable: <err>"`
    - `"%d of %d league stages have no readable table; their competitions show no table"` when found < `LeagueStageCount`

- [ ] **Step 1: Write the failing tests.**
  - **`TestTablesByCompetition`, `TestTablesByTeamDivisionFirst` and `TestTablesNoTable`:** synthetic; `TestTablesNoTable` must check that the error text contains "no table".
  - **`TestTableRowForm`:** bytes `[3,2,1,0,…]` give "WDL".
  - **`TestTestSavePremierLeague`:** Team 547 gives one table, Competition = Tottenham's division name, 20 rows, all with Played 0.
  - **`TestRealSaveLigue1Table`** (`FM_REAL_SAVE`, Team 652):
    - Row 1 is "Stade Rennais FC" on 33 points.
    - Row 2 is "FC Metz" on 27 points, GoalDifference 12.
- [ ] **Step 2: Run.** Expected: FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** the tests and lint. Measure the whole `Load` of the KŽ copy against the time before Task 3 (see Global Constraints).

### Task 8: `get_table` tool

**Files:**
- Modify: `internal/mcpserver/server.go`, `internal/mcpserver/server_test.go`

**Interfaces:**
- **Input:** `tableIn`: `team_index *int64`, `competition string`, `competition_index *int64`.
- **Output:** `tablesOut{ saveNotes; Tables []fm.Table }`.
- **Description** (exact copy): "Stored league tables in the game's order: leagues, groups, conferences, league phases and playoff tables. With a competition, all of its stage tables; with team_index, the team's league table (else every table it is in); by default the human manager's first team. Rows carry position, team and team_index, played, won, drawn, lost, goals for and against, goal_difference, points as the game stores them (deductions and carried-over points included), home and away splits, and form (last ten results, most recent first). shootout_losses appears in cup groups where drawn matches go to penalties: there drawn counts shoot-out wins. label_source \"results\" means the table was matched to a sub-competition by its results or teams; \"file\" means it is labelled by the competition it is stored with. Tables are never computed: when none is stored the tool says so. Knockout rounds have no table; use get_fixtures."
- **`ErrNoTable`:** passed through as the tool error text.

- [ ] **Step 1: Write the failing tests:** `TestGetTableDefaultsToManager` and `TestGetTableNoTable`.
- [ ] **Step 2: Run.** Expected: FAIL.
- [ ] **Step 3: Implement and register.**
- [ ] **Step 4: Run** `go test ./...` and lint.

### Task 9: Sub-competition labels, acceptance and backlog

**Files:**
- Create: `internal/fm/sublabel.go`, `internal/fm/sublabel_test.go`
- Modify: `internal/fm/tables.go` (call it after building tables)
- Docs:
  - `docs/format/competitions.md`: sub-competition tables, with the KŽ and test numbers below
  - `docs/backlog.md`: "Fixtures, results, league tables" leaves the open list. What stays open: the second row list's meaning, fixture events, the LEAGUE_STAGE gap (42 KŽ, 2 test), the stage-name field's garbage values, fixture byte 3 outside second legs.

**Interfaces:**
- Consumes: the resolved fixtures (Task 4) and the tables (Task 7).
- Produces: `func relabel(tables []Table, fixtures []Fixture, keys []savefile.CompetitionKey, …)`, which sets CompetitionIndex, Competition and `LabelSource "results"`. The implementer picks the exact parameters.

**Rule** (manager's decision "match by results", refined by the KŽ and test prototypes; record this refinement in the docs):

1. Group fixtures by (Comp, deepest stage index), where the deepest stage is the key's Group if set, else its Stage. Legs are ignored. Within each group, keep only the latest Season.
2. **Played table** (any row has Played > 0):
   - Candidates are the groups whose stage equals the table's Stage index (−1 included) and whose Comp has a played fixture involving a table team.
   - A candidate matches when, for every row, the played fixtures of that group give exactly the row's played, won, drawn + shoot-out losses, lost, goals for and goals against.
   - A fixture's goals are its after-extra-time score when set, else its 90-minute score. A win or loss is decided on those goals; equal goals count as drawn.
3. **Unplayed table:** a candidate matches when the set of teams in its group's fixtures equals the table's team set.
4. **Result:**
   - Exactly one matching Comp that is not the file's: relabel with that Comp (the Stage name is kept) and `LabelSource "results"`.
   - Otherwise (none, only the file's own, or several): the table keeps the file label.

Prototype outcome:
- **KŽ:** 339 tables confirmed for their file, 120 relabelled, 53 unmatched, no ambiguous played table. Of the unplayed tables, 48 matched their file, 41 were unmatched and 2 were ambiguous (Liga MX Opening/Closing).
- **Test save:** 47 relabelled before the season.

- [ ] **Step 1: Write the failing tests.**
  - **`TestRelabelByResults`:**
    - A parent file "Mexican First Division" holds two 2-team tables with the same teams. One has played rows matching the "Opening Stage" fixtures; the other has 0 played.
    - Expected: the first becomes "Mexican First Division Opening Stage" with source "results"; the second stays "Mexican First Division" with source "file".
  - **`TestRelabelAmbiguous`:** two Comps match, so the file label is kept.
  - **`TestRelabelExtraTime`:** a group match won on penalties after a 1–1 draw counts as drawn.
  - **`TestRealSaveSubCompetitions`** (`FM_REAL_SAVE`):
    - The played table in Mexican First Division's file is "Mexican First Division Opening Stage" (results), and the 0-played one with the same teams keeps "Mexican First Division".
    - Seven tables are labelled "Spanish U19 Division 1 Group 1"…"Group 7".
    - Ligue 1's table keeps source "file".
  - **`TestTestSaveSubCompetitions`:** tables labelled "English U18 Division 1 North" and "English U18 Division 1 South" with source "results".
- [ ] **Step 2: Run.** Expected: FAIL.
- [ ] **Step 3: Implement.** Index the fixtures once by group; work in O(fixtures + table rows).
- [ ] **Step 4: Run.** Do all of the following, in order:
  - `go test ./...` with all `FM_REAL_SAVE*` variables set, and lint.
  - Delete `.tmpprobe/`.
  - Update the docs and the backlog.
- [ ] **Step 5: Acceptance:** list for the manager the values to compare against the game:
  - Metz's last 3 and next 3 fixtures;
  - the top two of Ligue 1;
  - the 2034/35 Championship playoffs.

  The manager confirms them against the game (spec, "Acceptance").
