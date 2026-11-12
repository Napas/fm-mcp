# FM27 format: Match objects (match_stats_cfg.dat)

Part of the format reference. Conventions, dates and the index of all files: [`docs/format.md`](../format.md).

Per-match data beyond the goals in `FIXTURE_RESULT` (see [`fixtures.md`](fixtures.md)) lives in match objects, one per match the game played in full detail. Verified on bytes of 27.0.0 saves only. The serializer was not traced. No 26.x save was checked.

Samples are the Kauno Žalgiris (KŽ) career saves, named by game date (month-day): `data/test_save_27_interest.fm` (02-11), `data/test_save_27_instructions.fm` (05-04), `data/test_save_27_player_instructions.fm` (05-09) and `data/test_save_27_homegrown.fm` (07-16), all 2026. The `.scm` counts also use `data/test_save_27.fm` (01-19) and `data/test_save_27b.fm` (01-22). The in-game reference is the match report of Shamrock Rovers 3–2 KŽ (UEFA Champions League qualifier, 2026-07-15), viewed in game on 2026-11-11.

## match_stats_cfg.dat — verified 27.0.0 (bytes)

A pack file, inner file version 6 in every 27.0.0 save checked (01-22, 02-11, 07-16). It is 7.8 MB on 07-16 and holds 821 zstd-compressed match objects in four packs.

```
u32                            inference: the next match id (1 027 on 07-16, when ids 0..1026 exist; 4 on 02-11; 1 on 01-22)
u8 1, u8 1                     not decoded
u32 n                          packs: 4
n × pack:
  u32 index                    0..3
  string name                  "pks_0" .. "pks_3"
  u32, u8, u8                  not decoded (pks_0: 0x01068000, 1, 1); the u32 equals the trailer's first u32 in every pack
  u32 count
  count × (u32 match id, u32 offset, u32 size)
  u32, u32, u8 3, u32 length   the last u32 is the data length (= the largest offset + size); the first equals the u32 after the name; the second equals the data length except in pks_3 (below)
  data                         offset is relative to here; each entry is one zstd frame of exactly size bytes
  u8 1
```

Evidence (07-16):

- The 98 + 123 + 56 + 544 = 821 entries match the 821 zstd frames in the file. Every entry's offset lands on a frame magic. Each pack's data is followed by `01`, and the next pack starts right after it; the last `01` is the file's last byte. In `pks_3` the second trailer u32 (650 433) is not the data length (1 722 639); it is where the gap starts.
- Entries are in offset order and contiguous in `pks_0`..`pks_2`. In `pks_3` they are out of order, with one gap (inference: space freed by pruning, see below).
- **Match id**: every table id is the u32 after the version byte of a `FIXTURE_RESULT` in `fix_man.dat`, and the object carries the same id, score and attendance. All 821 objects, and the 30 `.scm` files below, map one-to-one to fixtures.

**Which matches have an object.** Only results with a match id (counts and the "full detail" inference: "27.0.0 match id and goal events" in [`fixtures.md`](fixtures.md)). On 07-16 all 821 objects belong to such results. Pruning (below) explains the 206 results with an id but no object.

**Packs.** On 07-16 (team indices from `get_fixtures` on a later save of the same career):

