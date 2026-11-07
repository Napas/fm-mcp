# Free-agent interest path — FM 27.0.0 (research note)

Binary: `game_plugin` 27.0.0 arm64 slice (`$TMPDIR/re27/gp_arm64`, sha256 `180ec7c1…2dff6c`, vmaddr == file offset).
Save: `/tmp/claude-501/kz_v03.fm` (copy of `KŽ (v03).fm`, game date 2026-02-11, KŽ first-team rep Rt = 3200).
Labels: **verified (code)** = read in the disassembly; **verified (save)** = computed from the save with the repo parser;
**inference** = reasoning.

Scratch test that produced the per-player numbers: `scratchpad/zz_fa_scratch_test.go.txt` (removed from the repo).

## 1. Locating the path (verified, code)

- `PERSON_INTERESTED_FILTER_RULE` typeinfo name `0x539161d` → typeinfo `0x55a74e8` → vtable `0x55a73c0`
  (slots 0x123xxxx). Loan / DP / MQ typeinfo names at `0x53b5eaf` / `0x53b5e8c` / `0x53b5ed4`.
- Interest entry (26.3.2 `0x1283a90`): **`0x1f6eebc`**. Returns −100 000 for hard gates; at the end clamps to
  ≥ −100 000 and calls `0x1f80df4` when the score is above −100 000.
