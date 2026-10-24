# player-interest-factors — the factors of the transfer interest path `0x1285498`

Research only. Binary `game_plugin` 26.3.2 arm64 (`research/tactics/gp_arm64`), read with llvm-objdump
(`role-weights/xref/fn.py`, full listing `lineup-formations/full.s`). Save: `data/kz-2036-01-16.fm`.
Probe: repo copy in `player-interest-factors/mod` (adds raw relationship items, club relationship
items, contract ring items, the FULL header and the ACTUAL_PERSON dates); outputs
`player-interest-factors/probe16.txt`, `probe_sonko.txt`, `r43.txt`, `ptlevels.txt`.
Labels: **verified (code)** = read in the disassembly; **verified (bytes)** = read in the save;
**inference** = my reasoning. Nothing here was emulated.

## 1. Answer: Barthes vs Cissokho

**Factor `0x128f550` ("He has only just joined his new club", reason 43) separates them.** Verified (code + bytes).

```
P = (11 − PT) × (D − dd + 1) × Loyalty × 100      (× 1000 instead of × 100 when FULL type byte +0xc3 = 2)
score −= P, reason (43, −P)
```

- PT is the playing-time level the filter offers. It is the 2nd argument of `0x128f550`, which is the 4th argument
  of `0x1285498`; 0 counts as 5. Levels run 1..10 (inference), so (11 − PT) is at least 1.
- dd is the days since the person joined the current club (ACTUAL_PERSON `+0xec`, below).
- D = 250. The other path, for renewals that end within 300 days, uses D = 100 and dd = days since signing, and
  pushes reason 59.
- Loyalty is the hidden attribute (`+0x42`).

The penalty applies only when every condition below holds (verified, code):
- kind is 4, 5 or 7;
- the main contract is FULL at a club team;
- `0x12ae024(person, contract)` returns 1. It returns 0 when the contract type is 3, when dd ≥ 61 and the type is 4,
  and when dd ≥ 61 and the contract ends within 300 days (`0x3279ad4(contract, 300, 0, 1)`). It also returns 0 for
  some open ring items and clause types 55/56;
- the original agreed playing time (contract `+0x56` = core byte 22) is in 1..6 or 15..20. This test applies when
  the current club's first team has a manager; otherwise the test is that `+0x57` bits 0 and 3 are clear;
- dd < D;
- not both bits 31 and 42 of contract `+0x28` are set (in-memory; not saved by the contract writers).

| | Barthes (2002112631) | Cissokho (2002099764) |
|---|---|---|
| joined Caen (`+0xec`) | 2035-08-16 → dd = 153 | 2030-07-15 → dd = 2 010 |
| contract signed / ends | same day / 2037-06-30 (530 days) | 2033 / 2037-06-30 |
| original PT (`+0x56`) | 4 | 2 |
| loyalty | 9 | 17 |
| **reason 43** | **−88 200 × (11 − PT)** (≥ 88 200 at PT 10) | **0** (dd ≥ 250) |

Thresholds are 0 / 10 000 / 20 000 plus `(200 − v) × 20`. One 88 200 penalty outweighs every positive factor seen
for them:
- reason 2 is +6 772 for both;
- playing time adds at most about +15 000 to +25 000 (below);
- reason 3 adds a few thousand.

So Barthes cannot reach Doubtful by transfer (inference from the magnitudes; the other factors are not all computed).

Every other factor that can differ between them is small or favours Barthes:
- **Playing time** (`0x128d22c` → `0x12ad438`) favours Barthes. His desired level is 7 against Cissokho's 9 (table below).
  At the same offered level L it gives him about +2 × CR more. Verified (code) for the formula; L itself comes from
  the in-memory squad ranking `0x2f9e580`.
- **Talented group** (`0x1289bf0`): CA 114 against 116, about 440 apart (inference: the ability term scales CA by 220).
  The flat −10 000 needs the current club to be ≥ 1.2 × the target's reputation; Caen 5525 × 1.2 = 6630 < Metz 7218,
  so neither gets it (verified, code + bytes).
- **Family finance** (`0x1289688`, reason 39) would favour Barthes, who has a lower wage (5104 against 7518) and lower
  loyalty (9 against 17). The target-wage input is in-memory.
