# Player Interest Filter Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `search_players` drops players who certainly won't join the human's club (transfer, loan or either), tags the rest `likely` / `uncertain`, and `get_player` shows the verdict.

**Architecture:** `savefile` exposes the new save fields (ACTUAL_PERSON dates, previous club, transfer ids, person flags, contract kind, flags and event ring) and a `transfer_man.dat` TRANSFER reader. `fm` keeps the per-player interest inputs in `playerAux`, computes the model at query time and filters in `View.Search`. `mcpserver` adds the input and output fields only.

**Tech Stack:** Go, `github.com/modelcontextprotocol/go-sdk`.

**Spec:** `docs/superpowers/specs/2026-10-23-player-interest-design.md`. Evidence and the reference model are in `docs/superpowers/specs/2026-10-23-player-interest/`. The Python reference model (`reference-model/code_rules.py`, `r2.py`, `score_c.py`) defines every formula. The plan does not repeat its bodies: port them.

## Global Constraints

- No git. Do not run git commands that write. "Done" for a task means its tests pass and lint is clean.
- Build/test: `GOCACHE=$TMPDIR/gocache go test ./...`.
- Lint: `GOCACHE=$TMPDIR/gocache GOLANGCI_LINT_CACHE=$TMPDIR/lintcache golangci-lint run --allow-parallel-runners` must print `0 issues.`
- TDD: write the failing test first, every task.
- Never write, move or touch files under `~/Library/Application Support/Sports Interactive/`. Real-save tests read copies in `data/` through env vars. The new one is `FM_REAL_SAVE_16JAN=$PWD/data/kz-2036-01-16.fm` (game date 2036-01-16).
- Comments describe current behaviour only. MCP handlers hold no parsing logic. Parsers know nothing about MCP.
- `fm.Player` must stay within 512 bytes (`TestPlayerFitsA512ByteAllocation`). Interest inputs go in `playerAux`.
- Record every decoded structure in `docs/format.md` (offsets, evidence, build 26.3.2) using the `/decode-structure` skill.
- Constants, from the spec: `interestOutBelow = -20_000`, `interestLikelyFrom = 30_000`, `recentlyLeftDays = 150`; best transfer level L = 10 (kind 4); loan level L = 7 (kind 3).
- Fog of war never hides the verdict. Factor texts never contain a hidden attribute value (Loyalty or the others).

## Review Focus

1. **Manager's club unknown** (`Info.ManagerTeamIndex` not a club team): with `interest` set, `Search` returns an error that says so; it never filters silently. Test in Task 7.
2. **`transfer_man.dat` missing or not recognised**: the load adds a warning; `transfer_arranged` never fires, so nobody is `out` from it. Test in Task 4.
3. **Unset dates** (1900 joined, signed or end dates; loanees whose player-part joined date is 1900): no just-joined penalty from an unset date, no panic. Test in Task 5.
4. **Players of the other gender, at national teams only, or the human's own players**: no `interest`, never dropped. Test in Task 6.
5. **Totals with the option on**: `total` and `needs_scouting` count only players kept after the interest drop; `interest_dropped.total` equals the sum of `by_reason`. Test in Task 7.

---

## PR breakdown (advisory on this profile; no git)

| PR | Tasks | What | Est. diff | Depends on |
|---|---|---|---|---|
| 1 | 1, 2 | savefile: person fields and contract kind/flags/events, plus format.md | ~450 | independent |
| 2 | 3 | savefile: `transfer_man.dat` TRANSFER reader, fuzz target, format.md | ~400 | independent |
| 3 | 4, 5, 6 | fm: interest inputs at load, the model, verdicts; golden and acceptance tests | ~650 | 1 and 2 |
| 4 | 7, 8 | fm View + mcpserver: `interest` option, row and profile fields, tool text | ~350 | 3 |

PR 3 may exceed the 500-line soft limit. Its golden test file is data, and splitting the model from its verdicts would leave a model nothing calls. If it grows past ~800, split Task 6 out.

---

