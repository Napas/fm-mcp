# FM26 format: Continents, nations and national-team containers

Part of the format reference. Conventions, dates and the index of all files: [`docs/format.md`](../format.md).

## Continents, nations and national-team containers — verified 26.3.2 (code + bytes), 26.2.0 (bytes)

Read for the reputation level (`savefile.readNationData`). All three tables are in `game_db.dat`, each starting `u32 count, u8 0` right after a table end marker; records end with a bare database record `u32 index, u32 uid, u32 uid`.

**CONTINENT** (writer `0x3260970`): 7 records, ending right before the CURRENCY_RECORD table's head. There is no CONTINENT memory pool, so the parser finds the currency head, takes the count from the last continent's index (just before the end marker there) and looks back for the matching head; the records must split exactly up to that end marker. Record: `u8 tag 0, 3 × u8, f32 strength (+0xe4), …` (the rest not decoded), uid = index + 1.

| Index | Continent | Strength | s1 |
|---|---|---|---|
| 0 | Africa | 0.85 | 0.70 |
| 1 | Asia | 0.85 | 0.70 |
| 2 | Europe | 0.99 (`a4 70 7d 3f`) | 0.98 |
| 3 | North America | 0.85 | 0.70 |
| 4 | Oceania | 0.85 | 0.70 |
| 5 | South America | 1.0 | 1.0 |
| 6 | World | 0.0 | 1.0 |

The parser rejects strengths outside 0..1. Body offsets of the head: 27 979 587 (12-14), 16 515 751 (test save).

**NATION** (writer `0x35bf574`, save path): count = the NATION pool (251), records in ascending uid order (the order the language rule relies on). The start of a record holds lists not decoded; the end before the database record has a fixed grammar:

```
u8 +0x1f3, i32 continent (+0xf8; −1 = none), i32 +0x100, i32 capital city (+0xf0), i32 +0x108,
u8 +0x1f4, u8 +0x1f8, f32 +0x1dc, u8 +0x1f5,
u8 n  + n × 5    (0x3609bf0: i32 ref, u8)
u32 m + m × 28   (0x35d3624: 5 × i32 ref, 2 × date)
u32 k + k × 28   (same)
u32 j + j × 12   (0x360944c: i32 ref, 4 × u16)
10 × i32, u16, u8, 11 × u8     (54 bytes)
```

The parser tries every position whose first byte is ≤ 20 and continent −1 or below the CONTINENT count and keeps the last one from which this tail ends exactly at the database record. One position fits for 249 of 251 nations; the United States and Ecuador have two, and the last is right (North America, South America). The result follows geography in the 12-02, 12-09, 12-14 and test saves: Europe 57 (including Israel and Kazakhstan), Asia 47 (including Australia), Africa 57, North America 43 (including Guyana, Suriname, French Guiana), Oceania 15, South America 10, none 22 (defunct nations such as the USSR and Yugoslavia). The table is split like COMP, except that of two database-record matches one byte apart the later wins: nation 0 (uid 5) also matches one byte early because its record ends with a zero byte. Head at body offset 28 258 413 (12-14), 16 745 015 (test save).

**NATIONAL_TEAM_CONTAINER** (writer `0x35f0368`): count = the NATIONAL_TEAM_CONTAINER pool (502). Containers 0..250 are the men's, in nation order, each ending with its nation's database record (`index, nation uid, nation uid`). At record offset 168 is `u8 +0x576`, at 169 `i16 +0x572`, the reputation factor. Samples (identical in the 12-09 and 12-14 saves): France 169, Italy 180, Spain 181, England 187, Croatia 137, United States 140, India 78, Equatorial Guinea 53; range 0..187. The women's containers (251..501) are not read. Head at body offset 28 370 369 (12-14), 17 111 983 (test save).

The CONTINENT and NATION tables and the factors are identical in the 12-09 and 12-14 saves. In the 26.2.0 test save the three tables parse and give the same continent counts; the factors were not compared with another source. When a table does not parse, `fm` leaves out every reputation level and adds a load warning.

## Open questions

- The national-team container body beyond the factor (the women's container → nation rule is inferred); the NATION record before its tail.
