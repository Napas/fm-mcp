# FM26 format: tactics_man.dat

Part of the format reference. Conventions, dates and the index of all files: [`docs/format.md`](../format.md).

## tactics_man.dat — current tactic, verified 26.3.2 and 26.2.0 (code + bytes)

Version 26 in all three saves. Writer `0x1eaffec`, loader `0x1eaf35c` (both reference `"tactics_man"`). `savefile.ReadTactics` walks the whole file with the grammar below: every human, every team entry, the last-lineup table, the presets and the empty fourth list, to the last byte. It serves the first human's first team entry as the current tactic; the walk validates the other entries and reaches the last-lineup table across all humans, but does not expose them. With more than one human the tactic is unsupported (`Tactics.TacticErr`, ErrUnsupported) and only the last lineups are served.

```
file           = u16 n + n × HUMAN_TACTICS                 (writer 0x16ef560, loader 0x16ef904)
                 u16 n + n × LAST_LINEUP (242 bytes)        (writer 0x39d66c0, loader 0x39d6a54)
                 u16 n + n × TACTIC                         presets: 19 in every save
                 u16 n + n × (u8, HUMAN_TACTICS)            n = 0 in every save
HUMAN_TACTICS  = u8 0x0d, u32 human person index (= humans.dat), u32 n
                 n × (u32 TEAM index, TEAM_ENTRY)           sorted by team index (std::map)
                 u16 n + n × TACTIC                         n = 0 in every save
                 u8 1, u32 n + n × TACTIC_HEADER            (0x4777fcc)
                 u8 1, u32 n + n × (u16 0x42, string, POSITION, POSITION)          named positions (0x4777578)
                 u32 n + n × (u32 key, u32 m + m × SELECTION)                       (0x445f4c4)
                 u8 1, u32 n + n × PLAN                     (0x483c1ac)
                 u32 n + n × (u32 team, u32)                map at +0x48
                 u32 n + n × (u32 team, u8)                 map at +0x60
TEAM_ENTRY     = u8 9                                       (0x47a1cc4)
                 SELECTION                                  -1 = empty
                 SET_PIECES                                 (0x4781670)
                 i32 active tactic (hypothesis)             int8 at +0x1018, sign-extended; 0 (Metz), -1 (test save)
                 u16 n + n × TACTIC
                 u16 3, u32 26, 26 × REC22                  (0x473fa94)
                 u16 n + n × (u32 key, REC22)               n = 0 in every save
                 i32                                        a u64 at +0x11b0 / 60 000; 0 in every save
                 SET_PIECE_ROUTINES                         (0x4748a6c)
SELECTION      = u8 3, string (empty), 26 × i32 person     (0x445e954)
REC22          = 4 × i32 (signed 3-bit fields), 6 × u8 flags  (0x473f170); 0,0,0,0, 0,0,0,0,0,1 in every save
TACTIC_HEADER  = u16 0x42, u32 key, u8 4, u8, u8, bitset, string, u32
PLAN           = u8 0x10, string, u32 m + m × (u8 2, u32 condition, u8 1, i32, string, 3 × u8, u16 0x42, bitset, 2 × u8),
                 u8 1, i32, string, 6 × u8                  (0x483c2cc)
SET_PIECE_ROUTINES = u16 0x3c, 4 × u8,
                 ROUTINE × 3 (0x475ad04, 0x475d484, 0x4764410), RVEC × 3,
                 u8 + ROUTINE (0x475f8a8), u8 + ROUTINE (0x4761e5c), RVEC1 × 2,
                 u8 + ROUTINE (0x476931c), ROUTINE (0x4766ef8), u8 + ROUTINE (0x476e0c0), RVEC1, RVEC, RVEC1,
                 2 × u8 + ROUTINE (0x4770674), 2 × u8 + ROUTINE (0x476b8d0), RVEC2 × 2
RVEC, RVEC1, RVEC2 = u32 n + n × (0, 1 or 2 bytes, ROUTINE)
ROUTINE        = 8 × (u8 category, u8 n, n × u8), u8 n + n × u8, u8 n + n × (u8, u8, u8),
                 string (empty), u8 (1), u32 "NULL", u8
SET_PIECES     = u16 0x42, 2 × list, u8, i32 person, 3 × u8, 10 × list, u8, 8 × list, u8 n + n × (u8, u8)
list           = u32 n + n × (u8 2, i32 person index)
TACTIC         = u8 0x22, u16 0x42, 4 bytes build stamp (1a 03 00 01 in 26.3.2, 1a 02 00 01 in 26.2.0),
                 u8 (2 human, 1 preset), string name, 3 × string (empty)
                 u8 4, u8 2, u8 mentality, bitset, string style ("Custom"), u32 "NULL"   (0x47ac118)
                 11 × (POSITION in possession, POSITION out of possession, u8 slot number 0..10)
                 u8 flag; when set: 2 × (u32 n + n × (u32, u32 m + m × POSITION)),
                   u32 n + n × (u32, u32 m + m × (u8, u8, u32)), 191-byte block (0x473d81c; all zero)
POSITION       = u16 0x42, u8, u32 position mask, u8 0xff, u8 0                                   (0x4923d74)
                 u8, u8, u32 n + n × (u64 role option, INSTR), u32 m + m × (u32, u64)              (0x4923e94)
                 u64 selected role | duty
INSTR          = bitset, u32;  bitset = u8, u8, u8 n, n × u32, u8
LAST_LINEUP    = u32 8, u8 0, i32 team, u8 0, i32 opponent, FORMATION in possession, FORMATION out of possession,
                 26 × i32 person (XI by slot, then bench), 26 × u8 shirt number, 26 bytes not decoded
FORMATION      = u8 3, u8 mentality, u8 2, 11 × (u8 5, u16 slot code)                            (0x490f744)
```