### Task 1: Person fields — joined date, previous club, transfer ids, flags

**Files:**
- Modify: `internal/savefile/player.go` (Player struct, `parsePlayer`)
- Modify: `internal/savefile/contract.go` (`readContracts` returns what the marker check found)
- Test: `internal/savefile/player_test.go`, `internal/savefile/integration_test.go`
- Docs: `docs/format.md` ("Contracts (ACTUAL_PERSON part)": the fields before the marker)

**Interfaces:**
- Produces, on `savefile.Player`:
  - `JoinedClubPerson Date`: ACTUAL_PERSON +0xec, the date 5 bytes before the id list's count byte (q − 5, where q = marker − 5 − 4n); 1900 when unset.
  - `PreviousClub int64`: ACTUAL_PERSON +0xd8, a CLUB index; −1 when none.
  - `TransferIDs []uint32`: ACTUAL_PERSON +0x68, the `u8 n + n × u32` list before the marker.
  - `Retiring bool` (trailer flags bit 18) and `OnlyOneClub bool` (trailer flags bit 15).
- The writer order before the marker (0x2f424a0): `u16, ref (+0xd8), u16, u16, u8, u8, date +0xe8, date +0xec, u8, u8 n + n × u32 (+0x68), u32, u8 5`. The probe `reference-model/probe/zz_gates_export.go.txt` (`PersonDates`, `PersonIDs`, `PrevClubRef`) reads the same bytes. Decode the ref's exact encoding with `/decode-structure`: the probe returns `b[ap-4 : q-9]` raw; check its decoded value against the samples below.

- [ ] **Step 1: Write the failing unit test** `TestParsePlayerPersonFields` with a synthetic record. Set a marker with n = 2 ids (10183, 10810), +0xec = 2035-08-16, a previous-club ref of 474, and trailer flags with bits 15 and 18 set. Assert the five fields. Add a case where n = 0 and the ref is unset: `PreviousClub == -1`, `TransferIDs == nil`.
- [ ] **Step 2: Run it**: `go test ./internal/savefile -run TestParsePlayerPersonFields`. Expect it to fail to compile (fields missing).
- [ ] **Step 3: Implement.** Make `readContracts` (or a helper next to it) return the id-list position q it validated, so `parsePlayer` reads the dates, list and ref from the same q. Flags come from `rec.Flags` like `FlagBit3`.
- [ ] **Step 4: Write the failing integration test** `TestRealSave16JanPersonFields`. Open `FM_REAL_SAVE_16JAN` and skip when it is unset. Assert, by uid:

  | Player | Field | Expected |
  |---|---|---|
  | Unai Marco (2002115699) | `TransferIDs` | `[10183, 10810]` |
  | Pablo Vilas (2002115328) | `PreviousClub` | 474 |
  | Jean-Pierre Bayal (2002077775) | `PreviousClub` | 474 |
  | Jean-Pierre Bayal | `JoinedClubPerson` | 2035 day 246 |
  | Julien Barthes (2002112631) | `PreviousClub` | 491 |
  | Julien Barthes | `JoinedClubPerson` | 2035-08-16 |
  | Gianfranco Fiaschetti | `PreviousClub` | 498 |
  | Giovanni Lotito (2002098987) | `PreviousClub` | 677 |
  | Lau Jiménez | `PreviousClub` | 1122 |
  | Gabriel Ahovey | `PreviousClub` | −1 |

  Look up the uids of Fiaschetti, Lau Jiménez and Ahovey in `labels-2036-01-16.tsv`. Also assert that no labelled player has `Retiring` or `OnlyOneClub` set, and log the save-wide counts.
- [ ] **Step 5: Run the savefile tests** with `FM_REAL_SAVE_16JAN` set. All pass.
- [ ] **Step 6: Document** the fields in `docs/format.md`, with evidence (code addresses from the spec, the byte samples above).
- [ ] **Step 7: Lint**: `0 issues.`

### Task 2: Contract kind, core flags and event ring