- **Reason 46** (`0x12895cc`) is age − 28: −4 for Barthes, −3 for Cissokho.
- **CR-scaled factors** (reasons 17, 41, 44, 49, 76, 99, 140) share the same club-level conditions. Only CR differs:
  5778 against 5338.
- **Relationship type 106** with Caen (value 10 for Barthes, 100 for Cissokho) is the one saved difference I could not
  place. No factor function compares an item type with 106 (searched the factor bodies and the full listing for
  `cmp …, #0x6a`). It is probably a club-bond value (inference).

Suggested in-game check: open Barthes' "Player's interest". The reason should read "He has only just joined his new club".

## 2. The same factor across all 23 players (2036-01-16)

Computed from bytes with the rule above (`r43.txt`; Sonko from `probe_sonko.txt`). Joined dates are ACTUAL_PERSON `+0xec`.

| Label (ground truth) | Player | dd (days) | Contract ends in | Loy | Orig PT | Reason 43 |
|---|---|---|---|---|---|---|
| Fail | Molefe (Sochaux) | 191 | 895 | 9 | 3 | **−54 000 × (11−PT)** |
| Fail | Ortega (Huesca) | 174 | 1 260 | 15 | 2 | **−115 500 × (11−PT)** |
| Fail | Barthes (Caen) | 153 | 530 | 9 | 4 | **−88 200 × (11−PT)** |
| Fail | Guibert (Blackburn) | 170 | 895 | 16 | 4 | **−129 600 × (11−PT)** |
| Fail | Andreani (Havre) | 204 | 1 260 | 10 | 4 | **−47 000 × (11−PT)** |
| Fail | Gamboa (Castellón) | 2 | 895 | 13 | 2 | **−323 700 × (11−PT)** |
| Fail | Sonko (Portland) | 164 | 1 079 | 7 | 3 | **−60 900 × (11−PT)** (first team has no manager; `+0x57` = 0, so it applies) |
| Fail | Marco (Coruña) | 1 832 | 166 | 10 | 10 | 0: must fail for another reason (gates; contract ends in 166 days, B-team status 14) |
| Doubtful only | Lafleur (Famalicão) | 194 | 1 260 | 12 | 4 | **−68 400 × (11−PT)** |
| Doubtful only | Tomašević (AZ) | 6 | 1 260 | 11 | 4 | **−269 500 × (11−PT)** |
| Interested | Nešić (Red Star) | 157 | **166** | 15 | 3 | 0: `0x12ae024` exempts contracts ending within 300 days |
| Interested | Garrido (Valencia) | 188 | 1 260 | **2** | 4 | −12 600 × (11−PT): small because loyalty is 2 |
| Interested | the other 11 | ≥ 555 | | | | 0 |

What the table shows (inference):
- All 7 decoded Fail players except Marco get a penalty of at least 47 000. Marco's failure is not from this factor.
- Both "Doubtful only" players get at least 68 400 under transfer. That fits "fails transfer Persuadable" and passes
  "transfer or loan" through the loan rule (kind 3). In `0x128f550`, kinds 3 and 8 take another branch that does not
  reach this penalty.
- No Interested player gets a large penalty. Nešić is exempt by contract length; Garrido's is −12 600 at PT 10.
- Overall this factor alone gives 22 of 23 labels at the level of "large negative or not"; Marco is the exception.
  The gates agent's `player-interest-gates/gate.py` found the same rule.

## 3. Factor table

The order is the call order in `0x1285498`. "Max |v|" is the largest value the code can produce for realistic inputs,
from constants and caps. "Saved?":
- D = decoded in docs/format.md;
- D* = saved, located in this work (§5);
- U = saved but not decoded;
- M = in-memory or derived;
- G = depends on a global or game mode.

Reasons come from the formatter (`case_ids.txt`). All formulas are verified (code) unless marked partly read.
Values for Barthes / Cissokho are verified (bytes) where marked; otherwise inference.

### Early-return factors (another agent covers them; listed for completeness)