- Kind dispatch in `0x1f6eebc` (r25 = kind, r27 = player's current club, r21 = target club):
  - kind 4 (transfer), 7, 9: r27 == 0 → **`0x1f70254`** (free agent; 26.3.2 `0x1284858`);
    r27 == target → `0x1f70e88`; else → `0x1f71b54` (contracted player; it calls reason 2 at `0x1f72ad0`).
  - kind 2/6: free agent → `0x1f70254` with kind 2 (`0x1f6fa84`), else `0x1f71b54`.
  - kind 3 and 8 (loans): r27 == 0 → −100 000 (a free agent cannot be loaned); else `0x1f73034` (loan path).
- Free-agent path call: `0x1f70254(person, target, L, kind, r24, r22, reasons, flags, …)`; x2 = the offered
  playing-time level L (same argument the gates compare with 3).

**Agent chat**: not traced. Whether "would he consider joining" calls `0x1f6eebc` with kind 4, and with which L
and threshold, is unknown.

## 2. Free-agent path `0x1f70254` (verified, code)

Gates (each −100 000 and return unless noted):

| Addr | Rule | Reason (payload table `0x53b01xx`) |
|---|---|---|
| `0x1f702fc` | he has a contract object with the target's first team of type 4 (trial): L ≤ 3 and `0xd35a08` → −100 000; otherwise **+30 000 and return** | 129 / 8 |
| `0x1f70340` | ACTUAL_PERSON flags `+0x19` bit 7 (trailer flags bit 15, "only one club") unless flags arg bit 2 | 7 |
| `0x1f70360` | `0x1f73e34` (gate, not read) | — |
| `0x1f70478` | draft object `+0xe0`, byte `+0x37` ∈ {11, 23} | 23 |
| `0x1f704c0` | target club relationship to him ≤ −50 | 26 |
| `0x1f70564` | `0x1f758bc` (gate, not read) | — |

Factors, in call order (score at `sp+0x34`):

| Addr | Args | Read? | Notes |
|---|---|---|---|
| `0x1f75e18` | target, prev club | yes | Target club's club-relationship list: rival ≥ 101 → −100 000; ≥ 85 → −50 × value × Loyalty (reason 65). 0 for KŽ's foreign prev clubs (inference: no relationship items). |
| reason 86 | only when prev club == target | yes | Returning player: −2000 × (Adaptability + Ambition) + … ; not relevant here. |
| `0x1f75fb4`, `0x1f76284`, `0x1f76b80` (level 7), `0x1f78728` (unless flags bit 11), `0x1f78ccc` | | no | |
| **`0x1f7a454`** | L, target, 0, 0, person, kind | partly | Playing time; core **`0x1f9af3c`** (§4). |
| block | only when **prev club `+0xd8` set** and prev club `+0xd8` set | | Kazlauskas (no prev club) skips the whole block. |
| reason 46 | age > 28 and both clubs pass `0x17d0b74` | yes | `0x17d0b74` tests a specific league (comp uid `0x1fbc1`); 0 for KŽ (inference: TOPLYGA is not it). |
| `0x1f78f34` (kind 7) | person, target, prev club | no | |
| **`0x1f79b0c`** (kind 7) | person, target, **prev club** | yes | **Reason 2, the club reputation gap against his previous club** (§3). |
| `0x1f7d9ac`, `0x1f7e374`, `0x1f7e700`, `0x1f7ee14` | | no | |
| reason 110 | `0xd2a7ac(person, target)` | yes | Age ≤ 21 and prev club linked to the target by a club-relationship item → +15 × CR. 0 here (inference: no KŽ affiliations). |
| `0x1f7b158` (7), `0x1f7bb98`, `0x1f7c4b8` (7), `0x1f7db90`, `0x1f7e060` | | no | |
| reason 84 | global flag `0x56d496a` | no | |
| `0x1f7f0ac`, `0x1f7f468`, `0x1f7f740`, `0x1f7fb60`, `0x1f7fd74`, `0x1f80c3c` | | no | |

## 3. Reason 2 for a free agent — `0x1f79b0c` (verified, code)

Called with x2 = previous club (ACTUAL_PERSON `+0xd8`), kind 7. Same function as the contracted path, but the
no-contract branch differs from the docs' 26.3.2 description ("0 without a contract"):

```
early out (no reason): PLAYER_CAREER_STATS[5] == 0     (ACTUAL_PLAYER +0x108 → 15 × i16; index 5 at +0xa)
T  = min(4·Rt, 36000)                      Rt = target first-team rep (6000 without one)
     × 1.1 only if 0xd00c50 (age ≥ 30 AND target club's nation/comp id == 11) → never for KŽ
C  = min(4·rep(prev club first team), 36000)
     if ability(player) < level7(prev club):  C = min(C, 1.2·T)
     days = game date − ACTUAL_PERSON +0xec   (365-day years)
     days ≥ 90 → C × 0.8 ; days ≥ 30 → C × 0.9
d  = T − C;  ≤ −4001 ×6, ≤ −3001 ×5, ≤ −2001 ×4, < −1000 ×2      (kind 7 is not 3/8, so d counts)
w  = T − min(4·world rep (+0x25a), 36000); ×8 below −500; only w < 0 counts
r2 = d + w            (the ×5 "ability" step is skipped: no contract sets the skip flag)
score += r2; reason (2, r2) pushed when r2 ≠ 0
```

`ability(player)` = `0x14ea848(person, …, mode 2, no assessor)` = **CA (`+0x25c`, `vt+0x30` = `0xd09d58`) × 220**
(verified, code). A global (`0x56d4987`) can make a manager adjust it (not read).

`level7(club)` = `0xe0327c(club, person, 7, 0)` → `0xe03b7c(club, 7, base)`:
`base × 0.87` when `[club+0x108]` float `+0x1b4` > −1, else `× 0.92` (`× 0.88` for comp ids 0x303/0x31f); capped
33 000. `base` = cached `[club+0x100]+0xc4` (dated `+0xbc`) or `0xe02d5c(club, 0)`:
- ≥ 12 players from `0x17c55ec(club)`: `0xe036b4` = mean over the **top 12** of the staff-assessed ability
  (`0x14a4cf0` via assessor `0xe039d0(club, 8, 1, 1)`) × 220, smoothed against the cached value
  (`× 0x53899fc`, `× 0x52f6ff0`; constants not read).
- fewer: `0xe033bc(club)` = piecewise in first-team rep (e.g. 4000–4999: (4000·rep − 16 000 000)/1000 + 18 000;
  5000–5999: (4000·rep − 20 000 000)/1000 + 22 000), × a nation factor from comp `+0x1aa` (not decoded).

Approximation used below (inference): capped when **CA < 0.92 × mean CA of the prev club's top-12 first-team
players** (true CA, no staff error, no smoothing). Clubs with < 12 loaded players use the reputation formula.

## 4. Playing time for a free agent — `0x1f9af3c` (verified, code; partly read)

No contract: wanted level m = **3 below age 22**; else **7** when world rep ≤ 0.95·Rt (3040 for KŽ); else 9 when the
target first team's competition byte `+0x1aa` > 0, else 8. d = L − m. With g = 5·(Rt − CR):
d ≥ 1 → max(0, g + d·CR); d = 0 → 0; d < 0 → a branch at `0x1f9b750` (not fully read). For L ≥ 9 and d ≥ 2, d may
drop by 1 via `0xd00f64` (inference: the expected level E). **L is not known** for the agent chat.

## 5. Per-player values (verified, save; cap from the §3 approximation)

T = 12 800. Days from `+0xec` to 2026-02-11. pt shown at L = 10 (upper bound).

| Player | Result | CA | WR | Prev club (rep) | Top-12 CA ×0.92 | Cap | Days | r2 | pt(10) | r2 + pt(10) |
|---|---|---|---|---|---|---|---|---|---|---|
| Kazlauskas | signed | 41 | 200 | none | — | — | — | block skipped | +20 100 (age 15) | +20 100 |
| Ghrissi | talked | 78 | 750 | Nice (7000) | 120.4 | yes | 225 | **+512** | +23 800 | +24 312 |
| Arcila | talked | 85 | 1750 | Córdoba B (2850) | n/a (C < T) | — | 23 | **+1 400** | +24 500 | +25 900 |
| Ashcroft | refused | 94 | 2500 | Tottenham (7650) | 140.1 | yes | 23 | **−10 240** | +25 400 (age 21) | +15 160 |
| Wenig | refused | 102 | 3000 | Ulm (4450) | 96.1 | no | 23 | **−30 000** | +26 000 (age 21) | −4 000 |
| Chahiri | refused | 109 | 3250 | Pau (5400) | 102.6 | no | 226 | **−27 080** | 0 | −27 080 |
| Vigaray | refused | 112 | 3200 | Teruel (4050) | 88.6 | no | 23 | **−17 000** | 0 | −17 000 |
| Meyerhöfer | refused | 111 | 3600 | Preußen Münster (5250) | 102.0 | no | 23 | **−62 000** | 0 | −62 000 |
| Chader | refused | 111 | 2950 | Luzern (5850; 1 player loaded → rep formula ≈ 23 400 × 0.92 < 24 420) | — | no | 226 | **−35 520** | +5 000 | −30 520 |
| Nieto | refused | 117 | 3600 | Tarazona (4200; 0 loaded) | rep formula ≈ 17 300 < 25 740 | no | 23 | **−32 800** | 0 | −32 800 |
| Serrano | refused | 114 | 4200 | A. Madrid B (4750; 0 loaded) | rep formula ≈ 19 300 < 25 080 | no | 23 | **−69 200** | 0 | −69 200 |
| Ilić | refused | 119 | 3700 | Oviedo (6100) | 117.3 | no | 23 | **−85 600** | 0 | −85 600 |
| Niane | refused | 118 | 3500 | Angers (6200) | 116.2 | no | 226 | **−51 840** | 0 | −51 840 |
| Kownacki | (joined Lech) | 120 | 4500 | Werder Bremen (6750) | 123.1 | yes | 23 | **−51 840** | 0 | −51 840 |

pt(10) assumes TOPLYGA's `+0x1aa` > 0 (want 9) for the over-22s with WR > 3040; with want 8 it is still 0 for
CR > 5333. Chahiri (CR 5350) gets 0 either way.

**Separation**: r2 alone splits the 13 tested free agents with a previous club by sign — both who talked are ≥ 0,
every refusal ≤ −10 240. Adding pt(10): talked ≥ +24 312, refused ≤ +15 160. FM's "Interested" level is 20 000; that
the agent chat uses it is an inference, not checked. With a lower L (e.g. 7: pt = 16 000 − CR for the under-22s) the
order holds (Ghrissi +12 612, Arcila +13 150, Ashcroft +1 060).

Borderline cap calls (the cap flips the sign only for Chahiri): Chahiri CA 109 vs 102.6 — capped would give
r2 = +312. Ilić (119 vs 117.3) and Niane (118 vs 116.2) stay negative even if capped (−26 240, −9 088).

## 6. What the parser has / lacks

| Input | Save | Parser |
|---|---|---|
| previous club `+0xd8` | yes | `Player.PreviousClub` (interest input `previousClub`) |
| date `+0xec` | yes | `JoinedClubPerson` (interest input `joined`) |
| CR `+0x258`, WR `+0x25a`, CA `+0x25c`, age | yes | yes |
| prev club first-team reputation | yes | `World.Teams[...].Reputation` |
| prev club top-12 CA | derivable | yes (players by contract club); game uses staff-assessed CA + a cached, smoothed value (`[club+0x100]+0xc4`; not known whether saved) |
| `PLAYER_CAREER_STATS[5]` (`+0x108`, 15 × i16) | unknown | no — the r2 early-out; meaning of index 5 not found (UI bridge reads indices 0–8 at `0x3f2ca4…0x3f8cb8`) |
| nation factor for the < 12-player club formula (comp `+0x1aa`) | unknown | no |
| offered level L / expected level E | in-memory | no (same blocker as 26.3.2) |

## 7. Unknown / blockers

- Agent chat entry, its L and its threshold.
- ~20 free-agent factors not read (list in §2). Any of them could add terms; none was evaluated.
- PLAYER_CAREER_STATS[5]: if zero for a player, r2 is skipped (no reason). Assumed non-zero for the 13.
- Exact `level7`: staff assessment error, cache smoothing, the `0x56d4951` game-mode switch in `0xe0327c`.
- `0x1f9b750` (pt with d < 0) not read.
- REA/Hopper session on `gp_arm64` left open (another fork may be using it).

## 8. Follow-up: club level, squad list, cache, career stats, competition byte (27.0.0)

Scratch: `scratchpad/zz_fa_dump_test.go.txt` (dumps `game_db.dat`), `scratchpad/zz_fa_scratch_savefile_test.go.txt`
(TEAM/COMP bytes), `scratchpad/clubcache.py` (walks the club-extra table). All removed from the repo.

### 8.1 `0xe033bc(club)` — club level from first-team reputation (verified, code)

R = first-team reputation (TEAM `+0xa8`); no first team → 0. r = max(R, 50) is used in the products, R in the range tests.
`base = trunc((u32 product) / 1000.0) + add`:

| R | product | add |
|---|---|---|
| ≥ 9000 | 3500·r − 31 500 000 | 36 500 |
| 8000–8999 | 3500·r − 28 000 000 | 33 000 |
| 7000–7999 | 3000·r − 21 000 000 | 30 000 |
| 6000–6999 | 4000·r − 24 000 000 | 26 000 |
| 5000–5999 | 4000·r − 20 000 000 | 22 000 |
| 4000–4999 | 4000·r − 16 000 000 | 18 000 |
| 3000–3999 | 3000·r − 9 000 000 | 15 000 |
| 2000–2999 | 3500·r − 7 000 000 | 11 500 |
| 1000–1999 | 2500·r − 2 500 000 | 9 000 |
| < 1000 | 1500·r | 7 500 |

(Continuous at the steps, e.g. R 3999 → 17 997, 4000 → 18 000.) Then `level = trunc(f · base)`; if `[club+0x108]` exists
and its byte `+0x1c2` bit 2 is set, level = max(level, 6000).

Factor f (object `o` = the first team's `+0x58`, else `+0x50`; both hold the TEAM record's division ref — KŽ 691 and 691,
Tottenham 7 and 7, verified bytes; which in-memory slot maps to which saved ref is inference):
- `o` null → 1.0.
- `o.+0xc` (the COMP uid) == 11 (Premier League: COMP 7 has uid 11, verified bytes) → 1.04 when R ≥ 7000, else 1.1
  (table `0x5389880`).
- else `o.+0x1aa` (i8) > 0 → 1 + 0.025 · min(v, 10); else 1.0.
- Game-mode globals (`0x56d4951`, `0x56d4953`) and a COMP uid 40 / nation `+0x56e` bit 7 test choose between the
  `+0x50` and `+0x58` object (`0xe03540…0xe0361c`); not resolved, irrelevant when both refs are equal.

Check: Luzern (R 5850, 1 player loaded) has cached level 25 400 = (4000·5850 − 20 000 000)/1000 + 22 000 with f = 1.0.

### 8.2 `0x17c55ec(club, out, 1, 0, 1, 0)` — who counts (verified, code; parts named by inference)

- Teams: the club's own team vector `[club+0x18 … +0x20]` (first team, reserves, B, youth); then the second vector
  `[club+0x30 …]` only when service `[0x56d57d0]` vt+0x30 returns true, and only those whose club `+0xd8`/`+0x170` is set
  (not decoded; inference: affiliates in a special mode).
- Players: each team's squad list `[team+0x38 … +0x40]` (the TEAM record's `u16 n + n × person` list, which the
  parser has as `Team.Players`).
