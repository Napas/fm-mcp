# FM26 format: Competition keys, stages and league tables

Part of the format reference. Conventions, dates and the index of all files: [`docs/format.md`](../format.md).

## Competition keys (ACTUAL_COMP_ID) — verified 26.2.0 and 26.3.2 (code + bytes)

A fixture's competition ref (FIXTURE base `+0x20`) is the index of an ACTUAL_COMP_ID record in `game_db.dat`, not a COMP index. Code: writer `0x33618e8`, reader `0x3362394`, storage `[0x5548760]+0x88`, created by `0x3363ba8`.

```
u32 count, u8 0                              right after the previous table's end marker
count × 33 bytes:
  u8 tag 0
  i32 A  competition (COMP index); -1 in a few keys no fixture uses
  i32 B  stage (STAGE_NAME index), -1 = none
  i32 C  sub-stage or group (STAGE_NAME index), -1 = none
  i32 D  leg or replay (STAGE_NAME index), -1 = none
  i32 E  second competition (COMP index) of a two-division playoff, -1 = none
  u32 index, u32 id, u32 id                  id = index + 1
end marker
```

The table has no memory pool. `savefile` finds it by its first record's database record (`0, 1, 1`) and accepts it only when it is the one table that parses with every record's index i and ids i+1 (`readCompetitionKeys`). Records: test 3 360, KŽ (2035-12-02) 7 753. Keys with A = -1: 24 test, 214 KŽ; no fixture of the test save uses one.

Examples (KŽ): 267 = Premier League; 268 = National League Cup / Group Stage / Group A; 5618 = `{8, 236, 17, 2, -1}` = Sky Bet Championship / Playoffs / Semi Final / First Leg; 5619 = the Second Leg; 5724 = `{8, 236, 19, -1, -1}` = Championship / Playoffs / Final; 3967 = Champions League / Champions Path / First Qualifying Round / First Leg; 4574 = French Cup / – / Fifth Round. E is set for promotion/relegation playoffs between two divisions (Ligue 1 v Ligue 2, Eredivisie v Eerste Divisie).

Checks: in the test save, all 56 league-like fixture groups of 2025 resolve to the teams' division. In KŽ, 124 of 153 do, and the other 29 resolve to the correct sub-stage, cup or qualifier. All 2 073 keys KŽ fixtures use are in range.

## Stage names (STAGE_NAME) — verified 26.2.0 and 26.3.2 (bytes)

```
u32 count, u8 0                              count = memory pool STAGE_NAME (285 in both saves)
count × 14 bytes: u8 tag 0, u8 0, u32 index, u32 uid, u32 uid
end marker
```

The uids are those of the language database's `8_23.dat` in ascending order (see [`language-db.md`](language-db.md)); uid 36 is missing, so for uid ≥ 37 the index is uid − 2. Examples: index 2 = uid 3 "First Leg", 3 = 4 "Second Leg", 17 = 18 "Semi Final", 19 = 20 "Final", 41 = 43 "Group 1", 74 = 76 "Group A", 236 = 238 "Playoffs", 275 = 2000223075 "League Phase". A key's label joins its B, C and D names: "Playoffs – Semi Final – First Leg".

## League tables (LEAGUE_STAGE rows) — verified 26.2.0 and 26.3.2 (code + bytes)

A LEAGUE_STAGE (vtable `0x54e9900`, typeinfo `0x54ea1a8`, load `0x404291c`) in `rgman/comp_<uid>.dat` holds two u8-counted lists of table rows (`+0xc0` and `+0x138`), back to back. The first is the table in the game's order; the second holds the same rows in another order (meaning unknown). SUB_LEAGUE_STAGE has vtable `0x54ef1e0`, VIRTUAL_LEAGUE_STAGE `0x54f00f0`. Past seasons' tables are not kept: Ligue 1's file holds one 18-row table on 2035-12-02.

```
ref        u8 (0 in every row seen), i32
stat       ref team (-1 = this row's team), u8 played, u8 (= played in every row seen),
           u8 won, u8 drawn, u8 lost, u8 shoot-out losses, u16 for, u16 against, i16 points      17 bytes (0x4077f90)
row base   ref team; u8 position (0-based) + 5 × u8 (guess: previous positions, highs, lows); u8; 4 × u8; 3 × u8;
           i32; 5 stats (overall, home, away, then two not decoded); u8 flag, when set u8 n + n × 2 stats
           (head-to-head); u8, u8, u8, u16, 7 × u8                                              (0x4046740)
row        base; i16 n + n × u8 (position after each round, 0xff = not yet; 34 for Ligue 1);
           i8 m + m × (i32, u16, u8, u8); u8 flag (1 = sub-object 0x4077bf0, not decoded, never seen);
           10 × u8 form (most recent first: 3 win, 2 draw, 1 loss); u8 flag (1 = a chained row follows)   (0x4044190)
list       u8 count, count × row
```

Form is verified on all 18 Ligue 1 rows of KŽ against their fixtures. The i16 17 bytes before the first list's count is the stage's STAGE_NAME index (-1 = none); it matches the stage of the table's fixtures for 369 of 374 KŽ tables (the 5 others are tables whose teams also met in a later knockout round). Some tables hold values out of range there (2025, -24320, -248 in KŽ); `savefile` treats them as no stage.

