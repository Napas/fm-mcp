# FM26 format: Role ability (game code)

Part of the format reference. Conventions, dates and the index of all files: [`docs/format.md`](../format.md).

## Role ability (game code) — verified 26.3.2 (code + in game)

Not stored in the save: the game computes it from a player's stored attributes (1..100), position ratings and current ability. Code in `game_plugin` 26.3.2, read by disassembly (REA was unusable; `llvm-objdump` plus a small arm64 emulator). `fm` implements the in-possession side (`internal/fm/roleability.go`, tables in `roletables.go`).

**Attributes per role.** The selector `0x48c4a94(u64 role)` returns one classifier per in-possession role bit (50 roles); `0x48c34a8` does the same for the 39 out-of-possession roles (its classifiers also check the build stamp `1a 03 00 01`). A classifier maps an attribute (index + 15, in `savefile.AttributeNames` order) to 0 key, 1 preferable, 2 negative or −1 unused. The in-possession lists, with each classifier's address, are in `internal/fm/roletables.go`. Verified in game (2035-12-14, attribute shading on the role screen: green = key, blue = preferable, negatives unshaded) for Channel Midfielder, Wide Centre-Back, Wide Midfielder, No Nonsense Goalkeeper, Inside Forward and Channel Forward: every shaded attribute is in the list and every listed one is shaded.

**Role rating** `0x48c0a50` (0..200, attributes `v` as stored):

```
K = mean(v over key)   P = mean(v over preferable)   N = mean(max(100 − v, 1) over negative)
R = trunc(2 · (16·K + 4·P + 3·N) / W)    W = 16·[key sum > 0] + 4·[P sum > 0] + 3·[N sum > 0]; R = 0 when W = 0
```

The game computes in float32; float64 gives the same R for every combination of list sums (checked exhaustively), and the same pull-up step below for every r < b ≤ 255.

Position rating `p` = the player's rating at the slot position. From 12 up, some roles blend a neighbouring position when it is higher (`p = (p + other) / 2`): Wide and Overlapping Centre-Back use DL on the left (mask `& 0x111248`), DR on the right (`& 0x208924`), else the higher of the two; Inside Full-Back uses DC; Inside Forward, Playmaking Winger and Inside Winger use AMC; Advanced Centre-Back uses DM. False Nine averages with AMC, halved when below 15, unless the halved value is 0; it does so even when AMC is lower. Penalty in percent: `20 − p` from 15, `trunc(2.5 · (20 − p))` from 12, else `4 · (20 − p)` (20 → 0 %, 18 → 2 %, 15 → 5 %, 14 → 15 %, 12 → 20 %, 11 → 36 %); `R = max(R − trunc(R · pen / 100), 1)`. The out-of-possession twin `0x48bc174` uses the same weights; its position handling (`0x48bec90`) is not decoded.

**Best role rating** `0x48ba160` (kept at player `+0x26b`; out of possession `+0x26c`): the highest R over positions rated 10 or more and the roles offered there (`0x48ba308` → table `0x48b6c98`). A rating below a role threshold counts as 0: 100 for Free Role, Wide Forward and Box To Box Playmaker; 80 for Advanced Centre-Back, False Nine and Playmaking Winger; 60 for Half Back and Advanced Wing-Back. The code also masks out `0xfc0484b656f02100`, which holds no role offered at any position, so `fm` leaves it out. Roles offered (`0x48b6c98`):

| Position | Roles |
|----------|-------|
| GK | Goalkeeper, Ball-Playing Goalkeeper, No Nonsense Goalkeeper |
| DL, DR | Full-Back, Wing-Back, Inside Wing-Back, Inside Full-Back, Playmaking Wing-Back |
| DC | Centre-Back, Advanced Centre-Back, Ball-Playing Centre-Back, No-Nonsense Centre-Back, Wide Centre-Back, Overlapping Centre-Back |
| WBL, WBR | Wing-Back, Advanced Wing-Back, Inside Wing-Back, Playmaking Wing-Back |
| DM | Defensive Midfielder, Deep Lying Playmaker, Box To Box Midfielder, Half Back, Box To Box Playmaker |
| ML, MR | Wide Midfielder, Winger, Playmaking Winger, Inside Winger |
| MC | Central Midfielder, Attacking Midfielder, Advanced Playmaker, Wide Central Midfielder, Channel Midfielder, Midfield Playmaker |
| AML, AMR | Winger, Inside Forward, Playmaking Winger, Wide Forward, Inside Winger |
| AMC | Attacking Midfielder, Advanced Playmaker, Free Role, Second Striker, Channel Midfielder |
| ST | Deep Lying Forward, Centre-Forward, Target Forward, Poacher, Channel Forward, False Nine |
| SW | none |