**Files:**
- Modify: `internal/savefile/contract.go` (Contract struct, `readFullHeader`, `coreEndingAt`)
- Test: `internal/savefile/contract_test.go`, `internal/savefile/integration_test.go`
- Docs: `docs/format.md` ("Contracts")

**Interfaces:**
- Produces, on `savefile.Contract`:
  - `Full bool`: the slot type is FULL.
  - `Kind uint8`: FULL header byte 0 (FULL +0xc3); 0 when not FULL.
  - `CoreFlags uint8`: core byte 23 (CONTRACT +0x57).
  - `Events []ContractEvent`: the core's `u32 n + n × 29` items.
- Produces:
  ```go
  type ContractEvent struct {
      Type   uint8  // item byte 1 (+0x19)
      ID     uint16 // byte 2 (+0x16)
      Status int8   // byte 4 (+0x18)
      Date   Date   // byte 13 (+0x0)
      Closed Date   // byte 17 (+0x4); 1900-01-01 = open
      U14    uint16 // byte 21 (+0x14)
  }
  func (e ContractEvent) Open() bool // Closed is 1900-01-01 (day 1, year 1900)
  ```

- [ ] **Step 1: Write the failing unit test** `TestCoreEvents`. Build a synthetic core with two items: one open (type 12, id 650, status 3, closed 1900-01-01) and one closed (type 37, closed 2033). Set the FULL header byte 0 to 2 and core byte 23 to `0x09`. Assert every field and `Open()`.
- [ ] **Step 2: Run it.** Expect it to fail.
- [ ] **Step 3: Implement.** `coreEndingAt` already knows k; decode the k items there. `readFullHeader` already reads byte 0. `parseSlots` sets `Full` and `Kind` for FULL slots.
- [ ] **Step 4: Write the failing integration test** `TestRealSave16JanContractEvents`:
  - Lotito's main contract has an open event of type 12, id 650, status 3.
  - Oumar Cissokho (2002099764) has 3 events, all closed, types 37, 37 and 19.
  - Barthes has no events.
  - Record the `Kind` and `CoreFlags` values the reference golden needs for Barthes, Nešić (2002114070) and Garrido (2002116366).
- [ ] **Step 5: Run the tests.** All pass.
- [ ] **Step 6: Document** the event item layout (spec research-factors §5) and the two bytes in `docs/format.md`.
- [ ] **Step 7: Lint**: `0 issues.`

### Task 3: `transfer_man.dat` TRANSFER records

**Files:**
- Create: `internal/savefile/transfers.go`, `internal/savefile/transfers_test.go`
- Modify: `internal/savefile/fuzz_test.go` (add `FuzzReadTransfers`)
- Docs: `docs/format.md` (new section "transfer_man.dat — TRANSFER records")

**Interfaces:**
- Produces:
  ```go
  type Transfer struct {
      ID     uint32 // record offset 21 (+0x5c): the value in Player.TransferIDs
      Person uint32 // offset 3, i32 person index
      Class  uint8  // offset 0: 2 or 3
      Team   int32  // offset 8 (u8 at 7, then i32): the team the move is to
      Status uint8  // offset 29 (+0x8d): 19 = arranged
      Date   Date   // offset 60 (+0x7c): the move's date
  }
  func ReadTransfers(a *Archive) ([]Transfer, error)
  func parseTransfers(body []byte) ([]Transfer, error)
  ```
- Ruling (spec): a pattern scan, not a full grammar walk. A candidate starts with class 2 or 3 and u16 `0x13`, and both dates parse. The body begins `u8 1, u32 count` (count 29 934 on 2036-01-16; what it counts is not decoded). Linking a record to a player (same person, id in `TransferIDs`) is the caller's check, so a false positive can never fire the gate. Cap the candidates at the body length / 30. Return `ErrCorrupt` when the body is shorter than its header.
- Marco's record, verified bytes (2036-01-16), from offset 0: `02 13 00 81cf0200 00 57030000 00 57030000 00000000 c7270000 ce720500 13 ffffffff …`.

