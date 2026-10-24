# FM26 format: Clubs, teams and competitions

Part of the format reference. Conventions, dates and the index of all files: [`docs/format.md`](../format.md).

## Clubs, teams and competitions — verified 26.2.0 and 26.3.2 (code + bytes)

A player's club field is a **TEAM** index: first teams, reserve and youth teams and national teams are separate TEAM records, each pointing at its container — a CLUB, or a national-team container. Men's and women's sides are separate CLUB records with the same names.

Tables are located deterministically: `u32 count, u8 0, first record tag`, right after the previous table's 9-byte end marker `00 00 00 00 01 + u32`, with the count from `memory_pools.dat`. Every record is parsed and must carry index 0..count−1 in its database record (`u32 index, u32 uid, u32 uid`), and an end marker must follow the last one.

| Table | Count (memory pools) | First tag | Test / real |
|-------|----------------------|-----------|-------------|
| CLUB | CLUB + LEAGUE_BODY_CLUB | 0 or 12 | 25 777 / 36 457 records |
| COMP | COMP | — (follows the CLUB table) | 8 093 / 8 201 |
| TEAM | TEAM + NATIONAL_TEAM | 11 or 10 | 59 370 / 86 136 |

**CLUB** (`0x308b1a0`; tag 12 = LEAGUE_BODY_CLUB `0x358514c` adds a ref): `u8 tag, 5 × ref (nation, …, city), u32, u8, u8, string long name, string short name, u8, u16, ref, u8, u8 n + n × ref, u8 finance type + finance, u8 n × 25 bytes, u8 n × 12 bytes, u16, ref`, then the TEAM_CONTAINER base (`0x39a2730`): `u8, u8, u8 women's flag, u8 n + n × own team, u8 n + n × affiliated team, 3 × (u8 n + n × ref), u8 n × 16 bytes, u16 n × 28 bytes, database record`. Names are stored in the save as shown in game (licensed-out clubs keep their fake names). Finance type 0 = CLUB_FINANCE_BASE (45 bytes, `i32 balance` first — bank balance in GBP, verified in game on a 26.3.2 save: £29 636 499 = €34 293 296 at the save's rate), 1 = CLUB_FINANCE (budgets, below; then fixed and counted blocks, then the base), 2 = none.

**TEAM** (`0x394e254`, tag 10) / **NATIONAL_TEAM** (`0x35dc620`, tag 11, a 41-byte prefix + 7 bytes, +1 byte from data version 0xe94, +4 bytes from 0xe93, + `u8 n × 10 bytes`): `u8, u8 last league position (guess), u16 reputation, ref division (COMP index), ref, ref, ref stadium, ref manager (person), ref continental competition, u8, ref, u16 n + n × person (squad list, below), ref captain (guess), ref vice-captain (guess), 5 × u8, ref, u8`, then TEAM_BASE (`0x398ec20`): `u8 team type, u8 national, u8 youth, u8 age limit (100/255 = none), u8, u8, ref container, string name, u8, string short name, u8, database record`. Team types (guesses): 0 first team, 1 reserves, 3 B team, 10 U21, 12 U18, 16 second team.

A first team carries its club's uid (all first teams of both saves). Men's national-team container index = nation index. Women's containers (251–501) follow them: nation index = container − 251 (guess, supported by reputations — 390 → England, and the strongest women's teams map to Spain, England, USA, Germany, Sweden, France). Only the reputation factor of the container record is read (see "Continents, nations and national-team containers").

**COMP** records are not decoded; they end with a bare database record followed by the next record's tag 0, which is how the table is split (`competitionTrailer`). Competition names come from the language database by uid. Fixtures refer to competitions through competition keys, and league tables live in `rgman/comp_<uid>.dat`: see [`competitions.md`](competitions.md).

Examples (test save): team 547 → club 416 "Tottenham Hotspur" (uid 728, division Premier League); team 20739 → club "Chelsea", women's flag set; team 1233 → Argentina's national team (container 187).

### Club budgets (CLUB_FINANCE) — verified 26.3.2 (code + in game)

CLUB_FINANCE (finance type 1; writer `0x3118c7c`) holds the budgets. Offsets are from the first byte after the finance type byte. The writer emits each field as `add x1, x20, #off; mov w2, #size; blr [stream+0x30]`, which gives the object offset.