Every in-possession role stored in the saves' tactics and last lineups is offered at its slot's position (165 187 slots on 2035-12-02 and 12-09, 165 198 on 12-14, 26 543 in the test save; verified, bytes; the real saves by `TestRealSavesStoredRolesAreOffered`). `search_players` accepts a role only at a position that offers it.

**Role ability** `0x2fbcb20(player, role, position, 1)` (person interface slot `0x60`; out of possession `0x2fbccac`), on the current-ability scale. With r = R at the slot, b = the best rating and CA the current ability:

```
r == 0 → CA;  b == 0 → r
p ≤ 14 and b > r:  r += (b − r) / (trunc(8 · max(1 − r/b, 0)) + 2)
v = r == b ? CA : trunc(min(r/b, 1) · CA)
v < T(role) → v / 2      T = 120 Complete Forward; 100 Free Role, Wide Forward, Box To Box Playmaker;
                          80 ACB, Enganche, Regista, False Nine, Playmaking Winger; 60 Half Back, AWB; else 0
```

It feeds the role list for a position (`0x2ffd7b4`), the shortlist's best roles (`0x2fef250`, top 3) and team selection.

**Evidence.** FC Metz's XI on 2035-12-09 (R / best / ability): Sami BPGK 131/133/125, Veiga AWB (WBR 18) 123/126/115, Nduquidi OCB (DC right, 18) 118/132/109, Medina OCB (DC left) 120/131/107, Løvik AWB 123/125/123, Tacalfred CHM 135/135/127, Vassiljev BPCB 125/129/125, Matković IF (AMR 18) 128/130/127, Reddy IF (AML) 129/132/124, Edubatey CHM 133/133/133, Ly CHF 138/141/127 (`TestRealSaveRoleAbility`). A Python version of the same rules (research model) produced `internal/fm/testdata/role_ability_golden.txt`: every role offered at every position, plus cases that each blend, threshold, floor and the pull-up change (`TestRoleAbilityMatchesTheModel`). In game, 2035-12-14: each player's role list (Rodríguez MC, Vilas DC, Castillo ML, Haddad GK, Matković AMR, Ly ST) is in role-ability order, 27 of 27 roles, and the top role's stars equal the player's current-ability stars (6 of 6). The abilities: Rodríguez CHM 129, AP 128, MP 126, AM 121, WCM 107; Vilas WCB 119, CB 119, NNCB 115, OCB 110, BPCB 106; Castillo WM 127, W 124, IW 120, PW 120; Haddad NNGK 107, GK 102, BPGK 99; Matković WF 129, IF 127, W 126, IW 123, PW 120; Ly CF 130, CHF 127, DLF 127, P 123, TF 120 (`TestRealSave14DecRoleAbility`).

**Stars: not reproduced.** The Tactics Planner and role list show role ability as stars. The mapping is not found: exact stars from save data match 0 of 38 samples. Across players a higher role ability never has fewer stars in 28 of 38 (10 of 11 on 12-09, 21 of 27 on 12-14). The stars follow a per-player assessment that changes over time: Rodríguez's 129 shows 3.5 stars and Matković's 129 shows 4 on the same day, and Ly's Channel Forward went from 3.5 to 4 stars between 12-09 and 12-14 with the same attributes, current ability and role ability. Hypothesis: the staff's assessed current-ability stars, scaled by role ability / CA.

Out-of-possession roles (selector `0x48c34a8`, 26.3.2 build stamp; not used by `fm`):

