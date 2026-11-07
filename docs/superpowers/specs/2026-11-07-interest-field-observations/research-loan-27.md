# FM27 (27.0.0) loan interest — research note for PR B

Binary: `$TMPDIR/re27/gp_arm64`, 27.0.0 arm64 slice (sha256 `180ec7c1…`), vmaddr == file offset. Disassembly with `/usr/bin/objdump`; region dump `$TMPDIR/re27loan/r1f.s` (0x1f50000–0x1fa0000). Save values from `/tmp/claude-501/kz_v03.fm` (game date 2026-02-11, KŽ first-team reputation Rt = 3200). Labels: **verified (code)** = read in the 27.0.0 disassembly; **verified (save)** = computed from the save with the repo parser; **inference** = my reasoning.

## Bottom line

- The two decoded loan terms are confirmed in 27.0.0 and both are 0 for the four refusing regular starters. The game itself gives them 0 here, so it is not an `fm` bug. Removing the `max(pt7, 0)` clip would change nothing.
  - Playing time at the loan level (`0x1f9af3c`): `min(d·CR − g, 0)` for d < 0. g is very negative when KŽ is far below both the player and his club, so the result is 0 (verified, code + save).
  - Club reputation gap (`0x1f79b0c`): skips the club term d for kinds 3 and 8 (loans). Only the world-reputation term counts, and it is 0 for these players (verified, code).
- **What separates the four refusals from Voitinovičius was not found.** The loan path calls about 30 factors. The best candidate is the "talented group / club level" factor `0x1f76b80` (reason 1). It runs for loans and gives clubLevel(KŽ) − playerLevel. playerLevel comes from `0x14ea848` (in-memory, not decoded). That it decides these cases is inference only.
- **Already on loan**: the *filter* rule (`PERSON_INTERESTED_LOAN_FILTER_RULE`) rejects a player whose first extra contract slot is an active loan (`0xcffffc`; verified, code). The one exception is a type-18 relation item on the loan club. Whether the agent chat applies this is unknown.
- **Unavailable for loan** (CONTRACT `+0x57` bit 6): no test of that bit was found in the interest code (0x1f50000–0x1fb0000) or in the parts of the loan rule I read. Not resolved.
- **Agent chat**: the caller was not identified. There are 139 call sites of the interest function. Loan calls use levels 7, 8, 9, 10 and 0.

## Call chain (27.0.0 addresses)

