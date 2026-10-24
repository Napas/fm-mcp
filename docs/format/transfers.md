# FM26 format: transfer_man.dat

Part of the format reference. Conventions, dates and the index of all files: [`docs/format.md`](../format.md).

## transfer_man.dat — TRANSFER records, verified 26.3.2 (code + bytes)

The body starts with `u8, u32 count`: the u8 is 1 in four KŽ saves and 0 on 2036-01-01; the count (29 934 on 2036-01-16, 31 054 on 2035-12-02) matches the id range of the records (0..29 933 on 2036-01-16), so it is probably the size of the TRANSFER table (inference). Records follow in ascending id order up to about byte 2 540 000 of the 21.4 MB body (2036-01-16; the record with the highest id, 29 933, starts at 2 534 027); the 18.8 MB after them are not decoded. Each record is the base part below plus a part per class (the writer ends with a call through vtable `+0x20`).

TRANSFER base part (writer `0x3a53bb4`, loader `0x3a53fec`; offsets from the record start, object offsets in brackets):

| Off | Size | Field | Status |
|-----|------|-------|--------|
| 0 | u8 | class: 2, 3, 4, 7, 8, 9, 10 seen | bytes; not written by `0x3a53bb4`, so probably by its caller (inference) |
| 1 | u16 | version 0x13 | code (constant at `0x52675aa`) + bytes |
| 3 | i32 | person index (`+0x38`) | verified (Marco 184 193) |
| 7 | u8 + i32 | team the move is to (`+0x40`); the u8 is 0 in every record seen | verified (Marco: 855 SS Calcio Bari; 649 En Avant Guingamp) |
| 12 | u8 + i32 | team (`+0x48`); u8 0 and the team equal to offset 8 in the samples | bytes |
| 17 | i32 | `+0x58` | unknown |
| 21 | i32 | id (`+0x5c`): the value a player's ACTUAL_PERSON `+0x68` list holds (`Player.TransferIDs`) | verified (code + bytes) |
| 25 | i32 | `+0x60` | unknown |
| 29 | u8 | status (`+0x8d`); 19 = arranged | verified (code: `0x2f54e10` tests `+0x8d == 0x13`) |
| 30 | i32 | `+0x64`, −1 in the samples | unknown |
| 34, 35 | u8, u8 | `+0x84`, `+0x85` | unknown |
| 36 | 8 | `+0x30` | unknown |
| 44, 45, 46 | 3 × u8 | `+0x86`, `+0x87`, `+0x8b` | unknown |
| 47 | i32 | `+0x68` | unknown |
| 51 | date | `+0x6c`; 1900 on Marco's arranged move | unknown |
| 55 | u8 | `+0x88` | unknown |
| 56 | date | `+0x78` | unknown (Marco: 2036-02-04 on both records) |
| 60 | date | `+0x7c`: the move's date | verified (Marco to Bari: 2036-07-01, the day after his contract ends) |
| 64 | date | `+0x80` | unknown |
| 68 | u8 + i32 | ref `+0x50` | unknown |
| 73, 74 | u8, u8 | `+0x8e`, `+0x89` | unknown |
| 75 | date | `+0x74` | unknown |
| 79 | u8 | `+0x8c` | unknown |
| 80 | date | `+0x70` | unknown |
| 84 | 4 × 8 | `+0x10`, `+0x18`, `+0x20`, `+0x28` | unknown |
| 116 | u8 | `+0x8a` | unknown; the class part follows |

**Finding the records** (`savefile.ReadTransfers`): the parser scans for a checked start, since the class parts are not decoded: class 2 or 3, u16 0x13, zero bytes at 7 and 12, and plausible dates (day 1..366, year 1900..2200) at 56 and 60. An id is kept from its first match. After the table come 34 copies of earlier records (2036-01-16; status 0 or 3, never 19), which this drops. A body with more than one candidate per 30 bytes is rejected as corrupt (the base part alone is 117 bytes). Other classes share the base layout but are skipped: in the five KŽ saves from 2035-12-02 to 2036-01-16 status 19 appears only in classes 2 and 3 (bytes). A record is linked to a player only when its person is the player and its id is in his `TransferIDs`, so a false match cannot mark a player.

Counts (2036-01-16): 12 049 records of class 2 and 3 (9 459 + 2 624 matches before dropping copies); status 20 ×5 876, 15 ×4 494, 8 ×647, 19 ×423, 2 ×362, 0 ×103, 16 ×96, 3 ×37, 10 ×7, 1 ×3, 18 ×1. All 423 status-19 records are linked from their person. Status meanings other than 19 are inference (research): 20 completed (the target team is the player's current team in 4 201 of 4 245), 15 rejected or collapsed.

Evidence: Unai Marco (uid 2 002 115 699, person 184 193) lists ids 10 183 and 10 810. Record 10 183 starts `02 13 00 81cf0200 00 57030000 00 57030000 00000000 c7270000 ce720500 13 ffffffff`: class 2, Bari, status 19, date 2036-07-01. The game shows "Trn" (Transfer Arranged) for him; the interest check `0x1283a90` returns −100 000 with reason 10 "has already arranged a future transfer" (code). Record 10 810 is class 3, Guingamp, status 15. Eneko Gamboa, Marko Tomašević and Aleksei Korshun list records too, none of status 19 (`TestRealSave16JanTransfers`).

## Open questions

- `transfer_man.dat`: the class parts of TRANSFER records, the fields marked unknown, status values other than 19, what the header count counts, and the 18.8 MB after the records (2036-01-16).