| Fn | Reasons | Effect |
|---|---|---|
| `0x1286a48`, `0x1286c48`, `0x1286f60`, `0x12870b4`, `0x12872c4` | 53, 11/12/13, 12, 62, 16 | Fixed ±100 000 or +10 000 and return |
| `0x12873d8` | 18, 20, 21, 55, 61, 105, 130 | Gives the starting score; returns early when below −1999. **Every branch first needs an open contract-ring item (item date `+0x4` = 1900-01-01)**; with none it returns 0. Barthes has no ring items. Cissokho has 3, all closed (types 37, 37, 19; dates 2033). Both get 0 (verified, code + bytes) |
| `0x1288540` | 24 | Contract type; early return |
| `0x12889f0` | constant | The target club dislikes the player: a relationship item in the club's vector with type 1 and value ≤ −50, or type 2 and value ≥ 50 → −100 000. Metz's vector holds none of the 23 (verified, bytes) |

### Additive factors, ranked by magnitude

| Rank | Fn | Reason | Formula (essentials) | Max \|v\| | Inputs → status | Barthes / Cissokho |
|---|---|---|---|---|---|---|
| 1 | `0x128f550` | 43 / 59 | §1 | ≈ 2.5 M | joined date ACTUAL_PERSON `+0xec` (D*); signed date CONTRACT `+0x44` (D); end `+0x48` (D); original PT `+0x56` (D); FULL type `+0xc3` = header byte 0 (D*); loyalty (D); offered PT (M) | **−88 200×(11−PT) / 0** |
| 2 | `0x128fc68` | 32 | Rivalry. The current club's relationship vector must hold the target as a rival (type 5 or 6, value r ≥ 1), both clubs in the same nation. P = f × r, where f is the player's favoured-club value for his current club (type 3), floored at 50 when he is 21 or under. P × 10 when r = 100; P × 30 when r ≥ 50 and the club nation's uid is 788 (France? inference) | 3 × 10⁵ | club relationship vector (D*, §5); favoured-club value (D) | 0 / 0. Caen's rivals are Havre 85, Rouen 40, Cherbourg 40, Évreux 40, Quevilly 40, Mondeville 25. Only Sochaux lists Metz (50), and Molefe has no type-3 item → 0 |
| 3 | `0x128d22c` → `0x12ad438` | 125/126/129 | Playing time. With L the offered level and m the desired level: m = max(LEVEL[b], scoreLevel), capped at 8 by some open ring items, and adjusted by `0x2f9e580` at star level. d = L − m. d ≥ 1: max(0, 5 × (rep_target − CR) + d × CR); at L ≥ 9 and d ≥ 2, d drops by 1 when L exceeds the squad-rank level by more than 1. d < 0: d × CR, × 10/30/5 when L ≤ 6 (WR, GK and CR tests), minus the reputation term. When m ≤ 4, or m ≥ 9 with nation tests, an early −8× or ×25 path compares rep_target with rep_current × (1.1 + loyalty/100). **Partly read**: the promise-array branch is not fully read | 10⁵ (3 × 10⁵ negative) | b, score (D); CR, WR (D); team reps (D); loyalty (D); offered L and the squad ranking `0x2f9e580` (M) | m = 7 / 9 (`ptlevels.txt`). Favours Barthes |
| 4 | `0x1289688` | 39 | Family finance. Needs L ≥ 8, target first-team division `+0x1aa` = 0 and finance type `+0x3c` ∈ {1,2}. With W = target `[club+0x100]+0xc0` (cap 40 000) and w = the player's wage (cap 40 000), it applies when W ≥ 2000 and W > 2w: value = (W − w)/10 × (21 − loyalty) × 3, with × 0.75, × 1.5 and × 0.1 variants. A second branch uses the career-plan value `0x128e658` | 2.4 × 10⁵ | wage (D); loyalty (D); W (M/U) | unknown; would favour Barthes |
| 5 | `0x1288ad4` | 65 | Fans and rival clubs. −100 000 when the target's vector rates the current club a rival at ≥ 101. A second test uses ACTUAL_PERSON `+0xd8`, an object whose vector holds the player as type 1 with byte 13 = `0x4f`. When the target rates that object a rival ≥ 85: −100 000 if the player has a favoured item for it, else −50 × r × loyalty | 10⁵ | club vectors (D*); loyalty (D); `+0xd8` object (U) | 0 / 0. Metz's rivals are Nancy 90, Strasbourg 85 and Sochaux 40; no player is tied to Nancy or Strasbourg |
| 6 | `0x1291b0c` | 49 | "Surplus, told to leave": 8 × CR when `0x208b3c8(global, club, person)` | 8 × 10⁴ | CR (D); surplus state (M/U) | 0 / 0 (inference: both are squad status 4) |
| 7 | `0x1289bf0` | 1 | Talented group. −50 000 for age ≥ 30, WR ≥ 6250 and a lower-tier target. −10 000 when the current club's reputation ≥ 1.2 × the target's (1.6 × in the same division) and m ≥ 7. Otherwise club level (4 × rep, capped 25 000, floored 8500/10 000/13 000, × 1.1–1.4, capped 33 000–38 000) minus ability (`0x36fa000` mode 2/3 × 220). **Partly read** | 5 × 10⁴ | reps (D); CA (D); staff-assessed ability (M); squad level `0x309b384` (M) | CA 114 / 116 → near equal |
| 8 | `0x128df64` | 27/54 | Continental football. Needs L ≥ 7; (target continental-comp reputation − current) × 50, × 2/3/5 | 5 × 10⁴ | TEAM continental-comp ref (D); COMP `+0x188` reputation (U) | equal (same clubs) |
| 9 | `0x128d1b4` → `0x12ae8f4` | 102/103 | Focus on the [comp]: −WR × 2 × max(1, Pro − 14) × (k − 14) when the current club faces a key fixture and the contract has > 200 days left | ≈ 4 × 10⁴ | WR (D); Professionalism (D); fixture/comp state (M/U) | equal (Pro 15 for both) |
| 10 | `0x1291504` | 76/135 | Current club in financial danger: 3.5 × CR when `[club+0x150]+0x8b1` ∈ {1,2} | 3.5 × 10⁴ | CR (D); CLUB_FINANCE `+0x8b1` (U) | equal flag; CR differs |
| 11 | `0x128db0c` | 14/108/136/61 | Transfer-listed or asked to leave: needs `+0x57` bit 0 or bit 3. ≈ 2 × CR × 1.25 × 1.25 (× 2.5 for GK) − 2 × (CR − rep + 4 × max(0, WR − rep)), floored at 0 | ≈ 3 × 10⁴ | **CONTRACT `+0x57` = core byte 23** (D*: bit 0 is set for Marco 0x41 and Korshun 0x01; bit 6 for Cipriano, Masson and Garrido); CR, WR (D) | 0 / 0 (core byte 23 = 0, bytes) |
| 12 | `0x1290554` | 17 | Work permit: 2 × CR when `0x2fd02ec` | 2 × 10⁴ | CR (D); permit state (M) | likely 0 |
| 13 | `0x128beec` | 3 | Competition: (target-division reputation − current, adjusted × 0.8 for lower tiers) × 100. Needs `0x2ff2054` (player level vs club) ≥ 5 | 2 × 10⁴ | COMP `+0x188`, `+0x1aa` (U) | equal |
| 14 | `0x129167c` | 44 | League performance: ±2 × CR × k, from the target's table position (`0x4172224`) | ≈ 2 × 10⁴ | table (M/U), CR (D) | same sign; CR differs |
| 15 | `0x1290240` | 34 | Admires the manager: a person item whose object is the target first team's manager (Metz: 158530, the human), type 1 or 2 (negated), value v. v × (loyalty + 10) × 5 when v ≥ 1, else v × (31 − temperament) × 5 | 1.5 × 10⁴ | relationships (D); TEAM manager (D) | 0 / 0. **Wong +840, Sonko +8 500** (bytes) |
| 16 | `0x128fe6c` | 33/58 | Passion for the club: a favoured-club item (type 3, or 4 negated) for the target with v ≥ 50 → v × (loyalty + 10) × 5; v ≤ −50 → score −100 001. Reason 58 adds −(`0x39b4d08` × 5?) × (loyalty + 10 or 31 − temperament) × (5 when CR > 0.9 × target rep), for m ≥ 7 and a contract ending within 200 days | 1.5 × 10⁴ (+ hard −10⁵) | relationships (D) | 0 / 0. None of the 23 has a type-3 item for Metz (bytes) |
| 17 | `0x1290380` | 99 | Not registered: CR (½ under age 23) when `0x39410f4` | 10⁴ | registration (M) | likely 0 |
| 18 | `0x128ba6c` | 127/133/145 | Loan spell: v/50 × CR (× 10 when negative), v from `0x142706c`; reason 145 sets −10 000 | 10⁵ (negative) | loan history (M/U) | 0 (inference) |
| 19 | `0x128b728` | 81 | Board ambition: 2 × CR or −3 × CR, clamped to −10 000..6000; needs L ≥ 5 | 10⁴ | board objective, table (M/U) | unknown |
| 20 | `0x1288d14` | 88–93 | Scouting and praise events, from the club's interest record `[club+0x108]+0x148`: (n − 3) × CR/10, CR/10, 0.15 × CR, CR/4… × decay 0.1–1 | ≈ 10⁴ | interest records (U/M) | likely 0 |
| 21 | `0x1290d64` | 41 | Surplus before the window shuts: {1, 0.6, 0.4, 0.2, 0} × CR | 10⁴ | `0x30f7014` (M) | likely 0 |
| 22 | `0x12930c4` | 141 | CR × 1–2 when the global `0x554791e` = 1 | 2 × 10⁴ | G | unknown, equal flag |
| 23 | `0x12905d8` | 36 | Living standard: only when the target nation ≠ the player's nation and ≠ the current club's nation. (nation `+0x1f4` level difference) × CR/2, × 20 in some cases | 10⁵ | NATION tail `+0x1f4` (D*: position decoded, meaning not); CR (D) | 0 / 0 (current and target club both in France) |
| 24 | `0x12892f4` | 35 | Closer to roots: only from age 28 (32 inside the same continent), via `0x2fc04b8` | 6 × 10⁴ | nation (D); birthplace city's nation (U); continents (D); adaptability (D) | 0 for all 23 (all under 28; verified code + ages) |
| 25 | `0x129260c` | 51/52 (+101/104?) | Feedback from players: person event history (`0x2f72cd8`); reason 52 adds loyalty × 100; a final step can raise the score to a 10 000 floor and push the difference. **Partly read** | 10⁴ | event history (U/M) | unknown |
| 26 | `0x1291f7c` | 132 | Manager's keenness: needs an open negotiation of the target's manager for this player (`club+0x370→+0xce8`); ≈ ±0.6–2 × CR | 2 × 10⁴ | negotiation objects (M) | 0 (none under negotiation; inference) |
| 27 | `0x1291b98` | 87 | Dressing room: −5000 / −2000 / −500 / +500 / +1000 / +2000 from team dynamics `0x19183b4` | 5000 | M | unknown |
| 28 | `0x1292268` | 110–124 | Club vision / recruitment policy: CR/2 or CR/4 | ≈ 5000 | club vision (U) | unknown |
| 29 | `0x12915fc` | 140 | Club needs funds: CR/2 when `0x3092a34(club)` | 5000 | M | equal flag |
| 30 | `0x128bcdc` | 96 | Prefers a move elsewhere: −(30 − days) × ambition × 10, from an open ring item (types 13, 22, 36) naming another team | 6000 | ring (D*) | 0 / 0 (no open items, bytes) |
| 31 | `0x12895cc` | 46 | Age: (age − 28), × 5000 when positive, when both clubs pass `0x39c3794` | small under 28 | age (D) | −4 / −3 |
| 32 | `0x1290974` | 40 | Full-time contract: only when the target club `+0x16a` ≥ 2 (non-professional). (distance in miles − (300/400 + ambition + professionalism)) × 1000 | big, but 0 for Metz | club status (U); city coordinates (U) | 0 (inference: Metz is professional) |
| 33 | `0x1290e6c` | 137 | Too young to complete the move: only age ≤ 17 | | | 0 for all 23 |
| 34 | `0x128ecc8` | 30 (+?) | Stage of career: WR ≥ 7000 and age ≥ 32 tests, nation/continent tests. **Partly read** | ≈ 10⁴ | WR, age (D) | probably 0 (inference) |
| 35 | `0x128e658` | 29 | Career plan: person `+0x98` career-plan vector, `0x2f4f3b0`, ×2 or ÷2; joined ≥ 30 days. **Partly read** | unknown | U/M | unknown |
| — | `0x12933cc` | — | Only when `0x5547918` ∈ 1..3 (the *_FMT modes) and `0x5547919` ∉ {1,2} | | G | not in the main game (inference) |
| — | `0x128c898` | 2 | Done (Follow-up 1) | 1 × 10⁵ | D | +6 772 / +6 772 |