| Pack | Objects | Holds | Oldest match kept |
|------|---------|-------|-------------------|
| `pks_0` | 98 | matches of the human's club's teams: KŽ 29 (all its results), KŽ U18 21, KŽ B 19, the team `get_fixtures` names "FK Kauno Žalgiris (other team)" 19 (88 of 98; the other 10 not identified) | day 20 (177 days old) |
| `pks_1` | 123 | mostly the other TOPLYGA clubs' matches (about 20 each); 37 involve no TOPLYGA club, e.g. senior international friendlies (key 514) 15 and Under-21 internationals (3156) 9 | day 49 |
| `pks_2` | 56 | UEFA Champions League qualifiers 26, friendlies 6, competition 382 (KŽ U18's league) 6, others | day 45 |
| `pks_3` | 544 | the rest: friendlies 303, competition `514` 99, … | day 97 (100 days old) |

Membership rules are an inference from these lists.

**Retention.** Match ids in the four saves:

| Save | `pks_0` | `pks_1` | `pks_2` | `pks_3` | Ids dropped since the previous save |
|------|---------|---------|---------|---------|-------------------------------------|
| 02-11 | 4 | 0 | 0 | 0 | — |
| 05-04 | 34 | 44 | 11 | 204 | 0 |
| 05-09 | 54 | 65 | 19 | 255 | 0 |
| 07-16 | 98 | 123 | 56 | 544 | 206, all from `pks_3`, of matches played on days 45–97 |

No id moved between packs. `pks_0`..`pks_2` kept every match since their first one. The oldest `pks_3` match kept is also from day 97, so the pruning rule is not a plain age cut-off (inference). Whether any pack is pruned at the end of a season is not known: no sample spans a season change.

## Match object — partly verified 27.0.0 (bytes + in game)

A decompressed object is 8 583–244 153 bytes on 07-16 (30.9 MB for all 821). Every one starts `01 00 01 3c 1f`; the bytes after that vary (07-15: `22 00 01 14 00 15 90`; 545 objects continue `02 00 ff 00 ff 00`). Two kinds:

- 251 objects (64 831 bytes and more) hold the match setup section with the build string `FM27.…`.
- 570 objects (8 583–14 064 bytes) have no setup section: all 544 in `pks_3` and 26 in the other packs. No object falls between the two sizes. On `data/test_save_27_matches.fm` (11-12) 689 of 3 872 objects have one (18 %); on `data/test_save_27_shortlist.fm` (11-25) 685 of 3 385 (20 %).

Player records are in all 821. Offsets below are in the decompressed 07-15 object (match id 1022, `pks_0`; 234 959 bytes), counting the 3-byte `01 00 01` prefix. In a `.scm` file body the same fields sit 3 bytes earlier up to the end of the player records, and 4 bytes earlier from the setup section on (see ".scm files").

Section map of the 07-15 object (inference from strings and the anchors below):

| Offset | Holds |
|--------|-------|
| 0 | header: competition and stage names, stadium id and name, referee. TOPLYGA objects also hold the competition uid 2300046 (= `comp_2300046`) |
| about 440 | the `FIXTURE_RESULT` fields in another order: score bytes, then match id (`fe 03 00 00` = 1022), attendance, the remaining bytes, then the goal events |
| about 600 | home team block |
| 1 149 | home player records, one per squad member (23 for Shamrock) |
| about 14 000 | away team block |
| 14 535 | away player records (21 for KŽ) |
| about 23 000 | the game's injury name table (English) |
| 26 807 | match setup: build string `FM27.fm27-rel-03-launch-freepreview.2493426`, teams, players with names, tactics |
| about 59 000 to the end | not mapped; refers to players many times (candidates: event stream, positions for the 2D view) |

**Team stats** (verified 27.0.0, bytes + in game; `savefile.TeamStats`). Each side's totals sit before its first player record (07-15: home records start at 1 149, away at 14 535). Offsets are relative to that record:

| Offset | Size | Field | 07-15 home / away (report) |
|--------|------|-------|----------------------------|
| −296 | u8 | the side's own shots that were blocked | 1 / 4 (shot maps: Shots Blocked) |
| −295 | u8 | clear-cut chances | 5 / 3 |
| −283 | u8 | corners | 6 / 4 |
| −280 | u8 | fouls | 13 / 7 |
| −279 | u8 | offsides | 10-25: 3 / 3 (Advanced Stats); the players' offsides (record offset 62) add up to it on all 7 744 sides of 11-12 |
| −278 | u8 | yellow cards | 3 / 2 |
| −264 | u16 | passes attempted (without a setup section, not always the total: inference, see checks) | 468 / 500 (inference: 402 / 468 = 86 %, shown 85 %; 435 / 500 = 87 %) |
| −262 | u16 | passes completed | 402 / 435 (pass maps: Total Passes) |
| −258, −256 | u16, u16 | tackles attempted, won | 6, 4 / 12, 7 (Tackles Won 4/6, 7/12) |
| −254, −252 | u16, u16 | headers attempted, won | 26, 16 / 25, 8 |
| −168 … −158 | 6 × u16 | passes attempted and completed in the own, middle and final third, in pairs | completed 118, 215, 69 / 156, 223, 56 (pass maps); attempted 144, 245, 78 / 170, 261, 69 (inference) |
| −160 | u16 | passes attempted in the final third (FM's Final Third Passes) | 10-25: 100 / 55 (Advanced Stats). Kept only with the completed split and when between the completed final-third passes and all passes (always so on the saves checked). The players' offset 207 adds up to it on that match but on only 784 of 1 378 sides of 11-12 (off by 1–11) |

Home xG 1.98 is a float32 at 705 and away xG 2.18 at 14 085, before these blocks (−444 and −450 from the first records, so not at one fixed offset). Not decoded: the players' xG sums give the same values.

Parser rules (`savefile.readTeamStats`): a side's block is read only when its first record is at least 296 bytes in and, for the away side, the block starts after the home records end; completed must not exceed attempted for passes, tackles and headers; passes are capped at 2 000, tackles and headers at 200 and the u8 counts at 40. A block that fails gives no team stats for the match (the players still read) and is counted in the load warning "match team stats not read". Completed passes by third are kept only when they add up to the completed total.

Checks (bytes; on the 07-16 save unless another is named):

- All 821 objects, all 1 642 sides: completed ≤ attempted for passes, tackles and headers; values in plausible ranges (fouls 1–20, yellow cards 0–7, corners 0–19, passes 197–961).
- In the 251 objects with a setup section, completed passes by third sum to the completed total on every side. In the 570 objects without one, completed by third are all 0, so the parser leaves the split out. In objects with a setup section, attempted passes minus attempted by third is 0 on 470 sides and 1–2 on 32; in the others it is 4–156 on every side and the by-third values often repeat, so they are not attempted passes there (meaning unknown).
- `data/test_save_27_shortlist.fm` (3 385 objects, 6 770 sides with passes): one side has completed above attempted passes, the away side of match 8736 (pks_2, no setup section): 313 completed, 306 attempted (tackles 5 of 6 won, headers 23 of 42). The parser rejects that block, the only one on that save. Without a setup section, attempted minus completed is under 20 on 269 sides of this save (completion up to 99.4 %, 102 % on match 8736) and on 52 sides of the 07-16 save (up to 98.5 %); with one it is at least 32 on both saves. So −264 may not hold the attempted total there (inference).
- Against `player_stats.dat`: KŽ's 19 stored TOPLYGA matches give 43 yellow cards, the sum of the league yellow cards of the players who played for KŽ in them (43); fouls 204 vs 209, completed/attempted passes 11 100 / 12 356 vs 11 090 / 12 388, tackles 120 / 160 vs 128 / 173, headers 284 / 511 vs 304 / 529. Players' league totals also count games for other clubs, and keepers' slots mean other things, so close but not equal is expected (inference). FK Žalgiris and FK Sūduva show the same pattern.

The players' records add up to fouls, yellow cards, blocked shots, clear-cut chances and tackles in this block on every side (all 7 744 sides of `data/test_save_27_matches.fm` and the 1 642 of 07-16). Passes add up only in objects with a setup section (all 1 378 sides of 11-12); on the 6 366 sides without one they never do, which supports the doubt about −264 there (inference: the player sums are right, since every other total agrees).

Shots, shots on target, long shots, distance, progressive passes and crosses are not in this block at one fixed offset (07-15's −421/−420 hold other values than 11-08's), but the players' records add up to the report's values (see "Player record").

**Third match** (10-25): Kauno Žalgiris 2–2 FK Žalgiris on 2026-10-25, match 11123 of `data/test_save_27_matches.fm` (`pks_0`, with a setup section), checked against the in-game Key and Advanced Stats. Every total above and every player sum below matches: shots 11/14, on target 5/11, blocked 4/1, clear-cut chances 2/2, corners 8/10, headers won 22 of 38 and 15 of 38, tackles won 4 of 7 and 10 of 17, fouls 17/11, yellow cards 3/2, passes 486 of 557 and 368 of 432, crosses 5 of 20 and 6 of 26, progressive passes 48/38, distance 115.3/115.1 km (shown 115/115).

**Possession** (inference; `fm` computes it): a side's share of both sides' completed passes, rounded, the other side taking the rest of 100. It matches all three reports: 402/837 = 48.0 % (07-15, shown 48/52), 423/953 = 44.4 % (11-08, 44/56), 486/854 = 56.9 % (10-25, 57/43). The share of attempted passes does not (11-08: 44.9 %; 10-25: 56.3 %). Not stored there as a number: on 10-25 and 11-08 no u8, u16, u32 or float32 in the 1 100 bytes before each side's first record holds the two sides' values (as percent or fraction), and no player record offset sums to them.

Not located: OPPDA (07-15 15.39/14.41, 11-08 27.29/15.81, 10-25 13.46/14.74) and Ball Share (10-25 55/45, a separate stat from possession). OPPDA is not a float32 or float64 anywhere in the three objects. As a u16 × 100 it is missing from the 07-15 object (no 1441) and the 11-08 one (no 2729); the 10-25 one holds 1346 and 1474 once each, 3.6 KB apart in the unmapped tail (inference: chance). Ball Share is not a float32 pair in the 10-25 object (one 45.0, no 55.0). No ratio of the opponent's passes (total, completed, or by third) to the side's tackles, interceptions, fouls, possession won, clearances or blocks, in any combination of up to 4 of them, gives OPPDA on all three matches.

**Player record** (07-15: all 44 records; the ratings of all 32 players who played checked against the match report, xG against both team totals. Offsets 41–190: match 12438 of `data/test_save_27_matches.fm` (11-12 below), FK TransINVEST 1–1 Kauno Žalgiris on 2026-11-08 (11-08), all 31 players who played checked against the in-game Key Statistics tables of both sides):

| Offset | Size | Field | Evidence |
|--------|------|-------|----------|
| 0 | u8 | 2 | every record; bytes 7 and 12 are 2 too |
| 1 | i32 | person index | the game_db person index used by last lineups and goal events |
| 5 | u8 | shirt number | KŽ: Mikelionis 22, Kastrati 25, Hernández 23, Renan Oliveira 19, Benchaib 7, as on the pass map |
| 6 | u8 | side: 0 home, 1 away | every record |
| 8 | u32 | position bits at kick-off; 0 = did not start (inference: the bit meanings) | non-zero for the 22 starters, 0 for the 10 substitutes who came on and the 12 who did not play. Goalkeepers 0x1; Renan Oliveira (striker) 0x4000; Benchaib (attacking midfielder) 0x2000 |
| 13 | u32 | position bits; meaning differs by side (below) | non-zero for the 10 substitutes who came on (Lekiatas 0x4, as Kastrati's 0x4 at kick-off). On KŽ's side 0 for the players taken off (Kastrati, Kalik, Baldassarra, Fiolić, Kreković); on Shamrock's side non-zero for all 11 starters, though 5 were replaced |
| 20 | u8 | inference: goals | 07-15 only: 1 for O'Reilly-O'Sullivan, Byrne and Noonan, 2 for Renan Oliveira, 0 for the rest |
| 25 | float32 | expected goals | KŽ's records sum to 2.182, Shamrock's to 1.980; the report shows 2.18 and 1.98. Renan Oliveira 1.679 (2 goals), Byrne 0.437 |
| 29 | float32 | inference: expected goals faced in goal | 0 except for the two keepers who played: McGinty 2.182 and Mikelionis 1.980, the opponents' xG (1 178 and 14 564 in the object) |
| 136 | u16 | match rating × 100 | Byrne 819: the header shows 8.19, the player list 8.2 (rounded), the formation view 8.1 (truncated). Iyobosa Edokpolor 777 shows as 7.8 and 7.7, Kalik 646 as 6.5 and 6.4, Benchaib 729 as 7.3 and 7.2, Mikelionis 693 as 6.9. All 32 ratings match |
| 41 | u8 | shots | team sums 6/12 (11-08) and 19/17 (07-15), as the reports |
| 42 | u8 | shots on target | team sums 3/7 and 10/5 |
| 43 | u8 | shots blocked | team sums equal the team block's −296 |
| 44 | u8 | long shots | 10-25: team sums 4/7 (Advanced Stats); not above the player's shots on any played record of 11-12 |
| 45 | u8 | clear-cut chances | Kalik, Baldassarra 1; team sums equal −295 on every side |
| 56 | u8 | inference: assists | Milošević 1 (he assisted the 85th-minute goal) |
| 60 | u8 | fouls | Lekiatas 3, Šluta 2; team sums equal −280 on every side |
| 62 | u8 | offsides | 10-25: team sums 3/3 (Advanced Stats); team sums equal −279 on every side of 11-12 |
| 63 | u8 | yellow cards | Šluta, Konatar 1 (the report: 1 a side); team sums equal −278 on every side |
| 64 | u8 | red cards | bytes only: 1 on all 157 players of 11-12 with two yellow cards, and on 297 with none (straight reds); no red card checked in game |
| 67 | u8 | minute taken off; 0 otherwise | all 9 starters replaced on 11-08, both sides (33, 57, 65, 74, 83, 85). Never set for a player sent off |
| 71 | u8 | minute brought on; 0 otherwise | all 9 substitutes who came on (the Time column). Up to 120 (extra time) |
| 73 | u8 | inference: minute of the yellow card | Šluta 90, Konatar 75 (not checked in game). Only in objects with a setup section, and there set exactly when a player has one yellow card and no red (1 884 of 1 884) |
| 74 | u8 | inference: minute of the red card | set on every record with a red card |
| 86, 87 | u8, u8 | passes attempted, completed | Hernández 73/65, Baldassarra 79/73; team sums 482/423 and 592/530 (pass maps: 423 and 530 Total Passes) |
| 88 | u8 | key passes | Moutachy 3 |
| 89, 90 | u8, u8 | crosses attempted, completed | Traoré 11/1, Moutachy 7/3; team sums over players who played give 11 %/18 % (shown truncated) and 07-15's 25 %/22 %. On 11-12, 797 records of players who did not play hold 1–16 here, all in objects without a setup section (meaning unknown) |
| 92, 93 | u8, u8 | tackles attempted, won | Hernández 4/4; team sums equal −258/−256 on every side |
| 94 | u8 | key tackles | Hernández 2, Girdvainis 2 |
| 98 | u8 | interceptions | Hernández 6 |
| 140 | float32 | distance covered: metres in objects with a setup section, kilometres rounded to 0.1 in the others | Bertašius 3 983.7 (4.0 km), Moutachy 12 064.3 (12.1 km); team sums 114.9/117.2 km (shown 114/117) and 07-15's 117.1/115.1. Unit split checked on all 7 744 sides |
| 151 | u8 | possession won | Hernández 16 |
| 153 | u8 | clearances | Bertašius 7 |
| 186 | u8 | progressive passes | team sums 33/55 and 07-15's 54/43; no per-player value checked in game |
| 190 | u8 | sprints | Kreković 11, Musolitin 12 |

Squad members who did not play still have a record. Their rating is 670 in the 251 objects with a setup section (2 357 records; 07-15: all 12 the report shows as "-", e.g. Corr, Matthews, Švedkauskas) and 640 in the 570 without one (5 913 records). 268 records of players who played are also rated exactly 670, so the rating does not tell who played. Offsets 8 and 13 do: both are 0 for exactly the 12 players the report shows as "-", and at least one is non-zero for the other 32 (44 of 44). Offset 8 = 0 with offset 13 ≠ 0 marks a substitute who came on.

**Taken off.** Offset 67 holds the minute a player was taken off, on both sides of every object (11-08: all 9). Offset 13 is a weaker sign: 0 for a starter not on at the end only on 23 of KŽ's sides across the 821 objects of 07-16 (in 23 of KŽ's 29 matches), and non-zero for replaced starters everywhere else, so it does not tell who was taken off.

**Record length** is not decoded: 222 bytes or more (07-15: 222, 233, 240, 258, 273, 276, 294, 330). Longer ones hold extra data, probably shots (inference: Renan Oliveira's holds float32 values after the fixed part). A parser has to find records by scanning, not by stepping. Substitution minutes are at 67 and 71. Minutes played are not stored as such; the match length (added and extra time) is not located.

**Finding records** (verified 27.0.0, bytes; `savefile.ReadMatchObjects`). Record length is not decoded, so the parser scans each object for the record shape: tag bytes 2 at offsets 0, 7 and 12, side 0 or 1, a person from 0 to 2^24, no position bits outside `0x00307fff` (bits 0–14 and the flags `0x100000` and `0x200000`) at offsets 8 and 13, a finite xG of 0–10 at 25 and 0–20 at 29, a rating of 300–1000 at 136, and a finite distance of 0–30 000 at 140 (before the kilometre scaling; 17 922 metres seen). After a record the scan resumes past its 222-byte fixed part: inside two goalkeeper records on 07-16 (the 382nd and 781st objects in file order), bytes 139 in read as a substitute who came on, with float bytes as the person (about 1e9) (hence the cap on persons). An object whose sides do not have exactly 11 starters each, or that repeats a person, is kept as unread: `get_match` shows its goals only (`goals_only`) and a load warning counts it. Results:

- 07-16: all 821 objects read; every side has 11 starters (1 642 sides), with 11–26 records. Records of a side are contiguous (222–399 bytes apart), the home side's first, and no person repeats. 05-04 (293 objects) and 05-09 (393) read the same way.
- Without the position-bit test, 4 sides gain a twelfth "starter" (bytes with junk bit patterns; the 7 rejected lookalikes next to real records have float bytes as the person).
- Small objects carry real values too: mean xG 1.28 per side (7 sides at 0), played ratings spread 5.0–9.8.
- All 2 312 goals and own goals of the 07-16 results have their scorer as a played record on the right side (own goals: the other side).
- Against `player_stats.dat` (KŽ's 21 squad players, TOPLYGA 2026, where all of KŽ's matches are stored): starts and substitute appearances match the league totals for 20 of 21 players (Kazlauskas: 9 starts in the objects, 6 in the totals). Rating sums (each rating rounded to one decimal) match exactly for the 14 players whose rated-match counts match. The other 7 have more records with a rating than rated league matches (Kastrati 7 vs 5): records hold a rating for some substitute appearances the game leaves unrated (inference: very short ones), and those ratings vary (6.35–7.67), so they are not placeholders.

## .scm files — verified 27.0.0 (bytes)

Root files named `<home team index>_<stamp>.scm` hold one uncompressed match object each, after the usual 8-byte inner file header with tag `mst.` and version 0 (not `mcs.`, the extension reversed; see [`container.md`](container.md)). The body equals the packed object without its `01 00 01` prefix, except that the packed one has one extra `01` byte 18–29 KB in (after the player records, before the setup section) and 2 different bytes about 4 bytes before the end (`ff ff` in the `.scm`). Checked on all 30. Stamp = day count × 256 + kick-off (the day count's epoch is not known): KŽ v FK Banga (`16521_265567010`, 2026-02-28) and KŽ v FK TransINVEST (`16521_265579554`, 2026-04-18) are 12 544 = 49 × 256 apart, 49 days. The low byte 34 is probably a 17:00 kick-off (inference). Two cup matches played at the same time share a stamp.

Counts: none on 01-19, 01-22 and 02-11; 17 on 05-04; 26 on 05-09; 30 on 07-16. On 07-16 they cover matches from 2026-02-21 to 2026-06-09, about a month behind the game date, including youth matches and foreign friendlies. Each also has an object in `match_stats_cfg.dat`. Why the game keeps them is not known (inference: matches watched by scouts or analysts).

## Open questions

- `match_stats_cfg.dat`: the undecoded header and pack bytes; what decides pack membership; the `pks_3` pruning rule; whether packs are pruned at a season change; the `pks_3` gap.
- Match object: the header grammar; what decides an object's kind (with or without setup); OPPDA and Ball Share; how the game computes possession; whether 73 and 74 are card minutes; the position bit meanings at offsets 8 and 13 and why offset 13 differs by side; record length; offsets 20 (goals?), 21 (non-zero on keepers only and equal to the goals each conceded on 07-15: McGinty 2, Mikelionis 3), 56 (assists?), 61, 65, 66 (card reasons?), 68–70, 72 (non-zero on 575 of 116 116 played records of 11-12, e.g. Kloniūnas 33, taken off in minute 33) and 75–85; the sections after 26 807; what the attempted-passes field (−264) counts in objects without a setup section (below completed in match 8736).
- Player records: offset 207, which adds up to the final-third passes (−160) on only some sides; what offsets 89 and 90 hold on unused substitutes' records in objects without a setup section; which substitute appearances the game leaves unrated; Kazlauskas's 3 extra starts.
- `.scm` files: why they exist and when the game deletes them.
- 26.x: whether `match_stats_cfg.dat` (a root file of the 26.2.0 test save) and the `*.scm` files of 26.3.2 saves use the same layout (see [`container.md`](container.md)).