- [ ] **Step 1: Write the failing unit test** `TestParseTransfers`. Use a synthetic body: header `01` + u32 count, Marco's record padded to 64 bytes with dates at 56 (2036-02-04) and 60 (2036-07-01), then a second record of class 3 with status 15. Assert both. Add a body of junk bytes that must give no records.
- [ ] **Step 2: Run it.** Expect it to fail.
- [ ] **Step 3: Implement** `parseTransfers` and `ReadTransfers` (file `transfer_man.dat`, through `a.ReadFile`).
- [ ] **Step 4: Write the failing integration test** `TestRealSave16JanTransfers`:
  - id 10183 has Person 184193, Team 855, Status 19 and Date 2036-07-01.
  - id 10810 has Team 649 and Status 15.
  - The count of status-19 records linked from their person is between 410 and 425 (research: 418 linked of 422).
  - Gamboa (2002115333), Tomašević (2002125884) and Korshun have no linked status-19 record.
- [ ] **Step 5: Add `FuzzReadTransfers`** over `parseTransfers`: no panic, and records ≤ len(body)/30.
- [ ] **Step 6: Run** the unit, integration and fuzz tests (fuzz for 30 s). All pass.
- [ ] **Step 7: Document** the record layout (spec research-gates "Follow-up: Marco" table), the scan and its check, and the counts.
- [ ] **Step 8: Lint**: `0 issues.`

### Task 4: Interest inputs at load

**Files:**
- Modify: `internal/fm/game.go` (`playerAux`, `PlayerExtras`, `NewGame`)
- Modify: `internal/fm/load.go` (read transfers; warning on failure)
- Modify: `internal/fm/world.go` (`Team.HasManager bool`, set from `savefile.Team.Manager >= 0`)
- Create: `internal/fm/interest.go` (types only in this task)
- Test: `internal/fm/game_test.go`, `internal/fm/load_test.go`

**Interfaces:**
- Consumes: Task 1 and Task 2 fields; `savefile.ReadTransfers`.
- Produces:
  ```go
  // in PlayerExtras
  Transfers []savefile.Transfer // nil when transfer_man.dat could not be read

  type interestInputs struct {
      joined       savefile.Date // JoinedClubPerson
      previousClub int64
      arranged     *arrangedMove // a linked status-19 transfer; nil when none
      retiring, onlyOneClub bool
      main         *savefile.Contract // the raw main contract (kind, flags, events, clauses, dates, playing time)
      loyalty      uint8              // Hidden[2]
  }
  type arrangedMove struct { team int32; date time.Time }
  ```
  `playerAux` gains `interest interestInputs`.
- Load: when `ReadTransfers` fails, add the warning "arranged transfers unavailable, so interest cannot tell who has agreed a move: <err>" and pass nil.

- [ ] **Step 1: Write the failing test** `TestNewGameArrangedMove`. Build a synthetic `savefile.Player` with `TransferIDs [7, 8]` and transfers {ID 7, Person = his index, Status 19, Team 855}, {ID 8, Status 19, Person = another index}. Assert `arranged.team == 855` from the first. A second player with no list gets nil.
- [ ] **Step 2: Write the failing test** `TestLoadWarnsWithoutTransfers` (Review Focus 2). Load the test save with the transfer reader failing: inject the reader through `LoadOptions` or a package variable, following how other readers are stubbed in `load_test.go`. Assert the warning text and that no player has `arranged` set.
- [ ] **Step 3: Run both.** Expect them to fail.
- [ ] **Step 4: Implement.** Index the transfers by ID once in `NewGame`.
- [ ] **Step 5: Run the fm tests.** All pass. Then lint: `0 issues.`

### Task 5: The interest model

**Files:**
- Modify: `internal/fm/interest.go`
- Create: `internal/fm/interest_test.go`
- Create: `internal/fm/testdata/interest_golden.tsv` (copy of `docs/superpowers/specs/2026-10-23-player-interest/reference-golden.tsv`)

