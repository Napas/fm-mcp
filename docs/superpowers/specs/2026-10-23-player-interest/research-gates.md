# player-interest-gates — which hard gates decide FM's "Player's interest" filter

Research only. Binary `game_plugin` 26.3.2 arm64 slice. I read the disassembly with `role-weights/xref/fn.py`. Save bytes came from a scratch copy of the repo parser (`player-interest-gates/mod`, command `cmd/gdump`). Save: `data/kz-2036-01-16.fm`, game date 2036-01-16. Ground truth: `player-interest/ground-truth-16jan.md` (23 players).
Scratch files are in `player-interest-gates/`: `f_*.s` (disassembly), `d23/` (player JSON and raw records), `gate.py` / `gate_table.txt` (the gate evaluated on the 23), `candgate.py` (the gate on the earlier 125-candidate list).

## Verdict

**Main finding: one gate, "only just joined his new club" (factor `0x128f550`, reason 43), explains 7 of the 8 failures and the Barthes / Cissokho pair.**
- The gate charges a penalty of `(11 − L) × (251 − days since joining) × Loyalty × 100`.
- It applies when the player joined his current club less than 250 days ago. The main exemption: 61 or more days since joining and a contract that ends within 300 days (exact rule in §2.3).
- For the 7 recent joiners who fail, the smallest possible penalty (offered level L = 10) is 47 000 to 323 700. Everyone who passes Interested has a penalty of 0, except Garrido (12 600 × (11 − L); his Loyalty is 2).
- Nešić joined 157 days ago but passes. His contract ends in 166 days (< 300), so the exemption predicts the pass (verified, code + bytes).
- Lafleur and Tomašević joined recently, so the transfer path fails for them, but they still show as Doubtful under "Transfer or loan". This fits a pass through the loan rule (inference).
- **Barthes joined SM Caen on 2035-08-16 (153 days ago, Loyalty 9), so his penalty is at least 88 200. Cissokho joined in 2030, so his is 0.**
- **Unai Marco** is explained in "Follow-up: Marco" at the end of this report. He has an arranged move to Bari (status-19 transfer in `transfer_man.dat`), so `0x1283a90` returns −100000, reason 10. All 23 players are now explained.
- The threshold is small next to these penalties. It is `(200 − v)·20 + flvl`, at most about 4 000. For Napas, v (non-player +0xda) is probably 200, which makes the threshold equal to the level value: Doubtful 0, Persuadable 10 000, Interested 20 000 (§1.4, inference, medium confidence).

## 1. Filter wrapper — verified (code)

### 1.1 Transfer rule eval `0x1695290(rule, item, ctx)` — returns 1 = keep the player, 0 = drop him
1. Global `[0x5547919]` in 1..2 → keep (another game mode).
2. The human's side: `ctx+0x10` (a team with `+0x2f` set = national team) → national path (not decoded). Club path: `x19` = the searcher's club, taken through `[ctx+8]+0x558` vt+0x270/+0x408.
3. `v` = i16 `+0xda` of `[ctx+0x18]->vt+0x698()`. `A` = `[ctx+0x20]->vt+0x698()`, the assessing staff member (passed down to `0x2f9e580`).
4. **Pre-check** `0x209efe8(global 0x5525910, player, club, 1, first team, 1, 0, 0)`. When it is true the player is dropped, whatever his interest. It is an eligibility check: it runs only when global `[0x5547934]==1` and returns reason codes 2, 13, 25, 30, 37, 40, 41, 58, 59, 68, 70, 71, 72, none of them decoded.
5. If rule byte `+0x34` ≠ 1 → keep: the interest test is off.
6. **Club age gate**: `[club+0x108]` with byte `+0x1fd == 1` and player age > int8 `+0x1ff` → drop. The human club's maximum signing age; not decoded for Metz. Inference: not set, because players aged 24–25 pass.
7. `threshold = (200 − v) × 20 + flvl`, where flvl is the rule's `'flvl'` property (0 / 10 000 / 20 000).
8. `0x2ff2964(player, club first team, threshold, 0, 0, &flag, 1, 0, A)` (§1.2). `flag == 0` → drop.
9. **Final check** `0x14c77a4(current club of the player (0x2f45d8c), human club, player)` true → drop (§1.3).

