# Season Stats Detail Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Return the season stats decoded on 2026-10-21 (minutes, player of the match, penalties, ~26 match stats such as fouls, and per-competition rows), fix the injury return upper bound, and decode the missing Champions League qualification bonus.

**Architecture:** `savefile` reads more of each `rgman/player_stats.dat` scope (totals offsets 2, 10–12 and 26 detail fields) and keeps slots 0–3 next to slot 5. `fm` adds `minutes` and `player_of_the_match` to the compact `SeasonStats` on every player row, and a new `season_detail` list (one row per competition group) on `Profile` only, so `get_player` and `compare_players` carry it and list rows stay small. `mcpserver` only updates its instructions.

**Tech Stack:** Go, `github.com/modelcontextprotocol/go-sdk`.

**Spec:** none. This plan carries the design and the in-game evidence (section "Ground truth" below). It comes from the session of 2026-10-21; the screenshots themselves are not in the repo.

## Global Constraints

- No git in this project: "there is no git - just write the code". Skip every commit; never run git commands that write.
- Real saves under `~/Library/Application Support/Sports Interactive/Football Manager 26/cloud/games/` are read-only. Never write, move or touch them. The snapshots this plan uses are copies: `data/kz-2035-12-02.fm` (game date 2035-12-02) and `data/kz-2035-12-09.fm` (2035-12-09), both gitignored.
- Go commands need `GOCACHE=$TMPDIR/gocache`; lint needs `GOLANGCI_LINT_CACHE=$TMPDIR/lintcache golangci-lint run` and must report `0 issues.`.
- Real-save tests: `FM_REAL_SAVE=data/kz-2035-12-02.fm` (existing `TestRealSave`) and `FM_REAL_SAVE_9DEC=data/kz-2035-12-09.fm` (new, Task 1). Use absolute paths when the test changes directory.
- Layering: savefile → fm → mcpserver. mcpserver non-test code imports `fm` only. Parsers know nothing about MCP.
- TDD for every change: watch each new test fail before the code that passes it.
- Comments describe current behaviour only, no "previously"/"now".
- `docs/format.md`: every field gets a status (verified with its evidence, guess, or hypothesis) and the build (26.3.2).
- Never call `set_fog_of_war` unless the user asks.
- Season stats are never hidden by fog of war. Verified 2026-10-21: the game shows full Season Stats for unscouted Haaland and Esposito.
- After all tasks: refactoring-specialist → text-simplificator (code comments and docs, never this plan) → tests-runner + code-reviewer + architecture-reviewer in parallel, until clean. Then rebuild `bin/fm-mcp` (`go build -o bin/fm-mcp ./cmd/fm-mcp`) and tell the user to run `/mcp`.

## Review Focus

- **A player with friendlies only** (pre-season, or a youngster who only played friendlies): the rows' `season` stays absent and `season_detail` holds just the `friendlies` row. Pinned in Task 2.
- **Negative or junk detail values** (the i16s are signed; unexposed fields hold negatives such as −27): an exposed field below 0 is reported as 0 (left out), never as a negative count. Pinned in Task 2.
- **xA rounding**: Matković's stored xA is 268 (2.68) while the game shows 2.67. Report two decimals of the stored value and say in the docs the game can differ by 0.01. Pinned in Task 2's conversion test.
- **Injury boundary**: when the stored earliest return is exactly the day after the game date, the +1 rule applies (it is "earliest is after the game date"). Pinned in Task 4.
- **`compare_players` size**: 10 profiles × up to 5 detail rows. Nothing to pin; keep the detail fields `omitempty` so zero counts cost nothing.

---

## Ground truth (2035-12-09 snapshot, in-game screenshots, 2026-10-21)

Game date 2035-12-09. Detail field `dN` = the N-th i16 (little-endian, signed) of the 120-byte detail block, at byte offset 2N. Per-90 figures in game are `count × 90 / minutes`.