**Interfaces:**
- Consumes: `interestInputs`, `Player`, `World`, `Info.Date`, the human club's first-team reputation Rt.
- Produces (unexported; ports of the reference model):
  ```go
  // justJoined is 0x128f550: the penalty at offered level L; loan selects kind 3.
  func justJoined(p *Player, in interestInputs, today time.Time, L int, loan bool) int
  // clubReputationGap is r2.reason2 (transfer) and code_rules.r2_loan (loan).
  func clubReputationGap(p *Player, in interestInputs, w World, rt int) int
  func clubReputationGapLoan(p *Player, rt int) int
  // playingTimeValue is code_rules.pt_value (0x12ad438); ok false where the reference returns None.
  func playingTimeValue(p *Player, in interestInputs, w World, rt, L int, loan bool) (int, bool)
  type interestTerms struct{ penaltyT, gap, pt10, penaltyL, pt7, gapLoan int }
  func (g *Game) interestTerms(p *Player) interestTerms
  ```
- Changes from the reference model, all from the spec:
  - The loan penalty needs `LEVEL[b] ≥ 5` (`0x32715dc`). The extra club `+0x16a == 1` test applies only when the interest call's flags have bit 3 set. The filter calls with flags 0 (research-gates §1), so skip it.
  - "Just signed" at age ≤ 23 takes P / 4.
  - Days use the game's 365-day arithmetic.
- The "club" in the gap and playing-time terms is the main contract team's club, else the current team's. Its first team is its TEAM with type 0, else its first team (`reference-model/probe/pidump_main.go.txt`).

- [ ] **Step 1: Write the failing branch tests** `TestJustJoined` (table-driven, synthetic inputs). One row per branch:

  | Case | Inputs | Expected P |
  |---|---|---|
  | Barthes-like | 153 days, Loyalty 9, L 10, FULL kind 0 | 88 200 |
  | settled | 251 days | 0 |
  | open ring item exemption | event type 12, open, status 3, id 650 | 0 |
  | clause 55 | | 0 |
  | contract type 3 | | 0 |
  | expiry exemption | 157 days, ends in 166 days | 0 |
  | "just signed" | renewed 2 days ago, 600 days left, age 25 | `(11−L)×99×Loy×100` |
  | "just signed", age 23 | as above | quarter of that |
  | contract type 2 | | ×1000 instead of ×100 |
  | loan, foreign national | | 0 |
  | loan, b = Breakthrough Prospect (LEVEL 4 < 5) | | 0 |
  | joined date unset (1900) | | 0 (Review Focus 3) |
  | free agent | | 0 |
  | on loan | | 0 |

- [ ] **Step 2: Write the failing tests** `TestClubReputationGap` and `TestPlayingTimeValue`. Use synthetic rows that match `r2.py` and `code_rules.pt_value`: one per stepped band of d (≤ −4001, ≤ −3001, ≤ −2001, < −1000), the ×8 world-reputation term, and d = 0, d > 0 and d < 0 for playing time.
- [ ] **Step 3: Write the failing golden test** `TestInterestTermsMatchReference` (skips without `FM_REAL_SAVE_16JAN`). For every row of `testdata/interest_golden.tsv` with numbers, compare the six terms. Columns: P_T = penaltyT, r2 = gap, pt10, P_L = penaltyL, pt7, r2L = gapLoan.
  - Allowed differences, with the reason checked by the test: P_L for players whose b has LEVEL < 5 (the new check); P_T and P_L for "just signed" at age ≤ 23.
  - Rows with `why` set (gates, free agents) are checked in Task 6.
  - The test lists every other mismatch and fails.
- [ ] **Step 4: Run them.** Expect them to fail.
- [ ] **Step 5: Implement** the four functions and `interestTerms` by porting the reference model.
- [ ] **Step 6: Run the fm tests** with the env var set. All pass. Then lint: `0 issues.`

### Task 6: Verdicts and factors

**Files:**
- Modify: `internal/fm/interest.go`, `internal/fm/interest_test.go`
- Create: `internal/fm/testdata/interest_labels.tsv` (copy of `labels-2036-01-16.tsv`)

