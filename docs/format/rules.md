# FM27 format: Competition rules

Part of the format reference. Conventions, dates and the index of all files: [`docs/format.md`](../format.md).

## 27.0.0 tagged trees — verified 27.0.0 (bytes)

The game's competition and rule files (bundled as `shared/data/sicomps/comps.dbc`, zstd-compressed) use a self-describing format. `game_db.dat` embeds the parsed files in the NATIONAL_TEAM_CONTAINER and COMP records. `savefile.readTree` decodes it.

```
value  = u8 type, payload
struct = u32 n, n × (4-byte tag stored reversed, u8 0x01, value)     type 0x0a
list   = u32 n, n × (u8 0x01, value)                                  type 0x0b
string = u32 length, UTF-8 bytes                                      type 0x1a
```

Tags read as the game names them once reversed: the bytes `rsqs` are the tag `sqsr`. Fixed-size types seen in the save:

| Type | Bytes | Seen as |
|---|---|---|
| 0x00 | 0 | an empty value, e.g. `dyow` in a date |
| 0x01 | 4 | a u32, e.g. a competition uid in `comp` |
| 0x02 | 4 | a tag, e.g. a struct's class in `id  ` (`comp`) |
| 0x03 | 1 | a flag |
| 0x0f | 8 | a (uid, uid) pair |
| 0x11 | 1 | a u8 |
| 0x12 | 2 | a u16 |
| 0x18 | 8 | not decoded (`lgfx`) |
| 0x19 | 4 | probably an f32 (`Clat`, `Clon`) |
| 0x20 | 4 | an edit date (`EdDt`) |

A reference is a scalar uid, a (uid, uid) pair, or a struct whose `id  ` names the field holding it: `{id "comp", comp (2300046, 2300046), DBID 2300046}`. `savefile` rejects other types as corrupt.

Evidence: every tree in `data/test_save_27.fm` and `test_save_27b.fm` that the readers below locate decodes to its end.

## 27.0.0 competition rules — verified 27.0.0 (bytes); meanings partly inferred

**Nation rules.** A men's NATIONAL_TEAM_CONTAINER record (see "Continents, nations and national-team containers") holds its nation's rules tree, a struct without its type byte, at record offset 180, right after the f32 1.0 at 176. A nation without rules has a struct with no fields there. In both FM27 test saves, 59 of the 251 men's containers have one. The tree merges the nation's files; its `fles` field names them, e.g. `lit_comps (Branch 4, Vers 33) / lit_reserve_comps (Branch 4, Vers 22) / lit_rules (Branch 4, Vers 49)`, and `file` is the rules file (`lit_rules`). A list of the three source trees follows; `savefile` reads only the merged one.

The rules tree's `fxrl` list holds the rule groups. Each has a `name` and an `sqsr` list of squad rules: `type` (the rule type), `valu`, `cmty`, and on some rules further fields such as `styr` and `enyr` (start and end year) or `GmTy`, `info`, `tfrd` (not decoded). Fee rules have `cash` and `curr` instead of `valu`. Lithuania (`lit_rules`):

| Group | Rules (type, name, value, `cmty`) |
|---|---|
| TOPLYGA | 9 max_squad_size 30 (255); 94 same_nation_players_in_11 3 (1); 141 under_22_players_in_squad 5 (1) |
| Promotion Playoff | 9 max_squad_size 30 (255); 165 min_squad_size 15 (1); 190 under_21_national_players_in_squad 5 (1); 94 same_nation_players_in_11 3 (1) |
| Lithuanian Cup | 94 same_nation_players_in_11 3 (1) |
| Super Cup | 94 same_nation_players_in_11 3 (1) |

Rule type names come from the game's table at `0x5677250` in the 27.0.0 arm64 slice (448 named types with ids 0..522; e.g. 9 `max_squad_size`, 18 `non_eec_players_in_squad`, 146 `homegrown_players_from_nation`). Whether a value is a minimum or a maximum depends on the type and is not stored in the rule: max_squad_size is a maximum by its name, the rest is inference.

**Competitions.** A 27.0.0 COMP record holds its NATION index as a u32 at record offset 57, and a list of format trees (one per format variant, e.g. `lit_a_lyga`, `lit_a_lyga_9_teams`, `lit_a_lyga_9_teams_compat`) as a list without its type byte at offset 122, 124, 178 or 180. `savefile` searches offsets 61..255 for a list that decodes and whose every item's `comp` is the record's uid. Each format's top-level `fxri` indexes the nation's `fxrl`; 255 means none. Stages and rounds have their own `fxri`, not read.

| Competition (format file) | `fxri` | Rule group |
|---|---|---|
| A Lyga (`lit_a_lyga`, three variants) | 0 | TOPLYGA |
| Premier League (`eng_prem`) | 6 | ENG_FIXTURE_RULES_PREM: 146 homegrown_players_from_nation 8; max_squad_size 25 |
| Serie A (`ita_a`) | 1 | SERIE A: 11 homegrown_players_15_21_ignore_loans_from_club_in_squad 4; 39 homegrown_players_15_21_from_nation 8; max_squad_size 25 |
| Bundesliga (`ger_first`) | 3 | FIRST: 39 … 8; 38 homegrown_players_15_21_from_club 4; 212 same_nation_players_in_squad 12 |
| Ligue 1 (`fra_first`, three variants) | 0 | FRE_FIXTURE_RULES_DIVISION_ONE: 18 non_eec_players_in_squad 4 |

These match the real leagues' squad rules (general knowledge, not checked in game). The nation at offset 57 agreed with the format files' prefix (`lit_`, `eng_`, …) for 710 of the 711 format lists whose prefix names a nation with rules. The exception is the Liechtenstein Cup (`swi_lie_cup`): its nation is Liechtenstein (no rules) and its files are Switzerland's, so it gets no rule group.

Women's competitions carry their men's nation (offset 57) but their formats' files start `wmn_` (`wmn_eng_wsl`) and their `fxri` indexes the women's rules (`wmn_eng_rules`), which are in the women's containers and are not read. `savefile` gives a competition a rule group only when every format names the same `fxri`, it is not 255 and is in range, and every format's file starts with the nation's rules prefix (`lit_` for `lit_rules`). That holds for 519 competitions in both FM27 test saves. Continental and international competitions (`inter_rules`, `afr` …) keep their rules in other records with a different layout, not read.

`cmty` is 255 or 1 (733 and 332 rules). That 255 means all of the club's competitions and 1 only this one is an inference: Lithuania's max_squad_size has 255, its XI and under-22 rules 1.

## Open questions

- Whether each rule's value is a minimum or a maximum, and what `cmty` means; FM's competition rules screen for A Lyga would settle Lithuania's.
- The rule fields not decoded (`GmTy`, `info`, `tfrd` …), the groups' other fields (`tcpf`, `usqn`, `snrp` …) and the trees' other lists (`trrl` transfer rules and fees, `lnrl` loan rules, `wkpm` work permits …).
- Stage and round rule groups; competitions whose format variants name different groups (4 lists).
- The women's and continental rules' layouts.
- The COMP record before the format list: why the list starts at four different offsets.