### 1.2 Playing-time loop `0x2ff2964` (called as the filter calls it)
- The player's expected level at the human club is E = `0x2f9e580(player, first team, A, 1, 4, 0, 0, 0)`; E == 1 is replaced by 5 when age > 21, else 3. When the player already plays for that club, E = min(E, max(LEVEL[b], scoreLevel)). An open negotiation item in `[A'->vt+0x730]+0x370 → +0xce8` that holds a promised status (`+0x201`) replaces E with that status's level (jump table `0x5249dde`).
- Offered level `L = min(E + (club byte +0x16a == 1 ? 3 : 5), 10)`. With arg w6 = 1, the filter evaluates **one call** at L: `interest(player, club, kind 4, L, …, flags 0)`. `flag = (score ≥ threshold)`.
- Because w3 = 0, no further levels are tried. The incremental search from E+1 runs only for other callers.
- Side branches (not taken for Metz, inference):
  - If the human club's link list `+0x118` holds an entry of type {4..8, 20}, the flag is `interest(kind 4, table[0x3780194(...)]) ≥ 0`.
  - If the player already has a YOUTH_TRIAL contract with the club, flag = 1.
- `L` enters the transfer path as its level argument and so reaches about ten factors.

**E sub-model** (code, partly decoded): loop L = 10 down to 2. The first L with `ability ≥ base × f(L)` wins.
- `ability = 0x36fa000(player, …, A)`: A's *assessed* CA, blended with assessed PA. Type 3: CA + 0.25·(PA − CA); type 4: 0.35·; types 1 and 2 also exist. The result is multiplied by 11 × the position rating, capped at 220. The assessment is `0x36b811c`, which uses the staff member's +0x2c/+0x2d attributes (judging ability/potential, /5) and a uid-difference hash.
- `base = 0x309a864(club)`: a squad-strength figure cached at `club+0x100 (+0xbc, date +0xb4)`, built from the top 12/16 players, with a fallback from reputation.
- `f(L)` (`0x309b69c`): 10 → 1.10, 9 → 1.00, 8 → 1.00 or 0.96, 7 → 0.87 / 0.92 / 0.88 by nation, 6 → 0.80, 5 and 2 → 0.75, 4 → 0.83 / 0.88, 3 → 0.5.

E is **not reproduced**. It needs A's identity and attributes, the assessment noise model and the squad cache. None of it is needed for the verdict, because the gate's penalty only grows as L falls (the factor is 11 − L ≥ 1).

### 1.3 Final check `0x14c77a4` (rivalry), code
The player is dropped only when all of these hold:
- the player's current club and the human club share `+0xd8` (the nation);
- the current club has a relationship item (16 bytes: `+0` object, `+0xa` kind 1/2, `+0xb` type 5/6, `+0xc` value) toward the human club with value ≥ 90 (≥ 100 always drops; 90–99 depends on further contract/date and player-attachment tests);
- the player's own favoured-club value for his club is below 50.

Not evaluated on the save: club relationship items are not decoded. Inference from the ground truth: none of the French clubs here (Nantes, Reims, Saint-Étienne, Troyes, Angers, Red Star) is a ≥ 90 rival of Metz, because their players pass.

### 1.4 v — the threshold shift
- **Non-player block layout** (writer `0x2f1e388`): `u16 n + n × 7 bytes`, then u16 `+0xd4, +0xd6, +0xd8, +0xda, +0xdc`, then u8 `+0xde`. On 8 ordinary staff records (tag 1) these read as three reputations (≤ 10000) followed by CA ≤ PA, e.g. 108/110, 139/140, 145/150, 80/93 (verified, bytes). So +0xda is the non-player's current ability: strong inference, by analogy with the player's `+0x25c`.
- **Human record** (tag 9, writer `0x34e8a04`): the human blob, then this non-player block, then ACTUAL_PERSON. 1 715 bytes before Napas's ACTUAL_PERSON part sits `n=0, 8912, 8456, 8294, 200, 200, 16`, which reads as HR / CR / WR / **CA = 200** / PA = 200. Bytes verified; the field identification is inference, medium confidence.
- Which person `ctx+0x18` is was not traced. If it is Napas, `threshold = flvl` exactly.
- In every case `(200 − v)·20 ≤ 4000`, which is negligible next to the gate's penalties.

### 1.5 Loan rule `0x16949bc` and "Transfer or loan"
- Loan pass: `interest(kind 3, level 7) ≥ threshold / 2`, then extra checks `0x20b89f4`, `0x209f708` and the value test `0x2f9f504` (verified, code; consistent with the earlier recon).
- In the loan path `0x128600c`, the just-joined factor runs only when `0x32715dc(contract) ≥ 5`. Inside it, kind 3 adds the test `service vt+0x290(person, club, 0)`; when that is false there is no penalty.
- "Transfer or loan" = the player passes either rule. This is an inference: the UI → rule mapping was not found. It explains Lafleur and Tomašević, who fail the transfer rule because of the gate but are listed.

## 2. Early outs and hard gates

### 2.1 `0x1283a90` (interest entry) — code; what holds for the 23 (bytes)

| Gate | Result | 23 players |
|---|---|---|
| no player, club or `+0x38` | −2000 | n/a |
| clause 54 / 63 date paths | treat as free agent | none of the 23 has clause 54 or 63 (bytes) |
| person flags bit 18 (trailer u64, byte +0x1a bit 2) | −100000, reason 6 "retiring" or 60 "considering his options" | clear for all 23 (bytes) |
| date from service vt+0x148 ≤ today + 90 days | same | date not identified |
| `0x2f54e10` future transfer arranged | −100000, reason 10 | FutureContract is null for all 23 (bytes) |
| relationship type 0x7e toward the human club, or human-side flag `+0x404` | +100000 (reasons 45, 5, 128) | not evaluated |
| MLS rules (target first team's division uid 40) | — | Metz's division index 11 has uid 16, so off (bytes; index 30 = uid 40 = MLS, Portland) |

Dispatch: kind 4 with a different club goes to `0x1285498`; no club goes to the free-agent path `0x1284858` (called at `0x12843a8`; Musongi).

### 2.2 `0x1285498` (transfer path), in call order

Each gate returns a final value; code verified. Reason texts come from `example.ltf`.

| # | Function | Fires when | Result | For the 23 |
|---|---|---|---|---|
| a | `[base+0x19]` bit 7 (flags bit 15) | — | −100000, reason 7 "not interested in any club other than … / considering retiring" | clear (bytes) |
| b | youth-trial contract and L ≤ 3 | — | −100000, reason 129 | n/a |
| c | `0x1286a48` | a record in manager `0x5548728` (`0x32fe440`) with status `+0x29 == 0xff` | −100000, reason 53; certain statuses with L ≥ 9 give +100000 | **not evaluable** (manager not mapped to a save file) |
| d | `0x1286c48` | MLS / salary-rule leagues only | ±100000, reasons 11, 12, 13 (wage demands) | off for Metz |
| e | `0x1286f60` | human team's competition uid 390 or 364 | −100000, reason 12 | club-level, off |
| f | `0x12870b4` | foreign move and L > 7, human division `+0x1aa ≥ 2` | −100000, reason 62 "lower leagues of a different country" | off: Metz is top flight (inference) |
| g | `0x12872c4` | human club linked to the player's club | +10000, reason 16 | club-level |
| h | `0x12873d8` | contract event ring `+0x30` (types 9, 12, 41, 0x2a3) gives < −1999 | returned as final; reasons 18 "bigger club" / 55 "guaranteed playing time" | **not evaluable** (ring not mapped; the core's `u32 n × 29` items are a candidate) |
| i | `0x1288540` | (1) contract type 3 or age ≤ 17, ability > 1.1–1.3 × club base | −100000, reason 50 | n/a for ages 24–25 |
| | | (2) human club offers a worse contract type than the player has, from the per-level table `[club+0x100]+0xd8` via `0x5203840` | −100000, reason 24 | pro club: off (inference) |
| | | (3) only when (2) does not return | −100000, reason 38 "move to another country" | off with (2) |
| j | `0x12889f0` | human club's relationship to the player ≤ −50 | −100000, reason 26 "fans' opinion" | not evaluated |
| | then | ~40 additive factors, among them the just-joined penalty below | | |

### 2.3 The just-joined penalty `0x128f550` — verified (code); inputs verified (bytes)
It is called in the transfer path when flags bits 3 and 5 are clear, which is the case for the filter (flags 0).

```
days   = 365·(today.y − J.y) + (today.d − J.d)       J = ACTUAL_PERSON +0xec = joined-club date
                                                     (the 2nd date before the contracts marker; equals player JoinedClub)
skip unless the current club is a club (not a national team); kind 3/8 have their own checks
days ≥ 120 and the club was relegated (div +0x1aa > [ft+0x60] +0x1aa and 0x3c2e8fc ∈ {1,2}) → no penalty
FULL contract required (non-FULL, e.g. loan → no penalty), and 0x12ae024(person, contract) must hold:
   no penalty if: contract ring has certain events | clause 55/56 | contract type 3 |
                  (days ≥ 61 and (contract type 4 or contract ends in < 300 days))
mode: 0x3274b28(contract) or contract+0x57 bit 3 or vt+0x128 → "just joined", T = 250, d = days
      else, if renewed after joining: contract ≥ 300 days left → "just signed", T = 100, d = days since signing
                                      else → "just joined", T = 250, d = days
AI club: contract vt+0x128 true → no penalty; require (contract+0x57 & 8) == 0
contract +0x28 has bits 31 and 42 → no penalty
P = (11 − L) × (T − d + 1) × Loyalty × 100        (× 1000 instead of × 100 when contract type == 2)
    Loyalty = person +0x42 (hidden attribute, 1..20)