| Offset | Size | Object field | Field | Status |
|--------|------|--------------|-------|--------|
| 0 | i32 | `+0x7d0` | starting transfer budget for the season, GBP | inference from code (`0x1318638`, `0x154dc3c`, `0x1318be8`, below); candidate for FM's "Original Budget", not confirmed in game; not exposed |
| 4 | i32 | `+0x7cc` | current transfer budget, GBP; can be negative | verified (code + in game, 12-14) |
| 8 | i32 | `+0x7d4` | next season's transfer budget, GBP | verified (code); not exposed |
| 12 | 54 bytes | | u8, 9 × i32 (`+0x814…+0x834`), i32 `+0x83c`, u8, i32 `+0x838`, i32 `+0x7f8`, i32 `+0x7fc` | not decoded |
| 66 | i32 | `+0x810` | wage budget, GBP per week: the total, not what is left | verified (code + in game, 12-14) |
| 70 | … | | sub-items, counted blocks, then CLUB_FINANCE_BASE (bank balance) | not decoded |

Code evidence (26.3.2):

- The editor property setter `0x311c088` logs changes under string keys: `'Ctfc'` → `"transfer_budget"` → `+0x7cc`; `'Cntb'` → `"next_seasons_transfer_budget"` → `+0x7d4`; `'Cwwb'` → `"wage_budget"` → `+0x810`; `'Cnsb'` → `"next_seasons_wage_budget"` → `+0x840` (written later by `0x3119704`; save offset not found).
- The inline transfer-budget getter (about 40 sites, e.g. `0x1318b10`, `0x154e390`, `0x311d990`) returns `+0x7cc`, but 0 when it is 1..499; negative budgets are returned as they are (`CLUB_HAS_NEGATIVE_TRANSFER_BUDGET` exists). `fm` applies the same rule.
- Fees and income change `+0x7cc` (`0x191b47c`, `0x20d38a8`, `0x314eb7c`, `0x3151e10`). Between 12-02 and 12-09, transfers move it in matching pairs: Arsenal −10 845 192, Manchester United +10 845 192 (bytes).
- Job and season set-up (`0x1318638`, `0x154dc3c`) set `+0x7cc` and `+0x7d0` to the same value; `0x1318be8` computes `max(0, +0x7d0 − +0x7cc)`. Next season's figure (`0x311c05c`, `0x311bf44`): 0 when `+0x7cc` < 0, else `max(0, +0x7d4 + +0x7e0)`.
- The wage budget getter `0x311c054` returns `+0x810` unchanged.

Values (bytes):

| Save | Club | Offset 0 | Current | Next season | Wage budget / week |
|------|------|----------|---------|-------------|--------------------|
| test (26.2.0, 2025-07-07) | Tottenham | 45 400 000 | 45 400 000 | 29 032 196 | 3 770 160 |
| KŽ 2035-12-02 | FC Metz | 5 634 364 | 8 312 454 | 2 353 289 | 1 069 817 |
| KŽ 2035-12-09 | FC Metz | 5 634 364 | 8 329 098 | 2 353 289 | 1 069 817 |
| KŽ 2035-12-14 | FC Metz | 5 634 364 | 8 329 074 | 2 353 289 | 1 069 817 |

Ground truth (in game, Recruitment Budgets screen, KŽ save 2035-12-14, FC Metz; € = £ × the f32 rate 1.1571305):

| Field | Save | In € | Game shows |
|-------|------|------|------------|
| Transfer budget, Current | £8 329 074 | €9 637 825 | €9.63M |
| Transfer budget, Original Budget | £5 634 364 (offset 0) | €6 519 694 | €6.5M: fits neither two-decimal format (truncated €6.51M, rounded €6.52M) |
| Transfer budget, Spent this season | not decoded | | €7.5M |
| Wage budget | £1 069 817 / week | €64 371 729 p/a | Available €16.43M p/a + Currently spending €47.93M p/a = €64.36M (fits truncation, see below) |
| Scouting budget | not decoded | | €2.41M available |

