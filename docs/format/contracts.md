# FM26 format: Contracts

Part of the format reference. Conventions, dates and the index of all files: [`docs/format.md`](../format.md).

## Contracts (ACTUAL_PERSON part) — verified 26.2.0 and 26.3.2

Contracts live inside the person record, not in a table (`contract_man.dat` is 52 bytes). The ACTUAL_PERSON writer (`0x2f424a0`) emits:

```
u16 +0x102, i64 ref +0xd8, u16 +0x104, u16 +0x106, u8, u8,
date +0xe8, date +0xec, u8 +0x10b, u8 n + n × u32 (+0x68), u32 +0xb0
u8 5                       marker
5 × (u8 type + body)       slots: main, loan, B club, future, B club 2
```

**Fields before the marker** — verified 26.3.2 (code + bytes). Offsets from q, the id list's count byte (q = marker − 5 − 4n):

| Offset | Size | Field | Status |
|--------|------|-------|--------|
| q − 25 | u16 | `+0x102`; 0x0000, 0x1000, 0xc000, 0xc010 in the samples | unknown |
| q − 23 | i64 | `+0xd8`: the previous club, a CLUB index; −1 = none (`Player.PreviousClub`) | verified (code + bytes, below) |
| q − 15 | 2 × u16, 2 × u8 | `+0x104`, `+0x106`, … | unknown |
| q − 9 | date | `+0xe8`; 1900 for 138 875 of 138 955 players (2036-01-16) | unknown |
| q − 5 | date | `+0xec`: the date he joined his current club; 1900 when unset (`Player.JoinedClubPerson`) | verified (bytes): equals the player part's joined date (offset 105) for contracted players, and is set for loanees, whose offset 105 is 1900 (player-interest research, 23 players) |
| q − 1 | u8 | `+0x10b`, 0..10 (10 for 135 361 players) | unknown |
| q | u8 n + n × u32 | `+0x68`: TRANSFER ids linked to the person (`Player.TransferIDs`) | verified (bytes: Unai Marco 10183, 10810; code from the player-interest research: the interest check `0x2f54e10` looks each id up in the TRANSFER table) |
| q + 1 + 4n | u32 | `+0xb0` | unknown |

The ref writer `0x335f42c` writes 8 bytes in saves: −1 for a null object, else the object's table index (`+0x8`) sign-extended. Its other branches (a uid in the high half, or 4 bytes) depend on globals that are off in every save seen. Every value is −1 or a CLUB index (2036-01-16: 65 824 set, 73 106 −1, none outside the CLUB table of 36 457). Samples (2036-01-16): Pablo Vilas 474 (FC Metz; he joined Gladbach 2036-01-13), Jean-Pierre Bayal 474 (joined US Thionville Lusitanos on 2035 day 246), Julien Barthes 491 (joined 2035-08-16), Gianfranco Fiaschetti 498, Giovanni Lotito 677, Lau Jiménez 1122, Gabriel Ahovey −1 (bytes; `+0xd8` as the previous club is from the player-interest research: Vilas and Bayal fail the game's "has only recently left the club" check against Metz).

**Which list.** A transfer id between 1900 and 2200 reads as a year, so a shorter list can pass the date checks too: 118 of 138 955 players on 2036-01-16 have two readings, n = 0 and n = 1. The false one is 4 bytes off, so its previous club reads as the real ref's high half plus the next two u16s: ≥ 2³² or 0. `savefile` takes the longest list whose ref is −1 or 0..2³¹−1, else the shortest. In those 118, the shorter reading has an invalid ref in 36 and the longer in 2. In 79 of the 116 where n = 1 wins, its `+0xec` equals the player part's joined date; the shorter reading's never does.

Types (from vtables): 0 FULL, 1 LOAN, 2 TRIAL, 3 B_CLUB, 4 YOUTH_TRIAL, 5 BASIC, 0xff empty. Every type but BASIC ends with:

```
CONTRACT core (0x3269674)   i32 weekly loyalty bonus, i32, i32, date end, date start,
                            u8 playing time, u8 (see below), u8 original playing time, 3 × u8,
                            u32 n + n × 29 bytes, u8, date signed,
                            u8, i8 playing-time score, i8 recent score, u8 squad number, 3 × u8, i64, date, u8
BASIC tail (0x302c3a0)      u32 person index, i64 team index, i32 weekly wage, u8, u16
```