## 4. Other per-player values from decoded inputs

| Player | Reason 2 (Follow-up 1) | Reason 34 (admires Metz manager) | Reason 33 (favours Metz) | Reason 32 (rivalry) | Desired playing-time level m |
|---|---|---|---|---|---|
| Wong | +736 | +840 (v 8) | 0 | 0 | 9 |
| Sall | +3 612 | 0 | 0 | 0 | 9 |
| Øverenget | +6 824 | 0 | 0 | 0 | 9 |
| Molefe | not computed | 0 | 0 | 0 (Sochaux→Metz 50, no favoured item) | 9 |
| Cissokho | +6 772 | 0 | 0 | 0 | 9 |
| Ortega | +3 072 | 0 | 0 | 0 | 9 |
| Barthes | +6 772 | 0 | 0 | 0 | 7 |
| Guibert | +2 116 | 0 | 0 | 0 | 9 |
| Cipriano | −2 072 | 0 | 0 | 0 | 4 |
| Nešić | +10 628 | 0 | 0 | 0 | 9 |
| Andreani | +3 716 | 0 | 0 | 0 | 8 |
| Masson | +8 312 | 0 | 0 | 0 | 6 |
| Diallo | +736 | 0 | 0 | 0 | 7 |
| Scotti | −3 328 | 0 | 0 | 0 | 8 |
| Catherine | +2 412 | 0 | 0 | 0 | 9 |
| Lafleur | +1 544 | 0 | 0 | 0 | 7 |
| Gamboa | +11 680 | 0 | 0 | 0 | 9 |
| Marco | +5 189 | 0 | 0 | 0 | 6 |
| Garrido | +2 956 | 0 | 0 | 0 | 6 |
| Korshun | +6 760 | 0 | 0 | 0 | 7 |
| Tomašević | −360 | 0 | 0 | 0 | 7 |
| Sonko | not computed | +8 500 (v 100) | 0 | 0 | 8 |
| Musongi | free agent | | | | |

