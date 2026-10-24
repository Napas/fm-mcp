# FM26 format: Player record

Part of the format reference. Conventions, dates and the index of all files: [`docs/format.md`](../format.md).

## Player record (tags 2 and 3)

Write order in `0x2f9c868`: tag, player part, ACTUAL_PERSON part (`0x2f424a0`, partly decoded: contracts, and the trait mask at its end), BASIC person part (`0x2f85600`), trailer.

Player part, offsets from the record start:

| Offset | Size | Field | Status |
|--------|------|-------|--------|
| 0 | 1 | tag | verified |
| 1 | 3 × u16 | home, current and world reputation, 0..10000 | verified (code): the writer `0x2f9c8e4` copies these 10 bytes (with CA and PA) from ACTUAL_PLAYER `+0x256`; the property setters log them under the editor field codes `"hr"` (`0x4eea978`), `"cr"` (`0x4eeb9d2`, setter `0x2f2c434`, clamped to 10 000) and `"wr"` (`0x50f7de3`) for home, current and world reputation, followed by `"ca"` and `"pa"` at offsets 7 and 9 |
| 7 | i16 | Current Ability, 1..200 | values verified; label a guess |
| 9 | i16 | Potential Ability, 1..200 | values verified; label a guess |
| 11 | 8 | unknown | |
| 19 | i32 | TEAM index of the national squad the player is called up to; -1 = none | verified (bytes): always a national team, and it agrees both ways with the national teams' player lists (see "Squad lists") for 12 731 players of the real 26.3.2 save and 5 547 of the test save |
| 23 | i64 | current **team** index (Tottenham's first team = 547); see "Teams" | verified |
| 31 | 15 × u8 | position ratings 1..20: GK SW DL DC DR DM ML MC MR AML AMC AMR ST WBL WBR | verified by code (loader `0x48c6228`, side-flag table `0x51f9658`; SW by elimination) |
| 46 | 54 × u8 | attributes 1..100, in `AttributeNames` order (`internal/savefile/player.go`) | verified by code (loader `0x48c6228`, display table `0x520380c`, foot tie-break `0x48b97c4`). The game shows `clamp((v+2)/5, 1, 20)` (`0x48b9fc0`). "Off the Ball" is "Movement" in the engine |
| 100 | i32 | value in GBP; the game caps it at 300 000 000, which it shows as "Not for Sale" | guess (strong): Maddison 51.5M, Kulusevski 84.7M; the cap verified as "Not for Sale" in game (Livramento, Sombath) |
| 104 | u8 | 1..20, unknown | |
| 105 | date | joined current club; 1900 = unset (see "Dates") | verified 26.3.2 in game: Leroux 2034-01-13 ("Transferred from Rennes") and Bernardi 2035-01-12 ("from Ajax"), both equal to the contract start; 1900-01-01 on loanees and free agents (seen in game) |
| 109 | date | 1900 | |
| 113 | u16 | guess: match sharpness 0..10000 | 2035-12-09: the game shows icons only; the five players whose sharpness icon is not a tick have the five lowest stored values (5150–5913). Consistent; exact percentages not checkable |
| 115 | i16 | 0 / −125 / −250 | |
| 117 | u16 | guess (strong): condition 0..10000 | injured players 9245–9480. 2035-12-09: the game shows icons only; starters of the last match have 5800–7400, the rest 8590–10000. Consistent; exact percentages not checkable |
| 119 | u16 | bitmask | |
| 121 | 3 × i16 | unknown | |
| 127 | u8 | constant 2 | |
| 128 | i16 | height in cm | Maddison 175, Vicario 194 |
| 130 | … | injury list (below), further fields not decoded | |

### Reputation level — verified 26.3.2 (code + 17 in-game samples)

The profile's reputation word (C# property `Reputation`, 4cc `n0Iz`) maps to the engine person property `'Pcrp'` (player bridge `0x3564d0`, case at `0x35b7d8`). The person provider (`0x36ae7c4`) calls `0x2fbe6a0` for players. `0x2f27b8c` is the staff rule: it reads the ACTUAL_NON_PLAYER reputations (`+0xd6`, `+0xd8`, setters only in the non-player vtables). A player-coach gets the better (lower) level of the two (`0x36a66ac`); the staff inputs are not decoded.

`0x2fbe6a0` reads ACTUAL_PLAYER `+0x258` and `+0x25a`, which the player writer copies from save offsets 3 (`cr`, current) and 5 (`wr`, world). All arithmetic is f32; `trunc` truncates toward zero:

```
N   = nation of the current team (offset 23): the club's nation, or the national team's
C   = N's men's national-team container (N +0x110); a women's club uses N +0x118
      without team, club or container: the person's nationality (vt +0x210)
s0  = max(1, int8(round(C +0x572 / 10))) / 20 round half away from zero, cut to int8 (0x2fbe778, sxtb); 1 without a container
f   = strength of N's continent (N +0xf8 → +0xe4); when N has none or it is 0 (0x2fbe7cc), of the nationality's
s1  = f != 0 ? f + (f − 1) : 1                 1 when neither has a continent of non-zero strength

Tn = clamp(trunc(4750 · s1 · s0), 2150, 5250)
Tr = trunc(2500 · s1 · s0);  if Tn < Tr: Tr = Tn − 1250
Tl = trunc(500 · s1 · s0);   if Tr < Tl: Tl = max(Tr − 1250, 1)
Tc = trunc(5250 · s1) + (s1 > 0.95 ? 1500 : 0);  if Tc > 7250: Tc = 6500

wr > 7249               → 1 Worldwide
wr ≥ 3626 and cr ≥ Tc   → 2 Continental
cr ≥ Tn                 → 3 National
cr ≥ Tr                 → 4 Regional
cr < Tl                 → 6 Obscure
otherwise               → 5 Local
```

The words are the language group 227312…227317 in order. 1 = Worldwide, 2 = Continental, 3 = National and 5 = Local are verified by the samples; 4 = Regional and 6 = Obscure follow the order (inference). The C# side looks the word up; its table was not found. The provider has no fog-of-war gate (code), and the game shows the word for unscouted players (Cardozo).

Inputs: the factor (container offset 169) and the continent strength are in "Continents, nations and national-team containers". With France (factor 169, Europe 0.99): s0 0.85, s1 0.98, Tc 6645, Tn 3956, Tr 2082, Tl 416. With the United States (140, North America 0.85): s0 0.70, s1 0.70, Tc 3675, Tn 2327, Tr 1225, Tl 245.

Ground truth (KŽ saves, 26.3.2, profile screens): 12/12 on 2035-12-14, 2/2 on 2035-12-09.

| Player (uid) | Team | cr | wr | Rule | In game |
|---|---|---|---|---|---|
| Carlo Bernardi (2 002 100 552) | Metz | 6671 | 3645 | Continental | Continental (12-09 and 12-14) |
| Anton Matković (2 000 301 456) | Metz | 6765 | 4975 | Continental | Continental |
| Stephen Ly (2 002 078 767) | Metz | 6663 | 5383 | Continental | Continental |
| Louis Leroux (2 000 191 040) | Metz | 6584 | 3387 | National (wr < 3626) | National (12-09 and 12-14) |
| Ilan Assongo (2 000 478 936) | Metz | 6460 | 3316 | National | National |
| Pablo Rodríguez (2 002 099 166) | Metz | 6748 | 3507 | National (wr < 3626) | National |
| Pablo Vilas (2 002 115 328) | Metz | 6153 | 3078 | National | National |
| Asier Castillo (2 002 098 873) | Metz | 6441 | 3629 | National (cr < 6645) | National |
| Mounir Haddad (2 002 206 940) | Metz | 4778 | 2655 | National | National |
| Jaka Čuber Potočnik (2 000 196 612) | Metz | 6269 | 4282 | National (cr < 6645) | National |
| Jesús Lobede (2 000 245 491) | Metz | 5931 | 4168 | National (cr < 6645) | National |
| Rahul Cardozo (2 000 280 809), unscouted | Jacksonville Armada U23 (USA) | 3046 | 1841 | National | National |

2036-01-01 (in game): Enzo Fernández (uid 14 229 525, Chelsea, wr 8532) Worldwide; Théo Emmanuelli (uid 2 002 296 691, FC Metz U19, France, cr 1988 between Tl 416 and Tr 2082) Local; Jesús Lobede National.

Castillo (wr 3629 against 3626) and Bernardi (cr 6671 against 6645) sit close to the cuts. `fm` computes the word for men's players only: women's clubs read the women's container, whose factor is not read, and player-coaches need the staff rule. For a player without a team it uses his nationality (inference: vt `+0x210` is not traced). Counts on 2035-12-14: Worldwide 291, Continental 1 958, National 29 707, Regional 41 069, Local 33 844, Obscure 2 009; 30 801 left out (23 210 women, 8 747 player-coaches).

### Current injuries — verified 26.2.0 and 26.3.2 (bytes)

Current injuries are a list `u8 n + n items` in the variable part of the player part, preceded by a `u8 1` in every sample. Each item:

```
u8   2
u16  injury index        into the save's INJURY table (see "Injury types"); 84 = bruised ankle
u8   m                    match records; 0 or 1 in every sample
m × 26 bytes              match record: u8 1, date (the match day), then not decoded
date start
u16  days min             expected return: start + days min … start + days max
u16  days max
u8, u16, i32              not decoded
u8, i32 team index        the player's team when injured
u8, i32                   -1 or 0 in every sample
3 × u8, u16 0xffff
```

Knocks picked up in a match carry a match record; an item with one is 26 bytes longer. Examples (real KŽ save, 2035-12-02):

| Player | Item | Decoded | In game |
|--------|------|---------|---------|
| Danilo Veiga (uid 83261126) | `02 54 00 01` + match record `01 50 5d f3 07 20 01 00 00 02 00 8c 02 00 00 00 9e 02 00 00 03 01 6e 0d 03 00` + `50 6b f3 07 00 00 02 00 01 01 00 ff ff ff ff 00 8c 02 00 00 01 ff ff ff ff 00 05 00 ff ff` | index 84 (bruised ankle), since 2035-12-02, 0–2 days | "A bruised ankle, 1–2 days" |
| Carlo Bernardi (uid 2002100552) | `02 58 00 01` + match record + start 2035-12-02, 0–3 days | index 88 (tight groin) | "A tight groin, 1–3 days" |
| Aziz Diakhaté (uid 2002183764) | `02 54 00 00 4e 43 f3 07 00 00 03 00 …` | index 84 (bruised ankle), since 2035-11-30, 0–3 days | not checked |

Counts: 1 321 items in the test save (= the PLAYER_INJURY pool, 29 with a match record) and 8 603 in the real one (1 054 with a match record). Every item's window equals an entry of `injury_manager.dat` (below) for the same person, and every entry for a decoded player matches an item. The list's position is not decoded, so the parser finds it by validating items before the contract marker. A Python implementation of the same rule finds identical items for every player of both saves.

The game shows the days left counted from the next day: Veiga's and Bernardi's stored minimum is 0 and the game shows 1. No item in the real save has a latest return before the game date (the game drops an injury at its latest return, see `injury_manager.dat`). So `fm` reports the earliest return as max(start + days min, game date + 1 day) and the latest as max(start + days max, that earliest day), plus one more day while start + days min is after the game date. Four in-game samples (26.3.2) fit: Veiga 0–2 from 12-02 on 12-02 shows 1–2 days; Bernardi 0–3 from 12-02 on 12-02 shows 1–3 days and from 12-07 on 12-09 shows 1 day; Leroux 7–12 from 12-04 on 12-09 shows 2–8 days (12-11 … 12-17). The rule is an inference from these four; Diakhaté gets 2035-12-03.

Example (test save): Maddison, index 27 (torn anterior cruciate ligament) since 2025-07-07, back 2026-03-31..05-01.

### BASIC person part — verified (code + bytes)

Found by scanning for the record's last match; the ACTUAL_PERSON part before it holds a dummy copy dated 1900:

```
u32 first-name index  + u8 flag (0; -1 = none)
u32 surname index     + u8 flag
i32 common-name index + u8 flag (-1 = none)
string full name      (often empty)
date  birth date      (verified: Maddison 328/1996 = 1996-11-23)
9 bytes               unknown
i64 nation index      (names are not in the save; 139 = England is inferred)
8 × u8                hidden attributes 1..20: Adaptability, Ambition, Loyalty, Pressure,
                      Professionalism, Sportsmanship, Temperament, Controversy (verified by code, below)
i64 guess: birthplace city index (1040 = London)
u8 present            1 when the person has relationships (below), else 0
[relationships]       when present: u8 n + n × 16-byte item, then a second vector (not decoded)
…                     not decoded
```

Verified on the test save: 37 011 players parse. 27 have team 547. Maddison: index 8804, uid 28 093 567, CA/PA 151/163, nation 139. Vicario: born 1996-10-07, GK 20. A Python implementation by a different route produces identical names, birth dates, nations, clubs and CA/PA for all 37 011.

The 8 hidden attributes: verified by code (26.3.2). The string-keyed loader `0x2f89dac` stores adaptability → person `+0x40`, ambition `+0x41`, loyalty `+0x42`, pressure `+0x43`, professionalism `+0x44`, sportsmanship `+0x45`, temperament `+0x46`, controversy `+0x47`, each clamped to ≤ 20. The BASIC writer `0x2f85600` writes the nation ref (`+0x38`), then these 8 bytes (`0x36bd9e0(obj+0x40)`), then the birthplace, which is the save order. Getters `0x2f92cf8`…`0x2f92dd8`. All values are 1..20 in the real KŽ save; Controversy is skewed low. Determination is attribute 51 of the 54 (the player override `0x2ffedf8` of the getter reads player `+0x18a` and returns `clamp((v+2)/5, 1, 20)`). Leadership is attribute 40, read the same way at `+0x17f`; the offsets differ by 11, as the indices do. Samples (in-game personality below): Ashby `07 0b 10 0c 0f 11 11 03`, Sami `0d 13 0d 08 0d 08 0a 06`, Livramento (uid 28 124 569) `0c 0e 0e 0f 11 0f 10 02` (2035-12-02); Bernardi `0a 10 0e 10 0f 12 0f 06`, Leroux `0b 0b 0d 0b 0d 0c 0b 05` (2035-12-09). The server shows them with the other attributes, hidden like Consistency under fog of war. The game never shows them.

### Relationships and favoured clubs — verified 26.3.2 (code + bytes + 4 in-game samples)

The BASIC writer `0x2f85600` writes, after the birthplace ref (`+0x50`), `u8 present` (1 when the vector at `+0x48` is set) and then the vector with `0x36e0d0c`: `u8 n` (the writer caps it at 125), n items written by `0x37bff84`, then a second vector `u32 m + m × …` (from `+0x18`; not decoded). Each item is 16 bytes, the same in memory and in the save except that the object pointer becomes a ref:

| Offset | Size | Field | Status |
|--------|------|-------|--------|
| 0 | i64 | object ref: a table index of the kind at offset 10, sign-extended (−1 = none) | verified (code + bytes): 474 = FC Metz club, 652 = Metz first team, 158 530 = the human manager, 139 = England |
| 8 | u8 | reason: for favoured clubs 1 = supporter, 2 = as a player | verified in game (Matković 1 and 2, Čuber Potočnik 2); other values (0, 4; 5, 6, 8, 11 on person items) unlabelled. The list builder `0x2f86778` copies it next to each listed club (`0x2f87298`) |
| 9 | u8 | 0 | |
| 10 | u8 | object kind: 1 CLUB, 3 PERSON, 4 TEAM, 8 NATION, 0 none; 6, 9, 0x33–0x38 written without an object | verified (code) |
| 11 | u8 | relation type: 3 = favoured club; the getter `0x2f92e50` (person vtable `+0x628`) matches type 3, its sibling `0x2f92eb8` type 4 | verified (code) |
| 12 | i8 | value: −50..100; the getter returns it | verified (code) |
| 13 | 3 bytes | e.g. `4e 00 ff` | not decoded |

Club affinities are the items with kind 1 and type 3 (`savefile.ClubAffinity`, `Player.ClubAffinities`); the game lists those with value ≥ 50 as favoured clubs (below), and a negative value is probably a disliked club (hypothesis). Counts (bytes, 2035-12-09): 48 322 players hold 51 475 club-affinity items; values 100 ×39 944, 50 ×8 224, 75 ×1 259, then smaller groups down to 1 (87 items of 1) and 7 negative items; reasons 1 ×35 572, 2 ×15 902, 0 ×1. The 26.2.0 test save has 11 731 items on 8 188 players, all positive. Every ref is a valid CLUB index in all four saves checked (test, 2035-12-02, 12-09, 12-14), and every player record still parses. The parser reads only the n items; it treats a present byte other than 0 or 1, n > 125 or items running past the record as no favoured clubs, without failing the player.

**Which items the game lists** (verified, code): three functions build favoured-club lists by walking the vector and keeping items with type 3, value ≥ 50 (`ldrsb [item, #0xc]; cmp #0x32; b.lt skip`), kind 1 and a non-null club: `0x2f87260…0x2f87280` (in `0x2f86778`, which also copies the reason), `0x1392144…0x1392168` and `0x17839c8…0x17839ec`. Which of them feeds the profile screen is not traced; all four in-game samples fit the rule. The affinity update `0x36e28e4…0x36e2910` keeps a lowered value at 50 for reasons 1–4 when it was ≥ 50, so a listed club stays listed (inference: the decrease path stores 50 instead of going lower).

Ground truth (in game, KŽ save 2035-12-14, profile "Favoured Clubs"; bytes from the same save):

| Player (uid) | Items (club, value, reason) | Game shows | Rule (≥ 50) |
|---|---|---|---|
| Anton Matković (2 000 301 456) | R. Madrid (1115) 100, 1; NK Osijek (190) 100, 2 | As a player (1): Osijek; As a supporter (1): R. Madrid | match |
| Jaka Čuber Potočnik (2 000 196 612) | NK Maribor 50, 2 | Maribor, as a player (1) | match |
| Louis Leroux (2 000 191 040) | AS Longeron Torfou (19 201) 40, 2; FC Metz (474) 8, 2 | "No favoured club yet." (0) | match |
| Jesús Lobede (2 000 245 491) | none | 0 | match |

Matković's, Čuber Potočnik's and Leroux's items are the same on 12-02, 12-09 and 12-14. Test save (26.2.0, bytes, not seen in game): Matković, then at NK Osijek, has only R. Madrid (1086) 100, 1; Čuber Potočnik has none.

**Favoured personnel** (verified, bytes against 5 in-game samples; not served). The profile's "Favoured Personnel" lists the items with kind 3 (PERSON), relation type 1 and value ≥ 50, the same cut as favoured clubs. The reason byte gives the group: 6 manager, 10 friend, 8 staff; 5 is probably a teammate (inference: every reason-5 item seen is below 50, e.g. Palmer, Caicedo, Nduquidi). Whether a manager shows as "Manager" or "Former Manager" is probably whether he manages the player's team now (inference). Kind 3 items of type 2 hold values 10, 20 and 25 in the samples (meaning unknown). Samples (2036-01-01):

| Player | Items ≥ 50 (person index, reason, value) | Game shows |
|---|---|---|
| Enzo Fernández | 2275 (6, 83), 9029 (6, 61), 10965 Julián Álvarez (10, 85), 10966 (10, 75) | Manager Anes Mravac, Former Manager Enzo Maresca, Friends Julián Álvarez and Federico Girotti; header count 5 (unexplained: 4 items) |
| Théo Emmanuelli | 158 530 human (6, 100), 53 811 (8, 100) | 1 Staff, 1 Manager |
| Anton Matković | 158 530 (6, 100), 2431 (6, 60) | 2 Managers (2035-12-14) |
| Jaka Čuber Potočnik | 158 530 (6, 74), 33 789 (6, 50) | 2 Managers (2035-12-14) |
| Louis Leroux | 158 530 (6, 100); 32 361 (6, 18) and 277 614 (6, 16) left out | the human manager (2035-12-14) |

Names of non-player persons (managers, staff, Girotti at 10966) need the non-player record layout, not decoded.

`fm` lists favoured clubs with value ≥ 50, in save order, with `as` "supporter" (reason 1) or "player" (reason 2) and no label otherwise (`ClubAffinity.ReasonName`). It names a club by its CLUB short name, else its name: the game shows "Osijek" and "Maribor" for NK Osijek and NK Maribor, which are their short names (2 of 2 samples; R. Madrid has the same name and short name). Like team names, women's clubs get " Women" (whether the game adds it is not checked). Each club carries its first team's `team_index`, left out for a club with no first team (e.g. Svendborg fB, club 276, on Hjalte Boe on 2035-12-09). Hiding favoured clubs under partial scouting, with the personality, is a server choice: the game's behaviour for a partly scouted player is not checked.

### Personality — verified 26.3.2 (code + 8 in-game samples)

The personality label is computed, not stored. `int8 personality(ACTUAL_PERSON*, bool restrict)` at `0x36b906c` returns 1..40, and the formatter `0x3926560` maps it to a text id (0xa87f7…0xa881d; same English text for women). Inputs on the 1–20 scale: Determination (attribute 51) and Leadership (attribute 40) through `clamp((v+2)/5, 1, 20)`; Ambition, Loyalty, Pressure, Professionalism, Sportsmanship and Temperament from the 8 bytes above (Adaptability and Controversy are not used); age in whole years at the game date (`0x36a4100`, clamped to 14..100; computing it from the birth date is an inference, and it matters only at the 23 threshold). `neg` = `restrict == 0` or trailer flags bit 3.

The first matching rule wins (compares from the asm, `0x36b9114`…`0x36b94c4`):

| # | Condition | Label |
|---|-----------|-------|
| 1 | D ≥ 14, Amb ≥ 12, Pro ≥ 15, Tem ≥ 15, Pre ≥ 14, Loy ≥ 15, Spo ≥ 15 | Model Citizen |
| 2 | D ≥ 14, Amb ≥ 14, Pro ≥ 14, Tem ≤ 9 | Perfectionist |
| 3 | neg, Tem ≤ 4, Pro ≤ 10 | Temperamental |
| 4 | age ≥ 23, player, Lead ≥ 19 | Born Leader if Lead = 20 and D = 20; else Charismatic Leader if Tem ≥ 18 and Spo ≥ 18; else Leader |
| 5 | Pro ≥ 18, Tem ≥ 10 | Model Professional if age ≥ 23 and Pro = 20; else Professional |
| 6 | D ≥ 18 | Driven if Amb ≥ 12; else Determined |
| 7 | neg, Pro ≤ 4, D ≤ 9 | Slack if Pro = 1; else Casual |
| 8 | neg, Loy ≤ 6, Amb ≥ 16 | Mercenary if Loy ≤ 3; else Fickle |
| 9 | Amb ≥ 16, Loy ≤ 9 | Very Ambitious if Amb = 20; else Ambitious |
| 10 | neg, Amb ≤ 5, Loy ≥ 11 | Unambitious |
| 11 | Spo ≥ 18, D ≤ 9 | Honest if Spo = 20; else Sporting |
| 12 | neg, Spo ≤ 4, D ≥ 11 | Unsporting if Spo = 1; else Realist |
| 13 | neg, D ≤ 5, Amb ≤ 9 | Easily Discouraged if D = 1; else Low Determination |
| 14 | Loy ≥ 18, Amb ≤ 7 | Loy = 20: Devoted if the club affinity (below) is ≥ 100, else Very Loyal; Loy < 20: Loyal |
| 15 | Pre ≥ 17, D ≥ 15 | Iron Willed if Pre = 20; else Resilient |
| 16 | neg, Pre ≤ 3, D ≤ 9 | Spineless if Pre = 1; else Low Self Belief |
| 17 | Pre ≥ 15, Tem ≥ 10 | Light-Hearted if Spo ≥ 15; else Spirited if Pro ≥ 11; else Jovial |
| 18 | D ≥ 15 | Fairly Determined if Pro ≤ 14; else Resolute |
| 19 | Pro ≥ 15 | Fairly Professional |
| 20 | Amb ≥ 15 | Fairly Ambitious |
| 21 | Loy ≥ 15 | Fairly Loyal |
| 22 | Spo ≥ 15 | Fairly Sporting |
| 23 | otherwise | Balanced |

Values → text: 1 Professional, 2 Determined, 3 Temperamental, 4 Balanced, 5 Model Professional, 6 Driven, 7 Slack, 8 Casual, 9 Very Ambitious, 10 Ambitious, 11 Unambitious, 12 Honest, 13 Sporting, 14 Unsporting, 15 Realist, 16 Easily Discouraged, 17 Low Determination, 18 Very Loyal, 19 Loyal, 20 Mercenary, 21 Fickle, 22 Iron Willed, 23 Resilient, 24 Spineless, 25 Low Self Belief, 26 Born Leader, 27 Charismatic Leader, 28 Leader, 29 Devoted, 30 Light-Hearted, 31 Jovial, 32 Spirited, 33 Resolute, 34 Fairly Determined, 35 Fairly Professional, 36 Fairly Ambitious, 37 Fairly Loyal, 38 Fairly Sporting, 39 Perfectionist, 40 Model Citizen.

`restrict`: of 30 call sites most pass 1 (e.g. `0x1457dd8`, `0x12761d8`, `0x178ccb4`), six pass 0 (`0xc855a0`, `0x15ae0c0`/`0d8`, `0x15af590`, `0x15afc7c`, `0x15b29e0`, `0x1c6b444`). Which one the profile screen uses is not traced (the UI gets the value through the C# side). `fm` uses `restrict = 1`, so negative labels (rules 3, 7, 8, 10, 12, 13, 16) apply only to players with flags bit 3 (inference: the majority of call sites). **Verified in game (2036-01-01):** Enzo Fernández (uid 14 229 525, bit 3 clear) shows Ambitious, the `restrict = 1` result; `restrict = 0` gives Mercenary. 433 players without bit 3 change label between the two (2035-12-02). Also in game (2036-01-01, bit 3 set): Jesús Lobede Realist and Théo Emmanuelli Unambitious, both negative labels. Predictions not checked in game (2035-12-09): Asier Castillo Realist, Mounir Haddad Mercenary.

Devoted (verified, code `0x36b93b8…0x36b93ec`): within rule 14 with Loy = 20, the game takes the person's club (vtable `+0x280` = `0x369d9bc`: the current team's club, else the contract team's club) and the affinity lookup `0x2f92e50`: the value of the first relationship item (above) whose object is that club and whose type is 3, else 0. Devoted when it is > 99, else Very Loyal. `fm` applies this with the main contract as the contract team (a server choice: which contract the game uses is not traced). Rule applied to the bytes (not checked in game): with `restrict = 1`, each real save (2035-12-02, 12-09, 12-14) has 3 players in rule 14 with Loy = 20: Yaser Yousefi (uid 2 002 168 088, Persepolis 100, reason 1) and Shane Goodwin (uid 2 002 181 080, Columbus Crew 100, reason 1) are Devoted; Artem Bondarenko (uid 71 101 325, a free agent with no favoured club) stays Very Loyal. The 26.2.0 test save gives Necip Uysal (Beşiktaş) Devoted and Rory McKenzie (Kilmarnock) Very Loyal.

Ground truth (in game, 26.3.2):

| Player | Save | Amb Loy Pre Pro Spo Tem; D; Lead; age | Rule gives | Game shows |
|--------|------|----------------------------------------|------------|------------|
| Chris Ashby (uid 2 002 078 397) | 2035-12-02 | 11 16 12 15 17 17; 14; 14; 25 | Fairly Professional | Fairly Professional |
| Bruno Sami (uid 2 002 114 753) | 2035-12-02 | 19 13 8 13 8 10; 20; 8; 24 | Driven | Driven |
| Tino Livramento (uid 28 124 569) | 2035-12-02 | 14 14 15 17 15 16; 15; 11; 33 | Light-Hearted | Light-Hearted |
| Carlo Bernardi (uid 2 002 100 552) | 2035-12-09 | 16 14 16 15 18 15; 15; 7; 25 | Light-Hearted | Light-Hearted |
| Louis Leroux (uid 2 000 191 040) | 2035-12-09 | 11 13 11 13 12 11; 15; 9; 29 | Fairly Determined | Fairly Determined |

### Player traits — verified 26.3.2 (code + bytes)

The last 8 bytes of the ACTUAL_PERSON part, just before the BASIC part, are a u64 trait mask (the writer `0x2f424a0` writes `obj+0x90`, 8 bytes, then calls the BASIC writer). The formatter `0x370e010` maps a bit to a text id: bits 0–47 → 691 311 + bit; the rest from its switch. Bits 48 and 53 have no label (48 is set on 271 players in the real KŽ save, 53 on none).

| Bits | Traits |
|------|--------|
| 0–7 | Runs With Ball Down Left, Runs With Ball Down Right, Runs With Ball Through Centre, Gets Into Opposition Area, Moves Into Channels, Gets Forward Whenever Possible, Plays Short Simple Passes, Tries Killer Balls Often |
| 8–15 | Shoots From Distance, Shoots With Power, Places Shots, Curls Ball, Likes To Round Keeper, Likes To Try To Beat Offside Trap, Uses Outside Of Foot, Marks Opponent Tightly |
| 16–23 | Winds Up Opponents, Argues With Officials, Plays With Back To Goal, Comes Deep To Get Ball, Plays One-Twos, Likes To Lob Keeper, Dictates Tempo, Attempts Overhead Kicks |
| 24–31 | Looks For Pass Rather Than Attempting To Score, Plays No Through Balls, Stops Play, Knocks Ball Past Opponent, Moves Ball To Right Foot Before Dribble Attempt, Moves Ball To Left Foot Before Dribble Attempt, Dwells On Ball, Arrives Late In Opponents' Area |
| 32–39 | Tries To Play Way Out Of Trouble, Stays Back At All Times, Avoids Using Weaker Foot, Tries Tricks, Tries Long Range Free Kicks, Dives Into Tackles, Does Not Dive Into Tackles, Cuts Inside From Both Wings |
| 40–47 | Hugs Line, Gets Crowd Going, Tries First Time Shots, Tries Long Range Passes, Likes Ball Played Into Feet, Hits Free Kicks With Power, Likes To Beat Opponent Repeatedly, Likes To Switch Ball To Wide Areas |
| 48–55 | —, Play Football For as Long as Possible (691 372), Possesses Long Flat Throw (691 359), Runs With Ball Often, Runs With Ball Rarely, —, Does not Move Into Channels (691 364), Uses Long Throw To Start Counter Attacks (691 362) |
| 56–63 | Refrains From Taking Long Shots (691 363), Cuts Inside From Left Wing, Cuts Inside From Right Wing, Crosses Early, Brings Ball Out of Defence, Demands to Take Free Kicks, Demands to Take Penalties, Plays Ball With Feet (691 371) |

Samples: Ashby `0x8000` = Marks Opponent Tightly (seen in game), Sami `0x0080000000000000` = Uses Long Throw To Start Counter Attacks, Livramento `0x0100000000000002` = Runs With Ball Down Right and Refrains From Taking Long Shots (2035-12-02); Bernardi `0x400008000` = Marks Opponent Tightly and Avoids Using Weaker Foot, Leroux `0x800001500080` (2035-12-09). The real KŽ save has 92 492 players with no trait and at most 14 on one player; 139 573 masks are equal in the 12-02 and 12-09 saves and 246 changed.

### Media handling — verified 26.3.2 (code + 11 in-game samples)

Computed, not stored. The C# property `MediaHandling` (4cc `MedH`) maps to the engine property `'Pghm'` (bridge type table `0x5163520`). The ACTUAL_PERSON provider (`0x2f6ca7c`, case at `0x2f6d964`) passes a fog-of-war gate (`0x16f393c`, knowledge type `0x5b`; a player who fails it gets 0) and returns `person vt +0x238(1)` = `0x2f958d0`. Its inputs are the 8 hidden attributes (person `+0x40…+0x47`, see "BASIC person part") and trailer flags bit 3:

```
m = Tem < 3 ? 0x100 : (Tem < 7 ? 0x4 : 0)                         Short-tempered / Volatile
if Pro ≥ 15 and Tem ≤ 15 and Pre ≥ 15 and (Spo > 12 or Con ≤ 14):   m |= 0x2     Evasive
elif Con ≥ 15:                                                       m |= 0x1     Outspoken
elif (Pro ≤ 12 and Spo < 12) or Loy ≤ 10:                            m |= 0x20    Media-friendly
if Spo ≤ 7 and Tem ≤ 7:                                              m |= 0x200   Confrontational
elif Tem ≥ 15 and Pre ≥ 15:                                          m |= 0x10    Unflappable
elif Con ≤ 5 and m & 0x104 == 0 and Pro > 14:                        m |= 0x80    Reserved
if m == 0: m = 0x40                                                               Level-headed
if restrict and flags bit 3 clear and m & 0x304: m = (m & ~0x306) + 1
```

The last step adds 1, so a mask that already has Outspoken turns into Evasive. `restrict` is the function's argument, as in the personality rule; the provider passes 1 (code: `vt +0x238(1)` above), and `fm` uses 1. No sample has flag bit 3 clear and a negative style, so that step is not checked in game (Enzo Fernández, bit 3 clear, shows Outspoken on 2036-01-01, but his rule gives Outspoken directly: Controversy 15).

Display (verified by code): `0x36976f4(mask, female)` walks bits 0..15 in order and joins the labels with ", "; `0x3697990` maps a bit to text (ids 691 489…691 498): `0x1` Outspoken, `0x2` Evasive, `0x4` Volatile, `0x8` Plays Mind Games (the rule never sets it), `0x10` Unflappable, `0x20` Media-friendly, `0x40` Level-headed, `0x80` Reserved, `0x100` Short-tempered, `0x200` Confrontational.

Ground truth (KŽ saves, 26.3.2, profile screens; hidden attributes in save order Ada Amb Loy Pre Pro Spo Tem Con):

| Player | Hidden, flag bit 3 | Rule | In game |
|---|---|---|---|
| Carlo Bernardi | 10 16 14 16 15 18 15 6, set | `0x12` | Evasive, Unflappable (12-09 and 12-14) |
| Louis Leroux | 11 11 13 11 13 12 11 5, clear | `0x40` | Level-headed (12-09 and 12-14) |
| Anton Matković | 14 17 12 13 12 9 11 6, clear | `0x20` | Media-friendly |
| Ilan Assongo | 10 10 10 10 14 10 10 10, clear | `0x20` | Media-friendly |
| Jaka Čuber Potočnik | 10 15 8 6 14 12 15 14, clear | `0x20` | Media-friendly |
| Jesús Lobede | 5 15 13 17 13 2 3 14, set | `0x204` | Volatile, Confrontational (12-14 and 2036-01-01) |

Also in game on 2036-01-01: Enzo Fernández Outspoken and Théo Emmanuelli Level-headed, both as the rule gives (bytes).

The gate's exact condition is not decoded. `fm` hides media handling wherever it hides the personality (the server's choice).

## Open questions

- Personality: the meaning of trailer flags bit 3, and an in-game Devoted sample. Relationships: reasons other than 1 and 2, item bytes 13–15, relation types other than 3, the second vector, which list builder the profile screen uses, Enzo Fernández's fifth favoured person, and the non-player person records that name managers and staff. Traits: bits 48 and 53. Media handling: the fog-of-war gate's condition (knowledge type `0x5b`), and whether flag bit 3 clear turns negative styles into Outspoken in game.
- Reputation level: the words for levels 1, 4, 5 and 6 (order inference); women's players (the women's container's factor); player-coaches (the staff rule's inputs); players without a team (vt `+0x210`, assumed to be the nationality).
