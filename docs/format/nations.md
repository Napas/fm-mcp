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
u32 j + j × 12   (0x360944c: trade blocs, see "27.0.0 trade blocs")
10 × i32, u16, u8, 11 × u8     (54 bytes)
```

The parser tries every position whose first byte is ≤ 20 and continent −1 or below the CONTINENT count and keeps the last one from which this tail ends exactly at the database record. One position fits for 249 of 251 nations; the United States and Ecuador have two, and the last is right (North America, South America). The result follows geography in the 12-02, 12-09, 12-14 and test saves: Europe 57 (including Israel and Kazakhstan), Asia 47 (including Australia), Africa 57, North America 43 (including Guyana, Suriname, French Guiana), Oceania 15, South America 10, none 22 (defunct nations such as the USSR and Yugoslavia). The table is split like COMP, except that of two database-record matches one byte apart the later wins: nation 0 (uid 5) also matches one byte early because its record ends with a zero byte. Head at body offset 28 258 413 (12-14), 16 745 015 (test save).

**NATIONAL_TEAM_CONTAINER** (writer `0x35f0368`): count = the NATIONAL_TEAM_CONTAINER pool (502). Containers 0..250 are the men's, in nation order, each ending with its nation's database record (`index, nation uid, nation uid`). At record offset 168 is `u8 +0x576`, at 169 `i16 +0x572`, the reputation factor. Samples (identical in the 12-09 and 12-14 saves): France 169, Italy 180, Spain 181, England 187, Croatia 137, United States 140, India 78, Equatorial Guinea 53; range 0..187. The women's containers (251..501) are not read. Head at body offset 28 370 369 (12-14), 17 111 983 (test save).

The CONTINENT and NATION tables and the factors are identical in the 12-09 and 12-14 saves. In the 26.2.0 test save the three tables parse and give the same continent counts; the factors were not compared with another source. When a table does not parse, `fm` leaves out every reputation level and adds a load warning.

## 27.0.0 national-team containers — verified 27.0.0 (bytes + in game)

The reputation factor is the i16 at record offset 168, one byte earlier than in 26.x (`savefile` gates on database version 3754, the first version seen; the game's gate may be lower: any version from 3734 up). Read at 169, every factor had a zero low byte (Lithuania `0xb300`). The 4 bytes before offset 168 match the 26.3.2 KŽ save (2037-07-10) in all 251 men's containers, and the 5 after the factor in 238. The u8 at 168 was 0 in all 251 containers of that save, so the dropped byte is probably that u8 (inference: no 27.0.0 writer code read). `savefile` rejects the table when a factor is outside 0..255, so a moved field gives the "reputation levels unavailable" load warning, not wrong levels.

Evidence: Lithuania 74 and Serbia 131 in `data/test_save_27.fm`, `test_save_27b.fm` and the 2026-01-23 save, whose factors range 0..189 with 22 nations at 0. Against the 26.3.2 save the factors correlate at 0.98: France 171 → 173, United States 140 → 140, Serbia 129 → 131, Slovenia 131 → 125, Lithuania 121 → 74. In game (2026-01-23), Davis Akusobi (KŽ B, Lithuania, current 2500, world 500) and Haris Kadrić (on loan at FK Novi Pazar, Serbia, 5000, 2000) are both National; the 26.3.2 rule gives that with these factors (Lithuania's national cut is the 2150 floor).

## 27.0.0 trade blocs — verified 27.0.0 (bytes); EU rule inferred

The NATION tail's last counted list (writer `0x360944c`) is the nation's trade-bloc memberships. `savefile` reads it as `Nation.TradeBlocs`.

| Offset | Size | Field |
|---|---|---|
| 0 | i32 | bloc type |
| 4 | date | from; 1900 when unset |
| 8 | date | until; 1900 when unset |

Evidence (`data/test_save_27.fm`, game date 2026-01-19; 175 items on 133 nations; every until is unset). Members by name, from the language database:

| Type | Nations | Members | Bloc (inference from the members) |
|---|---|---|---|
| 0 | 38 | the 27 EU members; French Guiana, Guadeloupe, Martinique, Saint Martin, Réunion, Mayotte; Albania, Bosnia and Herzegovina, Montenegro, North Macedonia and Serbia from 2030-07-01 | EU |
| 1 | 44 | type 0's nations, Iceland, Norway, Liechtenstein, Faroe Islands, Northern Ireland and Gibraltar; most from 1994 | EEA |
| 2 | 76 | African, Caribbean and Pacific nations (Nigeria, Jamaica, Fiji …) | ACP (Cotonou) |
| 3 | 7 | Bermuda, Cayman Islands, Anguilla, British Virgin Islands, Montserrat, Turks and Caicos Islands, Gibraltar | British overseas territories |
| 4 | 6 | Kazakhstan, Kyrgyzstan, Turkmenistan, Uzbekistan, Azerbaijan, Türkiye | Turkic states |
| 5 | 4 | Iceland, Liechtenstein, Norway, Switzerland | EFTA |

England has no items. Type 0's from is unset for 12 nations: Belgium, France, Germany, Italy, Luxembourg, the Netherlands and the six French overseas regions. The others, except the five 2030-07-01 entries, have 1 January of a year: Lithuania 2004, Croatia 2013. From general knowledge, not the save: these are the accession years, and the real days differ (1 May 2004, 1 July 2013).

`fm` serves `eu_national`: true when any of the player's nationalities has a type-0 item active on the game date (from inclusive, until exclusive; any of several items). No nation has more than one type-0 item in the 27, 27b and homegrown saves. That rule is an inference: the engine function that combines nationalities and blocs was not found. The C# front end has `IsEuNational`, `EuropeanUnion`, `EuropeanEconomicArea` and `EuropeanFreeTradeAssociation`. Other nationalities can include region nations (see "Relationships and favoured clubs" in players.md); whether the game skips them is not known.

KŽ squad on that save, for an in-game check:

| Player | Nationalities | `eu_national` |
|---|---|---|
| Fedor Černych | Lithuania, Russia | true |
| Franco Baldassarra | Argentina, Italy | true |
| Joris Moutachy | Martinique, France | true |
| Lirim Kastrati | Kosovo, Albania, Italy | true (Italy) |
| Renan Oliveira | Brazil | false |
| Anton Tolordava | Georgia | false |
| Marko Konatar | Serbia | false (until 2030-07-01) |
| Yukiyoshi Karashima | Japan | false |

Harry Kane is true through his second nationality, the Republic of Ireland.

## Open questions

- The national-team container body beyond the factor (the women's container → nation rule is inferred); the NATION record before its tail.
- Trade blocs: whether FM's EU National label follows the type-0 rule above (not checked in game), the blocs of types 1..5, and whether the 26.x tail holds the same list (not checked).