The walk checks every constant above (tags, versions 0x42/0x3c/3, the count 26, the `u8 1` list markers), the last-lineup records' version, zero bytes and ascending teams, an empty fourth list and the end of the file. When it fails, the parser falls back to a scan and `fm` adds a load warning naming the part where the walk stopped ("tactics layout not fully recognised; last lineups found by searching the file (stopped at a team's set pieces or records)"); `Tactics.WalkErr` keeps the full error with its offset. The scan takes the last-lineup table to be the first place after the first team entry's tactics (or from the start, when that entry was not read) where `u16 n` is followed by n records that each start `u32 8, u8 0, i32 team, u8 0` with teams strictly ascending, then u16-counted presets that parse as TACTICs, then `u16 0` at the end of the file. Other team entries' `i32 0, u16 1, 22 42 00` look like an empty table, so the full check is needed. The scan assumes at least one preset and an empty fourth list (both true in every save). The first team entry is kept if it was read up to its tactics; the other entries are dropped.

**Rest of the team entry and human** — verified 26.3.2 (code: the writers listed above, read with objdump) and 26.2.0 (bytes only). The walk reaches the last-lineup table where the scan finds it in all three saves (test: body offset 8 704, 2 413 records; KŽ 12-02 and 12-09: 31 267, 15 016 records; 12-14 agrees), and every constant check passes in all 7 team entries (`TestTacticsWalkMatchesScan`). Team entries: KŽ saves 652 (Metz first team, active 0), 19 023 (FC Metz 2, 0), 38 394 (0), 86 128 (-1, no tactic); entries 19 023 and 38 394 hold a byte-identical copy of the first team's tactic and an empty selection. Test save 547 (Tottenham), 27 845, 27 846, all active -1 with no tactic. `savefile` serves only the first entry; the others are read and validated but not exposed. The walk reads any number of humans; no save has more than one. The set-piece routine block is 912 bytes in every entry, with every vector holding one routine. Meanings (hypotheses, none checked in game):

- SET_PIECE_ROUTINES: set-piece routines (strong inference). Each of the first five routines places exactly 10 players across categories 0–3 and each of the last five 9 across categories 4–7 (all 7 entries), which fits defending (10 outfielders) and attacking (9 plus the taker) routines. C# names fit (`SetPieceRoutineCategoryReference`, `…CategoryPriorityReference` for the u8 list, `…CategoryInstructionReference` for the triples), and so do the engine keys `setpiecetype`, `setpiecescenario`, `setpiecepitchside`, `setpieceroutineindex`. The kinds of routine (corners, throw-ins, indirect free kicks), the prefix bytes (side or scenario?) and the last u8 of a routine in a vector (familiarity? Metz first team 48–57 on 12-02, 72–81 on 12-09; 0 elsewhere) are not mapped.
- REC22 × 26 and the keyed list: four signed 3-bit sliders and six flags each; opposition instructions would fit, but no sample holds a non-default value.
- Human tail: one TACTIC_HEADER in the KŽ saves (key 0, style "Custom", "NULL"), none in the test save; 8 PLANs with the same content in all three saves (hypothesis: default match plans; the third byte of each item is 1..7, which would fit a mentality); the first map holds (652, −1), (19 023, −1), (38 394, −1); the second lists teams 599, 652, 18 933, 18 959, 62 699, 62 700, 19 023, 38 394 with u8 0 (hypothesis: teams the human has managed). Both maps are empty in the test save.
- Writers skip null pointers in the TACTIC and ROUTINE vectors but count them; a save with a null slot would not parse (never seen).

