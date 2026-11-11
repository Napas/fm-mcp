# scout_man.dat

## 27.0.0 shortlists — verified 27.0.0 (bytes)

The human manager's shortlists are player list objects in `scout_man.dat` (file version 31). `shortlist_man.dat` (2.8 MB) does not: neither the name "interested" nor the 22-byte entry pattern below occurs in it. `savefile.ReadShortlists` reads them.

The body starts with `1f 00` and a u32 that went from 3 to 4 when the user added a second shortlist (meaning unknown). Then come the list objects: a header, a name, a filter tree (the list's saved search, tags such as `plrf`, `filt`, `pnip`; not decoded) and a person list. The bytes after a list are not decoded; after the first list in the shortlist save they are 14 zero bytes, a u32 count (75) and that many u32s. Every object but the first starts with `1e 00`.

Object header, from the u32 `n` that holds the name's length:

| Offset | Size | Field |
|---|---|---|
| n-33 | 4 | `06 81 c4 01` in every object seen (29 655 302; no other inner file holds it; meaning unknown) |
| n-29 | 2 | `00 01` |
| n-27 | 1 | list type: 0 shortlist; 1, 3, 5, 6 and 10 other lists |
| n-26 | 16 | zero |
| n-10 | 1 | 0 or 1 |
| n-9 | 4 | -1, or an id on type-3 objects |
| n-5 | 1 | 0 or 1 |
| n-4 | 4 | date: 1900 (unset), or a 2026 date on some type-3 objects |
| n | 4 | name length; 0 for the default list, which the game shows as "(Default)" |
| n+4 | L | name, UTF-8 |
| n+4+L | 11 | `01 00 01` and the tags `frlp` `tlif` (plrf, filt) |

The list follows the first `rSrP 00` (tag PrSr) after the name: a u32 count, then 22-byte entries.

| Offset | Size | Field |
|---|---|---|
| 0 | 1 | 2 |
| 1 | 4 | person index |
| 5 | 4 | date added (all "interested" entries: 329/2026 = 2026-11-25, the game date) |
| 9 | 4 | 0 (meaning unknown) |
| 13 | 4 | date, often 1900 (unset), sometimes a later date (meaning unknown) |
| 17 | 4 | -1 (meaning unknown) |
| 21 | 1 | 0 |

The parser finds each object by the `01 00 01 frlp tlif` marker after its name, walks back to the name length (zero run, plausible date, valid UTF-8) and reads the list, which must end before the next object's type byte. It ignores `06 81 c4 01`, which may be career-specific.

Evidence, `data/test_save_27_shortlist.fm` (KŽ, game date 2026-11-25), checked against the user's screenshots of the Shortlists screen:

- "interested" (7): person indexes 8213, 9553, 40122, 26060, 6981, 10156, 31569 = Déblé, Rahmani, Mejía, Kanté, Juli, Toni Silva, Mlapa.
- (Default) (12): 20570, 46968, 56518, 44382, 18294, 20610, 27376, 110079, 104681, 105833, 109065, 20758 = Gašpuitis, Arcila, Ghrissi, Wesley, Siaulys, Adamonis, Juskevicius, Burdzilauskas, Byrne, Rizki, Kihlberg, Magdušauskas.
- Type 0 is the only type both lists have, and no other object has it. Across the FM27 saves (`test_save_27`, `27b`, `27_interest`, `27_homegrown`) exactly one type-0 object exists, unnamed: the default list (empty in `test_save_27`; 5 players in `test_save_27_interest`, Gašpuitis first, added 35/2026). That type 0 means shortlist is inferred from these samples, not from game code.

### Open questions

- What the other list types are (1, 3, 5, 6, 10; 3 is the most common, with ids and dates in its header). Probably scouting assignments or reports (inference).
- Whose list an object is in a save with several human managers; every sample has one.
- The filter tree's grammar, the second entry date, and the u32s after each list.
