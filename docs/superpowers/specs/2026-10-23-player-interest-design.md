# Player interest filter — design

Date: 2026-10-23. Status: approved in chat (option A, both playback parts); this document awaits approval.

## Why

The manager (FC Metz in the KŽ career) wants `search_players` to answer FM's question "would this player be interested in joining my club?", the way the game's Player Database filter "Player's interest" (Transfer or loan / Transfer only / Loan only / Not considered; Interested / Persuadable / Doubtful) and the agent chat do. Without it, scouting lists fill with players who will refuse to talk.

FM computes interest live; the save stores no per-player interest. Research decoded most of the computation from the game binary (`game_plugin` 26.3.2) and tested it against 93 in-game labels. Some inputs live only in memory and cannot be read from the save, so the game's list cannot be reproduced exactly. The manager chose: **drop only the players who certainly won't come; keep everyone else, marked `likely` or `uncertain`.**

## Goals

- `search_players` takes an `interest` option that drops players certainly not interested in joining the human's club, for a transfer, a loan, or either.
- Every player kept gets an interest tag (`likely` / `uncertain`) and the main factors behind it.
- The response says how many players the option dropped and why (counts per reason), so the "certainly out" claim can be checked.
- `get_player` shows the same interest verdict for one player, with the reason when he is out.

## Anti-goals

- **Dropping a player FM would accept.** This is the failure that matters. On the 94 labelled players, no FM-accepted player may be dropped (acceptance test below). A dropped player must always carry a reason that a person can check in game.
- Claiming FM's exact levels (Interested / Persuadable / Doubtful). We cannot tell them apart reliably.
- Showing hidden values (Loyalty and the other hidden attributes) through the reasons when fog of war hides them.

## Non-goals

- Exact reproduction of FM's list. The squad ranking that sets the offered level (`0x2f9e580`), about 25 factors and the "wanted by a club" (Wnt) bonus are not decoded.
- Interest towards any club other than the human's.
- Favoured personnel (not approved earlier).
- More research on the Wnt lists.

## What the manager decided

- Option A: drop only the certain cases.
- Players like Erveodoso (Nice GK, whose agent says Nice would need to improve its standing in Ligue 1) stay in as `uncertain`. A club-reputation gap like his (−10 240) does not make a refusal certain on its own. A gap so large that the best-case sum stays below −20 000 does (builder ruling, below).
- The behaviour described in "Behaviour" below, as played back in chat.

## Behaviour

### `search_players`

New input `interest`: `transfer`, `loan` or `transfer_or_loan`. Absent = FM's "Not considered": no filtering, no interest fields.

With `interest` set:

- Players whose verdict for that kind is `out` are removed before sorting, limiting and counting `total`.
- Every returned row carries `interest`: `{"verdict": "likely" | "uncertain", "factors": [...]}`. Each factor has a stable code and a short English sentence, e.g. `{"code": "club_reputation_gap", "text": "Nice's standing is above Metz's"}`.
- The response carries `interest_dropped`: the number of players dropped and a count per reason code, e.g. `{"total": 9, "by_reason": {"only_just_joined": 6, "transfer_arranged": 2, "recently_left_your_club": 1, "club_reputation_gap": 0}}`. These are players who passed every other filter. Players are not listed by name.
- The tool description and the server instructions explain the three verdicts and that the option never drops an uncertain player.

### `get_player`

Always carries `interest` (no input needed) for players outside the human's club: `{"transfer": verdict, "loan": verdict, "reason"/"factors": ...}`. Verdict values: `out`, `uncertain`, `likely`. For `out` the reason is one sentence the manager can check, e.g. "Joined SM Caen 153 days ago; won't consider a move yet (FM: 'only just joined his new club')" or "Has agreed a move to SS Calcio Bari on 1 July 2036".

Builder's call: whether `compare_players` and `find_players` also carry it.

### Fog of war

Interest is computed and filtered under fog of war, as FM's filter is. Reasons and factors never contain a hidden value: "joined 153 days ago" is fine, "Loyalty 9" is not. Whether FM's filter respects scouting knowledge is not known; the ground truth used "Extensive knowledge" lists.

### Who is evaluated