Points are stored, not derived. Most leagues have 3 × won + drawn. In group stages where drawn matches go to penalties (KŽ: EFL Trophy, Scottish League Cup, Leagues Cup, National League Cup; 13 tables) they fit 3 × won + 2 × drawn + 1 × shoot-out losses, so drawn there counts shoot-out wins (inference from the fit). Swedish Second Division Norrland's second-stage tables fit neither (carried-over points, guess). Played = won + drawn + lost + shoot-out losses in every valid table.

**Finding tables** (`savefile.parseLeagueTables`): a validated scan, not a walk of the whole comp file. A candidate is a count byte 1..64 whose first row has ref byte 0, position 0 and a first stat record `00 ff ff ff ff` 24 bytes on. It is kept when every row parses, there are at least 2 rows, and each row has a team ≥ 0, position = its index, overall = home + away (played, won, drawn, lost, for, against) and played = won + drawn + lost + shoot-out losses. Points are never checked. The partner list that follows a table with the same teams is skipped: without that, the test save gives 407 hits for 238 tables. Each file's scan reads about 16 × its size + 1 MB at most (the budget is checked after each parse).

Coverage: test save 238 tables against 240 LEAGUE_STAGE objects (memory pools); KŽ 603 against 645. The gap does not hit live leagues: every current-season round-robin fixture group of 6+ teams has its teams covered by found tables (KŽ 216 of 217, the miss being the South American U20 Championship; test 74 of 74). MLS is two 15-team conference tables; no 30-team overall table is stored. Decompressing all comp files takes 54 ms on KŽ (60 MB, 1 238 files), 13 ms on the test save (13 MB).

Examples (KŽ, Ligue 1, `comp_16.dat`, 2035-12-02): 1st team 685 (Stade Rennais FC), 15 played, 10-3-2, 28–12, 33 pts, form D L W W W W L D W D; 2nd FC Metz (652), 7-6-2, 34–22, 27 pts, home 4-2-1, away 3-4-1. Test save: the Premier League (`comp_11.dat`) has 20 rows, all at 0 played.

Ground truth (in game, KŽ save of 2036-02-24, 26.3.2): the Ligue 1 table on the Stages screen matches the stored table row for row (position, played, won, drawn, lost, for, against, points, and the goal difference the server derives from them; all 18 teams), including the stored order of Metz, PSG and Strasbourg on 41 points. The five form icons match the stored form's last five results, oldest on the left (read from small icons). Position-change arrows are not served.

**Sub-competitions have no file of their own.** Their tables sit in the parent competition's file: Liga MX Opening and Closing Stage in Mexican First Division's, English U18 Division 1 North and South in English U18 Division 1's, Spanish U19 Division 1 Groups 1–7 in Spanish U19 Division 1's. These tables carry no stage name and no COMP index within 200 bytes before them.

**Sub-competition labels** (`fm.relabel`; the manager's decision of 2026-10-24, "match by results", refined on both saves). Fixtures are grouped by competition and stage (the key's group when set, else its stage; legs ignored), each group keeping its latest season. A table with matches played matches a group at its own stage whose played fixtures add up to every row's played, won, drawn + shoot-out losses, lost, for and against (goals after extra time when set). A table with none played matches a group at its stage whose fixtures hold exactly its teams. Exactly one matching competition other than the file's relabels the table (`label_source` "results"); otherwise it keeps the file's competition ("file"). Indistinguishable tables (same file, stage and rows) are served once and still count as found. They can be different stages of a season: the test save stores each of Spanish U19 Division 1's seven groups twice before the season, the 2036-01-16 save Argentine Premier Division's groups A and B twice (probably the opening and closing zones, guess). When two different tables would get the same competition and stage, both keep the file's label. Comparing at the stage, not over the whole competition, is needed for group stages whose competition also has knockout rounds.

Outcome. KŽ: 120 of 603 tables relabelled (Spanish U19 Division 1 Groups 1–7, Mexican First Division Opening Stage, English U18 Division 1 North and South, European International League Divisions A–D, European U19 qualifying groups, Swedish Norrland Spring North and South, Portuguese Third Division North and South…); no played table matched two competitions; the unplayed Liga MX Closing Stage table holds the same teams as the Opening Stage and keeps the parent's name. Test save (pre-season): 44 distinct sub-competitions labelled by their teams, among them English U18 Division 1 North and South, Spanish U19 Division 1 Groups 1–7 and the women's Nations League groups.

Different tables can share a label: KŽ has 38 such pairs, among them Major League Soccer's two conference tables (no stage name), the Vertu Trophy's north and south groups A–H, World Cup North American qualifying groups A–C, and the Brazilian Amazonas State Championship's two tables per stage with the same teams and different standings (probably the two turns, guess). Their teams tell them apart.

## Open questions

- What the keys with no competition (A = -1) are for.
- The second row list's order; the row fields marked guess; the 42 (KŽ) and 2 (test) LEAGUE_STAGE objects without a found table (stages without rows, other stage classes, guess).
- Why some tables hold an out-of-range stage value.
