# FM26 format: Language database (names)

Part of the format reference. Conventions, dates and the index of all files: [`docs/format.md`](../format.md).

## Language database (names) — verified 26.2.0 and 26.3.2 saves

Saves hold no names for nations, clubs or competitions. Each table record in `game_db.dat` ends with the trailer `u64 | u32 index | u32 uid | u32 uid`, and names come from the game's bundled language database, keyed by uid. Code evidence: the NATION serializer (`0x35bf574`) writes no strings, and RTTI `OBJECT_TRANSLATABLE<NATION>` exists.

File: `fm.app/Contents/PlugIns/game_plugin.bundle/Contents/Resources/shared/data/database/db/<label>/<label>_fm/lang_db.dat`, or `lang_update_db.dat` when the folder has no `lang_db.dat`. `<label>` is the `game_db.dat` header's Label ("2620" test, "2600" real). Some update files (e.g. 2310) are small deltas; 2620's is a full copy.

```
12-byte prefix   03 01 "tad." u16 4000, u16 db version, u16 7
container        same as a .fm save (header, zstd frames, directory), at offset 12
files            "<language>_<table>.dat"; language 8 = English (UK)
                 tables: 3 clubs, 5 continents, 9 nations, 23 stage names, 25 competitions (others: 0 awards, 2 cities, 6 injuries, 21 stadiums)
inner file       03 01 "tad." u16 version, u32 count, records
```

English nations (`8_9.dat`, 251 records, parses to the end): `u32 uid, string name, u8, string short name, u8, string code ("ENG"), string adjective, u8`. Other languages add grammar fields.

**Nation index → name.** The save's NATION table (251 records) lists nations in ascending uid order, so a player's nation index is the rank of the uid among the language table's uids (verified on both saves; 139 = uid 765 England, 150 Italy, 170 Spain, 187 Argentina; 32 known players checked). `fm` applies this rule only when the save's NATION pool count equals the language table size.

Injuries (`8_6.dat`, 136 records in 2600 and 2620, parses to the end): `u32 uid, string name, u8, string name with article ("a bruised ankle"), u8, u8`. **Injury index → name** uses the nation rule: the index is the uid's rank (verified by the INJURY table's uids, and in game: index 84 = uid 100 "bruised ankle" for Veiga, 88 = uid 104 "tight groin" for Bernardi). `fm` applies it only when the INJURY pool count equals the language table size.

Competitions (`8_25.dat`): `u32 uid, string name, u8, string short name, u8, string code, u8`. In-game-created competitions (uid ≥ 2 000 000 000) have no entry. `rgman/comp_<uid>.dat` is named by competition uid (comp_11 = Premier League).

Stage names (`8_23.dat`, 285 records in 2600, parses to the end): `u32 uid, string name, u8, string short name, u8, string code, u8, u8`. Examples: 1 "Replay", 3 "First Leg", 4 "Second Leg", 18 "Semi Final", 20 "Final", 22 "Playoff Final", 43 "Group 1", 73 "Group Stage", 76 "Group A", 238 "Playoffs", 2000223075 "League Phase". The save's STAGE_NAME table holds the same uids (see [`competitions.md`](competitions.md), "Stage names (STAGE_NAME)"); `fm` looks names up by the save's uid, not by rank.

## Open questions

- Names of stadiums and in-game-created competitions (uid ≥ 2 000 000 000).