**Interfaces:**
- Produces (exported; JSON shapes are what the server returns):
  ```go
  type InterestKind string
  const (
      InterestTransfer      InterestKind = "transfer"
      InterestLoan          InterestKind = "loan"
      InterestTransferOrLoan InterestKind = "transfer_or_loan"
  )
  type Verdict string
  const (VerdictOut Verdict = "out"; VerdictUncertain Verdict = "uncertain"; VerdictLikely Verdict = "likely")
  type InterestFactor struct {
      Code string `json:"code"` // only_just_joined, transfer_arranged, recently_left_your_club, retiring, only_one_club, club_reputation_gap, depends_on_role, playing_time
      Text string `json:"text"`
  }
  type Interest struct {
      Transfer Verdict          `json:"transfer"`
      Loan     Verdict          `json:"loan"`
      Reason   *InterestFactor  `json:"reason,omitempty"` // why out; set when either side is out
      Factors  []InterestFactor `json:"factors,omitempty"`
  }
  func (in Interest) For(k InterestKind) Verdict
  // interestOf returns nil for players not evaluated: the human's own players,
  // players of the other gender than the manager's team, players whose only team is a
  // national team, and every player when the manager's club is unknown.
  func (g *Game) interestOf(p *Player) *Interest
  ```
- Verdict table, gates and sums: copied from the spec's "Sums and verdicts". `For(InterestTransferOrLoan)` is `out` only when both sides are out, and `likely` when the transfer is likely.
  - Gate order (first wins): `transfer_arranged`, `retiring`, `only_one_club`, `recently_left_your_club`.
  - Free agents: only the first three gates.
  - A sum `out` takes its reason from its most negative term: `only_just_joined` or `club_reputation_gap`.
- Texts, e.g.:
  - `only_just_joined`: "Joined SM Caen 153 days ago; won't consider a move yet".
  - `transfer_arranged`: "Has agreed a move to SS Calcio Bari on 1 July 2036".
  - `recently_left_your_club`: "Left FC Metz 3 days ago".
  - `depends_on_role` is added when the transfer sum at L = 8 is below 0.
  - No text names a hidden value.

- [ ] **Step 1: Write the failing acceptance test** `TestInterestMatchesFM` (skips without `FM_REAL_SAVE_16JAN`). For each row of `testdata/interest_labels.tsv`:
  - FM `pass` → the matching verdict is not `out`. Use `fm_transfer_only` with `For(InterestTransfer)`, and `fm_transfer_or_loan` with `For(InterestTransferOrLoan)`.
  - FM `fail` → no check (option A).
  - Also assert reasons: Marco `transfer_arranged`; Vilas and Bayal `recently_left_your_club`; Barthes and Gamboa `only_just_joined`; Lafleur and Tomašević transfer `out` but transfer_or_loan not `out`.
  - Log the number of FM fails that are `out` (expected about 30 of 39).
- [ ] **Step 2: Write the failing tests** `TestInterestNotEvaluated` (Review Focus 4: own player, woman when the manager's team is men's, national-team-only player, manager club unknown → nil) and `TestInterestTextsHideHiddenValues`. For the second, give a player Loyalty 17 and a just-joined penalty: no factor text contains "17" or "Loyalty".
- [ ] **Step 3: Write the failing unit test** `TestInterestFor`, a table over the transfer/loan verdict pairs for all three kinds.
- [ ] **Step 4: Run them.** Expect them to fail.
- [ ] **Step 5: Implement** `interestOf` and `For`.
- [ ] **Step 6: Run the fm tests** with the env var set. All pass. Report the logged count. Then lint: `0 issues.`

### Task 7: `search_players` interest option