The wage field is the total: the game's "available" is the budget minus the current wage bill. That the bill includes staff wages is an inference (player main-contract wages alone are £672 499 a week, 63 % of the budget, on 12-02). The contract screen's "% of overall wage budget" divides by the current wage bill, not by the total: Assongo's €1M p/a shows as 2.1 % and Bernardi's €1.3M as 2.7 % (12-14); 1 / 47.93 = 2.09 % and 1.3 / 47.93 = 2.71 %, against 1.55 % and 2.02 % of €64.37M. The 12-02 save's £8 312 454 = €9 618 594 was reported as €9.61M in game (save not recorded). The amount spent this season is not in these fields: offset 0 minus current is negative for Metz.

Original Budget, a second candidate (verified, bytes; meaning unknown): the i32 at byte 17 of the fixed `4 + 1 + 20 + 8 + 16 + 2 + 20` block after CLUB_FINANCE's 49-byte records holds £5 617 344 = €6 500 000 for Metz on 12-02, 12-09 and 12-14, exactly the €6.5M shown (with two-decimal truncation it would read "€6.50M", so the match assumes the screen drops a trailing zero). It is a multiple of €50K for 61 of the 84 clubs where it is positive and round in pounds for none (Arsenal €30.5M, Manchester City €36M, Tottenham €25.5M), so it may be a fee set in euros rather than a budget (inference), and the €6.5M match may be a coincidence. What the screen's Original Budget shows is open; `fm` serves neither field.

`fm` gives the human's club's budgets in `game_overview` (`budgets`): `transfer_budget` in the user's currency, `wage_budget` in the user's currency and wage period. Other clubs' budgets are not served.

### Squad lists — verified 26.2.0 and 26.3.2 (bytes)

The TEAM record's `u16 n + n × person` list holds person indices.

- **National teams**: the called-up squad. In the real KŽ save (26.3.2, 2035-12-02), France (team 589) lists 23 players, the same 23 as the game's squad screen (Leonardi, Kambwala, Yoro, Bitshiabu, Gusto, Bouaddi, Camavinga, Methalie, Charron, Bahoya, Tel, Chevalier, Saliba, Lukeba, Gadou, Andrés Álvarez, Kanda-Tshongo, Pfister, Doué, K. Thuram, Flávio Gonçalves, Ekitiké, Cherki). England (585) lists 21 English players (Trafford, Saka, Wharton, Quansah, …; plausible, not checked in game), England U21 / U20 / U19 23 each, France Women and England Women 23 each. Test save: England (579) 23 with Kane, Rice, Saka; Argentina (1233) 23 with Messi and Tottenham's Romero. No player is in two national lists, and each listed player's offset-19 field names that team (12 731 players in the real save, 5 547 in the test save; no exceptions either way). 954 / 486 national teams have a non-empty list.
- **Club teams** (real save only): every club team lists all players whose current team it is (82 552 / 82 552 teams). Lists also hold 1 811 players who play elsewhere, 1 791 of them on loan; the rule is partly decoded, since some loanees out are not listed at their contract team (e.g. Braulio Reynoso, contracted to team 64).
- Both kinds also list persons with record tag 7 (VIRTUAL_PLAYER, hypothesis), which are not players: 315 in national and 2 963 in club lists in the real save, 350 and 58 in the test save.

The captain ref after the list fits the captain on the samples checked (test save: Kane for England, Mbappé for France, Messi for Argentina; real save: Camavinga for France), but only 405 of 792 national captains set in the real save are in the current list, so the label stays a guess.

`fm` treats a player as a member of his current team and of the national team in his offset-19 field. `get_squad`, `squad_depth` and `search_players`' `team_index` / `exclude_team_index` use both; its `club` filter matches club teams only, and `club` stays his club team.

## Open questions

- Squad lists: which loanees out a club team lists; what tag-7 persons in squads are.
- Club finances: which field FM's Original Budget shows (offset 0, €6 519 694 for Metz, or the €6 500 000 field; compare on a club or date where they differ by more than €0.1M); the amount spent this season and the scouting budget; the current wage bill the game subtracts from the wage budget (whether it includes staff); CLUB_FINANCE offsets 12–65 and after 66; where `+0x840` (next season's wage budget) is saved; how the game abbreviates amounts in millions.
