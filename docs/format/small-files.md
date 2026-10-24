# FM26 format: Small inner files

Part of the format reference. Conventions, dates and the index of all files: [`docs/format.md`](../format.md).

## game_info.dat — partly decoded

After the inner header: string build version (`"26.2.0+0"`), `00`, `" mf."`, then about 197 bytes not yet decoded, then the patch list: u32 count + count × string (92 entries; first `RULE_GROUP_MANAGER_patch_eppp_ratings`, last `TRANSFER_MANAGER_patch_old_save_games_unwanted_loan_fees`).

## memory_pools.dat — decoded

After the inner header: u16 count (467), then count × (string type name + u32 instance count). 231 entries have count > 0; first `ENTRY`, last `CUP_TIE_UI_HANDLER_SERVER`. Examples: `PERSON_NAME` 983 849, `ACTUAL_PLAYER` 37 675, `CLUB` 25 315, `TEAM` 55 784, `FULL_CONTRACT` 44 577. Hypothesis: pool sizes the loader allocates before reading `game_db`.

## save_game_summary.dat — partly decoded (verified 26.2.0 and 26.3.2)

Version 29. Layout (`savefile.ReadSummary`):

```
string  nation list: "<nation uid>,<MALE|FEMALE>,n,n,…" (765 = England)
string  full build, e.g. "26.2.0+2276383"
u32 n + n × string   league names ("Sky Bet League Two", "WSL2", …)
u8 1, u32 unknown, u16 database version, u8 1
string  manager name    "Napas Mačiulaitis"
string  club short name "Spurs" (test) / "FC Metz" (real)
u32     club uid        728 (test) / 844 (real)
date    game date
…       later: 25 squad short names ("G. Vicario", …); not decoded
```

## humans.dat — partly decoded

Version 21. `u16 count` at body offset 0 (file offset 8), then the human's person index as a u32 (88 351 in the test save, 158 530 in the real one; a tag-9 record). Only one-human saves seen; the layout for several humans is unknown, so `savefile.ReadHumanIndex` rejects any other count.

## Open questions

- `humans.dat` and `fog_of_war.dat` with more than one human manager.
- The ~197 undecoded bytes in `game_info.dat`; the `*.scm` / `*.apm` files of 26.3.2 saves.
