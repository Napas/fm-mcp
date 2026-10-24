# player-interest — FM26 "Player's interest" (would a player join the human's club?)

Recon only. Binary: `game_plugin` 26.3.2 arm64 slice (`research/tactics/gp_arm64`), read with llvm-objdump
(`role-weights/xref/fn.py`), a MOVZ/MOVK immediate scanner (`player-interest/imm`), a full BL edge list
(`player-interest/bl/edges.txt`) and the existing ADRP xref list (`role-weights/xref/refs.txt`). Save probe:
`player-interest/mod/cmd/piprobe` (repo copy) on `data/kz-2036-01-16.fm`. Disassembly kept as
`player-interest/f_*.s` and `player-interest/fac_*.s` (one file per interest factor).

## Verdict

**Computed live by game code, not stored per player.** The search filter calls the interest
computation directly for each candidate (verified, code). The computation adds up about 45 factor
functions. Each one reads club, player, contract, history and staff state. Some inputs are decoded
save fields, many are undecoded but probably saved, and some are derived in memory (squad-ability
rankings, expected playing time). **An exact reproduction is not feasible in a reasonable effort.**
**An approximation is possible.** The factor behind the ground-truth sentence (club reputation, reason 2)
reads mostly decoded inputs. The remaining work for a credible approximation is about 1–2 weeks of RE
plus in-game sampling. An exact match is open-ended: all ~45 factors, ~10 sub-models, several
undecoded save structures.

## 1. Levels and kinds

- **Levels** (verified, IL2CPP metadata enum `FM.UI.PlayerInterestLevel`, type 201): `Doubtful = 0`,
  `SlightInterest = 10000`, `Interested = 20000`. These are score thresholds, not ordinals. The UI label
  for SlightInterest is "Persuadable". This is an inference from KEY-706422 "Persuadable [Scale is Interested, Persuadable, Doubtful]"
  and KEY-694023 "Persuadable [Player interest in joining club]". Interested is KEY-389963 and Doubtful is KEY-694024.
- The score is an int clamped to about ±100 000. The code uses `-100000` (`0xfffe7960`) and `100000` (`0x186a0`) as the
  fixed results of `0x1283a90` (verified, code). The agent chat uses score ≥ 50 000 for "incredibly
  interested" (KEY-554294, `0x300e258`, verified, code). A negative score reads as "wouldn't be interested".
- **Kinds.** The engine has four filter rules (RTTI): `PERSON_INTERESTED_FILTER_RULE` (vtable `0x54b0a08`,
  eval `0x1695290`), `PERSON_INTERESTED_LOAN_FILTER_RULE` (`0x540b740`, eval `0x16949bc`),
  `PERSON_INTERESTED_DP_FILTER_RULE` (`0x540b5f0`, eval `0x1694428`) and `PERSON_INTERESTED_MQ_FILTER_RULE`
  (`0x540b890`, eval `0x1694db4`). The transfer rule computes interest with offer kind 4 and the loan rule with
  kind 3 (verified, code). Inference: "Transfer only" uses the transfer rule, "Loan only" the loan rule,
  "Transfer or loan" either, and "Not considered" no rule. DP and MQ are not decoded.
- Each filter rule stores the level in property `'flvl'` (i32 rule +0x30, default −100000 = unset; serializer
  `0x34508e4`, setter `0x3450828`) plus a byte at +0x34 (verified, code).

## 2. The computation

**Filter → threshold** (verified, code):
- Transfer (`0x1695290`): threshold = `(200 − v) × 20 + flvl`. It calls `0x2ff2964(player, team, threshold, …)`.
  That function tries playing-time levels in turn and calls
  `interest(player, team, kind 4, playing_time)` until the score reaches the threshold. It sets a flag when it does,
  then a final check `0x14c77a4` runs.
- Loan (`0x16949bc`): it passes when `interest(player, team, kind 3, 7) ≥ ((200 − v) × 20 + flvl) / 2`. Extra
  checks follow: `0x20b89f4`, `0x209f708`, and a value test `0x2f9f504`.