| Role | 26.3.2 | 27.0.0 | Evidence |
|---|---|---|---|
| Loan filter rule eval (vtable slot 34) | `0x16949bc` | `0x2325c10` | typeinfo `34PERSON_INTERESTED_LOAN_FILTER_RULE` at name `0x53b5eaf` → typeinfo `0x55e7c00` → vtable `0x55e7ad8`; slot 34 differs from the transfer rule (`0x23264f4`) |
| Interest service | `0x1283a90` | `0x1f6eebc` | `[0x56d2d08]` vtable +0xf0; vtable at `0x55def88`, typeinfo `18GAME_ACTUAL_PLAYER` |
| Loan path (kinds 3, 8) | `0x128600c` | `0x1f73034` | entry `0x1f6f57c`/`0x1f6f63c` → `0x1f6f7d4` |
| Transfer path, other club (kinds 4, 7, 9; 2, 6 with kind arg) | `0x1285498` | `0x1f71b54` | entry `0x1f6f744…0x1f6f784` |
| Free-agent path (no current club) | `0x1284858` | `0x1f70254` | entry `0x1f6f7fc…0x1f6f82c` (other fork's scope) |
| Playing-time wrapper (reasons 125 / 4) | `0x128d22c` | `0x1f7a454` | reason 125 at `0x1f7a910` |
| Playing time pt(L, kind) | `0x12ad438` | `0x1f9af3c` | |
| Club reputation gap r2 (reason 2) | `0x128c898` | `0x1f79b0c` | step thresholds −4001/−3001/−2001/−1000 at `0x1f7a15c…` |
| Talented group / club level (reason 1) | `0x1289bf0` | `0x1f76b80` | −50 000 branch (age ≥ 30, WR ≥ 6250) `0x1f76fe4`; reason 1 recorded at `0x1f78604` |
| Just-joined (reasons 43 / 59) | `0x128f550` | `0x1f7d064` | conditional call `0x1f73834` |
| Loan spell (reasons 127 / 133 / 145) | `0x128ba6c` | `0x1f78ccc` | |

### Loan rule eval `0x2325c10` (verified, code)

```
v = [x22 + 0xda]   (x22 = rule-context object vt+0x658; doc's "probably the human's CA")
threshold = (200 − v) × 20 + flvl          (flvl: Doubtful 0 / Persuadable 10 000 / Interested 20 000)
pass1: interest(person, club, kind 3, level 7) ≥ threshold / 2          (0x2325ec8…0x2325ee4)
pre-checks before it: 0x2d3d194(…) false, 0xcffffc(person) false (already-on-loan, below),
       a club squad-size check ([club+0x108]+0x207 vs a count; byte +0x205 bit 0)
then:  x = GAME_ACTUAL_PLAYER vt+0xd0 (0x1f6b364)(person, his team, club, 0, 1)
       fail when x > 2 999 999 or 0.2 × 0xd01ef0(person) < x      (meaning of x not decoded; inference: a fee or wage figure)
then:  0x2d5c454(…) and 0x2d3dd04(…, 3, 1, …, 1) must pass         (not decoded)
```

### Interest entry kind dispatch `0x1f6eebc` (verified, code)

- Kinds 3 and 8 → loan path `0x1f73034`, only when the player has a current club. With no club → −100 000.
- Kinds 4, 7 and 9 → `0x1f71b54` when his club ≠ the target club. With no club → free-agent path `0x1f70254`.
- Kinds 2 and 6 → `0x1f71b54` with kind 2. Kind 5 → `0x1f71b54` with kind 5, then a post-check `0xd316fc`. Kind 17 → separate branch. Kind 1 → `0x1f73034` with kind 1.
- Gate: a person flag at `[person + 0x1a]` bit 2 or an earlier date check → −100 000.

## Q1 — club gap and playing time in the 27.0.0 loan path

**r2 `0x1f79b0c`** (verified, code):
- T = min(4·Rt, 36 000), with 6000 when the target has no first team; C likewise for the current club.
- The club term `d = T − C` (×6/×5/×4/×2 steps) is computed only when kind ∉ {3, 8} (`0x1f7a144…0x1f7a150`: w9 = 0 for kinds 3 and 8).
- The world term is applied for all kinds: `w = T − min(4·WR, 36 000)`, ×8 below −500, only negative (`0x1f7a1ac…0x1f7a1d4`). WR is `[person+0x25a]`.

So the loan gap is min(w, 0), as in 26.3.2 and as `fm` does. For the four starters, WR is 2750–3100, so 4·WR ≤ 12 400 < T = 12 800, and w > 0 → 0 (verified, save).

**pt `0x1f9af3c`** (verified, code). Arguments: x0 = L, x1 = contract, x2 = person, x3 = target club, x4 = kind.

1. L == 1 → returns −100 000.
2. m = max(LEVEL[b] (table `0x53b053c`), scoreLevel `0xfec7f4`). Open ring items with status ≥ 2 cap m at 8.
3. want:
   - m == 6: 7 when GK rating (vt+0x390) > 14, else 6.
   - m == 4 at another club: 7.
   - otherwise: m.
4. Kinds 3 and 8 with age > 20 (`0x1f9b540…0x1f9b550`): want = max(want, 6).
5. d = L − want. At another club with L == 6 and d < 0, d drops by 1. L ≥ 9 and d ≥ 2 has a squad-rank adjustment `0xd00f64`.
6. CR = `[person+0x258]`. Rt = target first team `+0xa8` (1500 when there is none).
7. g = 5·(Rt − CR):
   - when g < 0, the player has a contract and his club ≠ the target: g ×5 more when the target's division `+0x1aa` ≥ 2 and his club's is 0 (top tier);
   - then g += 10·min(Rt − Rcur, 0), with Rcur = his club's first-team `+0xa8` (1500 when there is none).
8. Result:
   - d ≥ 1: max(0, g + d·CR);
   - d == 0: 0;
   - d < 0 and L ≤ 6: ×10/×30/×5 variants (WR, GK and CR tests), then the same as below;
   - d < 0 and L > 6: **min(d·CR − g, 0)** (`0x1f9b828`, `0x1f9b854`). For his own club the value is ×10 when `0xff1ed8(contract, 400)`.

Identical to `fm`'s `playingTimeValue`, except the division ×5 on g, which `fm` does not model. That term only makes g more negative, so it raises the d < 0 value toward 0 and never lowers it.

Values on the save (verified, save; `fm`'s inputs):

| Player | age | CR | WR | m | his club FT rep | g | pt(L 7) |
|---|---|---|---|---|---|---|---|
| Lasickas (Rijeka) | 28 | 5600 | 3100 | 8 | 5800 | −38 000 | min(−5600 + 38 000, 0) = **0** |
| Kažukolovas (Astana) | 25 | 5500 | 2750 | 8 | 6100 | −40 500 | **0** |
| Kučys (Celje) | 22 | 5250 | 2750 | 8 | 5550 | −33 750 | **0** |
| Tutyškinas (Celje) | 22 | 5250 | 2750 | 8 | 5550 | −33 750 | **0** |
| Voitinovičius (Benfica) | 19 | 5100 | 3100 | 5 (prospect) | 8150 | −59 000 | want 7 → d 0 → **0** |
| Abdullahu (Kickers) | 17 | 4000 | 1837 | 5 | 3500 | −7 000 | transfer, not loan |

So the game's own playing-time term is 0 for both groups. It cannot be what makes the starters refuse.

## Q2 — what separates the four refusals: not found

Loan path `0x1f73034`, in call order (verified, code: addresses and call order; formulas only where stated):

1. Gates:
   - person `[+0x19]` bit 7 → −100 000;
   - `0x1f73e34` (reasons 53/…) can return a fixed score;
   - a club-relation scan for type 0x12 → `0x1f7340c` path (+100 000 / −100 000 by a person-type byte);
   - `0x1f74004` / `0x1f742dc` / `0x1f74424` (reasons 18/55/105; early return on non-zero);
   - `0x1f74600` (start score; returns early when < −1999);
   - a relation item with value ≤ −50 → −100 000 (reason at `0x53b0130`).
2. Additive factors on the running sum `[sp+0x2c]`, in this order:

   | Fn | Reason / effect |
   |---|---|
   | `0x1f75e18` | not read |
   | `0x1f75fb4` | not read |
   | `0x1f76b80` | talented group / club level, reason 1, below |
   | `0x1f78f34` | not read |
   | `0x1f79b0c` | r2 (0 here) |
   | `0x1f78ccc` | loan spell (127/133/145) |
   | inline | reason 139: +2·CR for kind 3, sum > −100 000, age ≤ 23 and the two clubs' categories from `0x2b49e7c` matching a table (`0x53b03bc`) |
   | `0x1f9c180` | a subtraction, reason from the callee |
   | `0x1f7a454` | playing time (0 here) |
   | `0x1f7d064` | just-joined; only under a flag/club condition |
   | `0x1f7ad08`, `0x1f8208c`, `0x1f7b158`, `0x1f7bb98`, `0x1f7c4b8`, `0x1f7d9ac`, `0x1f7db90`, `0x1f7e060` | not read |
   | inline | reason 17: +2·CR when `0xd31054` |
   | `0x1f7e1a0`, `0x1f7e374` | `0x1f7e374` only when `0x14fdcb0` is false |
   | `0x1f7e700` | not read |
   | inline | reason 41: {1, …}·CR by `0xe5ee10` |
   | `0x1f7eac0` | not read |
   | inline | reasons 76/135: +3.5·CR when club finance `+0x8a5` ∈ {1, 2} |
   | inline | reason 49: +8·CR when `0x2d29eec` |
   | `0x1f7f0ac`, `0x1f7f740`, `0x1f7fb60`, `0x1f7fd74`, `0x1f80958` | not read |

3. sum < −10 000 → return. Kinds 3 and 8 continue with `0xcda694(person, …, 25, …)` (not read).

Talented group / club level `0x1f76b80` (verified, code for the shape; partly read):
- clubLevel = min(4·Rt, 25 000) (6000 without a first team).
  - ×0.8 when the target `+0x16a` == 3 and the current club's ≤ 2.
  - Floored at 8500 (10 000 / 13 000 for his own club).
- Loans (kind 3) skip the `0x1f9ad4c` adjustment.
- playerLevel = `0x14ea848(person, …, mode 2 or 3)`, capped 36 000/37 000/38 000 in some branches.
- value = clubLevel − playerLevel (`0x1f782b4`), with ×5 and −10·CR variants.
- The value is capped at +20 000 at age ≤ 21 with WR < 5251.
- There is a −50 000 branch for age ≥ 30, WR ≥ 6250 and a lower-tier target, and a −10 000 branch when the current club's rep ≥ s·Rt and m ≥ 7. That branch applies only at L 3–4, so not here.
- `0x14ea848` is not decoded. It looks like an ability or value figure from in-memory state (inference).
- This is the only factor read so far that can go strongly negative for a better player joining a weaker club on loan. For KŽ, clubLevel = 12 800.

Inference: the four refusals probably come from clubLevel(KŽ) − playerLevel and/or factors not read. Testing that needs `0x14ea848`. No threshold was fitted.

## Q3 — already on loan, unavailable for loan

- **Already on loan** (verified, code): `0xcffffc(person)` is called by the loan filter rule before the interest check (`0x2325de4`; non-zero → fail).
  - It reads the person's extra-contract array `[base+0x80][0]`.
  - When that contract's vt+0x90 == 1 (inference: a loan contract), it returns 1 (fail) in these cases:
    - its vt+0x98 is false;
    - its team is missing or national;
    - the team's club has no relation item (club `+0x118` list) naming that club with type 0x12 (18).
  - Otherwise it returns 0.
  - So the filter drops players who are on loan, unless a type-18 club relation applies. `fm` already has the loan contract (`Player.Loan`). The type-18 club relation items are in the CLUB relation vector, partly decoded in docs (clubs.md / research §5).
- **Unavailable for loan** (`+0x57` bit 6): not found.
  - Interest code tests `+0x57` bit 3 (asked to leave / listed) and contract vt+0x128 bits 0 and 5.
  - No bit-6 test appears in 0x1f50000–0x1fb0000.
  - Not found either in the first parts of the loan rule's extra checks (`0x2d3d194`, `0x2d5c454`).
  - Inference: it may sit in the club-side loan acceptance, not in player interest. Not resolved.

## Agent chat

Not identified. I scanned the whole text section for `[0x56d2d08]` vt+0xf0 calls and found 139 sites. Kind/level pairs include (3, 7) at `0x2325edc` (the filter) and `0x28cad1c`; (3, 8) at `0x19d423c`, `0x1fa0638/0734/0814`, `0x210cb74`, `0x2b0afb0`; (3, 9) at `0x22dece4`; (3, 10) at `0x23c2de4`, `0x2ab5400/54d4`, `0x2ac90e4`; (3, 0) at `0x28bd9a0`, `0x2d276dc`. Which one the "ask agent about interest" chat uses was not traced. At L = 8 the starters' pt is 0 (d = 0). At L = 10 it is max(0, g + 2·CR) = 0 for all four.

## Save fields

| Need | Parsed by internal/savefile? |
|---|---|
| CR, WR, LEVEL inputs, his club's first-team rep, Rt | yes (used by `fm` today) |
| Competition division `+0x1aa` of target/current first team (pt ×5 term) | not checked; COMP `+0x1aa` is listed as U in research-factors |
| Loan contract (already-on-loan) | yes (`Player.Loan`) |
| Club relation items type 18 | partly (club relationship vector, research §5) |
| playerLevel `0x14ea848` (talented group) | no (in-memory / not decoded) |

## Blockers

- `0x14ea848` and about 20 loan-path factors are not read. The deciding factor for the four starters is unknown.
- The agent-chat call site, and with it its offered level and threshold, is unknown.
- The unavailable-for-loan check was not located.

Scratch files: `$TMPDIR/re27loan/` (`r1f.s`, `pt.s`, `r2.s`, `f76b80.s`, `f7a454.s`, `loanpath.s`, `loanrule.s`, `interest_callers.txt`, `immscan`). The scratch tests in `internal/fm` were deleted, and `git status` is clean.

## Follow-up: `0xcffffc` (already on loan), pinned down

### 1. Which club's list is read, and what the type-18 item must name

Instructions (27.0.0, verified code):

```
cffffc: x8 = person + [vptr−0x20]          ; virtual base (ACTUAL_PERSON part)
d00014: ldr x8, [x8, #0x80]                ; extra-contract array
d00018: cbz x8 → return 0
d0001c: ldr x19, [x8]                      ; slot 0
d00020: cbz x19 → return 0
d00028: blr [vt+0x90](x19) ; cmp w0, #1 ; b.ne → return 0      ; only a LOAN_CONTRACT
d00040: blr [vt+0x98](x19) ; cbz w0 → return 1
d00050: ldr x8, [x19, #0x10]               ; the contract's team
d00054: cbz x8 → return 1
d00058: ldrb w9, [x8, #0x2f] ; cbz w9 → d00068 ; else return 1   ; national/other team → 1
d00068: ldr x8, [x8, #0x30]                ; C = that team's club
d0006c: cbz x8 → return 1
d00070: ldp x9, x10, [x8, #0x118]          ; C's CLUB_LINK list
d00080: ldr x11, [x9] ; ldr x12, [x11, #0x8] ; cmp x12, x8 ; b.ne next
d00090: ldrb w11, [x11, #0x30] ; cmp w11, #0x12 ; b.eq → return 0
...     return 1
```

The loan rule (`0x2325de4: bl 0xcffffc; tbnz w0 → fail`) drops the player when this returns 1.

- **The list read is C's**: C is the club of the loan contract's team (`[loan+0x10]+0x30`).
  - That `[contract+0x10]` is the contract's team is an inference. It is the same field pt (`0x1f9b1c8`) reads as "his club" for the main contract.
  - For a LOAN_CONTRACT that team is the loan club. `savefile` and `docs/format/contracts.md` take the loan slot's team as the club he is on loan at.
- **The type-18 item must have C itself as its second party** (`link+0x8 == C`). Neither the parent club nor the target club (the human's club) is tested.
- So he is exempt only when the club he is on loan at is the second party of some type-18 link, whatever the other party is and whoever asks.
- Inference: the second party is the junior side of an affiliation, so a player on loan at a feeder of some club stays eligible. Not confirmed.

### 2. vt+0x90 and vt+0x98; future loans

Verified code (vtables found through RTTI `N2db…CONTRACTE`):

| Class | vtable | vt+0x90 | vt+0x98 |
|---|---|---|---|
| FULL_CONTRACT | `0x55abd68` | 0 | 0 |
| LOAN_CONTRACT | `0x55b54e8` | **1** | **1** |
| TRIAL_CONTRACT | `0x55a0d28` | 2 | 0 |
| B_CLUB_CONTRACT | `0x559b768` | 3 | 0 |
| YOUTH_TRIAL_CONTRACT | `0x55a0af8` | 4 | 0 |
| BASIC_CONTRACT | `0x559b9f8` | 5 | 0 |

- vt+0x90 is the contract type, the same enum as the save's slot type byte (0 FULL … 5 BASIC, contracts.md).
- vt+0x98 is a constant "is a loan" that returns 1 only for LOAN_CONTRACT. It is **not** an active/started test. The `cbz → return 1` branch can never be taken for a loan.
- The function reads **only slot 0** of `+0x80`. The save writes the slots in the order main, loan, B club, future, B club 2 (contracts.md). That `+0x78` = main and `+0x80[0]` = loan slot is an inference from that order and from the type check.
- **Future loans**: a future, agreed move sits in the future slot (`+0x80[2]`, inference), which this function never reads.
  - On `kz_v03` (verified, save): all 2 540 loan-slot contracts have start and joined dates on or before the game date (2026-02-11), so the loan slot holds loans already running.
  - A loan agreed but not started therefore does not trigger the block (inference, consistent with the save).
- `fm`'s `Player.Loan` is exactly the loan slot, so "Loan != nil → loan out" matches `0xcffffc`, apart from the type-18 exemption.

### 3. Where CLUB_LINK (club `+0x118`) is saved; type 18

In-memory object (verified code): pool `CLUB_LINK` (`0xd66724`: object size 0x38, 9 221 in `memory_pools.dat`). The creator `0x107a674` sets:

| Offset | Field |
|---|---|
| +0x00 | club A |
| +0x08 | club B |
| +0x10, +0x14, +0x18 | u32 |
| +0x1c | u32 flags (bit 2 tested by the loan path, bit 30 = skip) |
| +0x20, +0x24 | u32 |
| +0x30 | u8 type |
| +0x31 | u16, default 0x3232 |
| +0x33, +0x34, +0x36 | u8 |

Option strings near the link code (`CLUB_CAN_ADD_LOANS_TO_CLUB_LINK`, `…_FIRST_OPTION_ON_PLAYERS_…`, `…_FOREIGN_YOUTH_ACADEMY_…`) show these are affiliations ("feeder clubs").

**Save location: not found.** Ruled out:

| Candidate | Why it is not CLUB_LINK |
|---|---|
| CLUB `u8 n × 25 bytes` | CLUB_OTHER_INCOME: 16 025 items on kz_v03 = that pool's count |
| CLUB `u8 n × 12 bytes` | CLUB_REGIONAL_DIVISION: 45 595 items = that pool's count |
| TEAM_CONTAINER `u16 n × 28 bytes` | 2 528 items: `i32, i32 (often −1), u32, u32, 2 bytes, date, date, u16`. No second-club/type layout |
| TEAM_CONTAINER `u8 n × 16 bytes` | the `+0x48` relationship vector (rivals) |
| `feeder_man.dat` | (2 032 bytes) groups of `u32 n + n × u32 club index`, 155 groups; no type bytes. Inference: affiliation groups |
| game_db table head with count 9221 | none after a table end marker |

The loader, which would build CLUB_LINKs from the save, was not traced. `0x107a674` is called from `0xe3cda0`, `0xe40d2c`, `0xe417b0`, `0xfc21f4`, `0x1ae05e8` and `0x267e0f8`; none was read.

**Type 18:**
- No UI string. The link-type formatter `0xe274fc` names only types 4, 6 and 8 (text ids `0x783ac`/`0x783ad`/`0x783ae`).
- Use in the loan path (verified code, `0x1f73228…0x1f73484`): a type-18 link between the player's current club and the target club, with flag `+0x1c` bit 2 set, makes interest **+100 000** when his age ≤ the target first team's age byte `+0x2c` (21 for two uids) and **−100 000** otherwise.
- Types 17 and 24 are skipped in that scan and in `0x107a70c`.
- Inference: type 18 is an affiliate link with a loan arrangement and an age limit.
- **Whether any club on kz_v03 has a type-18 link, and whether any loanee would be exempt: not checked.** The links' save location is unknown, so the parser cannot read them.

### 4. Transfer filter

- Only the loan rule applies it (verified, code). Callers of `0xcffffc` across the whole binary: `0x2325de4` (loan rule eval), plus `0x187cec`, `0x3b242c`, `0x3b31e0`, `0x3f7d1c`, `0x3fbb20`, `0x5521e8`. Inference: the last six are UI code.
- The transfer rule eval `0x23264f4` does not call it. Its pre-checks are `0x1f46e24` and `0x2d3d194(…, 1, …, 1)`.
- The interest function `0x1f6eebc` and the loan path `0x1f73034` do not call it either. In those, `+0x80` is only used to pick the current team.
- So the block is part of the "Player's interest" loan filter, not of the interest score. Whether the agent chat applies it is unknown (its call site was not found).

### Implementation notes for PR B (inference)

- "On loan (Player.Loan != nil) → loan out" reproduces `0xcffffc` except for the type-18 exemption, which `fm` cannot read today. Name the reason after the filter, e.g. `already_on_loan`.
- Transfer verdicts must stay unchanged.
- In the docs, say that the rule comes from FM's loan filter, not the interest score.
