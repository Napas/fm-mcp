# FM26 format: Injuries

Part of the format reference. Conventions, dates and the index of all files: [`docs/format.md`](../format.md).

## injury_manager.dat — partly decoded, verified 26.2.0 and 26.3.2 (bytes)

Body:

```
u32  0                    0 in both saves; meaning unknown
u32  n0, n0 × entry       queue 0, sorted by "from"
u32  n1, n1 × entry       queue 1, sorted by "to"
…                         more data (2 MB in the real save), not decoded
entry = u8 1, date from, date to, u32 person index, u8 (0 or 1)
```

`from`/`to` equal start + days min / start + days max of the person's injury item (all 1 321 entries of the test save, 1 176 + 145; all 8 631 of the real save, 4 669 + 3 962, of which 28 are for persons not decoded as players). Queue 0 holds injuries whose earliest return is today or later (5 of the test save's 1 176 are a day earlier). Queue 1 holds injuries whose earliest return is today or earlier, or whose window is a single day (days min = days max); their latest return is today or later. Diakhaté's bruised ankle (window 2035-11-30..12-03) is in queue 1 at 2035-12-02. Hypothesis: the game moves an injury from queue 0 to queue 1 at its earliest return, lets the player recover any day in the window, and removes it at the latest return, so a player in queue 1 is still injured. Code: the loader is `0x1a8a7d4` (references the `"injury_manager"` name); not decoded further.

## Injury types (INJURY) — verified 26.2.0 and 26.3.2 (bytes)

The INJURY table comes right after the CURRENCY_RECORD table's end marker: `u32 count` (= the INJURY pool, 136 in both saves), `u8 0`, then 136 records of 48 bytes (not decoded), each ending with the database record `u32 index, u32 uid, u32 uid`, then the end marker `u32 0, u8 1, u32 154`. Count at body offset 16 698 547 (test) and 28 171 821 (real). Record i carries the i-th smallest uid of the language table's 136 injuries in both saves (uids 1 … 154, with gaps), so an injury item's index is the rank of its uid and its name comes from the language database by the rank rule (below). `savefile` does not parse the table; it reads the INJURY pool count.

## Open questions

- Injuries: the undecoded fields of an injury item and its match record, the INJURY record body, the rest of `injury_manager.dat`, and whether a player in queue 1 shows as injured in game.