- `v` = i16 `+0xda` of `[ctx+0x18]->vt+0x698()`. Inference: this is the searching manager's non-player part, and +0xda is
  his current ability. The basis is that it follows the staff reputations at +0xd6/+0xd8, as a player's CA follows his
  reputations. Not traced. The human's non-player record is not decoded, so this one number per save is missing.

**Interest function** (verified, code): `GAME_ACTUAL_PLAYER` service (key `'eapl'`, class info `0x12b3bf0`,
vtable `0x5401f08`) slot `+0x118` = **`0x1283a90(player, team, kind, playing_time, …)`**. It holds early
outs (no club or no contract → −2000, ±100000 for hard states) and a dispatch by kind:
- kinds 4/7 from another club → `0x1285498`
- same club → renewal path `0x1284430`
- kinds 3/8 → `0x128600c`
- kinds 1, 2/6, 5, 9, 17 → `0x1285edc`, `0x1285dd0`, `0x1285f54`, `0x30b2a04`, `0x12866bc`
- free agents → `0x1284384`

`0x1285498` (transfer) calls 45 factor functions (`0x1286a48` … `0x12933cc`). Each adds its value to the score
and pushes a reason `{u32 type, i32 value}` into a vector at `ctx+0xc0` (push helper `0xe920b4`). The reason
formatter `0x36c2844` switches on the reason type 1..145 (jump table `0x525b1e8`) and the sign of the value.

Reason types found per factor (verified, code). Labels come from the formatter's first text.
"Saved?" says whether the main input is a decoded save field.

| Factor | Reason | Meaning (formatter text) | Main inputs / saved? |
|---|---|---|---|
| `0x128c898` | 2 | "joining such a reputable club" / "improve their standing" | team reputation (decoded TEAM u16), player world reputation (decoded), contract end (decoded), squad status (decoded), squad-ability ratio `0x36fa000`/`0x30a0498`/`0x309b384` (in memory), age, history ring (undecoded) |
| `0x128beec` | 3 | "playing in the [comp] for [club]" | competition (COMP record undecoded) |
| `0x1289bf0` | 1 | "talented group of players" | squad strength (in memory) |
| `0x12873d8` | 18, 55 | "want to play for a bigger club" / playing time | current vs target team reputation, comp `+0x188` (undecoded), player history ring at person `+0x30` (10 dated events, undecoded) |
| `0x128d22c` | 125 | more playing time | expected playing time (`0x2f9e580`, undecoded) |
| `0x128ba6c` | 127, 145 | loan spell enjoyed/not | loan history |
| `0x1288d14` | 88–93 | declared interest, scouts watched, praise | scouting/media events (undecoded) |
| `0x128df64`/`0x128e658`/`0x128ecc8` | 27, 29, 30 | regular football in [comp], career plan, stage of career | age, comp, history |
| `0x128fe6c`/`0x1290240` | 33/58, 34 | passion for club, admires manager | favoured clubs/personnel (**decoded**: relationships) |
| `0x128fc68` | 32 | rivalry | club rivals (undecoded) |
| `0x12905d8`/`0x1290974` | 36, 40 | infrastructure / living standard, full-time contract | nation data, contract type |
| `0x1291504`/`0x12915fc` | 76, 140 | current club finances, needs funds | club balance (decoded) |
| `0x1288540`/`0x128db0c` | 24, 108 | contract type, requested to leave | contract (decoded), transfer requests (undecoded) |
| `0x1292268` | 113, 119 | club vision / transfer policy (reasons 110–124) | club vision (undecoded) |
| `0x1291f7c`/`0x1291b98`/`0x129260c` | 132, 87, 51/52 | manager keenness, dressing room, feedback | interactions/dynamics (undecoded) |
| others | 17, 35, 39, 41, 46, 49, 53, 59, 65, 96, 99, 137, 141 | work permit, roots, family finance, surplus, just signed, fans, … | mixed |