"just signed" and age ≤ 23: P halved (or quartered in some modes)
score −= P; reason 43 ("only just joined his new club") or 59 ("only just signed a new contract")
```
The chat text for these reasons is KEY-554308 / 554307 (`0x300e4e4`, `0x3010c3c`).

## 3. The 23 ground-truth players

L is unknown, so P is given for L = 10, its minimum; the real P is this × (11 − L). Contract type is the FULL header byte (1 for all except Nešić, 0). No one has type 2 or 3, no clause 55/56, and no future contract. Flags bits 15 and 18 are clear for all 23. All values are bytes from the 2036-01-16 save.

| Label (in game) | Player | Joined (+0xec) | Days | Loyalty | Contract end | Just-joined gate | Gate predicts | Match |
|---|---|---|---|---|---|---|---|---|
| Interested | Wong | 2033-07-15 | 915 | 11 | 2038 | ≥ 250 days, 0 | pass | yes |
| Interested | Musongi | free agent | — | 15 | — | path not used | pass | yes |
| Interested | Sall | 2034-01-02 | 744 | 14 | 2039 | 0 (renewed 2035-06-03, 227 d > 100) | pass | yes |
| Interested | Øverenget | 2031-12-31 | 1476 | 13 | 2038 | 0 (renewed 131 d ago > 100) | pass | yes |
| Interested | **Cissokho** | 2030-07-15 | 2010 | 17 | 2037 | **0** | pass | yes |
| Interested | Cipriano | 2023 | 4504 | 12 | 2037 | 0 | pass | yes |
| Interested | **Nešić** | 2035-08-12 | 157 | 15 | **2036-06-30 (166 d)** | **exempt** (≥ 61 days, < 300 days left) | pass | yes |
| Interested | Masson | 2023 | 4422 | 11 | 2038 | 0 | pass | yes |
| Interested | Diallo | 2033-06-29 | 931 | 9 | 2038 | 0 | pass | yes |
| Interested | Scotti | 2034-07-01 | 564 | 4 | 2039 | 0 | pass | yes |
| Interested | Catherine | 2032-09-16 | 1216 | 9 | 2038 | 0 | pass | yes |
| Interested | **Garrido** | 2035-07-12 | 188 | **2** | 2039 | 12 600 × (11 − L) | pass if his other factors are larger (small penalty) | yes (inference) |
| Interested | Korshun | 2034-07-10 | 555 | 1 | 2038 (on loan) | 0 | pass | yes |
| Doubtful only | Lafleur | 2035-07-06 | 194 | 12 | 2039 | 68 400 × (11 − L) | transfer fails, loan may pass | yes (loan not checked) |
| Doubtful only | Tomašević | 2036-01-10 | 6 | 11 | 2039 | 269 500 × (11 − L) | transfer fails, loan may pass | yes (loan not checked) |
| fail | Gamboa | 2036-01-14 | 2 | 13 | 2038 | 323 700 | fail | yes |
| fail | Guibert | 2035-07-30 | 170 | 16 | 2038 | 129 600 | fail | yes |
| fail | Ortega | 2035-07-26 | 174 | 15 | 2039 | 115 500 | fail | yes |
| fail | **Barthes** | 2035-08-16 | 153 | 9 | 2037-06-30 (530 d) | **88 200** | fail | yes |
| fail | Sonko | 2035-08-05 | 164 | 7 | 2038-12-31 | 60 900 | fail | yes |
| fail | Molefe | 2035-07-09 | 191 | 9 | 2038 | 54 000 | fail | yes |
| fail | Andreani | 2035-06-26 | 204 | 10 | 2039 | 47 000 | fail | yes |
| fail | **Marco** | 2031-01-09 | 1832 | 10 | 2036-06-30 | 0 | pass | **no** |

**22 of 23 consistent; 7 of 8 failures explained; Barthes vs Cissokho explained.**

Note for the loan rule: Lafleur and Tomašević pass it, but the 6 other recent joiners fail it too. Why is not resolved. The candidates are the loan-path condition `vt+0x290` and the loan rule's extra checks (`0x20b89f4`, `0x209f708`, `0x2f9f504`).

Wider check (bytes, `candgate.py`) on the earlier 125-candidate list (2036-01-16). Of the 31 candidates with P > 0, 3 are listed: Tomašević, Lafleur and Garrido. The list mixes in the knowledge condition, so it cannot test the gate beyond this.

## 4. Unknown / hypothesis
- **Marco** (Coruña, playing for Coruña B, B Team Regular, contract to 2036-06-30, value £14 740). Candidates:
  - (c) a transfer-manager record with status 0xff, reason 53;
  - (h) a contract-ring event from `0x12873d8` (reason 55 "requires guaranteed playing time" or reason 18);
  - a hard −100001 from the playing-time factor `0x128d22c → 0x12ad438` (the other agent's factor work);
  - the eligibility pre-check `0x209efe8`;
  - the club rivalry check `0x14c77a4` (needs Coruña's club relationships).

  What is missing: the event ring at contract `+0x30` (10 slots: date `+4`, u16 `+0x16`, i8 `+0x18`, type `+0x19`). Its save location is not mapped; the core's `u32 n × 29-byte` items are a candidate (Marco has 0). Also missing: the manager behind `0x5548728`.
- **E and L**: not reproduced (§1.2). They change P only by the factor (11 − L) and do not change which side of the gate a player falls.
- The identity of `ctx+0x18` / `ctx+0x20`, and Metz's `club+0x16a`.
- The 0x3c2e8fc relegation exemption and contract `vt+0x128` / `+0x57` bit 3: not evaluated. The ground truth suggests they do not apply to these players.
- Loan pass/fail for the recent joiners.

## 5. Proposal for landing it
- **savefile**:
  - Expose the ACTUAL_PERSON joined-club date as `Player.JoinedClubPerson`, the 2nd date before the contracts marker (at q−5; `PersonDates` in the scratch export). It equals `JoinedClub` whenever both are set, and it is set when `JoinedClub` is 1900 (Cipriano 2023, Masson 2023).
  - Expose the FULL header contract-type byte as `Contract.Kind`.
  - Expose the non-player block (`NonPlayerAbility`) for the human record, to fix v.
- **fm**: add `transferBlock(p, human)`, returning `just_joined` when P(L=10) > 0 under the rule in §2.3. Use Loyalty = `Hidden[2]`, days in the game's 365-day arithmetic, the 61-day / 300-day expiry exemption and the 100-day renewal rule.
- **MCP**: on `get_player`, `interest_blockers: ["only_just_joined"]`, worded "He has only just joined his club (N days ago) and would not consider a transfer now (FM: 'only just joined his new club')". Do not claim a full interest level.
- **Integration test** (kz-2036-01-16):
  - Barthes (uid 2002112631): joined 2035-08-16, days 153, Loyalty 9, P(L=10) = 88 200, blocked.
  - Cissokho (2002099764): joined 2030-07-15, not blocked.
  - Nešić (2002114070): days 157, contract ends 2036-06-30, exempt.
  - Gamboa (2002115333): days 2, P = 323 700.
  - Garrido (2002116366): P = 12 600.
  - Tomašević (2002125884): days 6.

## Follow-up: Marco — explained (verified: code + bytes)

**Where he plays and is contracted.** He plays for team 1583 "Coruña B", which is its own CLUB record (a B club), first-team type 0. His main contract is with team 1296, Coruña's first team (rep 7401, division index 51, La Liga). Contract: B Team Regular, ends 2036-06-30, joined 2031-01-09. Bytes, repo parser. The earlier table's "Coruña, rep 7401" is the contract club.

**"Trn" = Transfer Arranged.** In `example.ltf`, KEY-661918 "Trn" carries the comment "Transfer Arranged, short string". KEY-661919 is "Transfer arranged" and KEY-661917 is "Joining [team] on [date]" (the transfer-arranged string on the profile panel). The neighbouring flags are Frt (leaving on a free), Bid, Enq and Dft.

**The gate**: `0x1283a90`, at `0x1283eec`, calls `0x2f54e10(person, 0, 0, 0, 0, 0)`. When that returns true, the interest is **−100000 with reason 10**: KEY-637332 / 628529 "has already arranged a future transfer". This happens before the kind dispatch, so the transfer path and the loan path both fail, which matches "fails even Transfer or loan · Doubtful".

`0x2f54e10` returns true in two cases:
- Some id in the person's **ACTUAL_PERSON +0x68** vector indexes a TRANSFER object (global table `[0x5526ca8]+8`) whose status **+0x8d == 0x13 (19)**. With these arguments there are no further conditions.
- Otherwise: the person's `+0x80` (future-contract) object passes vt+0x98, has `+0xb0 ≥ 0` and `+0xef == 100`.

**Save structures** (all verified by the writer code and by bytes):
- **Person side.** The ACTUAL_PERSON writer `0x2f424a0` writes `u8 n + n × u32` from `+0x68`, the list format.md shows before the contracts marker (`…, date +0xe8, date +0xec, u8 +0x10b, u8 n + n × u32 (+0x68), u32 +0xb0, u8 5`). The u32s are TRANSFER ids. Marco: n = 2, ids **10183, 10810**.
  - Caution for anyone reusing this: a scan for n that does not check the dates gives a false n = 0 for Marco. The repo's `validDates` check finds n = 2.
  - Across the save: 10 397 players have a non-empty list, and 9 015 of 9 425 parsed TRANSFER records are linked from their person.
- **`transfer_man.dat` TRANSFER record** (writer `0x3a53bb4`; loader `0x3a53fec`). Offsets from the record start:

| Off | Field | Marco, id 10183 | Marco, id 10810 |
|---|---|---|---|
| 0 | u8 class (2 or 3), u16 version 0x13 | 02 | 03 |
| 3 | ref: person (u8 0 + i32 at 3..6) | 184193 | 184193 |
| 7 | ref: team (u8 + i32) | 855 **SS Calcio Bari** | 649 En Avant Guingamp |
| 12 | ref: team | 855 | 649 |
| 17 | i32 `+0x58` | 0 | 0 |
| 21 | i32 `+0x5c` = id (the value the person list holds) | 10183 | 10810 |
| 25 | i32 `+0x60` | 357070 | 357738 |
| **29** | **u8 `+0x8d` status** | **19** | 15 |
| 30 | i32 `+0x64` | −1 | −1 |
| 34 / 35 | u8 `+0x84` / `+0x85` | 2 / 1 | 3 / 1 |
| 36 | 8 bytes `+0x30` | | |
| 44, 45, 46 | u8 `+0x86`, `+0x87`, `+0x8b` | | |
| 47 | i32 `+0x68` | | |
| 51 | date `+0x6c` | 1900 | 2036-01-21 |
| 55 | u8 `+0x88` | | |
| 56 | date `+0x78` | 2036-02-04 | 2036-02-04 |
| 60 | date `+0x7c` | **2036-07-01** | 2036-07-01 |
| 64 | date `+0x80`, … (more fields, not decoded) | | |

**Status values** (9 425 class-2 records, 2036-01-16 save; meanings are inference):
- 20 (4 608): completed. The target team equals the player's current team in 4 201 of 4 245 player records.
- 19 (422): arranged / pending. The target is not the current team in 419 of 422, and 418 are linked from their person.
- 15 (3 640): likely rejected or collapsed.
- Others: 8 (465), 2 (180), 0 (63), 16, 10, 3, 18.

The FutureContract slot is empty for every status-19 player, so an arranged move is stored only in `transfer_man.dat`. Marco's status-19 deal goes to Bari on 2036-07-01, the day after his contract ends. That fits a pre-contract (Bosman) agreement (inference) and matches the game's "Trn".

**Controls** (bytes): no other player in the 23 has a linked status-19 transfer.
- Gamboa: ids 10895 (status 15) and 11340 (status 20 → Castellón, his current club).
- Tomašević: 9761 (class 3, status 2) and 9800 (status 20 → AZ, his current club).
- Korshun: 11037 (class 3, status 20 → Nacional, his loan club).
- The other 20 have an empty list.

So the gate fires for Marco only, and **all 23 labels are explained**:
- 13 Interested: no gate fires (Garrido pays only a small just-joined penalty).
- 2 Doubtful only: just-joined penalty on the transfer rule, pass through the loan rule.
- 7 fail: just-joined penalty.
- Marco fails: arranged transfer.

Candidates that did not need decoding: the contract ring at `+0x30`, the −100001 in `0x12ad438`, the eligibility pre-check `0x209efe8`, and the B-team / same-club dispatch. The gate fires before any of them.

**Landing.**
- savefile: `Player.TransferIDs` (the +0x68 list).
- A `transfer_man.dat` reader for the TRANSFER records (class 2/3 header, person, team, id, status, date `+0x7c`), with `Player.ArrangedTransfer` = {team, date} when a linked record has status 19.
- fm / MCP: `arranged_transfer: {to: "Bari", date: "2036-07-01"}`, plus interest blocker `transfer_arranged` ("He has already agreed a move to Bari on 1 July 2036").
- Integration test (kz-2036-01-16): Marco (uid 2002115699, person 184193) → ids [10183, 10810], 10183 status 19, team 855, date 2036-07-01. Gamboa and Tomašević have no status-19 record.

## Validation 2 — DC, "Transfer or loan · Persuadable" (2036-01-16, blind test)

Scripts: `player-interest-gates/v2eval.py` (frozen rules), `v2_41.json` (the 41 names → uids), `v2_41_eval.json` (per-player results), `v2_raw.jsonl` (save inputs for every player born 2009–2013), `tm_recs2.json` (12 015 transfer records, classes 2 and 3).

The 41 in-game names were matched to save uids one to one. Two needed choosing: "Luigi" (Atromitos) = Luigi Lazzaretti, uid 2002125562; Jesús Blanco = the OGC Nice player, uid 2002098872.

### Rules scored (frozen before looking at the 41 labels)
1. **Arranged transfer**: a linked TRANSFER record with status 19 → fail, for transfer and loan alike.
2. **Just-joined penalty P on the transfer rule**, rule §2.3, with L = 10 (its minimum).
   - Exemptions as decoded: contract type 3; clause 55/56; joined ≥ 61 days ago and contract ending within 300 days.
   - The clause 55/56 exemption was in the decoded rule but missing from my first scorer. I added it **before** scoring the 41; it was found on Nonato in the 24-name list, not on the labels.
   - The original-PT / `+0x57` condition (`128f9c0…128fb14`) was also checked; it changes no player here.
   - Transfer counted as blocked when P(L=10) ≥ 40 000. This cut sits between the smallest failing P (47 000) and the largest passing P (12 600) in validation 1.
3. **Loan rule as decoded**: interest at kind 3, level 7. The just-joined penalty applies only when LEVEL[b] ≥ 5 and service `vt+0x290` (`0x129fcf8`) is true. I read `vt+0x290` as "the player's nationality equals his current club's nation" (or a tie item, or flags bit 2). The penalty is then × (11 − 7) = × 4.
4. Pass when the transfer is not blocked, or when the loan is not blocked.
5. Reason 2 (club gap) is reported but does not gate. Threshold: Persuadable, i.e. 10 000 + shift, with the shift probably 0 (§1.4).

### Confusion table (before any refit)

| | in game pass (24) | in game fail (17) |
|---|---|---|
| predicted pass | 23 | **12** |
| predicted fail | **1** (Lotito) | 5 (Sueur, Rougier, Cichy, Harmanus, Witkowski) |

28 of 41 correct.
- Cichy (PSV, "Trn") is caught by the arranged-transfer gate: status-19 record 9801 → team 761, 2036-07-01.
- Sueur, Rougier, Harmanus and Witkowski are caught by the just-joined penalty (Rougier through the "just signed 1 day ago" branch).

| Miss | Rules said | Inputs (bytes) | Likely cause |
|---|---|---|---|
| Lotito (Crotone), predicted fail, passes | transfer P = 93 600; loan P = 374 400 (Italian at an Italian club) | joined 2035-07-21, Loyalty 13, B 4, score −3, desired level m = 7 | The **loan nationality reading is wrong**. The loan passes because he is not playing (m = 7 = the offered loan level) |
| Adetunji (Ajax), Ejike (Santander), Folch (Lille), Diédhiou (Vigo), Fiaschetti (Mainz), Souaré (Sharjah) | transfer blocked by P = 46 800…354 000; loan let through because they are "foreign" | all regular starters: B 3, scores 72–96, m 8–9 | The loan offers level 7 < m, so playing time is negative (d = 7 − m < 0 → −CR and more, from the factor table). The loan fails Persuadable (threshold/2 = 5 000) |
| Blanco (Nice) | transfer P = 99 000 ("just signed" 2 days ago, contract to 2040); loan P = 0 | B 3, score 80, m 8; reason 2 = −10 240 | Same loan playing-time cause |
| Ahovey (Marseille) | no gate | B 7, score 71, m 8; **reason 2 = −17 120** (rep 8074) | Transfer: reason 2 alone is −17 120 against a +10 000 threshold. Loan: m 8 > 7 |
| **Vilas** (Gladbach) | transfer P = 347 200; loan let through (foreign) | joined 2036-01-13; **ACTUAL_PERSON `+0xd8` = club 474 = FC Metz** | New gate (below): reason 100, "has only recently left the club", −100 000, transfer and loan |
| **Lau Jiménez** (Strasbourg) | no gate (settled; reason 2 −476) | **Strasbourg's club vector holds Metz (474) as type 5 rival, value 90**; Metz holds Strasbourg at 85 (bytes) | Rival club: the filter's final check `0x14c77a4` (same nation, rivalry ≥ 90 → drop; §1.3) and/or reason 32 (`0x128fc68`). Lau has no favoured-club item for Strasbourg |
| Heed (Sparta) | no gate; reason 2 +3 676 | **B 1 Star Player, score 100, m = 10** | Not explained. Inference: playing time — he wants Star Player; at the loan level 7 that is strongly negative, and on a transfer it depends on L |
| David Silva (free agent) | free-agent path not modelled | — | Not explained. The free-agent path `0x1284858` is not decoded |

### Item 1 of the first follow-up (the 24 that pass): answer under the frozen rules
- No player in the 24 has a status-19 transfer: verified, bytes; their transfer-id lists hold no status-19 record.
- Five of the 24 joined recently:
  - Vega: P = 33 500, below the cut; "grey".
  - Nonato: exempt by clause 55.
  - Lotito: P = 93 600 — **miss**.
  - van Hattum and Musongi: free agents.
- Exempt by "ends within 300 days": Jelavić, Hedeager, Israelyan, Nødgaard, Luigi. Exempt by clause 55: Guignard, Fraile, Nonato.
- Loan rule (kind 3) as decoded (`0x128600c` → `0x128f550`):
  - the penalty runs only when LEVEL[b] ≥ 5 (`0x32715dc`);
  - for kind 3, `0x128f550` also needs service `vt+0x290` = `0x129fcf8`: true for a player of the current club's nationality, else only through a person relationship item (kind 2, type 9) or flags bit 2;
  - then the 12ae024 exemptions;
  - age ≤ 25 uses the "just joined" branch with L = 7, so the penalty is × 4.
  - **This reading is contradicted by Lotito (validation 2) and by Molefe and Sonko (validation 1).**
  - The factors agent read the same function as never reaching the penalty for kinds 3/8. The refit below agrees with that.

### Base set built from the save (before the 41 arrived)
- Men, aged 24–25, DC ≥ 15, not Metz: 2 788. With knowledge ≥ 30 (stand-in for "Extensive"): 583, of which 263 have an EU primary nationality. Second nationalities are not decoded, which is why Vega (URU), Nonato (BRA) and others could not be selected from the save.
- The frozen rules predicted 68 of the 583 to fail: 4 arranged transfers (Cichy, Pastoor, Marco, Tait) and 64 just-joined.
- In game, the base set has 41 players (the knowledge rule is not reproducible from the save). Within those 41 the frozen-rule result is the table above.
- Full list: `player-interest-gates/v2_base.json`.

### Refit (labelled: fitted after seeing the 41 labels; validation 1 used only as a check)

New gate, **reason 100 "re-joining a club he only recently left"** (`0x128e658`, `0x128ea88…0x128eae4`; verified, code). It is called in both the transfer path and the loan path. It gives −100 000 when:
- ACTUAL_PERSON `+0xd8` (the ref after the first u16 of the ACTUAL_PERSON part; the previous club, a CLUB index) is the target club;
- the current club is not the target;
- `0x2f43398(person)` ≤ 150;
- and `0x12ade1c` is false.

Bytes: Vilas `+0xd8` = 474 (FC Metz), joined Gladbach 3 days ago. Controls: Barthes 491, Fiaschetti 498, Lotito 677, Lau 1122, Ahovey none. The meaning of `0x2f43398` (days at the current club?) is inference.

R1 = the frozen gates plus three changes:
1. **The loan rule has no just-joined penalty.** It passes iff the desired level m = max(LEVEL[b], scoreLevel(score)) ≤ 7, the level the loan rule offers. Inference from the playing-time factor: d = 7 − m.
2. Transfer also blocked when reason 2 ≤ −5 000. This is the cut proposed in Follow-up 1, before either validation set.
3. The rival gate (Strasbourg → Metz 90) and the reason-100 gate.

| Set | R1 correct | Misses |
|---|---|---|
| Validation 2 (Persuadable) | **39 / 41** | Heed (Star Player, m 10; inference: playing time), David Silva (free-agent path) |
| Validation 1 (Doubtful), check only | **22 / 23** | Barthes: m = 7, so R1 lets the loan through, but he fails. Lotito (V2) has m = 7 too and passes. Not resolved; differences: score 38 vs −3, days 153 vs 179, Loyalty 9 vs 13 |

R1's loan rule is a fit on validation 2. It needs a third labelled set before landing. The rival and reason-100 gates are decoded (code) and need no fit.

## Follow-up: loan path, transfer-only labels, free agents (code-derived; no fitting)

Scripts: `player-interest-gates/code_rules.py` (the decoded rules), `score_c.py` (scoring), `emu/ptemu.py` (Unicorn emulator of `0x12ad438`), `f_128600c.s`, `f_129fcf8.s`, `f_1294*.s`, `f_1284858.s`.

### 1. Where the earlier loan reading went wrong
- **The nationality condition was read correctly** (verified, code). In the kind-3 branch of `0x128f550` (`0x128f680`), the penalty needs service `vt+0x290` = `0x129fcf8(person, current club, 0)`. That returns 1 when ACTUAL_PERSON `+0x38` (nationality) equals `0x3098f54(club)`, the club's nation (`+0xe8`, else `+0xd8`, with uid special cases). It also returns 1 through a person relationship item of kind 2, type 9, or through trailer flags bit 2 plus further checks.
  - The penalty also needs `0x39410f4(club, person, 0)` to be false. That function returns 1 when contract `+0x57` bit 0 is set and `+0x58 == 7` (core bytes 23/24, saved), or when the club's `+0x108` object lists the player (in-memory).
  - It also needs `0x2f71a5c` to find no item of type 25 or 51 in the person's `+0xa8` list (not saved by the ACTUAL_PERSON writer; in-memory).
  - The loan path calls `0x128f550` only when `LEVEL[b] ≥ 5` (`0x32715dc`), at level 7, so the penalty is × 4.
- **The missing piece was the ring-item exemption in `0x12ae024`** (verified, code; it applies to transfer and loan). The function returns 0 (no penalty) when two things hold:
  - the contract ring holds an *open* item (date `+0x4` = 1900-01-01) with status ≥ −2, an id not in 900–930 / {0, 1100, 1104, 1106} (`0x3755478`), and a type other than 8, 37, and (when status ≠ −2) 19/20;
  - and **no** open item has `u14` ∈ {2, 18, 34, 39, 42, 60, 238–240}.

  Ring items are the contract core's `u32 n × 29 bytes` (layout in player-interest-factors.md §5).
  - **Lotito has an open type-12 item** (id 650, status 3, dated 2036-01-11) and an open type-19 item, so he is exempt on both transfer and loan.
  - Garrido (validation 1) has an open type-12 item too, which explains his small penalty: it is in fact 0.
  - My earlier scorer ignored ring items.
- **What decides the loan is the playing-time factor at the loan level**, not a nationality rule. `0x12ad438` with L = 7 and kind 3 floors the desired level at 6 for players over 20 (`0x12adaa0`). It then gives d = 7 − max(m, 6):
  - d = 0 → 0;
  - d < 0 → d·CR − 5·(Rt − CR) − (10·(Rt − Rcur) when Rt < Rcur), capped at 0;
  - d > 0 → max(0, 5·(Rt − CR) + d·CR).

  Here m = max(LEVEL[b], scoreLevel), capped at 8 by some open ring items; 4 → 7 and 6 → 6/7 for keepers. Rt is Metz's first-team reputation (7218), CR the player's current reputation, Rcur his club's first-team reputation. So the refit R1 ("loan passes iff m ≤ 7") follows from code: a regular starter (m = 8–9) loses 9 000–23 000 on a loan offer.
- **Emulation** (Unicorn, real arm64 code of `0x12ad438` on save values; `emu/ptemu.py`). Values for L = 7 kind 3 / L = 10 kind 4 / L = 8 kind 4:

  | Player | L 7 (loan) | L 10 | L 8 |
  |---|---|---|---|
  | Lotito | **0** | 26 636 | 17 182 |
  | Adetunji | **−12 185** | 4 690 | −4 690 |
  | Folch | **−4 234** | 0 | 0 |
  | Ejike | **−11 526** | 17 667 | 0 |
  | Vega | −18 435 | 12 550 | −12 550 |
  | Heed | −22 892 | 0 | −16 293 |
  | Lafleur | 0 | 23 748 | 11 406 |

  Every value equals the hand derivation in `code_rules.pt_value`. Emulating the whole kind-3 path `0x128600c` was **not feasible**: it calls about 35 factors plus in-memory managers (`0x39410f4`'s club list, the person `+0xa8` list, the loan-count gate in `0x1294f2c`, scouting and negotiation records). Those parts are stubbed out or left as unknowns.
- **Loan-only factors** (verified, code):

  | Fn | Reason | Effect |
  |---|---|---|
  | `0x1294b88` | 139 | +2·CR for age ≤ 23 when the clubs' playing styles match; 0 for these ages |
  | `0x1294d40` | 28 | only with contract `+0x57` bit 1; 0 for all checked |
  | `0x1294f2c` | — | a hard −100 000 from a loan registration/count test (`0x1bf9454`, `0x19c4b28`, `0x1bf9868`); in-memory |

  The loan rule `0x16949bc` passes when interest(kind 3, level 7) ≥ threshold/2, then applies its extra checks.

### 2. Code-derived model C and blind scores
Model C:
- **transfer** = −P_T(L = 10) + reason 2 + playing time(L = 10);
- **loan** = −P_L + playing time(L = 7) + reason-2 w term.

Gates:
- arranged transfer (status 19);
- reason 100 (ACTUAL_PERSON `+0xd8` = Metz).

Thresholds: Doubtful 0 / 0 (transfer / loan); Persuadable 10 000 / 5 000. A player passes when the transfer or the loan reaches its threshold. Every factor not computed counts as 0. L = 10 is the best case for the transfer (E unknown). Free agents are not modelled.

| Set | Correct | Misses |
|---|---|---|
| V1, transfer or loan · Doubtful | **22 / 22** (+ Musongi not scored) | — |
| V2, transfer or loan · Persuadable | **36 / 38** (3 free agents not scored) | Vega (loan sum −18 435, yet he passes by loan only), Lau Jiménez (transfer sum +15 232, yet he fails) |
| V2, transfer only (th 0 = th 10 000 in game) | **36 / 38** at th 0; **37 / 38** at th 10 000 | Heed (at th 0), Lau |

The 10 transfer failures in the coordinator's list:
- **Adetunji, Ejike, Folch, Diédhiou, Fiaschetti, Souaré and Blanco** are blocked by the just-joined penalty. Their P_T(L = 10) is 46 800, 104 000, 108 000, 62 400, 354 000, 73 000 and 99 000. Blanco's comes from the "just signed" branch (renewed 2 days ago, contract > 300 days left). These were already predicted to fail the transfer in the blind test; the earlier misses were on the loan side.
- **Ahovey**: no penalty, reason 2 = −17 120, playing time at best +16 728, sum −392. He fails even at the best L.
- **Heed** is a Star Player (b 1, score 100, m = 10), so playing time is 0 at L = 10 and −16 293 at L = 8. His sum is +3 676 at best. He fails whenever L ≤ 9 (Metz's squad ranking E ≤ 6), which is likely for a CA 131 player at Metz; E is not reproduced.
- **David Silva**: free agent (below).
- **Lotito passes the transfer** because of the ring exemption: P_T = 0, sum +33 972.
- Molefe and Sonko (V1): P_T = 54 000 / 60 900 and loan playing time −20 265 / −10 502, so they fail both.
- Bimodality: the large terms are just-joined (≥ 47 000), arranged transfer (−100 000) and reason 100 (−100 000). Every other computed factor is within ±25 000. That fits "nobody between Doubtful and Persuadable" (inference).

### 3. Reason 2 ≤ −5 000 replaced by the sum
Best-case transfer sums (L = 10), from the components above:

| Player | Reason 2 | P_T | Playing time | Sum |
|---|---|---|---|---|
| Ahovey | −17 120 | 0 | +16 728 | **−392** |
| Blanco | −10 240 | 99 000 | +15 591 | **−93 649** |
| Folch | −18 220 | 108 000 | 0 | **−126 220** |
| Adetunji | −568 | 46 800 | +4 690 | **−42 678** |
| Fiaschetti | +4 152 | 354 000 | +17 556 | **−332 292** |

Only Ahovey needs reason 2 to fail, and his sum is below 0 even at L = 10. The separate cut is not needed.

### 4. Free-agent path `0x1284858` (from `0x1284384`)
Gates in order (verified, code):
1. youth-trial contract with L ≤ 3 → −100 000 (129);
2. flags bit 15 → −100 000 (7);
3. `0x1286a48`;
4. a draft object at ACTUAL_PLAYER `+0xe0` (byte `+0x36` ∈ {11, 23}) → −100 000 (23, "waiting for the draft");
5. the target club's relationship ≤ −50 → −100 000 (26).

Then the same factor family as the transfer path, plus `0x1293580`, `0x12937ac` and `0x1293834` (not decoded). There is no contract, so `0x12ad438` takes its no-contract branch: the desired level is 7 when the player's world reputation < 0.95 × Metz's (6857), which holds for all three free agents.

Silva (CA 80, PA 117, WR 4018), Musongi (CA 90, PA 140, WR 3745) and van Hattum (CA 84, PA 93, WR 2019) are **not separated** by any decoded input. Not resolved; the free-agent-only factors and L (E) are the open inputs.

### 5. What stays unknown
- E and L (squad ranking `0x2f9e580`, staff assessment `0x36b811c`): they decide Heed and set the playing-time terms of every transfer.
- In-memory loan exemptions (`0x39410f4`'s club `+0x108` list; the person `+0xa8` list, types 25/51) and the loan-count gate `0x1294f2c`.
- About 25 factors not computed (talented group, continental football — up to ±50 000 and needs COMP reputation `+0x188` —, family finance, scouting, dynamics, board).
- **Vega** (loan pass despite −18 435) and **Lau Jiménez** (transfer fail despite +15 232). Strasbourg rates Metz a rival at 90, but by code that matters only when the player has a favoured-club (type 3) item for Strasbourg, or Metz rates Strasbourg > 89 (it rates 85), or he is a liked person of Strasbourg's. None holds (bytes). His type-106 item with Strasbourg (66) is read by no factor found.
- David Silva.

## Validation 3 — ST, "Transfer only · Persuadable", held out (2036-01-16)

The scorer was frozen in `frozen-v3/` (checksums verified) and run unchanged through the harness `run_v3.py`. Predictions are in `predictions-v3.tsv`. The coordinator scored them blind: **29 / 31 correct; all 17 "confident" predictions correct.** The two misses, Jan Stáňa and Oscar Maxsø, were both flagged uncertain ("near threshold; uncomputed factors").

### (a) Why their playing time is 0 at L = 10 (code + bytes)
- Both are **Star Players** at their clubs: Stáňa b = 1, score 100; Maxsø b = 1, score 99. So the desired level m = max(LEVEL[1] = 10, scoreLevel = 10) = 10.
- `0x12ad438` gives d = L − m = 0 at the best-case L = 10 and returns 0 on the d = 0 branch (`0x12adcc0`, `mov x0, x24` with x24 = d·CR = 0). Any L < 10 is negative.
- Their sums are therefore reason 2 alone: +3 132 and +9 106. Heed (validation 2: Star Player, sum +3 676, **fails**) has exactly the same structure. Without some extra factor the frozen model cannot separate Stáňa and Maxsø (pass) from Heed (fail).

### (b) "Wnt"
- `example.ltf` KEY-661937: "Wnt — Wanted by a club, short string". The neighbouring flags are Trn (transfer arranged), Frt, Bid (under transfer bid) and Enq (enquiry).
- **Where it is in the save** (bytes; inference about meaning): `transfer_man.dat` holds 14-byte entries `u16 6, i32 person index, i32 x, date added (or 1900-01-01), u32 0`. Here x is 89 000–262 000, possibly a valuation in GBP (inference).
  - Every Wnt player in the three sets has at least one such entry: 16 / 16 (V3: Stáňa 2, Maxsø 2, Zbiciak 2, Anselmi 2, Nguyen 1; V1/V2: Lotito 5, Jelavić 8, van Hattum 4, Sall 1, Lafleur 1, Sonko 1).
  - Of 40 non-Wnt players checked, 38 have none. El Hariri (who shows "Int") and Togola have one each.
  - The entries are **not** TRANSFER records (none of these players has a linked TRANSFER record, apart from van Hattum's old status-15 one). They look like another club's transfer-target list (inference).
- **Factor tied to it: not found in the decoded code.**
  - The declared-interest factor `0x1288d14` (reasons 88–93) reads the *target* club's (Metz's) interest record (`[club+0x108]+0x148`; last item, person `+0xc`, count `+0x10`). Its value is about (n − 3) × CR/10, CR/10, 0.15·CR or CR/4 × a decay of 0.1–1, so at most about 1 600 for CR ≈ 6 500. That would cover Maxsø's gap (+894) but not Stáňa's (+6 868). It is also Metz's interest, not another club's.
  - Manager keenness (reason 132) needs an open negotiation by Metz's manager (none decoded).
  - The 3-key records behind `0x1286a48` (manager `0x5548728`, list `+0x8d0`: target club, current club, person, date `+0x20`, status `+0x29`) give ±100 000. For a status-based +100 000 at L ≥ 9 the record would have to name Metz as the target, which a "wanted by another club" entry would not (inference).
- **Across the Wnt players the frozen rules are never contradicted by a Wnt bonus:**
  - Lotito, Sall, Jelavić, Zbiciak, Anselmi and Nguyen pass with margin.
  - Lafleur and Sonko are dominated by the just-joined penalty (68 400 / 60 900).
  - van Hattum is a free agent.

  The data are consistent with a Wnt-linked bonus of at least +6 900 (from Stáňa) that the decoded factors do not contain (inference). Which code reads the kind-6 entries is not traced; candidates are `0x129260c` (partly read) and `0x12937ac`/`0x1293834` (free-agent path).

### (c) Bayal — reason-100 gate confirmed (code + bytes)
- ACTUAL_PERSON `+0xd8` = club 474 (FC Metz).
- He joined US Thionville Lusitanos on 2035-09-03: `+0xec` = 2035 day 246, TRANSFER 28978, status 20, team 29065.
- `0x2f43398` is days since `+0xec` (verified, code) = 135, which is ≤ 150.
- So `0x128e658` gives −100 000 (reason 100, "recently left the club"), unless `0x12ade1c` is true (not decoded). The game shows FAIL.
- The frozen scorer predicted FAIL through the just-joined penalty (P = 68 674 + …). The reason-100 gate would give the same result.

## Follow-up: Wnt — the "wanted by a club" data, and what is still missing

### Save structure (verified, bytes; meaning partly inference)
`transfer_man.dat` holds **1 243 transfer-target lists**, each `u32 n` followed by n × 18-byte entries:

```
u16 6 | i32 person index | i32 value (89 000–480 000; inference: a valuation in GBP) | date added (1900-01-01 = undated) | u32 0
```

- The list sits inside a per-team object whose header is `u16 0x0b, u16 1 or 0x10, i32 TEAM index`. The owner is found in 615 of the 1 243 lists; the other headers differ and are not decoded.
- Samples: Cagliari (team 859) lists Stáňa (value 135 630); FC Utrecht (773) lists Maxsø (103 560, added 2036-01-08); Cosenza (869) lists Lotito.
- FC Metz (652) has one list.
- The transfer_man save body (`0x3a17b8c`) walks the person table (`[0x5548790]`) and calls person vtable `+0x720` for each person (`0x3a17fc0`). These per-team objects are probably the AI managers' or teams' target lists (inference).

Owners per Wnt player (team index, value):

| Player | Owners |
|---|---|
| Stáňa | 859 Cagliari, 915 |
| Maxsø | 773 Utrecht, 6221 |
| Zbiciak | 708, 1295 |
| Anselmi | 1687, 1847 |
| Nguyen | 1847 |
| Lotito | 869, 898, 1667, 17161, 37372 |
| Sall | 669 |
| Lafleur | 772 |
| Sonko | 1435 |
| Jelavić | 546, 1847 |
| van Hattum | 1824, 1844, 1849, 4948 |

- Every Wnt player in the three sets is on at least one list: 16 / 16 counting each player once per set.
- Of 40 non-Wnt players checked, 38 are on none. El Hariri ("Int", team 704) and Togola (1485) are each on one list, both added 2036-01-15, the day before the game date. Inference: the status may need a day to appear.
- Heed is on none.

### Code — reader not found (honest result)
I did not find the code that reads these lists into the interest computation. What I checked:
- `0x1288d14` (reasons 88–93) reads only the **target** club's (Metz's) interest record (`[club+0x108]+0x148`). Its value is at most about CR/4 × decay, about 1 600. It is not another club's list.
- `0x129260c` (reasons 51/52/101/104; verified, code) can **raise the score to a floor**. It gets person events from `0x2f72cd8(person, 2)`, types 29/53/223/224/225/227, each tied to the target team by club ref or id. Events of type 29 or 53 give score = max(score + CR/2 (+ Loyalty×100), 5 000). Events of type 223/224/227 set a floor (0, `0x2fced68`, or 50 000).
  - When a floor below 10 000 is set **and** the person's `+0xa8` list holds an item of type 57 or 248 (`0x2f71a5c`), the floor becomes 10 000, and the score is raised to it (reason pushed).
  - Lifting Stáňa (+3 132) and Maxsø (+9 106) to exactly 10 000 would make them pass both Persuadable and Doubtful, as seen. Heed (no Wnt) would not be lifted.
  - Both the event list and the `+0xa8` list are **in-memory**: not written by the ACTUAL_PERSON writer and not mapped to a save file. So I cannot confirm that a target-list entry creates a type-57/248 item. This is a hypothesis.
- Manager keenness (132) and the `0x1286a48` records need Metz-side records. The playing-time loop `0x2ff2964` reads only Metz's negotiation list (`+0x370 → +0xce8`).

### Bonus for Stáňa, Maxsø, Heed
Not computable from decoded inputs. The frozen sums are +3 132, +9 106 and +3 676. A Wnt-linked lift of at least +6 900 (Stáňa) and at least +900 (Maxsø) is needed, and none for Heed.
- Checked against all Wnt players under the frozen rules, a Wnt lift to a 10 000 floor changes no other prediction:
  - Lotito, Sall, Jelavić, Zbiciak, Anselmi and Nguyen already pass;
  - Lafleur (transfer −43 108) and Sonko (−43 329) stay below 10 000 on transfer unless the floor overrides the just-joined penalty (the floor is applied after all factors, so it would). Lafleur passes Doubtful only through the loan, and Sonko fails Doubtful in game. So **a floor that overrides penalties would be contradicted by Sonko**, unless it needs the Metz-related event as well (inference);
  - van Hattum is not modelled.
- Not checked: whether the floor needs a Metz scouting or interest event (types 223/224/227). Metz has scouted all of these players ("Extensive knowledge"), so such events may exist for many of them.

### Next step that would settle it
Trace the loader of the per-team target lists: the person vtable `+0x720` read counterpart, or the class whose writer emits `u16 6` per entry. Then trace who appends to the person `+0xa8` list, and which item type it gets. That needs a vtable/RTTI walk for the per-team object, or runtime capture in the game.