- A player is skipped when person vt+`0x4e8` is true (not decoded), or when his current team (vt+`0x230`) is not this
  team (club byte `+0xba` = 0; = 1 uses vt+`0x220`, the contract team), or when `0x151156c(player, club)` is true:
  he is on loan at this club from a club it has no affiliation of types {4–8, 20} with (inference from the code shape:
  vt+`0x300`, current-team club vs the club's `+0x118` relationship list). Gender: men's and women's teams are separate
  clubs, so no gender test is needed (inference).
- In `0xe02d5c`: fewer than 12 players (mode 0) → `0xe033bc`; else `0xe036b4`: sort by `0x14e9694`, mean of the top 12
  of `0x14ea848(player, mode 2, assessor = 0xe039d0(club, 8, 1, 1))` (staff-assessed CA × 220).
- Reproduction from the parser: players in the club's teams' `Team.Players` whose current team is that team, minus
  loanees in. On the v03 save the first-team-only and whole-club top-12 means were identical for every previous club
  checked.

### 8.3 Smoothing and the cached level — **stored in the save** (verified, code + bytes)

- Constants: `0x53899fc` = **1.02**, `0x52f6ff0` = **0.2**.
- `0xe036b4` mode 0, with a cached value c ≥ 1 dated not 1900: k = trunc(1.02·c); if avg > k → avg when
  `[club+0x150]` byte `+0x34` == 1, else k + trunc(0.2·(avg − k)); if avg ≤ k → avg when `0x17d0b74(club)` (a specific
  league, uid `0x1fbc1`), else (c + avg)/2.
- `0xe0327c` (used by reason 2): with a player argument and `[club+0x108]+0x1c8` ≥ 1, and `0x1486a50(player, first
  team, 1, 1, 0)` false → uses `+0x1c8` (club `+0x108` object not decoded; inference: null for AI clubs). Else uses
  the cache `[club+0x100]+0xc4` when its date `+0xbc` is set and the value > 0; else computes `0xe02d5c(club, 0)`.
  Game-mode bytes 1–2 at `0x56d4951` always compute.
- Refresh `0xe03894` (daily): recomputes and re-dates a club's cache when it is unset, ≤ 0, more than 10 days old, or
  (epoch day − 1) mod 10 == uid mod 10.
- **Saved**: CLUB vtable slot 0xf0 (`0xdf714c`) writes the `[club+0x100]` object; the game_db save loop `0x1047e5c`
  (references `"game_db"`) calls it for every object of pool `[0x56d2db0]+0x88` as `u32 count, count × (u32 index,
  record)`. In the v03 `game_db.dat` body the table starts at offset **49 714 973**, right after 8 × `ff`:
  `u32 30 100` (= CLUB + LEAGUE_BODY_CLUB), then per club `u32 club index` + record.
- Record (from `0xdf714c`): `u8 +0x120, u8 +0x121, u8 +0x122, u16 n + n × u32, 5 bytes (0x1893860: version 1 + 4 bit
  fields of +0xda), u8 +0x123, u32 +0xc0, 6 dates (+0x88…+0x9e), u8, 6 dates (+0xa0…+0xb6), u16 +0xde, date +0xb8,
  u8 +0x125, date +0x84, date +0x80, u8 flag (+0x78 set)`, [if flag: 19 bytes (`0x11e0108`)], `5 × u8 (+0x126…+0x12a)`,
  sub-writers `0x172f288(+0xf8)`, `0x172f288(+0x10c)`, `0x100a270(+0xe0)`, `0x100d960(+0xe7)` (together 61 bytes in all
  records), **`date +0xbc` (u16 day, u16 year), `i32 +0xc4` = cached level**, `i32 +0xc8`, …
- Verified on all 30 100 records: the cache date sits exactly 72 bytes after the flag byte when the flag is 0 (25 207
  records) and 91 when it is 1 (4 893). 25 225 caches are dated 2026 (values 6 875 … 37 656), 4 875 are unset
  (1900-01-01, 0).

Cached level vs the CA approximation (× 220), and the cap call with level7 = trunc(0.92 · cache) (AI club, no
`+0x108`; the 0.88 comp case not checked):

| Prev club (index) | Cache (date) | Top-12 CA × 220 | level7 | Player CA × 220 | Capped |
|---|---|---|---|---|---|
| Nice (519) | 29 406 (2026-02-02) | 28 798 | 27 053 | Ghrissi 17 160 | yes |
| Córdoba B (4837) | 18 645 | — | 17 153 | (C < T anyway) | — |
| Tottenham (456) | 33 788 | 33 484 | 31 084 | Ashcroft 20 680 | yes |
| Ulm (585) | 23 026 | 22 990 | 21 183 | Wenig 22 440 | no |
| Pau (1443) | 24 640 | 24 530 | 22 668 | Chahiri 23 980 | no |
| Teruel (4921) | 21 211 | — | 19 514 | Vigaray 24 640 | no |
| Preußen Münster (570) | 24 365 | — | 22 415 | Meyerhöfer 24 420 | no |
| Luzern (1251) | 25 400 | (rep formula) | 23 368 | Chader 24 420 | no |
| Tarazona (18562) | unset → computed | rep formula ≈ 18 800 | ≈ 17 296 | Nieto 25 740 | no |
| A. Madrid B (1127) | 23 998 | — | 22 078 | Serrano 25 080 | no |
| Oviedo (1173) | 28 453 | 28 050 | 26 176 | Ilić 26 180 | no (by 4) |
| Angers (532) | 27 995 | 27 786 | 25 755 | Niane 25 960 | no |
| Werder Bremen (580) | 29 461 | 29 436 | 27 104 | Kownacki 26 400 | yes |

Same cap decisions as §5, so the r2 values there stand. Ilić is 4 points from the cap; capped he would still be −26 240.

### 8.4 `PLAYER_CAREER_STATS[5]` (verified, code; storage not found)

- `0xf1ce08` rebuilds the 15 × i16 array from the player's career history (`0xf1fb44(…, person, &rows, 1, 0, 1)`):
  per history row, a = row `+0x54` + `+0x55` (0xff counts as 0), g = row `+0x57`. Index 5 (`+0xa`) = **Σ a over all
  rows** (also index 7); indices 3, 4 = Σ g; 0/1/2 and 6/8 are the same sums restricted to the current club / nation
  (not fully read). Inference: a = appearances (starts + substitute apps), g = goals; index 5 = career appearances.
- When every sum is 0 and no array exists, none is allocated (`+0x108` stays null) → reason 2 early-outs.
- No writer of `+0x108` was found (no 30-byte write; the only writers of the array are this rebuild and two loaders
  `0x1c5d8d8`, `0x1c60d30`). Inference: the array is derived on load from the saved career history
  (`player_stats_hist` / history rows), which the parser does not read. Values for the 13 free agents: not read.
  Ghrissi (19) is the only one plausibly at 0 senior appearances; if so his r2 is skipped (0), which does not change his
  outcome.

### 8.5 The no-contract wanted-level byte (verified, code + bytes)

- `0x1f9b044`: object = the target first team's `+0x58`, else `+0x50` (the division COMP); wanted level 9 when its
  i8 `+0x1aa` > 0, else 8 (only for age ≥ 22 and WR > 0.95·Rt).
- COMP writer `0xef4130` (game_db, stream slot `+0x30`): `u8 0, u8 +0x16c, u8 +0x16d, 12 bytes (0xeef8a4: version 1,
  5 × u16 colours, u8 +0x14), u8 +0x1a8, u8 +0x1aa, …` → `+0x1aa` is byte 16 of the COMP record (from `Record.Start`).
- TOPLYGA (COMP 691, uid 2 300 046; KŽ's division ref) `+0x1aa` = **0** → wanted level **8**. Premier League (COMP 7)
  is 0 too. Across all 8 199 COMP records: 0 ×1489, 1–12 ×5052, 100 ×1648, −1 ×10. Meaning of the byte unknown.
- The parser does not read COMP bodies (it splits the table only; `Record.Start` is available inside `savefile`).
- §5's pt values assumed want 9; with want 8 every over-22 free agent still gets 0 at L = 10 (Chahiri −50 → 0).
