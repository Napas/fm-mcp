# FM26 format: Currencies

Part of the format reference. Conventions, dates and the index of all files: [`docs/format.md`](../format.md).

## Currencies (CURRENCY_RECORD) — verified 26.2.0 and 26.3.2 (bytes)

All money in the save is GBP. The CURRENCY_RECORD table holds the rates the game uses to show it in other currencies. It is located like the CLUB and TEAM tables: right after a table end marker, with the count from `memory_pools.dat` (CURRENCY_RECORD = 173 in both saves), a `u8 0` flag and the first record's tag 0. It comes after the CLUB table.

```
table    = u32 count, u8 0, record × count, end marker (u32 0, u8 1, u32 last uid)
record   = u8     tag 0
           string name          "European Euro"
           string short name    "Euro"
           u32    0             unknown
           string symbol        "€"; empty for some (Swiss Franc, Norwegian Krone, Czech Koruna)
           f32    rate          units of this currency per 1 GBP
           u32 n + n × (u8 1, u16 year, f32 rate)   yearly history; not used by the parser
           u32 index, u32 uid, u32 uid              database record
```

| Save | Count at body offset | Table end marker at |
|------|----------------------|---------------------|
| test (26.2.0, 2025-07-07) | 16 622 425 | 16 698 538 |
| real KŽ (26.3.2, 2035-12-02) | 28 095 699 | 28 171 812 |

Each table head matches once in its save. The table (76 122 bytes with its end marker) is byte-identical in the 2025 test save and the 2035 real save, so rates do not drift during a career. Examples: uid 86 GBP 1.0, uid 28 EUR 1.1571305 (f32 `0x3f941cda`), uid 87 USD 1.3508246, uid 2 Albanian Lek (index 0), uid 87 000 022 Guatemalan Quetzal (index 172).

Display (code `0x43de5b0`, from the investigation): shown amount = round(GBP amount × rate). Check: Ashby earns £15 315 a week; × 52 × 1.1571305 = €921 516, and the game shows "€922K p/a". Metz's bank balance £29 636 499 × the f32 rate (1.15713048…) rounds to €34 293 296, exactly the game's figure. `fm.Money` applies this rule; a wage is scaled to its period (52 weeks a year, 52/12 a month), then rounded once. It does the conversion for every amount the server returns. The parser rejects rates that are not finite and > 0.

Abbreviated amounts in millions (inference, 26.3.2, three samples): the Recruitment Budgets screen seems to truncate, not round, to two decimals: €9 637 825 shows as "€9.63M" (12-14) and €9 618 594 as "€9.61M" (12-02, as reported). The wage screen fits too: if Available + Spending = the total, truncated figures give a sum in [64.36, 64.38), which holds €64 371 729 (16.43 + 47.93 = 64.36); rounded figures give [64.35, 64.37), which does not. The Original Budget's "€6.5M" fits offset 0 (€6 519 694) under neither rule (see "Club budgets"). Amounts in K round (€921 516 → "€922K"). `fm.Money` returns whole amounts and does not abbreviate.

### Display preferences (outside the save)

The game's preferences folder (`~/Library/Application Support/Sports Interactive/Football Manager 26/Preferences/version 6/`) holds one XML file per setting, `<record><integer id="value" value="N"/></record>`. Read by `internal/fmprefs`.

- `selected_currency.xml`: N is 0..65, an index into a uid table in `game_plugin` 26.3.2 at `0x524fba4`: `86,4,5,11,13,20,23,28,30,31,32,56,33,41,43,45,58,63,64,67,74,77,78,84,87,73,6,95,16,18,61,94,14,17,22,34,35,38,37,36,42,51,54,66,72,69,70,71,80,91,2,92,93,71006962,15,62,57,79,68,85,19,96,48,65,89,26` (0 → GBP, 7 → EUR, 24 → USD; verified by code and spot checks). The names in the test save's table fit the language defaults below (4 Brazilian Real, 6 Danish Krone, 33 Chinese Yuan, 58 Saudi Riyal). Uids 79 and 96 (indices 57 and 61) have no record in either save.
- When the file is missing, the game picks a currency by `current_language.xml` (`<symbol id="value" value="english"/>`): german, french, italian, spanish, dutch, greek and portuguese → 7 (EUR), "english (us)" and "spanish (latam)" → 24, "portuguese (bra)" 4, danish 6, japanese 15, norwegian 16, polish 17, russian 19, swedish 21, turkish 23, korean 25, "chinese" and "chinese (traditional)" 33, indonesian 37, thai 48, arabic 58, others 0.
- `wage_string_type.xml`: 3 = per year (verified: the game shows "p/a"; per year = weekly × 52). 1 = per week and 2 = per month are inferred from the menu order; per month = weekly × 52 / 12 is also inferred.
- `currency.xml` (value 0 in the user's folder, while `selected_currency.xml` is 7) is a different setting; its meaning is unknown.