m = max(LEVEL[b], scoreLevel(score)), verified (code); the youth branch of scoreLevel is ignored (all are 24–25).
The playing-time value then depends on the offered level L. For example, Cissokho at L = 10 gets
5 × (7218 − 5338) + 5338 = +14 738; at L = 8 he gets about −14 738.

## 5. Save structures located in this work (decoded, not yet in docs/format.md)

**Club relationship vector.** The TEAM_CONTAINER `u8 n × 16 bytes` (readTeamContainer's `r.take(16 * u8)`) is the
club's relationship vector at club `+0x48`. It uses the same 16-byte item as a person's:
`i64 ref, u8 reason, u8 0, u8 kind, u8 type, i8 value, 3 bytes`.
- Type 5, kind 1 = rival club, value 0–100. Byte 13 = `0x4f` on most items, the flag `0x1288ad4` tests.
- Kind 3 = persons: type 1 liked (club icons and similar), type 2 disliked.
- Samples, verified (bytes):
  - FC Metz: Nancy 90, Strasbourg 85, Sochaux 40, human manager 158530 type 1 value 87;
  - Sochaux: Metz 50;
  - Caen: Havre 85 and others;
  - Havre: Caen 85;
  - Saint-Étienne: Lyon 100.
- That the factor code reads these vectors is verified (code): `0x1288ad4`, `0x12889f0` and `0x128fc68` read
  `club+0x48` with kind 1..2 and type 5..6. That `+0x48` is exactly this saved list is an inference from the
  matching layout and values.

**Person relationship types besides 1, 2 and 3.** Seen: 105/106 with kind 1 (106 on the current and earlier clubs,
values 10–100, bytes 13–14 like `24 01`, `23 05`), 72/70 (kind 1 or 8), 67/56/25 (kind 4, teams), 9, 116, 44, 71, 18.
No meaning is assigned.

**ACTUAL_PERSON `+0xe8` / `+0xec`.** The two dates before the contract marker (writer `0x2f424a0`: `add x0, x20, #0xe8/#0xec`
→ date writer `0x43fe348`). `+0xec` is the date the player joined his current club. It equals the player-part
"joined" field for every contracted player of the 23 except two loanees, whose `+0xec` is set while "joined" is 1900.
`+0xe8` = 1900-01-01 in all 23.

**Contract ring** (contract `+0x30`, up to 10 items; count byte at ring `+0`, start index at `+0x58`). This is the
core's `u32 n + n × 29 bytes`. Writer `0x3753254` (verified, code):

| Item byte | Field |
|---|---|
| 0 | u8 7 |
| 1 | type `+0x19` |
| 2 | u16 `+0x16` |
| 4 | i8 status `+0x18` |
| 5 | i32 `+0x8` |
| 9 | i32 `+0xc` |
| 13 | date `+0x0` |
| 17 | date `+0x4`; 1900-01-01 = open |
| 21 | u16 `+0x14` |
| 23 | u8 `+0x1a` |
| 24 | u8 `+0x1b` |
| 25 | i32 `+0x10` |

Many factors (`0x12873d8`, `0x12ad438`, `0x128bcdc`, `0x128db0c`, `0x12ae024`) act only on open items.
Cissokho (3 closed) and Molefe (1 closed) are samples.

**Contract core byte 23 = CONTRACT `+0x57`.** The FULL vtable `+0x128` getter `0x1dbc91c` returns it.
- Bit 0 and bit 3 gate reasons 14/108 and the just-joined branch.
- Bit 6 is set on the three players with very low playing-time scores (Cipriano, Masson, Garrido; inference:
  "unhappy with playing time").

**FULL contract vtable** `0x54b5430`:

| Slot | Function | Returns |
|---|---|---|
| `+0x90` | | 0 (FULL) |
| `+0xd8` | `0x34924b4` | `+0xc3`, header byte 0 |
| `+0xf0` | `0x34a6cec` | the end date |
| `+0x100` | | `+0x54` |
| `+0x108` | | `+0x56` |
| `+0x128` | | `+0x57` |

## 6. What stays unknown

- The offered playing-time level L and the squad ranking `0x2f9e580`. They set the (11 − PT) factor and the
  playing-time value (gates/filter agent).
- In-memory inputs: talented-group ability and club levels; target wage figure `[club+0x100]+0xc0`; finance flag
  `+0x8b1`; COMP reputations `+0x188`/`+0x1aa`; league table; negotiation and scouting records; registration,
  work permit and surplus states; team dynamics; career plan `+0x98`; event history.
- Not fully read: the rest of `0x128d22c` (promise array), `0x1289bf0` (ability branch), `0x128e658`, `0x128ecc8`,
  `0x129260c`, `0x1292268`.
- The meaning of relationship type 106. No factor reads it by immediate.
- Contract `+0x28` bits 31 and 42 (in-memory exemption in `0x128f550`).
- Marco's failure (not reason 43) and the role of his `+0x57` = 0x41.

## 7. Proposal for landing

The just-joined rule is the strongest deterministic signal found:
- it explains Barthes vs Cissokho;
- it gives 22 of 23 labels at the "large negative or not" level (Marco is the exception);
- every input is in the save.

Proposal:
- **savefile**
  - `Player.JoinedClubAP Date`: ACTUAL_PERSON `+0xec`, from the two dates before the contract marker.
  - `Contract.FullType uint8`: header byte 0.
  - `Contract.Flags57 uint8`: core byte 23.
  - `Contract.OriginalPlayingTime` is already read.
  - Optional: `Club.Relations []Relation`: the 16-byte TEAM_CONTAINER items, giving rivals (type 5) and liked/disliked
    persons (types 1/2).
- **fm**
  - `JustJoinedPenalty(p, today) (days int, perLevel int)`. It applies the §1 conditions: dd < 250 from `+0xec`; the
    `0x12ae024` exemptions (contract ends within 300 days once dd ≥ 61; contract type 3, or 4 once dd ≥ 61);
    original PT in 1–6 or 15–20; FULL contract. It returns (250 − dd + 1) × loyalty × 100, × 10 for type 2.
- **MCP**, on `get_player`: `interest_blockers: ["only just joined his club (N days ago)"]` when perLevel > 0. Phrase it
  as FM does: "He has only just joined his new club".
- **Integration test** (kz-2036-01-16), asserting perLevel:
  - Barthes 88 200; Cissokho 0; Guibert 129 600; Ortega 115 500; Molefe 54 000; Andreani 47 000; Gamboa 323 700;
    Lafleur 68 400; Tomašević 269 500; Garrido 12 600; Sonko 60 900; Nešić 0 (contract ends in 166 days).
  - Joined (`+0xec`): Barthes 2035 day 228; Cissokho 2030 day 196.