FULL contracts start with a header (`0x3490034`): `u8 contract type (guess: 1 full-time, 0 part-time, 2 amateur, 3 youth, 4 non-contract), i32 total loyalty bonus, i32, i32, date, i64, u8, u16, u8 n + n × clause, u8 n + n × competition bonus, u8 has-flag, [variable when set], u32 0, u8, 13 bytes`, then the core. A clause is `i32 value, u16 extra, u8 type, u8 0`; clause and bonus names (in `internal/savefile/contract.go`) come from the game's negotiation-flag strings and from which players hold them, so they are guesses. Exceptions, verified in game on the KŽ save (26.3.2, 2035-12-09): Louis Leroux (uid 2 000 191 040) and Carlo Bernardi (uid 2 002 100 552) both show appearance fee (type 32), clean sheet bonus (34), unused substitute fee (38) and sell-on percentage 10% (8); Bernardi also shows the qualification bonus (below). The game also showed a relegation release clause of €3M; it matches Leroux's type-1 clause (£2 627 393 ≈ €3.04M), and Bernardi's contract holds no type-1 clause. Which player's screen showed it was not recorded, so attributing it to Leroux is an inference.

**Contract kind, core flags and event ring** — verified 26.3.2 (code + bytes, 2036-01-16 save). `savefile.Contract` serves `Kind` (FULL header byte 0, FULL `+0xc3`; the mask test uses it), `CoreFlags` (core byte 23, CONTRACT `+0x57`; getter `0x1dbc91c`; bits 0, 3 and 6 are read by the transfer-interest rules) and `Events`, the core's `u32 n + n × 29` ring (CONTRACT `+0x30`, up to 10 items, writer `0x3753254`):

| Item byte | Field |
|-----------|-------|
| 0 | u8 7 |
| 1 | type (`+0x19`) |
| 2 | u16 id (`+0x16`) |
| 4 | i8 status (`+0x18`) |
| 5, 9 | i32 (`+0x8`, `+0xc`), not served |
| 13 | date (`+0x0`) |
| 17 | date closed (`+0x4`); 1900-01-01 = open |
| 21 | u16 (`+0x14`) |
| 23, 24, 25 | u8 `+0x1a`, u8 `+0x1b`, i32 `+0x10`, not served |

Samples: Lotito has an open event (type 12, id 650, status 3); Cissokho has three closed events (types 37, 37, 19); Barthes has none. Kind and flags: Barthes 1 / 0x00, Nešić 0 / 0x00, Garrido 1 / 0x40.

**Competition bonus** — verified 26.3.2 (bytes), 31 bytes each:

| Offset | Size | Field | Status |
|--------|------|-------|--------|
| 0 | u8 | 1 | constant in all items |
| 1 | u8 | 0 | constant |
| 2 | date | 1900-01-01 | constant |
| 6 | i32 | competition (COMP index) | verified (bytes): the contract team's division in 107 of 109 type-10 items (2035-12-09) |
| 10 | i32 | amount in GBP | verified (Bernardi, in game) |
| 14 | u16 | 0xffff | constant |
| 16 | u8 | 1 | constant |
| 17 | i32 | −1 or 20 (type 0 only); unknown | |
| 21 | i32 | −1 (one item: 20); unknown | |
| 25 | u8 | bonus type | 10 verified; 0, 3, 6, 14 (real save) and 1 (test save) unknown |
| 26 | i32 | target competition (COMP index), −1 = none; set only for type 10 | verified |
| 30 | u8 | 0 | constant |

Type 10 is the bonus for qualifying for the target competition: Bernardi's item is `01 00 01 00 6c 07 0b 00 00 00 24 ee 00 00 ff ff 01 ff ff ff ff ff ff ff ff 0a 7d 02 00 00 00` = Ligue 1 (index 11), £60 964, type 10, target 637 = UEFA Champions League (uid 1 301 394). The game shows "Qualifying for the UEFA Champions League €71K" (£60 964 × 1.1571305 = €70 543). Leroux, with no such bonus in game, has no item. Asier Castillo (uid 2 002 098 873, Metz) has the same item with £64 150 (not checked in game). Counts (2035-12-09): 2 257 items on 2 180 contracts, at most 3 per contract; type 10 = 109 items with targets Champions League 22, Europa League 55, Conference League 32 (inference from names: same bonus for those competitions). The 2035-12-02 save has 2 249 items and Bernardi's is identical; the 26.2.0 test save has 3 284 items, types 0, 3 and 1. `fm` shows type-10 items as "<target> qualification bonus" and leaves other types out.

