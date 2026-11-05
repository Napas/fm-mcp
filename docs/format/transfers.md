# FM26 format: transfer_man.dat

Part of the format reference. Conventions, dates and the index of all files: [`docs/format.md`](../format.md).

## transfer_man.dat — TRANSFER records, verified 26.3.2 (code + bytes)

The body starts with `u8, u32 count`: the u8 is 1 in four KŽ saves and 0 on 2036-01-01; the count (29 934 on 2036-01-16, 31 054 on 2035-12-02) matches the id range of the records (0..29 933 on 2036-01-16), so it is probably the size of the TRANSFER table (inference). Records follow in ascending id order up to about byte 2 540 000 of the 21.4 MB body (2036-01-16; the record with the highest id, 29 933, starts at 2 534 027); the 18.8 MB after them are not decoded. Each record is the base part below plus a part per class (the writer ends with a call through vtable `+0x20`).

TRANSFER base part (writer `0x3a53bb4`, loader `0x3a53fec`; offsets from the record start, object offsets in brackets):

| Off | Size | Field | Status |
|-----|------|-------|--------|
| 0 | u8 | class: 2, 3, 4, 7, 8, 9, 10 seen | bytes; not written by `0x3a53bb4`, so probably by its caller (inference) |
| 1 | u16 | version 0x13; 0x14 in 27.0.0 (see "27.0.0 records") | code (constant at `0x52675aa`) + bytes |
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

**Finding the records** (`savefile.ReadTransfers`): a header-only body has no records. Record 0 follows the header; its u16 at body offset 6 is the record version (0x13 in the 26.3.2 KŽ save of 2037, 0x14 in both 27.0.0 saves; any other is `ErrUnsupported`). The parser scans for a checked start, since the class parts are not decoded: class 2 or 3, the record version, zero bytes at 7 and 12, and plausible dates (day 1..366, year 1900..2200) at 56 and 60. An id is kept from its first match. After the table come 34 copies of earlier records (2036-01-16; status 0 or 3, never 19), which this drops. A body with more than one candidate per 30 bytes is rejected as corrupt (the base part alone is 117 bytes). Other classes share the base layout but are skipped: in the five KŽ saves from 2035-12-02 to 2036-01-16 status 19 appears only in classes 2 and 3 (bytes). A record is linked to a player only when its person is the player and its id is in his `TransferIDs`, so a false match cannot mark a player.

Counts (2036-01-16): 12 049 records of class 2 and 3 (9 459 + 2 624 matches before dropping copies); status 20 ×5 876, 15 ×4 494, 8 ×647, 19 ×423, 2 ×362, 0 ×103, 16 ×96, 3 ×37, 10 ×7, 1 ×3, 18 ×1. All 423 status-19 records are linked from their person. Status meanings other than 19 are inference (research): 20 completed (the target team is the player's current team in 4 201 of 4 245), 15 rejected or collapsed.

Evidence: Unai Marco (uid 2 002 115 699, person 184 193) lists ids 10 183 and 10 810. Record 10 183 starts `02 13 00 81cf0200 00 57030000 00 57030000 00000000 c7270000 ce720500 13 ffffffff`: class 2, Bari, status 19, date 2036-07-01. The game shows "Trn" (Transfer Arranged) for him; the interest check `0x1283a90` returns −100 000 with reason 10 "has already arranged a future transfer" (code). Record 10 810 is class 3, Guingamp, status 15. Eneko Gamboa, Marko Tomašević and Aleksei Korshun list records too, none of status 19 (`TestRealSave16JanTransfers`).

## 27.0.0 records — verified 27.0.0 (bytes)

Record version 0x14; file version stays 35. The base part reads the same through the move date at 60: every record whose person is a player is linked from that player, none from another person. The other 128 records of the 27b save have a non-player person. Offsets after 60 were not checked.

| Save | Header | Records of class 2/3 | Status 19 | Player ids | Linked |
|------|--------|----------------------|-----------|------------|--------|
| `data/test_save_27.fm` (2026-01-19) | u8 0, u32 1 726 | 33 (all class 2) | 33 | 609 | 33 |
| `data/test_save_27b.fm` (2026-01-22) | u8 0, u32 1 726 | 884 (853 class 2, 31 class 3) | 33 | 1 332 | 756 |

The header u32 is 1 726 in both saves, but the highest ids are 32 and 1 459: it is not the record count.

The first save also has 576 class-10 and one class-7 record, skipped as in 26.3.2.

Evidence: record 0 is `02 14 00 f0cf0000 00 1a020000 00 1a020000 …`: Cavan Sullivan (uid 2 000 365 091, person 53 232, at team 20 023) to team 538, status 19, on 2028-01-01. Record 8 is Ibrahim Dosso (person 8 353) to team 463 on 2027-01-01 (`TestTestSave27Transfers`).

17 of the 33 arranged moves are dated before the game date (2024-11-30 to 2025-02-02); 16 of those are to the player's current team, e.g. Lucas Beltrán (uid 14 217 414) to team 77 on 2025-01-01. All 33 ship with the database (date at 56: 2024-12-30 on 31, 2024-12-31 on 2). The 26.3.2 KŽ save of 2037 has 47 arranged, none dated earlier or to the current team.

In game (27b save, 2026-01-22), past- and future-dated moves look the same: status "Not set" and one transfer offer with a future date. `fm` counts all 33 as arranged, like the 26.3.2 check `0x2f54e10`, which reads only the status.

| Player | Byte 37 | Date at 60 | Offer in game | Offer date in game |
|--------|---------|------------|---------------|--------------------|
| Lucas Beltrán (on loan at River Plate) | 0x21 | 2025-01-01 | River Plate, Transfer, €7.25M | 2027-07-01 |
| Rafa Mir (on loan at Aris) | 0x21 | 2027-07-01 | Aris, End Of Contract, €0 | 2027-07-01 |
| Mark McKenzie (Toulouse) | 0x20 | 2025-01-26 | Philadelphia, Transfer, €2.5M | 2027-01-01 |
| Cavan Sullivan (Philadelphia) | 0x20 | 2028-01-01 | Man City, Transfer, €0 | 2028-01-01 |

For a past date the game shows a later date that is not in the record (bytes: no 2027 date in Beltrán's, no 2027-01-01 in McKenzie's). Beltrán's is the day after his loan ends; how the game picks it is unknown. `fm` omits a past date from the interest reason.

Byte 37 is 0x21 on the 19 moves to the club the player is on loan at and 0x20 on the other 14; it is 0 on all 47 in the 26.3.2 save. Its meaning is unknown.

## Open questions

- `transfer_man.dat`: how the game picks the offer date it shows for a past-dated 27.0.0 arranged move, and byte 37; the class parts of TRANSFER records, the fields marked unknown, status values other than 19, what the header count counts, and the 18.8 MB after the records (2036-01-16).