**Files:**
- Modify: `internal/fm/game.go` (`Filter.Interest string`)
- Modify: `internal/fm/view.go` (`Profile.Interest *Interest`, `SearchResult.InterestDropped *InterestDropped`, `compile`, `Search`)
- Modify: `internal/mcpserver/server.go` (`searchIn.Interest`, `PlayerSummary.Interest`, `searchOut.InterestDropped`, tool description), `internal/mcpserver/prompts.go` or the server instructions text
- Test: `internal/fm/game_test.go`, `internal/mcpserver/server_test.go`

**Interfaces:**
- Produces:
  ```go
  type InterestDropped struct {
      Total    int            `json:"total"`
      ByReason map[string]int `json:"by_reason"`
  }
  ```
- `Filter.Interest`: "", "transfer", "loan" or "transfer_or_loan". Anything else is an error naming the three values. `searchIn` must keep the same field order and types as `fm.Filter`: `searchPlayers` converts with `fm.Filter(in)`.
- In `Search`: after `q.judge(x) == matched`, players whose verdict is `out` are counted in `InterestDropped` (by `Reason.Code`) and not kept. Kept rows get `Profile.Interest`. Players with nil interest are kept with no field. With the manager's club unknown, `compile` returns the error "interest needs your club, but the manager's team is unknown" (Review Focus 1).
- searchIn jsonschema text: "transfer, loan or transfer_or_loan: leave out players who certainly won't join your club for that move (FM's Player's interest). Kept rows carry interest: likely or uncertain, with the main factors. Never drops an uncertain player."

- [ ] **Step 1: Write the failing fm test** `TestSearchInterest`, on a synthetic game with three players at another club: one just joined (`out`), one settled with a big transfer sum (`likely`), one near the line (`uncertain`). With `Interest: "transfer"`, assert:
  - `Total == 2`;
  - `InterestDropped == {1, {only_just_joined: 1}}`;
  - each kept row has its verdict;
  - `NeedsScouting` excludes the dropped player (Review Focus 5).

  Also assert that `Interest: "bogus"` errors, and that the unknown manager club errors.
- [ ] **Step 2: Write the failing server test** `TestSearchPlayersInterest`: call `search_players` with `interest` on the test save and assert the JSON has `interest_dropped` and rows with `interest.transfer`. Add the new field to the schema-description check at `server_test.go:323`.
- [ ] **Step 3: Run them.** Expect them to fail.
- [ ] **Step 4: Implement** and update the tool description and server instructions (explain the three verdicts, that `out` players are dropped and counted, and that FM's exact levels are not reproduced).
- [ ] **Step 5: Run all tests.** All pass. Then lint: `0 issues.`

### Task 8: `get_player` interest

**Files:**
- Modify: `internal/fm/view.go` (`Profile` built in the single-player path sets `Interest`)
- Modify: `internal/mcpserver/server.go` (profile output, tool description)
- Test: `internal/fm/game_test.go`, `internal/mcpserver/server_test.go`

**Interfaces:**
- Consumes: `(g *Game) interestOf`.
- `get_player` always carries `interest` when `interestOf` is non-nil, whatever the fog-of-war state. `compare_players` gets it too, since it shares the profile path (builder's call in the spec; chosen here because it costs nothing).

- [ ] **Step 1: Write the failing test** `TestProfileInterest`:
  - a just-joined player's profile has `Interest.Transfer == out` and a Reason with code `only_just_joined`;
  - with fog of war on and the player unscouted, the interest is the same and his personality is hidden;
  - the human's own player has no interest.
- [ ] **Step 2: Write the failing server test**: `get_player` JSON for a real-save player (skip without `FM_REAL_SAVE_16JAN`): Marco shows `interest.reason.code == "transfer_arranged"` with "SS Calcio Bari" in the text.
- [ ] **Step 3: Run them.** Expect them to fail.
- [ ] **Step 4: Implement.**
- [ ] **Step 5: Run all tests.** All pass. Then lint: `0 issues.`
- [ ] **Step 6: Update `docs/backlog.md`.** Add the open items: the relegation exemption, the Wnt bonus (Stáňa, Maxsø), the free-agent path, the squad ranking `0x2f9e580`, Vega and Lau Jiménez, and the untested ages outside 24–25.
