# FM26 format: rgman/player_stats.dat

Part of the format reference. Conventions, dates and the index of all files: [`docs/format.md`](../format.md).

## rgman/player_stats.dat — season stats, verified 26.2.0 and 26.3.2 (code + bytes)

Version 15 in both saves. Writer `0x31bb2a8` (file name from `0x31c3cd0`), per-player writer `0x3200400`; the loader `0x31b97b4` has a separate path for versions ≤ 13 (not decoded). The writer walks the ACTUAL_PLAYER list and writes a record for each player that has a stats object; the record names the player itself, so file order does not matter (it differs from the person table's). `savefile.ReadSeasonStats`:

```
file    = record*, u8 0, 4 × u32 (unknown)
record  = u8 1
          u32 person index
          3 × u32, u16, u8, u8                 unknown (+0x64…+0x73)
          8 × scope                             slots, meanings below
          20 bytes                              five (u8, u8, u8, u8) groups, unknown
          u32 n + n × team                      earlier teams this season
          u32 n + n × (u8 2, u16, 5 × u8)        per-position stats (below)
          u8 16 + 16 × 3 bytes                   recent matches (below)
team    = u8 0 + u32 index (ref writer 0x335f5a0; hypothesis: team index), 6 × scope
scope   = u8 0                                  absent
        | u8 1, u8 6, 17 bytes totals, 120 bytes detail   (block writer 0x31f50dc)
```

**Stat ids** (verified, code). A scope in memory (writer `0x31f50dc`, loader `0x31f5290`) is a pointer to the 60 i16 detail values and the 17 totals bytes. The raw getter `0x31f4f74(scope, id)` returns `detail[id]` for ids 0..59 and a totals byte for ids 60..74 (jump table `0x524d7c8`; offsets in the totals table below). The derived getter `0x31f55ec` computes ids 75..152 (per 90 when minutes ≥ 90, and ratios). The scoped getter `0x3204498(stats, id, scope)` reads slots 0..7 and sums some (scope 8 = slots 1 + 2 + 3, scope 12 = slots 1 + 2). The detail fields were named through the data binding (C# property from the IL2CPP registration → C# 4cc → engine 4cc, switch `0x34fc4c` → engine stat provider `0x1429d00`, whose cases call `0x3204498` with the stat id) and through the per-match accumulation `0xf0f220`, which shows where each value comes from in the match record. The loader (`0x31f5368`) reads 60 values from version 6 of the block, 59 in version 5 and 56 before.

Totals (offsets within the 17 bytes; id = the getter's stat id):

| Offset | Size | id | Field | Status |
|--------|------|----|-------|--------|
| 0 | u16 | 70 | sum of match ratings × 10 | verified (Ashby, in game) |
| 2 | u16 | 74 | minutes played | verified 26.3.2 (Matković's Ligue 1 minutes per goal 155.43 = 1 088 / 7 and per assist 272.00 = 1 088 / 4 in game). Matković: 1 088 league, 520 continental (7 starts), 2 for the unrated international substitute appearance |
| 4 | u8 | 60 | starts | verified (Ashby) |
| 5 | u8 | 61 | substitute appearances | verified (Ashby) |
| 6 | u8 | 71 | matches with a rating; ≤ starts + subs in every scope of both saves | verified 26.3.2: the game's average is rating sum / this count (Bernardi: 1 301 / 19 = 6.85 over 20 appearances). Hypothesis: late substitutes get no rating (Matković's 0 (1) international appearance has 0 here and shows "-") |
| 7 | u8 | 62 | goals | verified (Matković, in game: 7 league, 3 continental, 10 in all) |
| 8 | u8 | 66 | assists | verified (Matković: 4, 4, 8) |
| 9 | u8 | 63 | goals conceded (goalkeepers) | verified (code): the season row bridge `0x3584c0` maps id 0x3f to the C# property Conceded. Sami (12-09): 22 in Ligue 1, 27 in all club competitions. **In game** (Conc, 12-28): Ligue 1 22, Overall 27 |
| 10 | u8 | 64 | penalties taken | verified 26.3.2 (Esposito: Serie A 2 and 2 in slot 1; continental slot 1 taken, 0 scored, the game's Champions League Pens column shows 0). Taken >= scored in every scope |
| 11 | u8 | 65 | penalties scored | verified 26.3.2 (same samples). The game's "Overall (Club)" Pens column shows the taken total (3 for Esposito) while competition rows show scored (display quirk, inference) |
| 12 | u8 | 69 | player of the match awards | verified (Matković: 2 league, 1 continental, 3 in all). Save-wide (2035-12-09): one per 20–31 starts by slot, at most 13 in slot 5. With 22 starters a match that is 0.7–1.1 awards a match (about 0.7 in slots 1, 2 and 5), which fits one award a match where the competition gives one (inference) |
| 13 | u8 | 68 | red cards | verified (Ashby: 1; Matković: 0) |
| 14 | u8 | 67 | yellow cards | verified (Ashby: 7; Matković: 0) |
| 15 | u8 | 72 | clean sheets (goalkeepers) | verified (code): bridge id 0x48 → CleanSheets. Sami (12-09): 3 in Ligue 1, 6 in all club competitions. **In game** (Cln, 12-28): Ligue 1 3, Overall 8 |
| 16 | u8 | 73 | matches with a rating last season: the season-rollover snapshot (below) | verified (code); not exposed |

The 120 detail bytes are 60 i16 values (little-endian, signed), stat ids 0..59. `dN` is the N-th value, at byte 2N. Most are additive. Status: **in game** = equal to an in-game Season Stats column on 26.3.2 (Matković, Bernardi and Esposito in Ligue 1 / Serie A, Haaland in the Carabao Cup); **code** = named by the engine property chain or the accumulation code, not checked in game; **bytes** = a relation that holds on every scope checked. `savefile.MatchDetail` holds the fields marked "served"; `fm` reads them.

| dN | Field | Engine 4cc / source | Status | Served |
|----|-------|---------------------|--------|--------|
| d0 | team goals in the player's matches | POTg TeamGoals, match +0xc9 | code ("while on the pitch" is not shown: d0 ≥ goals fails in 10 % of scopes) | yes |
| d1 | team goals conceded in the player's matches | POTc TeamConceded, +0xca | code | yes |
| d2 | matches won | POgw | in game (8, 7, 1, 7) | yes |
| d3 | matches lost; drawn = appearances − d2 − d3 (derived id 0x4b) | POgl, `0x1ee5300` | code; d2 + d3 ≤ appearances in every scope | yes |
| d4 | passes attempted | POpa | in game (386, 384, 57, 307) | yes |
| d5 | passes completed | POpC | in game (320, 336, 46, 271) | yes |
| d6 | outfield: tackles. **Keeper: penalties faced** | POta / PGpf | in game (outfield: 21, 13, 0, 2; keeper: Sami PensF 1, Overall 2) | yes |
| d7 | outfield: tackles won. **Keeper: penalties saved** | POtc / PGps | in game (outfield: 17, 13, 0, 1; keeper: Sami "Penalties Conceded From/Faced" 0 / 1 = faced − saved) | yes |
| d8 | outfield: headers. **Keeper: saves held** | POha / POvh | in game (outfield: 62, 46, 9, 72; keeper: Sami Svh 27, Overall 41) | yes |
| d9 | outfield: headers won. **Keeper: saves parried** | POhc / POvp | in game (outfield: 24, 22, 1, 22; keeper: Sami Svp 25, Overall 31) | yes |
| d10 | dribbles | | in game (21, 1, 2, 10) | yes |
| d11 | fouls | | in game (11, 4, 0, 10) | yes |
| d12 | fouls against (times fouled) | Pofa FoulsAgainst, +0xa4 | code (bridge `0x358580`) | yes |
| d13 | shots | | in game (29, 2, 4, 66) | yes |
| d14 | shots on target | | in game (17, 1, 3, 32) | yes |
| d15–d18 | last season's goals, assists and points per game and average rating, × 100 | season rollover (below) | code | no |
| d19 | keepers: an expected-goals-against figure × 100 (post-shot, inference) | round(float +0x2c × 100) | code for the source; label inference | no |
| d20 | mistakes leading to a goal | POgm GoalMistakes, +0xc8 | in game (Sami 1, Matković 0, Nduquidi 1, Vassiljev 0) | yes |
| d21 | minutes since the last goal scored (set, not additive) | POlg, `0x1ee3adc` | code | no |
| d22 | keepers: minutes since the last goal conceded (set) | POlc, `0xf10830` | code | no |
| d23 | distance, tenths of a km | | in game (1606, 916, 161, 1056). The game shows distance per 90 as one tenth of the stored value (Matković 13.3 km per 90 shown as "1.3 km"; display quirk, inference) | yes |
| d24 | offsides | POof | in game (4, 0, 3, 8) | yes |
| d25 | outfield: open-play crosses attempted (= d45). **Keeper: saves tipped** | PopC / POvt | code; keeper in game (Sami Svt 6, Overall 10) | yes |
| d26 | outfield: open-play crosses completed. **Keeper: expected goals on target faced × 100**; the Season Stats screen's xG Prevented is d26 / 100 − goals conceded | PoPC / round(float +0x30 × 100) | code (outfield); keeper in game (Sami, below) | outfield; keepers as xG prevented |
| d27 | key passes | | in game (31, 6, 1, 17) | yes |
| d28 | key tackles | | in game (0, 1, 0, 0) | yes |
| d29 | key headers | | in game (3, 11, 0, 1) | yes |
| d30 | interceptions | | in game (28, 17, 2, 5) | yes |
| d31 | chances created | | in game (9, 1, 0, 2) | yes |
| d32 | outfield: blocks. **Keeper: shots on target faced** (inference) | Pobl Blocks, +0xd2 / keeper +0xd5 | code; not on the default Season Stats panel, whose "Shots Blocked" is d58 (Nduquidi 2036-01-01: d58 5 shown, d32 8); the keeper label is inference (save % 0x82 = saves / d32; saves + conceded ≤ d32 in 86 % of keeper scopes) | outfield only |
| d33 | goals from direct free kicks | PGFK, +0xd6 | code; ≤ goals in every scope | yes |
| d34 | possession lost | | in game (120, 51, 32, 124) | yes |
| d35 | possession won | | in game (77, 67, 4, 22) | yes |
| d36 | expected goals × 100 | | in game (426, 8, 62, 812; Vassiljev 5). Nduquidi's 35 shows as 0.33 (2036-01-01); with d37 (268 shown as 2.67) this fits sums of per-match values rounded to 0.01 while the screen sums unrounded ones (inference) | yes |
| d37 | expected assists × 100 | | in game (268, 55, 11, 8). The game shows 2.67 for Matković's 268 (differs by 0.01) | yes |
| d38 | clearances | | in game (9, 17, 2, 1) | yes |
| d39 | matches with a card byte of 2 (sent off for two yellows, inference) | +0xa6 == 2 | code for the source; label inference; ≤ red cards in every scope | no |
| d40 | free-kick shots | PFKS, +0xec | code; ≤ d46 in every scope | yes |
| d41 | goals from outside the box | PGOB, +0xea | code; ≤ goals in 98.8 % of scopes (unexplained) | yes |
| d42 | a shot subtype (18.6 % of shots) | +0xd3 | unknown | no |
| d43 | a subset of d42 | +0x97 | unknown | no |
| d44 | goals from d42 | +0xd4 | unknown | no |
| d45 | open-play crosses attempted, every player | +0xd7 + +0xd8 | code; equals d25 on all 303 191 outfield scopes (12-09) | yes |
| d46 | shots from outside the box | PSOB, +0xeb | code; d40 ≤ d46 ≤ shots in every scope | yes |
| d47 | an expected-goals-prevented figure × 100, signed; keeper branch only. The Season Stats screen does not use it (see d26) | PxGP, (float +0x2c − conceded) × 100 | code | no |
| d48 | sprints | +0xed | in game (209, 163, 7, 61) | yes |
| d49 | always 0 (the accumulator writes 0, `0xf110c0`) | | code | no |
| d50 | progressive passes | PGRp, +0xe9 | in game (37, 56, 0, 14) | yes |
| d51 | pressures attempted | PRSa, +0xe4 | code | yes |
| d52 | pressures completed | Pres, +0xe5 | code; ≤ d51 in 99.97 % of scopes | yes |
| d53 | open-play key passes | Popk, +0xbd | code; ≤ d27 in every scope | yes |
| d54 | non-penalty expected goals × 100 | PNxg | code; ≤ d36 in every scope | yes |
| d55 | expected-goals overperformance × 100 (goals − xG, per match) | PxGO | code | no |
| d56 | crosses | | in game (17, 2, 0, 4) | yes |
| d57 | crosses completed | | in game (10, 2, 0, 2) | yes |
| d58 | shots blocked | | in game (1, 4, 0, 0; Vassiljev 5, Nduquidi 5) | yes |
| d59 | headers lost; 0 for keepers | POhL, +0xc4 | code; equals d8 − d9 on every outfield scope (bytes, below) | yes |

**Goalkeepers** (verified, code + bytes). The accumulation tests the player's GK rating (`0xf10a78`: `vt+0x398(person) > 14`) and writes keeper stats into d6, d7, d8, d9, d25, d26 and d32, and nothing into d59. The engine providers apply the same test before reading keeper meanings (PGpf case `0x142e078`). In the bytes, GK rating (player offset 31) ≥ 15 splits the scopes exactly: d59 is 0 on every keeper scope and d8 − d9 on every other (test save 876 / 10 229 scopes, 12-02 63 792 / 524 729, 12-09 35 170 / 303 191 in slots 0–5 (`checkKeeperSlots`), 12-14 64 244 / 527 483). Other relations (12-09): penalties saved ≤ faced in every keeper scope; d47 = d19 − 100 × conceded in 2 466 of 3 107 league scopes with ≥ 900 minutes. Derived ids from code: total saves 0x80 = d8 + d9 + d25, save % 0x82 = saves × 100 / d32, expected save % 0x84 = 100 × (1 − (d26 / 100) / d32). `fm` serves the keeper meanings for players with GK rating ≥ 15 and marks their rows `goalkeeper`: goals conceded, clean sheets, saves held, parried and tipped, penalties faced and saved and xG prevented (d26 / 100 − conceded), in place of tackles and tackles won, headers and headers won, blocks and open-play crosses completed. d32's keeper meaning (shots on target faced) is an inference and not served. The game tests the GK rating per match; `fm` applies the player's current rating to the whole scope, so a player whose rating crossed 15 during the season would get a scope of mixed meanings served under one of them. None was seen: the split above holds on every scope of the four saves (about 1.2 M).

Sample (bytes, 2035-12-09): Bruno Sami (GK 20), Ligue 1: conceded 22, clean sheets 3, penalties faced/saved 1/1, held 27, parried 25, tipped 6, d32 79, d47 −250 (19.50 − 22), d19 1950, d26 1238, mistakes 1; all club competitions: 27, 6, 40/31/10, 106, d47 −132, d26 1567.

**Ground truth 4** (KŽ save 2035-12-28, 26.3.2, in-game Season Stats; Metz played no league match between 12-09 and 12-28, so the Ligue 1 bytes equal the 12-09 ones for both players). Sami, Ligue 1: Apps 16, Conc 22, Cln 3, Svt 6, Svp 25, Svh 27, PensF 1, "Penalties Conceded From/Faced" 0 / 1, total saves 58 (= 27 + 25 + 6), Saves/90 3.62, Conceded/90 1.38 (1 440 minutes), Mistakes Leading to Goals 1, Clearances 6, Interceptions 2: all equal the decoded keeper slots. **Expected Goals Prevented/90 −0.60** = (12.38 − 22) × 90 / 1 440 = −0.601, so the screen's xG prevented is d26 / 100 − conceded; d47 (−2.50) would show −0.16, and d19 − conceded the same. Overall (Club) on 12-28: Apps 24, Conc 27, Cln 8, Svt 10, Svp 31, Svh 41, PensF 2, equal to slot 5 of the 12-28 save. Matković, Ligue 1: the panel's figures (key passes 31, interceptions 28, clearances 9, headers won 24 of 62, tackles 21 at 81 % won, crosses 17 at 59 %, Sprints/90 17.29, Possession Won/Lost/90 6.37 / 9.93, Progressive Passes/90 3.06, Mistakes Leading to Goals 0) equal the decoded fields over 1 088 minutes. The default panel shows no team goals, pressures, open-play key passes, npxG or shots outside the box (fouls against: Fouls/90 0.91 fits d11 = d12 = 11), so those stay unchecked.

Scopes (per player):

| Slot | Meaning | Evidence |
|------|---------|----------|
| 0 | friendlies | **verified** 26.3.2: Matković's slot 0 equals the game's "Non Competitive" row: 3 (1), 1 goal, 2 assists, 7.25, 1 player of the match. Also the only scope for 531 players in the pre-season test save |
| 1 | league | **verified**: Matković's slot 1 equals the game's Ligue 1 row: 13 (2), 7 goals, 4 assists, 7.18 |
| 2 | domestic cups | **verified** 26.3.2: Haaland's slot 2 equals the game's Carabao Cup row: 2 apps, 2 goals, 1 assist, 7.65, 1 penalty |
| 3 | continental | **verified**: Matković's slot 3 equals the game's Champions League (2) and Europa League (5) rows summed: 7 starts, 3 goals, 4 assists, rating sum 546 = 2 × 69 + 5 × 81.6 |
| 4 | internationals: the senior national team; this international season (inference: where slot 4 is reset is not traced) | **verified** for competitive internationals: Matković's slot 4 equals the game's Euro Championship qualifiers row, 0 (1). Esposito's slot 4 equals the Euro Championship qualifiers row (6 apps, 7 goals, 2 penalties, 2 player of the match). Code: the match → scope function `0x31be038` returns 4 for a match between two national teams whose type byte `+0x28` is 0 without checking the competition, so senior friendlies go here too (that `+0x28` is the senior/youth type is inference), and 0xff (not recorded) when the player's own national team has `+0x28` ≠ 0 (youth). Esposito's "International League" (10 apps, 6 goals) and "European League Div. A" (6, 4) rows are in no slot 0–7 (inference: last season's Nations League, see Open questions). Vassiljev (2036-01-01): slot 4 holds 6 matches (rating 6.48, 6 lost), while the visible "International League" and "European League Div. C" rows are 6 other matches (rating 6.98 in both, the same competition listed twice, inference). Where slot 4 is reset is not traced. Not in slot 5 |
| 5 | current season, club competitions | **verified** (the game's "Overall (Club)" row): offsets 0–14 equal slot 1 + 2 + 3 in all 101 061 real-save records that have slot 5; Ashby's slot 5 is what the game shows. **Verified** (bytes) 26.3.2 for the detail block: the decoded detail fields equal slot 1 + 2 + 3 in 101 297 of the 101 298 records of the 2035-12-09 save that have slot 5 (d5 differs in one), and minutes (offset 2) in all. **Verified** (code): the accumulation `0xf0f220` adds a match to its slot and, unless the slot is 0, 4 or 0xff, again to slot 5 (`0xf10844…0xf1088c`) |
| 6 | club competitions, calendar year | **verified** (code): the add-to-scope function `0x3208e0c` adds every slot-5 add to slot 6 too; `0x1931264` calls `0x31be6bc` on day 2 of the year, which clears slots 6 and 7 for every player. Bytes: slot 6 ≥ slot 5 on 99.7 % of additive fields; Ashby 38 (3), Esposito 39 apps (slot 5: 17). Not exposed |
| 7 | internationals, calendar year | **verified** (code): every slot-4 add also goes to slot 7, cleared with slot 6. Esposito (12-09): 10 apps (slot 4: 6). Not exposed |

The team list (6 slots each, same order up to slot 5) holds stats at clubs the player left this season; slot 5 already includes them.

**Season rollover** (verified, code `0xf13bc4`, `0xf14904…0xf14a7c`). The game resets slots 0, 1, 2, 3 and 5 (a second site also 4); the reset `0x3208770` zeroes ids 0..73 except 15–18 and 73. Before it, for each slot 1..7, it stores a snapshot of the ending season: with r = the rated matches (id 71) and v(x) = x / max(r, 1), times r / 20 when r < 20, d15 = trunc((v(goals) + 0.005) × 100), d16 the same for assists, d17 = trunc((points per game + 0.005) × 100) (150 without appearances; points per game, id 0x62: 1.5 at 0 matches, (points + (10 − n) × 1.5) / 10 up to 9 matches, else points / n), d18 = trunc((average rating + 0.005) × 100), and offset 16 = r. Bytes (12-02 → 12-09): 33–60 of about 99 000 league scopes change d15–d18, those of players whose league rolled over that week; Alexandre Valbon (index 29 622: 13 rated, 1 goal, 2 assists) has (5, 10, 86, 658), and the rule gives 5 and 10. Not exposed (`fm` does not serve last season).

**Per-position stats** (verified, bytes): the `u32 n + n × 8` list after the team list holds `u8 2, u16 rating sum, u8 position (PositionNames index), u8 appearances, u8 goals, u8 assists, u8 rated`. Matković (12-09): ST (480, 7, 2, 0, 7) and AMR (1143, 15, 8, 8, 15), which sum to slot 5 (22 apps, 10 goals, 8 assists, rating sum 1 623). Keepers are position 0 in 3 266 of 3 365. The list equals slot 5 for 9 998 of 38 899 players and slots 1 + 2 + 3 + 4 for 9 136 others; which matches it counts is not resolved (hypothesis: the game's positional stats table, C# `PositionalStatsRowReference`). Not parsed.

Recent matches: 16 × 3 bytes from a packed u16 (`v & 0x7f`, `(v >> 7) & 0x1f`, `v >> 12`); the first byte is a match rating × 10 (Ashby 72, 64, 65…). The other two are unknown.

**Values not checked in game** (bytes, 2035-12-09, slot 1): Matković d0 31, d1 16, d3 1, d12 11, d25 = d45 16, d26 10, d46 7, d51 84, d52 20, d53 31, d54 426, d55 274 (7 goals − 4.26 xG), d59 38; slot 5 d51 122, d59 49. Esposito d12 19, d40 4, d41 2, d46 19, d51 22, d52 5, d53 17, d54 735, d55 −12, d59 50; slot 4: 6 starts, 7 goals, 2 penalties scored, 2 player of the match. Haaland d54 303, d46 6. Bernardi d12 1, d51 37, d52 5, d59 24.

**Ground truth** (real KŽ save, 26.3.2, 2035-12-02): Chris Ashby (index 146 891, uid 2 002 078 397, FC Metz) — the game shows 16 (2) appearances, average rating 6.58, 7 yellow and 1 red card. Slot 5: rating sum 1185, 16 starts, 2 subs, 18 rated (1185 / 18 = 65.8 → 6.58), 0 goals, 1 assist, 7 yellows, 1 red. League only (slot 1) would be 12 (1). The test save (2025-07-07) holds 890 season totals from the database's leagues running in July (e.g. Cavani, Boca: 28 starts, 16 goals). Both files parse to the last byte: 139 889 records (real) and 37 011 (test), every person a player.

**Ground truth 2** (real KŽ save, 26.3.2, 2035-12-09): Anton Matković (index 77 416, uid 2 000 301 456, FC Metz). Season Stats screen: Champions League 2 apps, 0 goals, 0 assists, 6.90; Europa League 5, 3, 4, 8.16, 1 penalty, 1 player of the match; Ligue 1 13 (2), 7, 4, 7.18, 2 player of the match; Euro Championship qualifiers 0 (1), no rating; Overall (Club) 20 (2), 10 goals, 8 assists, 3 player of the match, 0 yellow, 0 red, 7.38. Slot 5: rating sum 1 623, 20 starts, 2 subs, 22 rated (7.38), 10 goals, 8 assists, offsets 10–12 = 1, 1, 3, no cards. Slot 2 is absent (no domestic cup row in game).

**Ground truth 3** (real KŽ save, 26.3.2, 2035-12-09, in-game Season Stats): Matković: Ligue 1 minutes per goal 155.43 and per assist 272.00 (1 088 minutes); slot 0 equals the "Non Competitive" row (3 (1), 1 goal, 2 assists, 7.25, 1 player of the match); slot 5 minutes 1 608. Bernardi: Ligue 1 6 (8) and Overall 9 (11) show averages 6.75 and 6.85, which are rating sum / rated matches (877 / 13, 1 301 / 19), not / appearances. Haaland: slot 2 equals the Carabao Cup row (2 apps, 2 goals, 1 assist, 7.65, 1 penalty). Esposito: Serie A 2 penalties taken and 2 scored; his continental slot holds 1 taken, 0 scored. Fog of war does not hide Season Stats in game (Haaland and Esposito are unscouted).

## Open questions

- `rgman/player_stats.dat`: d42–d44 (a shot subtype, match record +0xd3, +0x97, +0xd4; counted with team counter +0x159 when a shot-type byte is 2, 7 or 10, `0x48e2b94`); the keeper labels of d19 and d32 and what d47 (PxGP) feeds and the d39 label; whether d0 and d1 count only while the player is on the pitch; in-game checks of every field marked "code" (Sami's goalkeeping columns, Matković's attacking and possession columns, a centre-back's blocks and mistakes, Matković's 2034/35 career row for the snapshot); where Esposito's "International League" and "European League Div. A" rows come from (in no slot; candidates `rgman/comp_<uid>.dat` match records or `fix_man.dat`) and where the game keeps per-competition rows (Champions League vs Europa League); where slot 4 is reset and the club path of `0x31be038` (`0x3caea64`); which matches the per-position list counts; recent-match bytes 2–3, the file trailer, versions ≤ 13.