| Role | Mask | Classifier | Key | Preferable | Negative |
|------|------|------------|-----|------------|----------|
| Goalkeeper | `0x1` | `0x48c371c` | Handling, AerialReach, CommandOfArea, Communication, Positioning, Reflexes, Agility, Concentration | Anticipation, Decisions, OneOnOnes, RushingOut |  |
| Sweeper Keeper | `0x2` | `0x48c3778` | Handling, AerialReach, CommandOfArea, Communication, Anticipation, Decisions, OneOnOnes, Reflexes, RushingOut, Agility | Positioning, Eccentricity, Acceleration, Concentration |  |
| Line-Holding Goalkeeper | `0x4` | `0x48c37dc` | Handling, AerialReach, CommandOfArea, Communication, Positioning, Reflexes, Agility, Concentration | Anticipation, Decisions, OneOnOnes | RushingOut |
| Centre-Back | `0x8` | `0x48c38c0` | Heading, Marking, Tackling, Anticipation, Positioning, Strength, JumpingReach | Decisions, Pace, Bravery, Aggression, Composure, Concentration |  |
| Stopping Centre-Back | `0x10` | `0x48c3988` | Heading, Marking, Tackling, Anticipation, Positioning, Strength, JumpingReach, Bravery, Aggression | Decisions, Pace, Concentration |  |
| Covering Centre-Back | `0x20` | `0x48c3924` | Heading, Marking, Tackling, Anticipation, Decisions, Positioning, Strength, Pace, JumpingReach, Concentration | Acceleration, Bravery |  |
| Wide Centre-Back | `0x40` | `0x48c39ec` | Heading, Marking, Tackling, Anticipation, Positioning, Strength, JumpingReach | Decisions, Acceleration, Pace, Bravery, Aggression, Agility, Concentration |  |
| Stopping Wide Centre-Back | `0x80` | `0x48c3ab4` | Heading, Marking, Tackling, Anticipation, Positioning, Strength, JumpingReach, Bravery, Aggression | Decisions, Acceleration, Pace, Agility, Concentration |  |
| Covering Wide Centre-Back | `0x100` | `0x48c3a50` | Heading, Marking, Tackling, Anticipation, Decisions, Positioning, Strength, Pace, JumpingReach, Concentration | Acceleration, Bravery, Agility |  |
| Full-Back | `0x200` | `0x48c3b18` | Marking, Tackling, Anticipation, Positioning, Teamwork, Acceleration | Decisions, WorkRate, Stamina, Pace, Aggression, Agility, Concentration |  |
| Pressing Full-Back | `0x400` | `0x48c3bdc` | Marking, Tackling, Anticipation, Positioning, Teamwork, WorkRate, Acceleration, Aggression | Decisions, Stamina, Pace, Bravery, Agility, Concentration |  |
| Holding Full-Back | `0x800` | `0x48c3b7c` | Marking, Tackling, Anticipation, Decisions, Positioning, Acceleration, Concentration | Teamwork, Pace, Agility |  |
| Wing-Back | `0x1000` | `0x48c3c40` | Marking, Tackling, Anticipation, Positioning, Teamwork, Acceleration, Stamina | Decisions, WorkRate, Pace, Aggression, Agility, Concentration |  |
| Pressing Wing-Back | `0x2000` | `0x48c3d08` | Marking, Tackling, Anticipation, Positioning, Teamwork, WorkRate, Acceleration, Stamina, Aggression | Decisions, Pace, Bravery, Agility, Concentration |  |
| Holding Wing-Back | `0x4000` | `0x48c3ca8` | Marking, Tackling, Anticipation, Decisions, Positioning, Acceleration, Concentration | Teamwork, Stamina, Pace, Agility |  |
| Defensive Midfielder | `0x8000` | `0x48c3d6c` | Tackling, Anticipation, Decisions, Positioning, Teamwork, WorkRate | Marking, Strength, Stamina, Aggression, Concentration |  |
| Dropping Defensive Midfielder | `0x10000` | `0x48c3dcc` | Marking, Tackling, Anticipation, Decisions, Positioning, Teamwork, WorkRate, Strength, JumpingReach, Concentration | Stamina, Aggression |  |
| Pressing Defensive Midfielder | `0x20000` | `0x48c3e2c` | Tackling, Anticipation, Decisions, Positioning, Teamwork, WorkRate, Stamina, Aggression | Marking, Strength, Pace, Concentration |  |
| Screening Defensive Midfielder | `0x40000` | `0x48c3eb4` | Marking, Tackling, Anticipation, Decisions, Positioning, Concentration | Teamwork, WorkRate, Strength, Stamina |  |
| Wide Covering Defensive Midfielder | `0x80000` | `0x48c3f10` | Marking, Tackling, Anticipation, Decisions, Positioning, Teamwork, WorkRate, Stamina, Agility | Strength, Pace, Aggression, Concentration |  |
| Central Midfielder | `0x100000` | `0x48c3fd4` | Tackling, Decisions, Teamwork, WorkRate | Marking, Anticipation, Positioning, Stamina, Aggression, Concentration |  |
| Pressing Central Midfielder | `0x8000000` | `0x48c4034` | Tackling, Anticipation, Decisions, Teamwork, WorkRate, Stamina, Aggression | Marking, Positioning, Strength, Pace, Concentration |  |
| Screening Central Midfielder | `0x10000000` | `0x48c4098` | Marking, Tackling, Decisions, Positioning, Concentration | Anticipation, Teamwork, WorkRate, Stamina |  |
| Wide Covering Central Midfielder | `0x20000000` | `0x48c40f8` | Marking, Tackling, Decisions, Teamwork, WorkRate, Stamina, Agility | Anticipation, Positioning, Pace, Aggression, Concentration |  |
| Attacking Midfielder | `0x40000000` | `0x48c4190` | Anticipation, Decisions, WorkRate | Marking, OffTheBall, Teamwork, Stamina, Aggression |  |
| Tracking Attacking Midfielder | `0x80000000` | `0x48c43f0` | Anticipation, Decisions, Teamwork, WorkRate, Stamina, Aggression | Marking, OffTheBall, Positioning |  |
| Central Outlet Midfielder | `0x100000000` | `0x48c41ec` | OffTheBall, Anticipation, Decisions, Teamwork, Balance, Concentration | Marking, FirstTouch, Strength, Composure | WorkRate, Stamina |
| Splitting Outlet Attacking Midfielder | `0x200000000` | `0x48c42d0` | OffTheBall, Anticipation, Decisions, Teamwork, Pace, Concentration | Marking, FirstTouch, Acceleration, Composure | WorkRate, Stamina |
| Wide Midfielder | `0x800000000` | `0x48c444c` | Decisions, Teamwork, WorkRate, Acceleration | Marking, OffTheBall, Anticipation, Stamina, Pace, Aggression, Agility |  |
| Tracking Wide Midfielder | `0x1000000000` | `0x48c4570` | Anticipation, Decisions, Teamwork, WorkRate, Acceleration, Stamina, Aggression | Marking, OffTheBall, Tackling, Positioning, Pace, Agility |  |
| Wide Outlet Wide Midfielder | `0x2000000000` | `0x48c44ac` | OffTheBall, Anticipation, Decisions, Teamwork, Pace, Concentration | Dribbling, Marking, FirstTouch, Acceleration, Agility, Composure | WorkRate, Stamina |
| Winger | `0x4000000000` | `0x48c45d0` | Anticipation, Decisions, Teamwork, WorkRate, Acceleration | Marking, OffTheBall, Stamina, Pace, Aggression, Agility |  |
| Tracking Winger | `0x8000000000` | `0x48c4778` | Anticipation, Decisions, Teamwork, WorkRate, Acceleration, Stamina, Aggression | Marking, OffTheBall, Tackling, Positioning, Pace, Agility |  |
| Inside Outlet Winger | `0x10000000000` | `0x48c462c` | OffTheBall, Anticipation, Decisions, Teamwork, Balance, Concentration | Dribbling, Marking, FirstTouch, Acceleration, Strength, Composure | WorkRate, Stamina |
| Wide Outlet Winger | `0x20000000000` | `0x48c46b4` | OffTheBall, Anticipation, Decisions, Teamwork, Pace, Concentration | Dribbling, Marking, FirstTouch, Acceleration, Agility, Composure | WorkRate, Stamina |
| Centre-Forward | `0x40000000000` | `0x48c47d8` | Anticipation, Decisions, WorkRate | Marking, OffTheBall, Teamwork, Stamina, Aggression |  |
| Tracking Centre-Forward | `0x80000000000` | `0x48c4a38` | Anticipation, Decisions, Teamwork, WorkRate, Stamina, Aggression | Marking, OffTheBall, Positioning |  |
| Central Outlet Centre-Forward | `0x100000000000` | `0x48c4834` | OffTheBall, Anticipation, Decisions, Teamwork, Balance, Concentration | Marking, FirstTouch, Strength, Composure | WorkRate, Stamina |
| Splitting Outlet Centre-Forward | `0x200000000000` | `0x48c4918` | OffTheBall, Anticipation, Decisions, Teamwork, Pace, Concentration | Marking, FirstTouch, Acceleration, Composure | WorkRate, Stamina |

## Open questions

- Role ability: the mapping to the stars the game shows (a per-player anchor, not in the save as far as decoded); out-of-possession position handling (`0x48bec90`) and best-role set.
