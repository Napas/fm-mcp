# FM26 format: Fixtures and results

Part of the format reference. Conventions, dates and the index of all files: [`docs/format.md`](../format.md).

## fix_man.dat — verified 26.2.0, 26.3.2 and 27.0.0 (bytes)

`rgman/fix_man.dat` holds every fixture of the past, current and next seasons: 19 239 in the test save (seasons 2024–2027), 138 714 in KŽ (2035-12-02; seasons 2033–2035), 23 460 in the FM27 test save (2026-01-19; 209 played). The KŽ file is 31.5 MB decompressed. 27.0.0 uses the same list grammar.

```
u32, u32, u16, u32                       header (0, 1, 1929, 0 in the test save)
list                                     the first list
u32 n, n × list
…                                        not decoded (5 993 bytes test, 14.5 MB KŽ)

list:
  u16 season key, u8 present             not present: nothing follows
  u32, u32
  items until i32 -1:
    i32 index, u8 flag                   1 = FIXTURE_RESULT then FIXTURE base; 0 = base only
  u32 count                              equals the items read
```

`savefile.parseFixtures` caps lists at 64, items per list at 2^20 and events at 1 024, and rejects any other flag or a version byte other than `0x1c` (26.x) or `0x1f` (27.0.0). In each save checked, all FIXTURE and FIXTURE_RESULT records share one version.

## FIXTURE — verified 26.2.0 and 26.3.2 (code + bytes)

Vtable `0x54e6c20`, load `0x3e5c748`, save `0x3e5c0a8`. The base is 63 bytes:

| Offset | Size | Field |
|--------|------|-------|
| 0 | u8 | version `0x1c` |
| 1 | i32 | competition key: an ACTUAL_COMP_ID index (object `+0x20`; see [`competitions.md`](competitions.md)) |
| 5 | u32 | stadium |
| 9 | u16, u8 | unknown |
| 12 | i32 | home team (TEAM index) |
| 16 | 3 × u8 | unknown |
| 19 | i32 | away team (TEAM index) |
| 23 | 2 × u8 | unknown |
| 25 | date | match date |
| 29 | date | unknown (1900 for friendlies; often the season's end) |
| 33 | u32, u8 | unknown |
| 38 | u16 | season: its start year; 0 for friendlies |
| 40 | u32 | unknown |
| 44 | u32 | referee (guess) |
| 48 | 2 × u8 | unknown |
| 50 | u8 | round (object `+0x3b`); 0-based in leagues, 255 for friendlies |
| 51 | 12 bytes | unknown |

Evidence: Tottenham (547) v Burnley (442), key 232 (Premier League), match date 228/2025 = 2025-08-16, the real 2025/26 opening fixture; UEFA Super Cup (key 99) v Paris Saint-Germain on 2025-08-13.

### 27.0.0 FIXTURE base — verified 27.0.0 (bytes)

Version `0x1f`, 61 bytes. Offsets 0–32 are unchanged. Later fields sit 4 bytes earlier, and 2 bytes are added at the end. Which 4 bytes went is an inference: the u32 at 33 is `ff ff ff ff` in 127 759 of 128 128 `0x1c` bases of a 26.3.2 KŽ save (copied 2026-11-04), and the u8 after it is `0xff` in every base of both versions.

| Offset | Size | Field |
|--------|------|-------|
| 33 | u8 | unknown (offset 37 in `0x1c`) |
| 34 | u16 | season: its start year; 0 for friendlies |
| 36 | u32 | unknown |
| 40 | u32 | referee (guess) |
| 44 | 2 × u8 | unknown |
| 46 | u8 | round; 0-based in leagues, 255 for friendlies |
| 47 | 14 bytes | unknown |

Evidence, `data/test_save_27.fm` (2026-01-19):

- With 61-byte bases the whole file parses: every list count matches its items, leaving only the undecoded tail (2 029 bytes). Each base ends at the next item's i32 index (then flag 0 or 1, version `0x1f`) or at the list's `ff ff ff ff` end marker.
- The season and round offsets come from comparing value counts with that 26.3.2 KŽ save. Offset 34 holds 2025–2028 or 0, and the 1 054 fixtures with 0 are exactly those with 255 at 46. That matches 38 and 50 in `0x1c`, where the 17 190 friendlies have season 0 and round 255.
- The 2026 A Lyga (key 379) has 144 fixtures between 9 teams: 36 rounds (0–35) of 4 matches, so one team sits out each round. Kauno Žalgiris (team 16521) play each of the other 8 four times. They sit out round 0 and open at home to team 6215 on 2026-02-28 (59/2026). Dates mostly rise with the round; some rounds are out of order (round 31 on 2026-07-18).

Not checked in game. Not checked in code: the FM27 FIXTURE loader was not traced.

## FIXTURE_RESULT — verified 26.2.0, 26.3.2 and 27.0.0 (bytes)

```
u8 version 0x1c (26.x) or 0x1f (27.0.0), u32
4 bytes home score, 4 bytes away score   [0] 90 minutes, [1] after extra time, [2] penalties, [3] aggregate; 0xff = none
u32 attendance (guess: values like 62 500), u32, 4 × u8
u32 n, n × 16-byte events                not decoded
```

27.0.0 keeps the layout (29 bytes before the events). All 209 results in the FM27 test save parse, and each is followed by a `0x1f` base. The first is a 2026-01-03 friendly (key 2895) between teams 16654 and 16587, `00 ff ff ff` / `01 ff ff ff`: 0–1.

Byte 3 is the aggregate after a second leg (KŽ, 4 second legs checked). Leeds 0–2 Fulham on 2035-05-18 carries 0–4 after Fulham's 2–0 first-leg win; Blackburn 0–1 Crystal Palace on 2035-05-19 carries 1–5 after a 4–1 first leg. In the 2033/34 semi-final second leg at team 520 (2034-05-12), the bytes are `00 00 04 01` and `00 00 01 01`: 0–0 after 90 minutes and after extra time, 4–1 on penalties, 1–1 on aggregate.

Ground truth (in game, KŽ save of 2036-02-24, 26.3.2): FC Metz's Calendar → Fixtures screen matches `get_fixtures` for every match shown: 27 Jan to 24 Feb (7 results, including the French Cup Eleventh Round 1–1 shown as "p 1-1", stored as penalties 5–4) and 2 Mar to 23 Mar (7 fixtures). Dates, opponents (the screen shows short names such as "LOSC" and "Paris SG"; the server gives full names), home/away and stage labels match; the screen shows "UEFA Europa League League Phase", "French Cup Eleventh Round", "French Cup Quarter Final", "UEFA Europa League Round of 16 First Leg" / "Second Leg", as the server labels them. The Ligue 1 side panel's 23–24 Feb results and the unplayed Reims v Nice match too. Kick-off times are shown but not decoded.

## Open questions

- What the 14.5 MB after the last list in KŽ holds (5 993 bytes in the test save). A scan for FIXTURE bases (version `0x1c`, key in range, two distinct teams, a 2030–2037 date) finds none there.
- The second date, the unknown base bytes and the 16-byte events.
- Byte 3 outside second legs.
- The 27.0.0 base: the 2 added bytes, and whether versions `0x1d` and `0x1e` exist (no save has them; the parser rejects them).
