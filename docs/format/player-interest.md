# FM26 format: Player interest (game code)

Part of the format reference. Conventions, dates and the index of all files: [`docs/format.md`](../format.md).

## Player interest (game code) — partly decoded 26.3.2

FM's Player Database filter "Player's interest" and the agent chat ask whether a player would consider joining a club. The save stores no per-player interest: the game computes a score live. Evidence: `docs/superpowers/specs/2026-10-23-player-interest/research-*.md`; `internal/fm/interest.go` and `interest_verdict.go` implement the decoded part. Labels: **verified (code)** = read in `game_plugin` 26.3.2 by disassembly; **inference** = reasoning or fit to the labelled players.

**Filter wrapper** (verified, code). The transfer rule (`PERSON_INTERESTED_FILTER_RULE`, eval `0x1695290`) computes the interest with offer kind 4, the loan rule (`PERSON_INTERESTED_LOAN_FILTER_RULE`, eval `0x16949bc`) with kind 3. Each rule stores a level `'flvl'`: Doubtful 0, Persuadable 10 000, Interested 20 000 (C# enum `FM.UI.PlayerInterestLevel`, metadata). The transfer rule keeps a player when `interest(kind 4, L) ≥ threshold`, with `threshold = (200 − v) × 20 + flvl` and offered level `L = min(E + 3 or 5, 10)` (`0x2ff2964`); E is the level the player expects at the club (`0x2f9e580`, a squad ranking from staff-assessed ability; not reproduced). v is probably the human's current ability, 200 (inference, medium confidence), so the threshold equals the level. The loan rule keeps a player when `interest(kind 3, level 7) ≥ threshold / 2` and further checks pass (`0x20b89f4`, `0x209f708`, `0x2f9f504`; not decoded). "Transfer or loan" passes either rule (inference: the UI → rule mapping was not found). Also not decoded: the eligibility pre-check `0x209efe8`, the club's maximum signing age, and the rivalry check `0x14c77a4` (club relationship items).

The score is clamped to about ±100 000; the entry `0x1283a90` returns −100 000 for the hard gates (verified, code).

**Hard gates** (verified, code; each −100 000 for both kinds):

| Gate | Rule | Code | Reason |
|------|------|------|--------|
| transfer arranged | a TRANSFER record linked from the player (`+0x68`) has status 19 (see "transfer_man.dat") | `0x2f54e10`, called before the kind dispatch, so free agents too | 10 "has already arranged a future transfer" |
| retiring | trailer flags bit 18 | `0x1283a90` | 6 / 60 |
| only one club | trailer flags bit 15 | `0x1285498` and the free-agent path | 7 |
| recently left | previous club (`+0xd8`) is the target club, his current club is not, and he joined it (`+0xec`) at most 150 days ago | `0x128e658`, transfer and loan paths only | 100 |

`0x12ade1c` can exempt the recently-left gate; it is not decoded, and `fm` assumes no exemption (Vilas and Bayal both fail in game).

**Just-joined penalty P(L, kind)** — `0x128f550`, reasons 43 / 59 (verified, code). Inputs: the joined date `+0xec`, the main FULL contract (type byte `+0xc3`, end, signed date, clause types, core byte 23 `+0x57`, original playing time) and its ring items, and Loyalty (`+0x42`).

- No penalty for free agents or players on loan; when `0x12ae024` exempts (an open qualifying ring item and no open item with u14 in {2, 18, 34, 39, 42, 60, 238–240}; item types 19 and 20 count only at status −2, types 8 and 37 never); with clause 55 or 56; for contract type 3; and from 61 days after joining when the contract type is 4 or the contract ends within 300 days.
- Window: "just signed" (T = 100, d = days since signing) when he renewed after joining and at least 300 days are left, else "just joined" (T = 250, d = days since joining). Days use the game's 365-day count. No penalty when T − d ≤ 0.
- Core byte 23 bits 0 or 3 exempt him unless his contract club's first team has a manager and his original playing time is 1..6 or 15..20.
- `P = (11 − L) × (T − d + 1) × Loyalty × 100`; × 1 000 for contract type 2.
- Loan kind: only when his nationality equals his club's nation (`0x129fcf8`: his current team's club, else his contract team's) and the playing time's level is at least 5 (`0x32715dc`); at age 25 or younger it always counts from joining.
- "Just signed" at 23 or younger: the code halves or quarters P by a mode not decoded; `fm` takes the quarter (the lenient value).
- Not applied: the relegation exemption (from 120 days, at a club relegated since; relegation is not decoded).

**Club reputation gap r2** — `0x128c898`, reason 2 (verified, code). For a transfer, with Rt the target's first-team reputation and C his contract club's first-team reputation:

```
T = min(4 · Rt, 36 000), × 1.15 with clause 54 or 63 under age 30
C = min(4 · C, 36 000),  × 0.8 when his playing-time level is 6 or less, except a prospect (4)
d = T − C;  × 6 at ≤ −4 001, × 5 at ≤ −3 001, × 4 at ≤ −2 001, × 2 at ≤ −1 001
w = T − min(4 · world reputation, 36 000);  × 8 below −500;  only w < 0 counts
r2 = d + min(w, 0)        0 without a contract, playing time or club first team
r2 loan = min(w, 0)
```

`fm` names the gap by its more negative term: "<his club> is a bigger club than <yours>" for d, "His (Her) reputation is above <your club>'s" for w.

**Playing-time value pt(L, kind)** — `0x12ad438` (verified, code; arm64 emulation on save values for 7 players gave equal values). m is the better of the LEVEL of his playing time (table `0x5203960`) and his score's level, capped at 8 by an open ring item with status ≥ 2. He wants m, except a fringe goalkeeper (GK ≥ 15) and a prospect want 7, and a loan from age 21 wants at least 6. With d = L − wanted level, CR his current reputation and his club's first-team reputation (1 500 when unknown or 0):

```
g = 5 · (Rt − CR);  when g < 0 and Rt < club: g += 10 · (Rt − club)
d ≥ 1: max(0, g + d · CR);  d = 0: 0;  d < 0: min(d · CR − g, 0)   (L ≤ 6 with d < 0 not modelled)
```

The division-tier term is not modelled.

**Sums and verdicts** (`fm`; the cut-offs are inference from the labelled players, not code):

```
transfer = −P(10, kind 4) + r2 + pt(10, kind 4)
loan     = −P(7, kind 3) + r2 loan + max(pt(7, kind 3), 0)
```

L = 10 is the best level the club can offer, so the transfer sum is an upper bound of the decoded factors. A player is out for a kind when a gate fires or its sum is below −20 000: every labelled player below it failed in game. A transfer from +30 000 is likely: every confident pass was above it. A loan is never likely (no confident loan pass was tested). transfer_or_loan is out when both are, and likely when the transfer is. An out sum names its most negative term (just-joined penalty or reputation gap). `fm` evaluates no player of the human's club (current team or main contract), none with an arranged move to it, none of the other gender, none whose only team is a national team, and none when the human's club has no first team with a reputation (load warning "interest unavailable"). An arranged move's reason omits a move date before the game date ("27.0.0 records" in transfers.md).

**Ground truth** (KŽ save 2036-01-16, FC Metz, Rt 7 218): 95 rows, 94 players (Musongi is in two lists), from three in-game lists with "Extensive knowledge": DR (23 rows), DC (41) and ST (31), labelled pass or fail for "Transfer only" and, for DR and DC, "Transfer or loan" (`internal/fm/testdata/interest_labels.tsv`). No player FM passes is out (`TestInterestMatchesFM`). FM's fails that are out: transfer DR 10 of 10, DC 13 of 18, ST 12 of 13; transfer or loan DR 5 of 8, DC 5 of 17. Certain fails carry the expected reason: Marco transfer arranged (Bari, 2036-07-01), Vilas and Bayal recently left, Barthes and Gamboa just joined. The terms equal the reference model's for all 86 labelled players with terms (`TestInterestTermsMatchReference`, 0 allowed differences; gated players and free agents have none).

**Not decoded**: the expected level E (squad ranking `0x2f9e580`), the "wanted by a club" (Wnt) bonus, the free-agent path `0x1284858` (27.0.0: see "27.0.0 free agents"), about 25 further factors (research-factors), the loan rule's extra checks, the DP and MQ rules, and the relegation exemption.

## 27.0.0 free agents — partly decoded 27.0.0

Addresses are in the 27.0.0 arm64 slice. Evidence: `docs/superpowers/specs/2026-11-07-interest-field-observations/research-free-agent-27.md`; in-game outcomes: `field-observations.md` in the same folder.

**Path** (verified, code). The interest entry `0x1f6eebc` sends a transfer of a player without a club to `0x1f70254`, and gives a loan −100 000 (`fm`: loan `out`, reason `free_agent`). Its gates: youth-trial contract (129), only one club (7), waiting for the draft (23), target relationship ≤ −50 (26), and two not read. About 20 factors follow; two are decoded:

**Reason 2 against the previous club** (verified, code, `0x1f79b0c` with kind 7 and the previous club `+0xd8`; the 26.3.2 reading "0 without a contract" does not hold for free agents):

```
T = min(4·Rt, 36 000)
C = min(4·rep(previous club's first team), 36 000)
    C = min(C, 1.2·T) when CA × 220 < level 7 of the previous club
    × 0.8 from 90 days after +0xec, × 0.9 from 30 days
r2 = stepped(T − C) + world reputation term       (steps and world term as for contracted players)
```

Not applied: `0xd00c50` multiplies T by 1.1 for a player aged 30 or more when the target's division and his previous club's division are both COMP uid 11 (the Premier League); the checks after `0xd00df4` were not read, and whether it fires for free agents is inference.

Level 7 (`0xe0327c`) is 0.92 × the club's level, capped at 33 000. The club's level is the mean ability (CA × 220) of its 12 best players (`0xe036b4`; players of its teams, not those on loan there), or with fewer than 12 players a step function of its first-team reputation (`0xe033bc`, ten steps from 7 500 + 1.5·R below 1 000 to 36 500 + 3.5·(R − 9 000) from 9 000). The game uses its staff's estimate of ability, smooths the level and caches it per club in `game_db.dat`; `fm` uses true current ability and no cache (the cached values are within 1.5% on the 2026-02-11 KŽ save, with the same cap decisions for every labelled player). The step function's division factor (`1 + 0.025·min(COMP +0x1aa, 10)`) is not applied. The game skips reason 2 when the player's career stats array is empty (`PLAYER_CAREER_STATS[5]`, built from career history; not read): inference, rare for a senior free agent.

**Playing time** (verified, code, `0x1f9af3c` without a contract). He wants level 3 below age 22; else 7 when his world reputation is at most 0.95·Rt; else 9 when the target's division COMP `+0x1aa` > 0, else 8. `fm` does not read that byte and takes 8, the more lenient value (TOPLYGA and the Premier League both hold 0). The value at the best level, 10, is `max(0, 5·(Rt − CR) + (10 − want)·CR)`.

**Verdict** (`fm`; the rule is the user's choice, tested on 13 agent-chat approaches): transfer `out` when r2 < 0 and r2 + playing time < 20 000, FM's Interested level; else `uncertain`, never `likely` (no labelled free agent reached a confident pass). The sum alone is not enough: on the 2026-02-11 KŽ save it is below 20 000 for 8 618 of 10 676 free agents, mostly low-reputation players a 3 200 club would sign, because the undecoded factors are missing. With the gap condition, 1 415 are out; for 126 of them the world-reputation term is the larger part, and the reason says his reputation is above yours. The hard gates `fm` applies to every player (transfer arranged, retiring, only one club) still come first. `fm` applies these rules to every build; the 26.3.2 ground truth (Musongi, van Hattum) was not re-run, as its save is gone.

**Ground truth** (KŽ, Rt 3 200, approaches around 2026-02-10, `data/test_save_27_interest.fm`, `TestLoadTestSave27Interest`): talked or signed — Kazlauskas (no previous club), Ghrissi (+512 + 23 800), Arcila (+1 400 + 24 500); refused — Ashcroft (−10 240 + 25 400), Wenig (−30 000 + 26 000), Chahiri, Vigaray, Meyerhöfer, Chader, Nieto, Serrano, Ilić and Niane (r2 −17 000 to −85 600). All 13 match. Chahiri is the closest call: he is 7 ability points above Pau's level 7, and capped his r2 would be +312.