**Totals bytes** (17-byte block): offset 2 u16 = minutes (verified: Matković Ligue 1 minutes per goal 155.43 = 1088 / 7, per assist 272.00 = 1088 / 4); offset 10 = penalties taken, offset 11 = penalties scored (verified by Esposito: Serie A 2/2 matches slot 1 (2, 2); his continental slot has (1, 0) and the game's Champions League Pens column shows 0, the scored count; taken ≥ scored in every scope); offset 12 = player of the match (verified earlier). The game's "Overall (Club)" Pens column shows 3 for Esposito = the taken total, while competition rows show scored (game display quirk, inference).

**Scopes**: slot 0 = friendlies (verified: Matković's slot 0 = the game's "Non Competitive" row 3 (1), 1 goal, 2 assists, 7.25, 1 PoM). Slot 2 = domestic cups (verified: Haaland's slot 2 = Carabao Cup 2 apps, 2 goals, 1 assist, 7.65, 1 pen). Slot 4 = Euro Championship qualifiers for Esposito (6 apps, 7 goals, 2 pens, 2 PoM, exact) but his "International League" (10 apps, 6 goals) and "European League Div. A" (6, 4) rows are in no slot 0–5; so internationals stay out of this plan. Average divisor = rated matches (verified: Bernardi 9 (11), 19 rated, sum 1301 → game 6.85 not 6.51; Ligue 1 6 (8), 13 rated, 877 → 6.75 not 6.26).

**Detail fields** (each verified on all four samples below unless marked):

| dN | Field | JSON | Matković L1 | Bernardi L1 | Haaland cup | Esposito L1 |
|----|-------|------|-------------|-------------|-------------|-------------|
| totals 2 | minutes | `minutes` | 1088 | 679 | 161 | 933 |
| d2 | matches won | `matches_won` | 8 | 7 | 1 | 7 |
| d4 | passes attempted | `passes` | 386 | 384 | 57 | 307 |
| d5 | passes completed | `passes_completed` | 320 | 336 | 46 | 271 |
| d6 | tackles | `tackles` | 21 | 13 | 0 | 2 |
| d7 | tackles won | `tackles_won` | 17 | 13 | 0 | 1 |
| d8 | headers | `headers` | 62 | 46 | 9 | 72 |
| d9 | headers won | `headers_won` | 24 | 22 | 1 | 22 |
| d10 | dribbles | `dribbles` | 21 | 1 | 2 | 10 |
| d11 | fouls | `fouls` | 11 | 4 | 0 | 10 |
| d13 | shots | `shots` | 29 | 2 | 4 | 66 |
| d14 | shots on target | `shots_on_target` | 17 | 1 | 3 | 32 |
| d23 | distance, 0.1 km | `distance_km` (÷10) | 1606 | 916 | 161 | 1056 |
| d24 | offsides | `offsides` | 4 | 0 | 3 | 8 |
| d27 | key passes | `key_passes` | 31 | 6 | 1 | 17 |
| d28 | key tackles | `key_tackles` | 0 | 1 | 0 | 0 |
| d29 | key headers | `key_headers` | 3 | 11 | 0 | 1 |
| d30 | interceptions | `interceptions` | 28 | 17 | 2 | 5 |
| d31 | chances created | `chances_created` | 9 | 1 | 0 | 2 |
| d34 | possession lost | `possession_lost` | 120 | 51 | 32 | 124 |
| d35 | possession won | `possession_won` | 77 | 67 | 4 | 22 |
| d36 | xG × 100 | `expected_goals` (÷100) | 426 | 8 | 62 | 812 |
| d37 | xA × 100 | `expected_assists` (÷100) | 268 (game 2.67) | 55 | 11 | 8 |
| d38 | clearances | `clearances` | 9 | 17 | 2 | 1 |
| d48 | sprints | `sprints` | 209 | 163 | 7 | 61 |
| d50 | progressive passes | `progressive_passes` | 37 | 56 | 0 | 14 |
| d56 | crosses | `crosses` | 17 | 2 | 0 | 4 |
| d57 | crosses completed | `crosses_completed` | 10 | 2 | 0 | 2 |
| d58 | shots blocked | `shots_blocked` | 1 | 4 | 0 | 0 |

Samples: Anton Matković (person index 77 416, uid 2 000 301 456) slot 1; Carlo Bernardi (169 046, uid 2 002 100 552) slot 1; Erling Haaland (uid 29 179 241) slot 2; Francesco Pio Esposito (uid 2 000 146 883) slot 1. Not decoded: d0, d1, d3 (draws?), d12, d15–d22, d25–d26 (cross-like), d32, d33, d39–d47, d49, d51–d55, d59. d17 and d18 are not additive. The game shows distance per 90 as one tenth of the stored value (Matković 13.3 km/90 shown as "1.3 km"; inference: display quirk).

Other screenshot facts (Task 6 records them):
- **Injuries** (game text "Expected to return in …"): Leroux (index 62 597, uid 2 000 191 040) twisted ankle, start 2035-12-04, days 7–12, on 12-09 shows **2–8 days**; Bernardi tight calf, start 12-07, days 0–3, on 12-09 shows **1 day**. On 12-02: Veiga (0–2 from 12-02) "1–2 days", Bernardi (0–3 from 12-02) "1–3 days". Leroux is in `injury_manager.dat` queue 0 (from 345 to 350 = 12-11..12-16), Bernardi on 12-09 in queue 1 (341..344). "Injury risk: so far 5 days" (Leroux) and "2 days" (Bernardi) match the stored start dates.
- **Joined club** verified: Leroux 2034-01-13 ("Transferred from Rennes"), Bernardi 2035-01-12 ("from Ajax"); both equal the contract start.
- **Clause names** verified for Leroux and Bernardi: appearance fee, clean sheet bonus, unused substitute fee, relegation release clause (€3M), sell-on percentage (10%). Missing from the server: Bernardi's "Qualifying for the UEFA Champions League €71K" bonus. Not decoded: "Remaining Loyalty Bonus" (Bernardi €725K, Leroux €625K).
- **Condition and sharpness**: the game shows icons only. The five players whose sharpness icon is not a tick (Castillo, Čuber Potočnik, Rodríguez, Assongo, Karaboué) have the five lowest stored sharpness values, 5150–5913. Starters of the last match have condition 5800–7400, the rest 8590–10000. Consistent; exact percentages not checkable.
- **Reputation level**: Bernardi "Continental" (reputations current 6671, home 5670, world 3645); Leroux "National" (6584, 5850, 3387). **Personality**: Bernardi "Light-Hearted", Leroux "Fairly Determined". **Media handling**: Bernardi "Evasive, Unflappable", Leroux "Level-headed". **Playing time**: both "Important Player", agreed "Regular Starter".
- **England squad** (team 585) at 12-09: the save's 21 players are exactly the game's 21.

---

### Task 1: Decode minutes, penalties, player of the match, match stats and scopes 0–3 (savefile)

**Files:**
- Modify: `internal/savefile/playerstats.go`
- Test: `internal/savefile/playerstats_test.go`, `internal/savefile/integration_test.go`, `internal/savefile/fuzz_test.go` (keep `FuzzParseSeasonStats` compiling)
- Docs: `docs/format.md` section "rgman/player_stats.dat — season stats" (totals offsets 2, 10, 11 to verified; scope slots 0 and 2 to verified with the evidence above; slot 4 note about Esposito's other international rows; a detail-field table with the dN, field and evidence; Ground truth 3 paragraph; Open questions trimmed)

**Interfaces:**
- Produces:
  ```go
  type MatchStats struct {
      RatingSum        uint16 // offset 0
      Minutes          uint16 // offset 2
      Starts, SubApps, Rated, Goals, Assists uint8 // 4..8
      PenaltiesTaken   uint8 // 10
      PenaltiesScored  uint8 // 11
      PlayerOfTheMatch uint8 // 12
      RedCards, YellowCards uint8 // 13, 14
      Detail           MatchDetail
  }
  type MatchDetail struct { // i16 values from the detail block; field = dN in the plan's table
      MatchesWon, Passes, PassesCompleted, Tackles, TacklesWon, Headers, HeadersWon,
      Dribbles, Fouls, Shots, ShotsOnTarget, Distance /* 0.1 km */, Offsides,
      KeyPasses, KeyTackles, KeyHeaders, Interceptions, ChancesCreated,
      PossessionLost, PossessionWon, ExpectedGoals /* ×100 */, ExpectedAssists /* ×100 */,
      Clearances, Sprints, ProgressivePasses, Crosses, CrossesCompleted, ShotsBlocked int16
  }
  type SeasonRecord struct {
      Club        *MatchStats // slot 5: all club competitions this season ("Overall (Club)")
      Friendlies  *MatchStats // slot 0
      League      *MatchStats // slot 1
      Cups        *MatchStats // slot 2: domestic cups
      Continental *MatchStats // slot 3: all continental competitions together
  }
  func ReadSeasonStats(a *Archive) (map[uint32]SeasonRecord, error) // by person index; a player appears when any of slots 0, 1, 2, 3, 5 is present
  ```

- [ ] **Step 1: Write the failing tests**
  - Extend the test encoder `statsScope(s MatchStats)` to write `Minutes`, `PenaltiesTaken`, `PenaltiesScored`, `PlayerOfTheMatch` at totals offsets 2, 10, 11, 12, and every `Detail` field at its dN (byte 2N of the detail block).
  - `TestParseSeasonStats`: expected values become `map[uint32]SeasonRecord`. Case "every part present": person 146891 gets `Club: &ashby, Friendlies: &friendly, League: &league`; case "no season" (friendlies only) now gives `{7: {Friendlies: &friendly}}`; give `league` a full `Detail` copied from the Matković L1 column and `Minutes: 1088, PlayerOfTheMatch: 2` so every field round-trips.
  - New `TestParseSeasonStatsIgnoresInternationalsAndOtherSlots`: a record with only slots 4, 6 and 7 present is absent from the map.
  - `TestRealSave9Dec` (skips unless `FM_REAL_SAVE_9DEC` is set): Matković `League` equals the table column (plus `RatingSum 1077, Starts 13, SubApps 2, Rated 15, Goals 7, Assists 4, PlayerOfTheMatch 2`), `Club` has `Minutes 1608, PlayerOfTheMatch 3, PenaltiesTaken 1, PenaltiesScored 1`, `Friendlies` has `Starts 3, SubApps 1, Goals 1, Assists 2, PlayerOfTheMatch 1`; Bernardi `League` column plus `RatingSum 877, Rated 13`; Haaland `Cups` column plus `Goals 2, Assists 1, PenaltiesTaken 1, PenaltiesScored 1, RatingSum 153`; Esposito `League` column plus `PenaltiesTaken 2, PenaltiesScored 2`, `Continental` has `PenaltiesTaken 1, PenaltiesScored 0`. Find the players by uid through `ReadGameDB`.
  - Keep `TestRealSave`'s Ashby check working on `season[ashbyIndex].Club`.

- [ ] **Step 2: Run them to verify they fail**
  Run: `GOCACHE=$TMPDIR/gocache go test ./internal/savefile -run 'TestParseSeasonStats|TestParseSeasonStatsIgnoresInternationals' -count=1`
  Expected: build failure on `SeasonRecord` / `MatchDetail`, then (once types exist) value mismatches.

- [ ] **Step 3: Implement** in `playerstats.go`: `statsScope` fills the new fields from the totals and detail bytes (a table of dN → field is fine); `parseSeasonStats` keeps slots 0, 1, 2, 3, 5 in a `SeasonRecord`, drops the rest, and stores a record when any kept slot is present. Leave the earlier-team scopes unread as now. Keep the existing `Rated ≤ Starts + SubApps` check.

- [ ] **Step 4: Run the tests**
  Run: `GOCACHE=$TMPDIR/gocache go test ./internal/savefile -count=1` and `FM_REAL_SAVE=$PWD/data/kz-2035-12-02.fm FM_REAL_SAVE_9DEC=$PWD/data/kz-2035-12-09.fm GOCACHE=$TMPDIR/gocache go test ./internal/savefile -run 'TestRealSave' -count=1 -v`
  Expected: PASS; `TestRealSave9Dec` runs (not skipped).

- [ ] **Step 5: Update `docs/format.md`** as listed under Files.

### Task 2: Expose minutes, player of the match and `season_detail` (fm)

**Files:**
- Modify: `internal/fm/season.go`, `internal/fm/game.go` (`PlayerExtras.Season` type, `NewGame`), `internal/fm/load.go`, `internal/fm/view.go` (`Profile`, `profile`), `internal/fm/player.go` (unexported detail field on `Player`)
- Test: `internal/fm/season_test.go`, plus fixtures that build `PlayerExtras` (grep `PlayerExtras{`)

**Interfaces:**
- Consumes: Task 1's `SeasonRecord`, `MatchStats`, `MatchDetail`.
- Produces:
  ```go
  type SeasonStats struct { /* existing fields */ Minutes int `json:"minutes,omitempty"`; PlayerOfTheMatch int `json:"player_of_the_match,omitempty"` }
  type SeasonDetail struct {
      Competition string `json:"competition"` // "club", "league", "domestic_cups", "continental", "friendlies"
      SeasonStats        // embedded; its JSON fields are flattened
      PenaltiesTaken, PenaltiesScored, MatchesWon, Passes, PassesCompleted, ProgressivePasses, KeyPasses,
      Crosses, CrossesCompleted, ChancesCreated, Shots, ShotsOnTarget, ShotsBlocked, Dribbles, Offsides,
      Fouls, Tackles, TacklesWon, KeyTackles, Headers, HeadersWon, KeyHeaders, Interceptions, Clearances,
      PossessionWon, PossessionLost, Sprints int // JSON names: the plan's table, snake_case, all omitempty
      ExpectedGoals, ExpectedAssists float64 `omitempty` // stored ÷ 100, two decimals
      DistanceKm float64 `json:"distance_km,omitempty"` // stored ÷ 10, one decimal
  }
  // Profile gains: SeasonDetail []SeasonDetail `json:"season_detail,omitempty"`
  // PlayerExtras.Season becomes map[uint32]savefile.SeasonRecord
  ```
  Row order: club, league, domestic_cups, continental, friendlies; absent slots are skipped. `Player.Season` (rows) comes from `SeasonRecord.Club` only, as now.

- [ ] **Step 1: Write the failing tests** in `season_test.go`:
  - `TestSeasonStatsCarryMinutesAndPlayerOfTheMatch`: `seasonStats` of Matković's club totals gives `Minutes 1608, PlayerOfTheMatch 3`; JSON contains `"minutes":1608,"player_of_the_match":3`.
  - `TestProfileListsSeasonDetailByCompetition`: a player with Club, League and Friendlies gets three rows in order `club, league, friendlies`; the league row equals Matković L1: `ExpectedGoals 4.26, ExpectedAssists 2.68, DistanceKm 160.6, Fouls 11, …`, `AverageRating 7.18`.
  - `TestSeasonDetailWithFriendliesOnly`: `Profile.Season` nil, `SeasonDetail` = one `friendlies` row.
  - `TestSeasonDetailDropsNegativeValues`: a detail field of −5 comes out as 0 and is left out of the JSON.
  - Extend `TestSeasonStatsAreVisibleWhateverTheScouting` so `SeasonDetail` is identical with fog on and off.

- [ ] **Step 2: Run to verify they fail**
  Run: `GOCACHE=$TMPDIR/gocache go test ./internal/fm -run 'Season' -count=1`
  Expected: FAIL (build errors on the new names first).

- [ ] **Step 3: Implement** `seasonDetail(name string, s savefile.MatchStats) SeasonDetail` and `seasonDetails(r savefile.SeasonRecord) []SeasonDetail` in `season.go`; `NewGame` stores the rows on the player; `View.profile` copies them into `Profile` (shared slice, read-only, like injuries). Rounding: `math.Round(v*100)/100` for xG/xA, `math.Round(v*10)/10` for km. Negative → 0 via one helper.

- [ ] **Step 4: Run** `GOCACHE=$TMPDIR/gocache go test ./internal/fm ./internal/mcpserver -count=1`. Expected: PASS.

### Task 3: Server instructions and README (mcpserver)

**Files:**
- Modify: `internal/mcpserver/server.go` (instructions text around line 90), `README.md` (tool notes)
- Test: `internal/mcpserver/server_test.go` (`TestInstructionsCoverSeasonStatsAndNationalTeams`, `TestPlayerRowsAndProfilesCarrySeasonStats`)

**Interfaces:**
- Consumes: Task 2's JSON fields.

- [ ] **Step 1: Write the failing tests**
  - Instructions contain: `"season_detail"`, `"count × 90 / minutes"`, `"continental counts all continental cups together"`, `"international matches are not included"`.
  - `TestPlayerRowsAndProfilesCarrySeasonStats`: a find_players row's `season` has `minutes` and `player_of_the_match`; the get_player result has `season_detail` with a `club` row; no row of find_players has `season_detail`.
- [ ] **Step 2: Run to verify they fail**: `GOCACHE=$TMPDIR/gocache go test ./internal/mcpserver -count=1`.
- [ ] **Step 3: Implement**: add to the instructions, after the season paragraph, this text (adjust wording only if a test phrase stays intact):
  `Rows' season also has minutes and player_of_the_match. get_player and compare_players add season_detail: one row per competition group, club (all club competitions, the game's "Overall (Club)"), league, domestic_cups, continental and friendlies, with penalties_taken/penalties_scored, matches_won and match stats (passes, tackles, fouls, xG, distance_km…). continental counts all continental cups together; international matches are not included. FM shows many stats per 90 minutes: compute them as count × 90 / minutes.`
  Update the README's season note to match.
- [ ] **Step 4: Run** `GOCACHE=$TMPDIR/gocache go test ./... -count=1`. Expected: PASS.

### Task 4: Injury upper bound +1 day when the earliest return is still ahead (fm)

Independent of Tasks 1–3.

**Files:**
- Modify: `internal/fm/contract.go` (`injuryInfo`)
- Test: the existing `injuryInfo` tests in `internal/fm` (grep `injuryInfo(`)
- Docs: `docs/format.md` "Current injuries" paragraph on how `fm` reports the return; mark the rule as inference from 4 samples.

**Interfaces:**
- Produces: `injuryInfo(in savefile.Injury, on time.Time, w World) InjuryInfo`, signature unchanged. Rule: `earliest = max(start + DaysMin, on + 1 day)`; `latest = max(start + DaysMax, earliest)`, plus 1 day when `start + DaysMin` is after `on`.

- [ ] **Step 1: Write the failing test** `TestInjuryReturnMatchesTheGame` (table):
  - Leroux: start 2035-12-04, 7–12, on 12-09 → 12-11 .. 12-17
  - Bernardi: start 12-07, 0–3, on 12-09 → 12-10 .. 12-10
  - Veiga: start 12-02, 0–2, on 12-02 → 12-03 .. 12-04
  - Bernardi on 12-02: start 12-02, 0–3, on 12-02 → 12-03 .. 12-05
  - Boundary: start 12-01, 8–10, on 12-08 (earliest 12-09 = the next day, still after `on`) → 12-09 .. 12-12
- [ ] **Step 2: Run** `GOCACHE=$TMPDIR/gocache go test ./internal/fm -run TestInjuryReturnMatchesTheGame -count=1`. Expected: FAIL on Leroux and the boundary row.
- [ ] **Step 3: Implement** the rule in `injuryInfo`; update its comment.
- [ ] **Step 4: Run** `GOCACHE=$TMPDIR/gocache go test ./internal/fm -count=1`. Expected: PASS.

### Task 5: Decode the Champions League qualification bonus (savefile)

Independent of Tasks 1–4. Investigation first; stop and report to the user if Step 2 finds no match.

**Files:**
- Modify: `internal/savefile/contract.go` (`clauseNames`, or the FULL header's `u8 n + n × 31 bytes` list if the bonus lives there)
- Test: `internal/savefile/contract_test.go` (or the existing clause test file), `internal/savefile/integration_test.go`
- Docs: `docs/format.md` "Contracts" (the bonus, and clause names verified in game for Leroux and Bernardi)

- [ ] **Step 1: Probe** (throwaway in-package test in a copy of the repo under `$TMPDIR`; see Appendix): print Bernardi's (person 169 046, uid 2 002 100 552) decoded clauses and the raw bytes of his FULL contract header in `data/kz-2035-12-09.fm`. The bonus is €71K displayed, so GBP 60 927 … 61 791 (÷ 1.1571305). Search his record for an i32 in that range.
- [ ] **Step 2: Decide** from the probe: either an existing clause list item whose type has no name (then add `clauseNames[T] = "Champions League qualification bonus"`), or an item in the 31-byte list (then decode that item's value and type, from at least two players holding such a bonus; find a second one by scanning for the same type). Record the type number T and the evidence.
- [ ] **Step 3: Write the failing test**: a synthetic clause/bonus item of type T yields name `"Champions League qualification bonus"` and amount 61 359 (GBP); `TestRealSave9Dec` checks Bernardi's contract has it.
- [ ] **Step 4: Run, implement, rerun** `GOCACHE=$TMPDIR/gocache go test ./internal/savefile -count=1`. Expected: PASS.

### Task 6: Record the remaining ground truth; review; rebuild

**Files:**
- Modify: `docs/backlog.md`, `docs/format.md`, `README.md` if a limit changes

- [ ] **Step 1: Backlog**: delete "Next up 2" (England checked: 21/21 at 2035-12-09); rewrite "Next up 1" to the undecoded detail fields, internationals (Esposito's "International League" and "European League Div. A" rows are in no slot 0–5) and sort keys on the new stats; add the screenshot facts from "Other screenshot facts" to the Player data items (reputation level, personality, media handling, loyalty bonus, playing time).
- [ ] **Step 2: `docs/format.md`**: joined date verified (2 players); condition and sharpness evidence (icons, ordering); clause names verified in game.
- [ ] **Step 3: Full checks**: `GOCACHE=$TMPDIR/gocache go test ./... -count=1` (3 times), `-race`, both real-save tests, lint `0 issues.`, `go vet ./...`, `gofmt -l .` empty.
- [ ] **Step 4: Review agents** in the mandatory order (Global Constraints) until clean.
- [ ] **Step 5: Rebuild** `bin/fm-mcp`; tell the user to run `/mcp`. Then check live with the MCP Inspector:
  `npx -y @modelcontextprotocol/inspector@0.17.2 --cli $PWD/bin/fm-mcp -e FM_SAVE=$PWD/data/kz-2035-12-09.fm --method tools/call --tool-name get_player --tool-arg uid=2000301456` (sandbox off). Expected: `season.minutes` 1608 and a `season_detail` league row with `fouls` 11 and `expected_goals` 4.26.

---

## Appendix: probe recipe

The values above were read with a throwaway test placed in a copy of the repo (`cp -R go.mod go.sum internal $TMPDIR/probe/`), in package `savefile` so it can use the unexported reader:

```go
a, _ := Open(os.Getenv("PROBE_SAVE")); db, _ := ReadGameDB(a) // map uid → p.Index
f, _ := a.ReadFile("rgman/player_stats.dat"); r := newReader(f.Body)
// per record: u8 more, u32 person, take(statsRecordHead), 8 × (u8 present, u8 6, take(17) totals, take(120) detail),
// take(statsRecordMiddle), u32 n × (u8, u32, 6 × r.statsScope()), u32 n × take(statsSummarySize), u8, take(3*16)
```

Run with `PROBE_SAVE=<abs path> GOCACHE=$TMPDIR/gocache go test ./internal/savefile -run TestProbe -v`. Never add probes to the real repo.