**Playing time** (squad status) — verified 26.3.2 (code + in game). Core bytes 20–22 are CONTRACT `+0x54`, `+0x55`, `+0x56`; the writer `0x3269674` writes them after the start date, then `+0x57..+0x59` (bytes 23–25, mostly 0, not decoded). Getters: `0x1dbc8f4` (+0x54), `0x1dbc904` (+0x55), `0x1dbc8fc` (+0x56). After the signed date (core offset 31 + 29n, n = the item count at offset 26) the writer emits `+0x5a…+0x60`:

| Core offset | Field | Status |
|-------------|-------|--------|
| 20 | playing time: the status the club has agreed with the player. The squad list's Playing Time column and the contract screen's "Agreed:" line show it | verified (code): the rule `PLAYER_CLUB_CONTRACT_HAS_CURRENT_PLAYING_TIME` (eval `0x18277bc`) compares `+0x54`. Verified (in game): the Metz squad screen, 12 of 12 players on 2035-12-02 and 44 of 44 not out on loan on 2035-12-14 (byte 22 fits 24 of 44); "Agreed:" on Bernardi's and Assongo's contract screens (12-14) and Bernardi's and Leroux's (12-09) |
| 21 | 0, or an earlier playing time | hypothesis: the previous value, shown by the string "Previously {an}[%playing_time#1]" (key 493441; C# property `PreviousAgreedPlayingTime`). Always 0 in the test save; set to the old byte 20 in 561 contracts between 2035-12-02 and 12-09 (Ashby `03 02 02` → `16 03 02` when made Surplus), but also set without a change (Obata `0f 00 0f → 0f 0f 0f`) or cleared (Stejskal `02 02 02 → 02 00 02`). Not exposed |
| 22 | original playing time: the one agreed when the contract was signed. The game shows it on neither screen seen | verified (code): `0x326b068` sets `+0x54` and `+0x56` together when a playing time is agreed; later changes touch `+0x54` only (Ashby keeps 2 after being made Surplus). The rule `PLAYER_LOAN_CONTRACT_HAS_ORIGINAL_AGREED_PLAYING_TIME` (eval `0x1827364`) compares a loan contract's `+0x56`. Never 0 or 22 in main contracts of the 2035-12-02 save |
| 35 + 29n | `+0x5a`, unknown | 0..100 or −60..100 |
| 36 + 29n | playing-time score, i8 −100..100 (`+0x5b`) | verified (code): reset to the middle of byte 20's band (below) when a playing time is set (`0x326ff2c`, `0x32702c0`, `0x326b0cc`), moved after each match and clamped to ±100 (`0x3271308…0x3271328`). Assongo −9 → −12 → −15 over two weeks |
| 37 + 29n | recent score, i8 −100..100 (`+0x5c`) | hypothesis: a short-term score; the rule below reads it for cup goalkeepers (16–19) only |
| 38 + 29n | squad number (`+0x5d`), 255 = none | verified (in game): equals the No. column of the Metz squad-number screen for 60 of 60 players, those out on loan included (main contract, 2035-12-14) |

The FULL header's date at offset 13 (FULL `+0xb4`) is the date the playing time was last set: verified (code: FULL vtable `+0x118` returns it). Ashby's moved 2035-10-30 → 2035-12-07 when he was made Surplus. Not exposed.

Values (enum; string → value parser `0x377b6f4`, value → name `0x39279bc` via the u32 table `0x5265ab0` and the key switch `0x3769b88`; names from the game's `example.ltf`):

| Value | Engine name(s) | Shown as |
|-------|----------------|----------|
| 0 | NONE | never stored in bytes 20 and 22 |
| 1 | STAR_PLAYER | Star Player |
| 2 | IMPORTANT_PLAYER | Important Player |
| 3 | REGULAR_STARTER | Regular Starter |
| 4 | SQUAD_PLAYER, SQUAD_PLAYER_COMPETING_FOR_POSITION | Squad Player |
| 5 | IMPACT_SUBSTITUTE, SQUAD_PLAYER_IMPACT_SUBSTITUTE | Impact Sub |
| 6 | SQUAD_PLAYER_FMT | Squad Player |
| 7 | FRINGE_PLAYER, FRINGE_PLAYER_OCCASIONAL_OPPORTUNITIES | Fringe Player |
| 8 | FRINGE_PLAYER_FMT | Fringe Player |
| 9 | EMERGENCY_BACKUP | Emergency Backup |
| 10 | BREAKTHROUGH_PROSPECT, HOT_PROSPECT_ON_VERGE_OF_FIRST_TEAM | Breakthrough Prospect |
| 11 | FUTURE_PROSPECT, HOT_PROSPECT_FOR_THE_FUTURE | Future Prospect |
| 12 | HOT_PROSPECT_FMT | Hot Prospect |
| 13 | YOUNGSTER | Youngster |
| 14 | B_CLUB_REGULAR | "<B team> Regular"; `fm` shows "B Team Regular" (which team the game names is not decoded) |
| 15 | FIRST_CHOICE_KEEPER | First-Choice Goalkeeper |
| 16 | ALL_CUPS_KEEPER, CUP_KEEPER_ALL_CUPS | Cup Goalkeeper |
| 17 | DOMESTIC_CUP_KEEPER, CUP_KEEPER_DOMESTIC_CUPS | Domestic Cup Goalkeeper |
| 18 | CONTINENTAL_CUP_KEEPER, CUP_KEEPER_CONTINENTAL_CUPS | Continental Cup Goalkeeper |
| 19 | CUP_KEEPER_FMT | Cup Goalkeeper |
| 20 | BACKUP_KEEPER | Backup |
| 21 | EMERGENCY_BACKUP_KEEPER | Emergency Backup |
| 22 | SURPLUS_TO_REQUIREMENT | Surplus to Requirements |

Values 15–21 are goalkeeper statuses: every main contract of the 2035-12-02 save with byte 20 in {15, 16, 17, 18, 20, 21} belongs to a player rated ≥ 15 at GK (6 901 of 6 901). The `*_FMT` values (6, 8, 12, 19) are never stored (display-only, inference). The display function first checks feature ids `0xc80…0xc94` for an override (`0x376946c`, not decoded); the table above is the fallback the binary ships.

Ground truth (KŽ save, 2035-12-02, Metz squad screen; bytes 20 21 22): Reddy `01 04 04` Star Player; Edubatey and Matković `02 00 02` Important Player; Sami `0f 00 0f` First-Choice Goalkeeper; Haddad `0a 0d 0d` Breakthrough Prospect; Ashby, Leroux and Veiga `03 02 02` and Nduquidi `03 03 01` Regular Starter; Vilas, Rodríguez and Ly `04 00 04` Squad Player. In the 26.2.0 test save byte 20 equals byte 22 in all 32 787 main contracts (Vicario 15).

Ground truth (KŽ save, 2035-12-14, squad-number screen, 44 players not out on loan): byte 20 matches the Playing Time column for all 44, e.g. Ashby `16 03 02` Surplus to Requirements, Assongo `04 03 02` and Ly `04 00 04` Squad Player, Reddy `01 04 04` Star Player, Bernardi and Leroux `03 02 02` Regular Starter. Bytes 20–22 of every Metz player are the same on 12-09 and 12-14.

**Actual playing time** (the contract screen's headline) — partly decoded 26.3.2 (code; 4 in-game samples, all at levels 9 and 1). It is not stored. `0x32703c0(contract)` computes it from the score `p` (`+0x5b`), the recent score `q` (`+0x5c`), byte 20 `b`, whether the player is a goalkeeper (GK rating ≥ 18, vtable `+0x398`) and whether he is a youth (`0x377fae0`: age ≤ 21, or ≤ 25 under a nation/competition rule not decoded):

1. Level (`0x32705ec`): `p ≥ 85` → 10 when `b` = 1, else 9; `p ≥ 60` → 8; `b` in 16..19 and `q ≥ 0` → 7; `p ≥ 30` and not a keeper → 7; `p ≥ 10` → 4 for a youth, else 6; youth → 3; `p ≥ −10` → 5; else 1.
2. Level 7 only (`0x3270410`), before step 3: the game counts the entries of the player's match list for the contract's team (`0x3270418…0x3270548`; the team is the contract's, `[contract+0x18]`) that have `+0x15` set. With at least 5, it returns Impact Sub when at least half of them also have `+0x11` set (likely substitute appearances), else Squad Player (`0x32705b4…0x32705d0`). The step is skipped when global `0x5547918` is 1–3, the `*_FMT` modes, whose values are never stored (inference: the main game runs it). The list is not decoded; `fm` leaves level-7 results out (below).
3. When `b`'s own level equals it, the result is `b` (table `0x524f3e8`: 1→10, 2→9, 3→8, 4–6→7, 7–8→6, 9→5, 10–12→4, 13→3, 14→2, 15→8, 16–20→7, 21→5, 22→1).
4. Else level → status (`0x3780194`): 10 Star Player, 9 Important Player, 8 Regular Starter (keeper: First-Choice Goalkeeper), 7 Squad Player (keeper: Cup Goalkeeper), 6 Fringe Player (keeper: Backup), 5 Emergency Backup (keeper: 21), 4 Future Prospect, 3 Youngster, 2 B Team Regular, 1 Surplus to Requirements (keeper: 21, the keeper floor). The game then clamps the result: no worse than 13 for youth contracts (FULL header byte 0 = 3; `savefile` serves it as `Contract.Kind`, and 3 = youth is a guess), capped by an active playing-time promise (4 or 6, keepers 20) and floored by some team settings (9 or 21). The promise and team-setting inputs are not decoded. `fm` applies none of the clamps.

   Club teams take a longer path through `0x3780194` with more branches (code reading, not checked in game):
   - Level 4 gives Breakthrough Prospect (10) instead of Future Prospect when `0x36fa000(player…)` ≥ 0.7 × `0x30a0498(club, player)`, an ability-ratio test (`0x378078c…0x37807f8`).
   - Levels 6 and 7 give Impact Sub (5) to outfield players aged 33 or more with vtable `+0x428` > 14 (`0x3780744…0x3780784`, `0x3780890…0x37808d0`).
   - Level 7 gives Fringe Player (7) when `0x39a9984(club, 1)` fails and the competition's `+0x188` ≤ 109 (`0x3780604…0x3780654`, `0x3780878`).
   - After the clamp, 9, 21 or 22 becomes B Team Regular (14) when the player is over 23 (vtable `+0x178` > 23) and a list at `+0x118` of the object at team `+0x30` holds an entry pointing back to the team with a type in {4…8, 20} (`0x37809b8…0x3780a64`). The team is the contract's (`[contract+0x18]`, inference). The object is likely the team's club (inference: it has a `+0x18` team vector tested like a team's and a `+0x48` vector matched against the person). The same entry sets the clamp's best bound to 9 (keepers 21), so every result there is Emergency Backup or worse (`0x37804d0…0x3780540`). Which clubs have such an entry is not decoded, so this branch can change any served value. Assongo (26, Metz first team) shows Surplus in game, which shows only that Metz has no such entry for its first team.

Score bands (tables `0x524f2d4` low / `0x524f280` high, by status − 1): Star and Important 85–100; Regular Starter 60–85; Squad Player and Impact Sub 30–60; Fringe 10–30; Emergency Backup, B Team Regular and the prospects −10..10; Youngster −30; First-Choice Goalkeeper 60–85; cup and backup keepers 30–60; Emergency Backup keeper −10..10; Surplus −30.

| Player (save) | Bytes 20 21 22 | Score | Rule | In game (headline) |
|---------------|----------------|-------|------|--------------------|
| Bernardi (12-14) | `03 02 02` | 88 | Important Player | Important Player; "Agreed: Regular Starter" |
| Assongo (12-14) | `04 03 02` | −15 | Surplus to Requirements | Surplus to Requirements; "Agreed: Squad Player" |
| Bernardi (12-09) | `03 02 02` | 87 | Important Player | Important Player; agreed Regular Starter |
| Leroux (12-09) | `03 02 02` | 88 | Important Player | Important Player; agreed Regular Starter |

On 12-09 the contract screen's "Important Player" is this headline and "Regular Starter" is byte 20. Not checked in game (12-14): Leroux 86 → Important Player, Ashby 80 → Regular Starter, Reddy 81 → Regular Starter, Ly 100 → Important Player, Sami (keeper, 97) → Important Player.

`fm` leaves the actual playing time out when step 2 or one of the step-4 branches above may change it. It does not apply the step-4 clamps or the `+0x118` entry, so a youth contract, an active promise, a team setting or that entry can still change a served value. It leaves it out:
- when the player is away from his current contract's team (not on loan, playing for a reserve or youth team) or that team is not a club's first team, because reserve and youth players' scores are not checked in game. Reserve players (FC Metz 2) mostly score −100 and would get Surplus to Requirements (Sow, Vallet, Sanogo). The `+0x118` entry does not set them apart: all 32 FC Metz 2 players are contracted to Metz's first team (652), Assongo's team.
- at level 7 (step 2), also when `b` has level 7.
- when step 4 runs at level 4 or 6 (step 3 does not return `b`): the Breakthrough Prospect, Impact Sub and Fringe Player branches. At these levels step 3's result is kept: it returns before `0x3780194`.
- when the player is 25 or younger and the result with the youth flag set differs from the result without it. The youth test (`0x377fae0`) accepts up to 25 under a rule not decoded and can fail for younger players (`0x35cbcb4`), so neither value is certain.

The four in-game samples stay served (Bernardi is 25, but at score 88 the youth flag does not matter). On 2035-12-14, 68 250 of 133 312 players with a playing time get one; 44 216 are left out for their team, 15 439 aged 25 or under for the youth test or a level-4/6/7 result, and 5 407 older players for a level-6/7 result. Metz first team: Lobede (25, Surplus without the youth flag, Youngster with it; the game shows Surplus to Requirements on 2036-01-01, so the youth test fails for him at 25, one sample), Delrieu (Squad Player at level 7), Rodríguez, Vilas, Čuber Potočnik, Karaboué, Medina and Haddad are left out.

The contract screen's **"Expected:"** line is computed too (likely `0x1c10a44`, logged as "expected_playing_time"): byte 20 within 30 days (plus person `+0x44`) of the date the playing time was set, else the better of the actual playing time and a status from a squad-ability ranking (`0x2f9e580`). The ranking is not decoded, so `fm` does not report it. In game (12-14): Bernardi Important Player, Assongo Squad Player.

Loan cores carry their own bytes (2035-12-02: 1 792 loans, values 0–22, only 41 equal to the player's main contract; byte 20 is 0 in 218 and byte 22 in 264) and scores (2035-12-14: spread over −100..100). `fm` reports a loanee's playing time, original and actual playing time from his loan contract (inference: the game shows the loan club's status; not checked in game) and none when the loan's byte 20 is 0. Squad numbers: a loanee wears his loan contract's number. On 2035-12-14, 1 028 of 1 720 last-lineup places (see "Last lineup") held by loanees carry the loan contract's number and 7 the main contract's; the rest are teams that renumber per match. `fm` reports the current contract's number (the loan's while on loan). In a club's out-on-loan list it reports the main contract's (what that club's squad-number screen shows: Le Ven 69, his loan club's 14). In a squad list it leaves the number out for players whose current contract is with another team: every row of a national-team squad, and reserve players contracted to the first team (FC Metz 2 on 12-14: 19 of its 32 players have a Metz number). Whether FM numbers reserve teams separately is not known. Of 133 526 main contracts, 78 980 have no number (255); of 1 801 loans, 589.

**Loyalty bonus** — verified 26.3.2 (code + bytes; 4 of 4 in game, display rounding inferred).

| Where | Field | Evidence |
|-------|-------|----------|
| FULL header offset 1, i32 | total loyalty bonus, GBP (FULL_CONTRACT `+0xa8`) | header writer `0x3490034` writes `+0xc3` (contract type), then `+0xa8` |
| core offset 0, i32 | weekly loyalty bonus, GBP (CONTRACT `+0x38`) | core writer `0x3269674` writes `+0x38` first; read at `0x34b5fc0` |

The game sets the weekly amount at `0x3490ff4…0x34915c0`: when the total is ≥ 1 and the end date is set, weekly = total / whole weeks between two dates of the contract (integer division). With the core's signed date as the first, it holds for 26 484 of 30 746 main contracts with a weekly amount (2035-12-02); the others have whole-number ratios, so their dates changed after the amount was set (inference). Both fields stay unchanged between the 12-02 and 12-09 saves (weekly identical for 133 305 of 133 669 players; the rest renegotiated). Main contracts only: loan and future contracts' cores carry the field, but it is 0 in all of them (1 792 and 1 795 loans in the real saves, 3 247 in the test save). About half the main contracts with a total have weekly 0 (31 043 of 63 474 on 12-02); the game's remaining amount is then 0.

The remaining loyalty bonus is not stored. `0x34b5a24` computes it from the contract end date and the game date (global `0x553e3a8`):

```
end unset (1900-01-02)            → 0
days = end − game date, at most 30 000; 1 825 when the core's end date is 1900-01-01
days < 7                          → 0
remaining = weekly × (days / 7)   whole weeks
remaining ≤ 100                   → 0
```

`fm` applies this rule to the main contract (`loyalty_bonus_remaining`; hidden by fog of war where the wage is). Ground truth (KŽ saves, contract screens; € = £ × 1.1571305):

| Player (save) | Total | Weekly | End | Weeks left | Remaining | In game |
|---------------|-------|--------|-----|------------|-----------|---------|
| Chris Ashby (12-02) | £482 888 | £2 299 | 2038-06-30 | 134 | £308 066 = €356 473 | €350K |
| Bruno Sami (12-02) | £772 867 | £2 972 | 2040-06-30 | 238 | £707 336 = €818 480 | €825K |
| Carlo Bernardi (12-09) | £780 434 | £3 363 | 2039-06-30 | 185 | £622 155 = €719 915 | €725K |
| Louis Leroux (12-09) | £932 281 | £4 018 | 2038-06-30 | 133 | £534 394 = €618 364 | €625K |

The game shows the value rounded to the nearest €25K (inference: fits all 4, not found in code). Rounding down, rounding up or showing whole K fits none of them. Smaller amounts round finer: Jesús Lobede's €92 513 shows as €93K (2036-01-01), so the step is probably tiered by size (inference: €1K below €100K, €25K above; his transfer value €370 194 shows as €375K).

**Start date** (verified in game, 2036-01-01): the contract screen's "Start" is the core's signed date, not the start field (the joining date). Enzo Fernández: start 2023-01-31, signed 2034-02-02, the game shows 2 Feb 34; Jesús Lobede (2033-11-06) and Théo Emmanuelli (2035-04-24) have equal start and signed dates, as shown. In that save the signed date is set on every contract, equals the start on 59 213 of 131 356 main contracts and every loan, and is later on the rest (renewals). `fm` serves the signed date as `start` and the start field as `joined`, and would fall back to the start field for a contract without a signed date (none seen: every contract with a start field in this save and the 26.2.0 test save has a signed date). The player's joined date (offset 105, `joined_club`) is later than the main contract's start field for 4 579 players on 2036-01-01. Only Enzo's sample separates the two in game.

Wages are **GBP per week** (English clubs' wages are round; euro contracts are round after dividing by 0.82869). The LOAN header is not decoded.

Parsing: the cores are found by their tail carrying the record's own person index; the marker is found by validating the fields before it (`u8 n` at marker − 5 − 4n, dates at −5 and −9 before that), closest to the first core first; the slots are then matched to the cores. All players of both saves parse: 32 787 with a main contract and 3 247 loans in the test save (= the LOAN_CONTRACT pool), 133 966 and 1 810 in the real one. A Python implementation of the same rules gives identical main contracts, loans and injuries for every player of both saves.

## Open questions

- ACTUAL_PERSON fields before the contracts marker: `+0x102`, `+0x104`, `+0x106`, the two u8s, the `+0xe8` date, `+0x10b` and `+0xb0`.
- Playing time: byte 21 and when it is set or cleared; core bytes 23–25 and `+0x5a`; whether the game shows a loanee's status from his loan contract; the B team named by value 14. Actual playing time: the level-7 match rule (Squad Player or Impact Sub), the other `0x3780194` branches (Breakthrough Prospect ability test, Impact Sub at 33+, Fringe via the competition, B Team Regular over 23 and which clubs hold the `+0x118` entry), the youth rule from 22 to 25 (Lobede fails it at 25), the youth-contract, promise and team clamps, and whether reserve players' −100 scores show as Surplus in game. In-game checks: Sow (24, FC Metz 2; whether a reserve player's −100 score shows as Surplus). "Expected" playing time: the squad-ability ranking `0x2f9e580`.
- Contracts: does Bernardi's screen show a relegation release clause the save lacks? Competition bonus types 0, 3, 6, 14 (and 1 in 26.2.0) and item offsets 17 and 21 are not decoded; whether Europa League and Conference League type-10 items mean the same as the Champions League one. FULL header offsets 5 and 9 (`+0xac`, `+0xb0`; the second drops in steps between saves on some contracts, hypothesis: another bonus paid in instalments); the rounding rule of the remaining loyalty bonus on screen; which date the weekly loyalty amount is counted from.