The early outs in `0x1283a90` and the text family KEY-554307..554310 ("only just signed a new contract / only
just joined / under any circumstances") cover the hard cases: retiring, just joined, just renewed, and so on.

**Cached value?** None for the filter (verified, code): the filter evals call the computation per candidate.
No pool name in `memory_pools.dat` matches INTEREST/REASON/VALUATION. One object does cache a score: the
negotiation/enquiry object behind the agent chat (`+0x1b4` score, `+0x1bc` person index, `+0x200`/`+0x201` status;
a club holds a list of them at `+0x370 → +0xce8`). It exists only for players under negotiation and is not
the filter's source (inference). Where it is saved is not checked.

## 3. "Would require FC Metz to improve their standing in the Ligue 1"

Text KEY-628500 (joining) or KEY-555353/555386 (staying). The formatter uses it for **reason 2** (club reputation,
case at `0x36c319c`) or **reason 18** (bigger club, `0x36c3340`) when the value is negative and the mode is 4
(verified, code). It picks:
- 628500 "improve their standing" when the club's division object (`[ctx+0xf8]`) has `+0x60` set and byte `+0xf8 == 0`;
- otherwise 628501 "get promoted from the [comp]";
- 628502 "stature in the game" when there is no division context.

Inference: byte `+0xf8` = "a higher division exists". So the wording says *where* the club is. The *trigger* is
the reputation comparison.

**Reason 2 value** (`0x128c898`, verified, code; all reputations ×4 and capped at 36 000):

```
T = 4·rep(target first team), default 6000; ×1.1 when 0x2f9e25c(player, target) (age ≥ 30 and other tests); ×1.15 for a history item of type 0x36
C = 4·rep(current first team); ×0.8 when ≥ 90 days left, ×0.9 when ≥ 30 days left (contract-date test on person +0xec);
    ×0.8 unless playing-time level (0x32705ec / table 0x5203960) is ≤ 6 or = 4 (partly read); 1.2× cap from a squad-ability test
d = T − C   (not for kinds 3 and 8, the loans): d ≤ −4001 → 6d; ≤ −3001 → 5d; ≤ −2001 → 4d; < −1000 → 2d; else d
w = T − 4·min(world reputation, 9000); w < −500 → 8w; keep only w < 0
value = d + w; ×5 when negative, kind 4/5/7, player ≤ 19 or status 3, and ability 0x36fa000 > (1.1|1.2 [+0.1]) × club level 0x30a0498
```

**Ground truth check** (2036-01-16, bytes via the repo parser): Leinilson **Ervedoso** (the save's spelling;
uid 2 002 078 288, OGC Nice team 666). His world reputation is 3456, Nice's reputation 7858 and FC Metz's (team 652) 7218,
both in division 11 (Ligue 1). That gives T = 28 872, C = 31 432 before the contract/status multipliers,
d = −2560 → ×4 = −10 240, and w = 28 872 − 13 824 > 0 → 0. Reason 2 is therefore strongly negative, and Ligue 1 is a top flight,
so the text is "improve their standing in the Ligue 1". **This fits the in-game sentence (1 of 1, inference:
the multipliers and the other 44 factors were not evaluated).** Only the reason was checked, not the total score.

The "Transfer or loan · Doubtful, DR" count of 991 was **not reproduced**: it needs the full model.

## 4. Feasibility

- **Exact reproduction: not feasible** (inference from the factor count and inputs). It needs about 45 factors plus the sub-models:
  - expected playing time `0x2f9e580` (squad-ability ranking, already blocked in the squad-status work);
  - squad and club ability `0x36fa000`, `0x30a0498`, `0x309b384`;
  - the per-person history ring (person `+0x30`, 10 dated events);
  - COMP records, club vision, rivals, scouting events and dynamics;
  - the manager's CA (`v`).
- **Approximation: possible.** A reputation-gap model covers the commonest negative reason: reason 2/18 with
  team reputation, player world reputation and division, all decoded. Add the hard early outs (just joined,
  just renewed, transfer-listed, retiring) and favoured clubs/personnel (decoded). The approximation would miss
  playing time (often decisive for "Persuadable"), finances, wage affordability and the history-based factors.
  Its accuracy is unknown until it is checked against in-game filter results. The 991-row DR list is a ready test set.
- **Remaining RE effort (estimate):**
  - reason 2/18 plus the early outs exactly, with the multipliers' inputs (contract date, status table `0x5203960`, `0x2f9e25c`): about 3–5 days;
  - playing time (reason 125/55, needs `0x2f9e580`): about 1 week more;
  - the full model: several weeks, with an uncertain end.
- MCP wording for an approximation: something like `likely_interest: "unlikely" | "possible"` with the reason
  ("FC Metz's reputation is below what he wants"). Do not use FM's level names, because the values would not match FM's filter.

## Unknown / hypothesis
- UI kind → filter rule mapping; DP and MQ rules.
- `v` (+0xda) identity and value.
- Factor inputs beyond reason 2.
- Whether the negotiation object's score is saved.
- The meaning of division byte `+0xf8`.

## Follow-up 1

### 1. Reason 2 (`0x128c898`) run in an emulator

Setup:
- Emulator: Unicorn 2.1.4 (`player-interest/venv`, script `player-interest/emu/r2emu.py`). It runs outside the
  sandbox only: inside the sandbox Unicorn's JIT dies with SIGILL.
- The real arm64 code runs on a fake object graph:
  - person: `+0x78` contract, `+0x108`, `+0x25a` world reputation, vtable `+0x178` age;
  - contract: vtable `+0x90` type, `+0x128` flags, `+0x100` playing time b, `+0xd8` contract type; `+0x18` team; `+0x30` event ring; `+0x68/+0x70` clause vector of 8-byte `{i32, u16, u8 type}`;
  - clubs: `+0x18` team vector, first team `+0xa8` reputation.
- Stubbed calls:
  - `0x2f9e25c` → 0 (it returns 0 below age 30, verified, code);
  - `0x32705ec` → the playing-time level from `fm.scoreLevel`, the decoded rule;
  - the reason push `0xe920b4` is captured.
- Any call not expected on the club path fails loudly. None happened.

**Ervedoso** (save, 2036-01-16):
- inputs: FULL contract at OGC Nice (first team 666, rep 7858), b = 2 Important Player, score 97 → level 9;
  clauses 15, 32, 34, 38, 19, 22; WR 3456; age 25; target FC Metz (652, rep 7218), kind 4;
- **emulated: reason (2, −10 240), score −10 240.** This equals the hand derivation (d = −2560 × 4, w = 0).

Variants (emulated). They resolve the parts first read only partly:

| Change from Ervedoso | Result | Meaning |
|---|---|---|
| score level 6 or 4 (b = 2) | −10 240 | the ×0.8 test uses m = max(level(b), score level); b = 2 keeps m = 9 |
| b = 7 (level 6), score level 5 | +3 727 | m ≤ 6 and m ≠ 4 → C × 0.8 (25 145). Positive values are pushed too |
| clause type 63 or 54 | +1 770 | T × 1.15 (33 202) |
| `0x2f9e25c` true (age ≥ 30 path) | +327 | T × 1.1 |
| kind 3 (loan) | none, 0 | the club gap d is skipped for kinds 3 and 8; only w counts |
| WR 8000 | −35 264 | w = 28 872 − 32 000 = −3128 → ×8 |
| Nice rep 9500 | −42 768 | d = −9128 → ×6 (−54 768)… plus w = 0; the code caps C at 36 000 and rep×4 = 38 000 → 36 000, giving −7128 × 6 = −42 768 |
| contract flag `+0x128` bit 5 set, `[P+0x108]` null | none | early out |
| non-FULL contract, `[P+0x108]` null | none | early out |

**Corrected formula.** This is a club player with a FULL contract at another club. All parts are verified by
code and emulation unless marked.
```
T = min(4·rep(target first team), 36000)  (6000 without one)
    ×1.1 if 0x2f9e25c(player, target)  [0 below age 30; tests for 30+ not decoded]
    else ×1.15 if the contract's clause list holds type 63 or 54
    [that +0x68 is the saved clause list is an inference; the save holds 51 contracts with type 63 and 61 with type 54]
C = min(4·rep(current club first team), 36000)
    ×0.8 if m = max(LEVEL[b], scoreLevel(contract)) ≤ 6 and m ≠ 4
    [LEVEL = table 0x5203960, byte-identical to 0x524f3e8: 10 9 8 7 7 7 6 6 5 4 4 4 3 2 8 7 7 7 7 7 5 1]
d = T − C (skipped for kinds 3 and 8): ≤ −4001 ×6, ≤ −3001 ×5, ≤ −2001 ×4, < −1000 ×2, else ×1
w = T − min(4·WR, 36000); ×8 if < −500; min(w, 0)
value = d + w; when value < 0, kinds 4/5/7, different clubs, contract flags bits 0 and 5 clear,
    contract +0x57 bit 3 clear, and (contract type (vt +0xd8) = 3 or age ≤ 19)
    → ×5 if ability (0x36fa000) > (1.1 or 1.2, +0.1 when 0x327597c) × club level (0x30a0498)
score += value; reason (2, value) is pushed when value ≠ 0
```

Where the previous reading was wrong:
- The contract-days multipliers (C × 0.8 / × 0.9, from the date at person `+0xec` against the game date) and the 1.2·T cap
  on C do **not** run for a FULL contract whose team belongs to a club. Corrected reading: the days test runs only
  when the contract team has byte `+0x2f` set or no club. The cap runs only without a contract or with a non-FULL one.
- The ×0.8 test uses the max of the agreed status's level and the score's level. "≠ 4" means a Prospect-level
  player never gets the cut.
- Early outs, when no reason 2 is given at all:
  - contract flag bit 5 set (or a non-FULL contract) together with `[person+0x108]` null or its u16 `+0xa` = 0 (meaning unknown);
  - a pending event in the contract's ring with date 1900-01-01, type 12, status 5.

### 2. The 15 "Transfer or loan · Doubtful" DR players (in game, 2036-01-16)

All 15 are found in the save (bytes, repo parser). Reason 2 is for kind 4 to FC Metz, with the formula above.
The "current club" is the main contract's club (an inference for loanees). Nation is the primary nationality; the
save's second nationalities are not decoded, which explains the non-EU primaries.

| Player (uid) | Club (first-team rep, division) | WR | b / score | Knowledge | Reason 2 |
|---|---|---|---|---|---|
| Maitere Wong (2002078283) | FC Nantes (7034, 11) | 3395 | 2 / 97 | 50 | +736 |
| Jean Eloi Musongi (2002078285) | no club | 3745 | – | 66 | not modelled (free agent path `0x1284384`) |
| Modou Sall (2002082525, Senegal) | Genoa (6315, 23) | 4158 | 3 / 90 | 99 | +3 612 |
| Frode Øverenget (2002098107, Denmark) | Vålerenga (5512, 619) | 2867 | 2 / 98 | 70 | +6 824 |
| Oumar Cissokho (2002099764, Senegal) | SM Caen (5525, 12) | 2668 | 4 / 94 | 78 | +6 772 |
| Maxime Cipriano (2002114039) | Saint-Étienne (7477, 11) | 2829 | 10 / −100 | 75 | −2 072 |
| Zvonko Nešić (2002114070) | Red Star FC (4561, 13) | 2717 | 2 / 82 | 92 | +10 628 |
| Benjamin Masson (2002114299) | Troyes (6425, 11) | 2648 | 7 / −100 | 98 | +8 312 |
| Ibrahim Diallo (2002114387) | FC Nantes (7034, 11) | 3124 | 4 / 49 | 84 | +736 |
| Simone Scotti (2002114633, Italy) | Stade de Reims (7634, 11) | 3378 | 3 / 80 | 50 | −3 328 |
| Jérémy Catherine (2002115040) | Angers SCO (6615, 11) | 3171 | 3 / 100 | 30 | +2 412 |
| Logan Lafleur (2002115106) | Famalicão (6832, 45) | 3174 | 4 / 36 | 75 | +1 544 |
| Álvaro Garrido (2002116366, Spain) | Valencia (8099, 51) | 3137 | 7 / −7 | 92 | +2 956 (×0.8 on C: Fringe, level 6) |
| Aleksei Korshun (2002123791, Belarus) | Estrela da Amadora (5528, 46); on loan (game shows Nacional) | 3822 | 7 / 47 | 96 | +6 760 |
| Marko Tomašević (2002125884, Croatia) | AZ (7308, 20) | 6135 | 4 / 45 | 85 | −360 |

Results:
- **None of the 15 has a strongly negative reason 2.** The lowest is −3 328 (Scotti). Three are negative
  (Scotti, Cipriano, Tomašević) and 11 positive.
- Being listed is consistent with reason 2 never exceeding about −3 300. It does not measure how much the other factors
  outweigh a large negative, because no listed player has one.

### 3. Candidates the list leaves out

Conditions used:
- men, not FC Metz;
- age 24–25 at the game date (fm.Age);
- DR ≥ 15 (all 15 listed have DR 20);
- primary nationality EU (27 members);
- stand-in for the knowledge condition: effective knowledge ≥ 30. The listed minimum is 30 (Catherine). FM's "Extensive Knowledge" condition
  cannot be reproduced: it is not per-player knowledge, since the listed players range 30–99.

Result: 125 players (121 EU plus the 4 listed non-EU), 15 listed. The list is in
`player-interest/cand16.txt`. Without the knowledge stand-in there are 767 EU candidates, 596 of them with DR 20.

| Reason 2 | Candidates | Listed |
|---|---|---|
| ≤ −5000 | 19 | **0** |
| −5000 < r ≤ 0 | 11 | 3 |
| 0 < r ≤ 5000 | 48 | 6 |
| > 5000 | 45 | 5 |
| free agent (not modelled) | 2 | 1 |

All 19 strongly negative candidates are left out:
- Di Marino (Liverpool) −74 392;
- Lefebre (Villarreal) −42 624;
- Partheniou (Milano) −41 424;
- Malenović (Newcastle) −38 688;
- Cinalli (Man Utd) −32 400;
- Carrero (Spurs) −30 456;
- Bah (Stuttgart), Etxabe (Bilbao), Codeço (Benfica), Boscolo (Leverkusen), Schmidt and Larrazabal (Porto), Folch (Lille), Vázquez and Curbelo (Sevilla), Panzeri (Juventus), Errington (Marseille): all between −26 640 and −17 120;
- Helmstedt (Freiburg) −10 512;
- Marzierli (Bologna) −10 256.

Inference: these fit "below the Doubtful threshold because of reason 2". A reputation-gap rule would exclude them correctly.

The 93 candidates with reason 2 > 0 that are not listed show the limit of reason 2 alone. Inclusion among them
depends on the other factors or on the knowledge condition, which cannot be told apart here. Examples:
- Dell'Oro (Como) +376;
- Macchi (Lecce) +652;
- Havel (Cagliari) +980;
- Dinkelmann (Bielefeld) +1 084;
- … up to +18 960.

### Summary for design
- A reputation-gap test (reason 2 ≤ about −5000 → "not interested") has 0 false negatives against this list, which has 15 players. It
  catches the clear big-club cases (19 of 19).
- A positive reason 2 is not enough to predict "at least Doubtful": 11 of 93 positive candidates are listed, and the
  knowledge condition muddies this.
- An exact level still needs the other ~44 factors and the threshold shift `(200 − v) × 20`.