- Players at another club (current team or main contract with a club other than the human's): full model.
- Free agents: only the save-flag gates (retiring, only one club) and `transfer_arranged` (it runs in `0x1283a90` before the free-agent dispatch) can make them `out`; otherwise `uncertain`. `recently_left_your_club` sits in the transfer and loan paths only. The free-agent path (`0x1284858`) is not decoded enough to separate Musongi, David Silva and van Hattum.
- The human's own players (current team's club or main contract club is the human's club): not evaluated. Builder's call how they appear with the option set; the default is to keep them without an `interest` field.
- Players on loan at another club: the model handles them as the reference model does (no just-joined penalty while on loan).

## The model

All rules below are code-derived (game binary, verified by disassembly and, for the playing-time factor, by emulation) unless marked. Addresses are in the arm64 slice of `game_plugin` 26.3.2. The research notes next to this file hold the evidence: `2026-10-23-player-interest/research-*.md`. The Python reference model (`reference-model/code_rules.py`, `score_c.py`, `r2.py`) is the frozen version that was tested blind. It does not run from the repo: its inputs were scratch files. Its per-player outputs for all 94 labelled players are saved in `reference-golden.tsv` (P_T, r2, pt10, P_L, pt7, r2L, T, L). The Go code builds every input from the repo parser; where the code above says nothing about an input's definition (a loanee's or B-club player's "current club", `ClubFirstTeamRep`, `FTManager`, `TeamClubNation` vs `ContractClubNation`), the scratch probe `reference-model/probe/v2_main.go.txt` is the authority. Known, intended differences from the golden file: the loan bound uses `max(pt7, 0)`; reason 100 is evaluated for every player (the reference hard-codes Vilas); the loan-penalty check and the age ≤ 23 reduction below.

"Target" = the human's club (FC Metz) and its first team. Rt = the target first team's reputation (TEAM record; 7218 for Metz on 2036-01-16). Dates use the game's 365-day arithmetic: `days = 365 × (year difference) + (day-of-year difference)`.

### Hard gates (verdict `out` for both transfer and loan)

| Code | Rule | Source |
|---|---|---|
| `transfer_arranged` | A TRANSFER record linked from the player has status 19. FM shows "Trn". | `0x2f54e10`, −100 000, reason 10. Save: the ACTUAL_PERSON `u8 n + n × u32` id list before the contracts marker (+0x68), TRANSFER records in `transfer_man.dat` (layout in research-gates "Follow-up: Marco") |
| `recently_left_your_club` | ACTUAL_PERSON +0xd8 (previous club, a CLUB index: the ref after the first u16 of the ACTUAL_PERSON part) is the human's club, his current club is not, and he joined his current club ≤ 150 days ago | `0x128e658`, −100 000, reason 100. Vilas, Bayal. `0x12ade1c` can exempt; not decoded. Assume no exemption: both tested cases fail in game |
| `retiring` | Person trailer flags bit 18 | `0x1283a90`, −100 000, reason 6 / 60 |
| `only_one_club` | Person trailer flags bit 15 | `0x1285498` and the free-agent path, −100 000, reason 7 |

### Just-joined penalty P(L, kind) — `0x128f550`, reason 43 / 59

Reference: `code_rules.just_joined`. Inputs: J = ACTUAL_PERSON +0xec, the date he joined his current club (the second date before the contracts marker; set for loanees where the player-part joined date is 1900); the main FULL contract's header type byte (FULL +0xc3), end date, signed date, clause types, core byte 23 (CONTRACT +0x57), byte 22 (original playing time) and its contract-ring items; Loyalty (hidden attribute +0x42).

- No penalty: free agent; on loan; ring exemption (`0x12ae024`: an open qualifying ring item and no open item with u14 in {2, 18, 34, 39, 42, 60, 238–240}); clause 55 or 56; contract type 3; days ≥ 61 and (contract type 4 or contract ends in < 300 days).
- Mode: "just signed" (T = 100, d = days since signing) when he renewed after joining and ≥ 300 days are left, else "just joined" (T = 250, d = days since joining). Loan kind at age ≤ 25 always uses "just joined".
- T − d ≤ 0: no penalty.
- The +0x57 bit 0/3 exemption as in the reference model.
- `P = (11 − L) × (T − d + 1) × Loyalty × 100`, × 1000 instead of × 100 for contract type 2.
- Loan kind (3): only when the player's nationality equals his club's nation (`0x129fcf8`), and only when `0x32715dc(contract)` ≥ 5 (read as LEVEL[b] ≥ 5; with flag bit 3 it also needs club `+0x16a == 1`; `0x128641c…0x1286450`). The reference model lacks this check.
- "Just signed" at age ≤ 23: the code halves or, in some modes, quarters P; the modes are not decoded, so use the quarter (the lenient value: anti-goal).
- Not applied: the relegation exemption (days ≥ 120 and the club was relegated); relegation is not decoded. See Known risks.

### Club reputation gap r2 — `0x128c898`, reason 2

Reference: `r2.reason2` (transfer) and `code_rules.r2_loan` (loan). The transfer value uses the target and current first-team reputations, the player's world reputation, his playing-time level and clauses 54/63; only a negative world-reputation term counts. The loan value is the world-reputation term alone.

### Playing time pt(L, kind) — `0x12ad438`

Reference: `code_rules.pt_value`. Inputs: byte 20 of the current contract (b), the playing-time score and recent score, GK rating, current reputation CR, his club's first-team reputation, Rt, open ring items (cap at level 8). The LEVEL table is `0x5203960`. Verified by emulating the real arm64 code on save values (7 players, every value equal).

### Sums and verdicts

```
transfer sum = −P(10, kind 4) + r2 + pt(10, kind 4)
loan bound   = −P(7, kind 3)  + r2_loan + max(pt(7, kind 3), 0)
```

L = 10 is the best level Metz could offer. Its pt is the largest possible, so the transfer sum is an upper bound for the decoded factors. For the loan, pt is taken as ≤ 0 only: the loan playing-time term is not trusted to sink a loan. Vega passes FM's loan filter with a loan sum of −18 435.

| Kind | `out` when | `likely` when | otherwise |
|---|---|---|---|
| transfer | a hard gate, or transfer sum < −20 000 | transfer sum ≥ +30 000 | `uncertain` |
| loan | a hard gate, or loan bound < −20 000 | never (no confident loan pass was tested) | `uncertain` |
| transfer_or_loan | transfer `out` and loan `out` | transfer `likely` | `uncertain` |

The two cut-offs (−20 000 and +30 000) are named constants. They come from the 94 labelled players (inference): every factor not decoded would have to add more than +20 000 for an `out` player to pass. Every tested player below −20 000 failed in game; every confident pass was above +30 000. They are not from code.

Builder ruling: a sum `out` gives the most negative term as its reason: `only_just_joined` or `club_reputation_gap`. Only when the best-case sum is below −20 000 can the reputation gap be the reason on its own (e.g. Di Marino −74 392). In the research, every DR candidate with a gap ≤ −5 000 was missing from FM's list, and Cubelo (−17 800) and Buckingham (−26 640) failed in game.

Factors shown on a kept player: the terms that moved the sum most (just-joined penalty if any, club reputation gap if negative, playing time and whether it depends on the role offered), plus "depends on the role you'd offer" when the sum at L = 8 would fall below 0.

## Acceptance

Integration tests on `data/kz-2036-01-16.fm` (env var to add, e.g. `FM_REAL_SAVE_16JAN`) against `2026-10-23-player-interest/labels-2036-01-16.tsv` (95 rows, 94 players; Musongi is in two sets):

- No player FM passes is `out` for that kind. Use `fm_transfer_or_loan` for the DR and DC sets, and `fm_transfer_only` for all three sets.
- Players named as certain fails in the research are `out` with the expected reason, e.g. Marco (`transfer_arranged`), Vilas and Bayal (`recently_left_your_club`), Barthes and Gamboa (`only_just_joined`).
- Spot values equal the reference model: Barthes P(10) = 88 200; Nešić and Lotito exempt; Erveodoso `uncertain`.
- The builder runs the reference model and the Go code over the 94 players (`reference-golden.tsv`) and lists every disagreement. Each one is fixed or explained in the plan.

DR labels: "Transfer only · Doubtful" lists the same 13 players as Persuadable and Interested (in game, 2036-01-16). Lafleur and Tomašević, the two players who joined their clubs recently, are not on it, so the model is right to make them `out` on transfer. They pass only through the loan rule.

## Save work needed

The save-side parsers come first. Each decoded structure goes into `docs/format.md` with offsets, evidence and build, using the `/decode-structure` skill.

- ACTUAL_PERSON +0xec joined date and +0xd8 previous club.
- The person's TRANSFER id list (+0x68).
- `transfer_man.dat` TRANSFER records (class 2/3, version 0x13, person, teams, id, status at +29, date +0x7c). The research found them by pattern and checked them against the person id lists (9 015 of 9 425 linked). Builder ruling: a pattern scan checked against the person id lists is enough (a full grammar walk of `transfer_man.dat` is not needed now). When the check fails, the gate is skipped (players stay `uncertain`, never wrongly `out`) and the load reports a warning, as the tactics parser does. The writer `0x3a53bb4` and loader `0x3a53fec` are the reference for the record layout.
- Person trailer flags bits 15 and 18 (the trailer u64 is already read for bit 3).
- The FULL header contract type byte (already read, dropped) and the contract ring items (core `u32 n × 29 bytes`; layout in research-factors §5).
- Club relationship vector, the human's non-player CA and the Wnt lists: not needed by this design.

## Known risks

- ⚠️ **Relegation exemption not applied.** A player who joined ≥ 120 days ago at a club that has since been relegated pays no just-joined penalty in game; the model still charges it, so such a player can be wrongly `out`. Rare (inference), and not decodable yet.
- ⚠️ **Ages outside 24–25 are not tested.** Every labelled player is 24 or 25.
- ⚠️ One save, one date (2036-01-16), one human club. The cut-offs may not hold for a much bigger or smaller club.
- ⚠️ In-memory inputs (contract events beyond the ring, person +0xa8 items, negotiation and scouting records) can exempt or block players in ways the save does not show. Vega and Lau Jiménez are the tested examples.

## Left to the builder

- Package layout: parsing in `savefile`, the model in `fm`, wiring in `mcpserver`. MCP handlers hold no parsing logic.
- Field names, factor code names and sentence wording.
- How `interest_dropped` is shaped, as long as it gives a total and per-reason counts.
- Whether the computation is cached per load (it is per player and cheap).
- PR split: savefile structures first, then the model behind the option, then `get_player`.