Values (C# enums from the IL2CPP metadata, verified):

- **Position mask** (`PositionMask`): GK 0x1, SW 0x2, DR 0x4, DL 0x8, DC 0x10, WBR 0x20, WBL 0x40, DM 0x80, MR 0x100, ML 0x200, MC 0x400, AMR 0x800, AML 0x1000, AMC 0x2000, ST 0x4000, plus 0x100000 left / 0x200000 right for doubled positions. 0x8000 and 0x10000 (attacker right/left) are not seen. The 19 presets' in-possession shapes match their names ("4-4-2 2DM" = GK DR DC DC DL DM DM MR ML ST ST).
- **Role** (u64): one role bit plus one duty bit. Duties: 0x200000 Defend, 0x400000 Support, 0x800000 Attack, 0x2000000 Defend (stopper), 0x4000000 Defend (cover), 0x400000000 Float. In-possession roles use `InPossessionRole` (Goalkeeper 0x1, BallPlayingGoalkeeper 0x1000, OverlappingCentreBack 0x40000000000000, ChannelForward 0x80000000…), out-of-possession roles `OutOfPossessionRole` (OPGoalkeeper 0x1, OPCentreBack 0x8, OPFullBack 0x200…). Every selected role is one of its slot's options. Full tables in `internal/savefile/tacticnames.go`. FM26 renamed some roles; their labels (Libero → Advanced Centre-Back, Trequartista → Free Role, …) are matched from the English language file (inference; the game's abbreviation BCB fits Ball-Playing Centre-Back). The Tactics Planner shows no duties, so `fm` does not report them; the save still stores Attack, Support and Defend bits, so the engine may use them.
- **Selection**: index i is the player in tactic slot i (strong inference: all 11 fit their slot's position on 12-09, and all 11 of the 12-02 last-match XI); 11..25 are the bench.
- **Mentality**: header byte 6 in the human tactic, 4 in all presets. The C# `Mentality` enum (metadata, verified) is None 0, Very Defensive 1, Defensive 2, Cautious 3, Balanced 4, Positive 5, Attacking 6, Very Attacking 7. Verified in game: 6 shows as Attacking (12-09).

Evidence (real KŽ saves, 26.3.2): human 158 530 has 4 team entries (652 Metz first team, 19 023 FC Metz 2, 38 394, 86 128). Entry 652 holds one tactic, "ZaZ - Autumn 3.39 ST", identical in the 2035-12-02 and 12-09 saves: in possession GK, WBR, DC (right), DC (left), WBL, MC (right), DC, AMR, AML, MC (left), ST; out of possession GK, DR, DC (right), DC (left), DL, DM (right), DC, AMR, AML, DM (left), AMC. On 12-09 the selection is Sami, Veiga, Nduquidi, Medina, Løvik, Tacalfred, Vassiljev, Matković, Reddy, Edubatey, Ly and 9 substitutes, equal to Metz's last-lineup XI (opponent 1 545). On 12-02 the selection is empty (saved after a match); the last lineup holds the XI against team 670. Last-lineup records: 15 016 (12-09), 2 413 (test save). Test save (26.2.0): human 88 351, first entry 547 (Tottenham) with active -1, no tactic and an empty selection; Tottenham's last lineup is all -1.

Ground truth (in game, Tactics Planner, KŽ save 2035-12-09): name "ZaZ - Autumn 3.39 ST", mentality Attacking, style "Clean Slate" (stored "Custom"). In possession: Sami BGK; Medina OCB (left), Vassiljev BCB, Nduquidi OCB (right); Løvik and Veiga AWB; Edubatey (left) and Tacalfred (right) CHM; Reddy and Matković IF; Ly CHF. Out of possession: GK; FB, CB ×3, FB; DM ×2; W, AM, W. Bench S1–S9 Haddad, Vilas, Castillo, Čuber Potočnik, Ashby, Tchaouna, Rodríguez, Assongo, Karaboué. All 22 positions, sides, roles and players and the bench order match the decoded values; CHM = Channel Midfielder, CHF = Channel Forward and BCB = Ball-Playing Centre-Back are read from the abbreviations (inference). No duty is shown.

`fm` (`get_tactic`) uses the active tactic, or the first when the active index is out of range. Its players are the selection, or the team's last-match lineup (XI and bench) when the selection's XI is empty (`lineup_source`). Placing the last-match XI in the current tactic's slots assumes that match used this tactic (inference). When the first team entry is not the manager's first team, `fm` drops the tactic and adds a load warning naming the team. When the manager's team is unknown the check is skipped and the first entry is served. `lineup_source` is `none` when the selection and the last-match lineup are both empty (slots without players).

### Last lineup — verified 26.3.2 and 26.2.0 (code + bytes)

Every team with a stored tactic has one LAST_LINEUP record: its last match. Layout (writer `0x39d66c0`, loader `0x39d6a54`; offsets within the 242-byte record):

| Offset | Size | Field | Status |
|--------|------|-------|--------|
| 0 | u32 | record version 8 | verified |
| 4 | u8 0 + i32 | team (TEAM index) | verified |
| 9 | u8 0 + i32 | opponent in that match (TEAM index), -1 = none | verified (bytes, below) |
| 14 | 36 | in-possession FORMATION | verified |
| 50 | 36 | out-of-possession FORMATION | verified |
| 86 | 26 × i32 | person index: XI by tactic slot, then bench; -1 = empty | verified |
| 190 | 26 × u8 | shirt number per lineup place (as at 86); 255 = none | verified (bytes + in game, below) |
| 216 | u32 | bitfield; bits 29–31 follow the mentality | unknown |
| 220 | 4 × u32 | 50..250 | unknown |
| 236 | 6 × u8 | 0..20; byte 238 is a function of the mentality, byte 240 is 0 | unknown |

The record holds no match date and no competition: every field the writer writes is listed above. The loader reads 23 players for record version ≤ 4, has no out-of-possession formation below 7 (it copies the in-possession one with default roles) and no tail bytes at 190 below 8.

**FORMATION** (writer `0x490f744`): `u8 3`, the mentality (`Mentality` enum; the same byte in both formations of every record), `u8 2` (unknown; 2 in every record), then per tactic slot 0..10 `u8 5` and a u16 slot code:

```
bits 0..5    cell, signed
bits 6..12   role index, signed
bit 13       set on every out-of-possession slot, never on an in-possession one
bits 14..15  0
```

The cell (`0x48cea4c` maps a position mask to it) is a pitch grid, rows from goal to attack, columns right to left: −15 GK; −13 DR, −12 DC (right), −11 DC, −10 DC (left), −9 DL; −8 WBR, −7 DM (right), −6 DM, −5 DM (left), −4 WBL; −3 MR, −2 MC (right), −1 MC, 0 MC (left), 1 ML; 2 AMR, 3 AMC (right), 4 AMC, 5 AMC (left), 6 AML; 7 attacker right (0x8000), 8 ST (right), 9 ST, 10 ST (left), 11 attacker left (0x10000). SW maps to the GK cell, so a sweeper decodes as GK; an empty mask maps to −16 (not seen). The role index is the role without its duty: `0x48cecd0` numbers the in-possession roles, `0x48cf070` the out-of-possession ones (role bits 0–20, 27–33 and 35–45 in order from −31). Duties are not stored. Both tables are `lineupCells`, `lineupInPossessionRoles` and `lineupOutOfPossessionRoles` in `internal/savefile/tacticnames.go`.

`savefile` decodes both formations into `TacticSlot`s (position mask with side bit, role bit without duty). A record whose formation does not decode (wrong tag, separator or phase bit, a cell or role index outside the tables, or different mentality bytes) keeps its players and opponent, has no formation, and is counted in a load warning.

Evidence (bytes): every record of both KŽ saves (15 016 each) and the test save (2 413) decodes, with the goalkeeper in slot 0 and 11 different cells per formation. FC Metz's formations (identical in the 2035-12-02 and 12-09 saves; in-possession codes 6961, 56, 1076, 1078, 60, 1214, 7541, 7554, 7558, 1152, 7817) equal the human's tactic slot for slot without duty bits, mentality 6, so all 22 positions and roles match the Tactics Planner ground truth above. The mentality byte is the tactic's for Metz; that AI teams' values are the same enum is an inference (2035-12-09: 4 in 14 846 records, 3 in 75, 5 in 67, 2 in 19, 6 in 9).

**Shirt numbers** (+190), verified 26.3.2: FC Metz's bytes on 2035-12-09 are 88, 22, 5, 18, 32, 8, 43, 17, 16, 6, 1 for the XI (Sami, Veiga, Nduquidi, Medina, Løvik, Tacalfred, Vassiljev, Matković, Reddy, Edubatey, Ly) and 77, 70, 25, 15, 4, 3, 13, 28, 27 for the bench (Haddad, Vilas, Castillo, Čuber Potočnik, Ashby, Tchaouna, Rodríguez, Assongo, Karaboué): all 20 equal the squad numbers on the game's squad screen (2035-12-14; the game's squad-number screen says registered numbers cannot change until next season). On 12-02 the XI's bytes (Bernardi 19, Ashby 4, Leroux 41, …) match too. Records without a match hold 255 throughout. Of the places holding a person, 280 hold 255 and none hold 0 or more than 99 (both KŽ saves). Places without a person can still hold a number (Reims: 22..27), so `fm` reads a number only for a person. Most clubs keep a player's number between matches; teams numbered 1..11 and 12, 14, 15… on the bench renumber most matches (646 of 770), which fits per-match numbering in some competitions (inference).

The opponent field: every opponent ≥ 0 has its own record (12 093 of 12 093 on 12-09); 2 923 records have opponent -1 and an empty lineup (teams with a tactic but no match). All 18 Ligue 1 clubs form 9 symmetric pairs on 12-02, and 16 of 18 on 12-09 (Nice → Angers and Troyes → Reims point at teams that have played again). Of the 1 765 records that changed between the two saves, 1 637 point at an opponent that points back. Metz played 670 Olympique de Marseille (12-02) and 1 545 Stade de Reims (12-09); both point back to 652. Not seen in game.

`fm` (`get_last_lineup`) serves a team's record when its opponent is set: opponent, mentality, per slot the player and both positions and roles, the bench, and a shape per phase: players per row from defence to attack without the goalkeeper, rows D (DL, DC, DR), WB/DM (WBL, WBR, DM), M (ML, MC, MR), AM (AML, AMC, AMR) and ST, empty rows left out. A sweeper would read as a second GK and be left out of the shape (none seen: every formation has 11 different cells, so no second GK cell). Metz: 3-2-2-2-1 in possession, 5-2-3 out of it; Reims (12-09) 4-2-3-1 and 4-2-2-1-1. FM's own shape names ("4-2-3-1 DM AM Wide") are not derived. Each person carries his shirt number; a person the save does not decode as a player is served as `unnamed_player`.

Unnamed persons (2035-12-09, served lineups): 58 % of the XI places that hold a person (74 796 of 128 380) hold one the save does not decode as a player, mostly tag-7 records (VIRTUAL_PLAYER, hypothesis; 72 165; tag 19 1 671, tag 16 675, tag 1 285). 4 253 lineups have no player in the XI. They are mostly women's teams and clubs outside Europe (Portland Thorns Women 2 of 11 named, Raja Club Athletic 1, Club Sport Emelec 0); big European men's clubs are complete.

## Open questions

- `tactics_man.dat`: the style string ("Custom" shows as "Clean Slate"); whether duties appear anywhere in game or affect the match engine; the active-tactic index and where other tactic slots live; team instructions and per-role player instructions (bitsets); the set-piece lists; the meanings in the rest of a team entry and human section (set-piece routines, REC22, plans, the two team maps); saves with more than one human; the last-lineup record's 26 bytes after the shirt numbers (bitfield and unknown values) and the formation byte that is always 2; where the date of a team's last match is (not in the record; probably a fixture in `rgman/comp_*`); the renamed role labels.
