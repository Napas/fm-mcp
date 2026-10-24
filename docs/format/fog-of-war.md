# FM26 format: fog_of_war.dat

Part of the format reference. Conventions, dates and the index of all files: [`docs/format.md`](../format.md).

## fog_of_war.dat — scouting knowledge, verified 26.2.0 and 26.3.2 (code + bytes)

Version 2. Loader `0x1a3c6f8` (FOG_OF_WAR_MANAGER) reads the header and calls the per-human loader `0x16f20e8` once per human (`savefile.ReadFogOfWar`):

```
u8   1                    read, not checked
u32  human count
per human:
  u32  human person index (the humans.dat index)
  u32  n
  n × (u8 knowledge, u32 player person index)
```

The loader keeps an item only if the person is a player (vtable `+0x720`) and links it into the player object at `+0xc8`. Knowledge is a signed byte, 1..100 in every save seen. The parser rejects values outside 0..100, a player or human listed twice, and more than 1 000 humans. Samples: test save 1 human (88 351), 73 players (100 ×53, 80 ×15, 60 ×5), body 378 bytes; real saves 2 677 (2035-11-29) and 2 742 (2035-12-02) players, levels 1..100. Every entry is a player and every file parses to its last byte. The human's own players are not listed.

**Effective knowledge** (code): the larger of the stored level (`0x16f456c`, 0 when there is no entry) and a baseline (`0x16f2dbc(human, player)`). Every caller checked does this, then compares the result to 100 (fully known), 94 and 79. The baseline is 100 for players at the human's own club. The human's loaned-out players also show exact values in game, so the club check probably uses the contract club (hypothesis). Otherwise it uses reputation thresholds (which of the in-memory reputations is not mapped) and nation/competition knowledge: reputation ≥ 8000 → 80, ≥ 6500 → 50, ≥ 5000 → 30, nation knowledge level 0..10 × 10, with +10 in some competition cases. These inputs are in-memory fields not mapped to save fields, so `savefile` does not compute the baseline (partly decoded). The `fm` package approximates it: 100 for players whose current team or main-contract team belongs to the human's club (loanees out show exact values in game), else the reputation thresholds on the current reputation (the labels are verified by code, see "Player record"; that the game's baseline uses the current one is a guess), without the nation/competition terms.

**Decay** (code `0x16f2b4c`…): a periodic update lowers a stored level by 1 and drops the entry once it is below 1 or below the baseline. The interval is unknown. Between the 2035-11-29 and 2035-12-02 saves, no stored level went down (2 590 unchanged, 87 up, 65 new).

**From knowledge to the attribute screen**: not found. Straňák (uid 2002195395, WBL, index 263 889) has no stored entry for the human, yet the game shows 8 of his 36 outfield attributes as ranges containing the true value: Crossing 3–12 (10), First Touch 4–10 (8), Marking 10–16 (12), Tackling 13–20 (13), Positioning 2–9 (9), Acceleration 10–19 (17), Pace 15–20 (15), Strength 7–13 (11). Unclipped widths are 6–9 and the true value sits anywhere in the range. That fits a baseline of about 10–30 % (hypothesis). One sample does not show which attributes are shown or how ranges are drawn.

## Open questions

- Scouting knowledge: the full baseline rule (`0x16f2dbc`) and its inputs (which reputation, nation/competition knowledge, likely in `scout_man.dat`); which attributes are shown and how their ranges are drawn; the decay interval.
