# Per-match stats and data — design

Date: 2026-11-11. Status: approved 2026-11-11. Format findings: `docs/format/matches.md`, and "27.0.0 match id and goal events" in `docs/format/fixtures.md`.

## Intent

The assistant can see fixtures, scores, each team's last lineup and season totals, but nothing per match. The user wants the assistant to:

- review the manager's last match;
- follow a player's form game by game;
- scout the next opponent.

The in-game references are four FM27 screens: match report stats, a player's match log, match events, and opposition analysis. FM27 only; FM26 is out of scope.

## What the save holds (recon, verified on 27.0.0 bytes)

| Data | Where | Coverage |
|------|-------|----------|
| goals: scorer, assister, minute, added time, side, type (goal, own goal, penalty, shootout kick, missed penalty in play: inference) | `FIXTURE_RESULT` events in `rgman/fix_man.dat` | every result with a match id: 1 027 of 3 246 on the 07-16 save. Quick-simulated matches have a score only |
| match id | `FIXTURE_RESULT` u32 after the version | same 1 027 |
| match object: per-player rating (×100), xG, shirt number, side, played / started / came on (two position bitmasks; "taken off" only in some of the human's matches) for every squad member; team xG and passes by third (anchors in one object only) | `match_stats_cfg.dat`, four zstd packs keyed by match id | 821 objects on 07-16. pack roles are an inference: `pks_0` (the manager's club's teams), `pks_1` (mostly league rivals) and `pks_2` (mostly continental) kept everything this season; `pks_3` (the rest) lost 206 matches of days 45–97 between the 05-09 and 07-16 saves. 570 objects (all of `pks_3`, 26 others) are small (8.6–14 KB) and have no match setup section; player records are in all |
| team totals: passes (and completed by third), tackles, headers, blocked shots, clear-cut chances, corners, fouls, yellow cards | match object, fixed offsets before each side's first player record (found in #5) | every stored object whose stats pass the parser's checks (all 821 on 07-16) |
| not located yet | match object | shots, shots on target, possession, distance, progressive passes, OPPDA, red cards, which player was booked, substitution times, minutes played |

Consequences:

- Goals and match objects exist only for results with a match id; inference: matches the game played in full detail (the manager's nation's competitions, friendlies, continental competitions). A foreign opponent's domestic league is probably quick-simulated, so for those clubs the assistant gets results only (inference).
- Season-change pruning is unknown. Tools must say when a match has no stored data rather than guess.
- Every squad member has a record; unused ones are rated 670 or 640. "Played" comes from the two position bitmasks (44 of 44 on the 07-15 match), never from the rating: 268 records of players who played are rated exactly 670.
- "Taken off" shows only in 23 of KŽ's 29 matches and nowhere else. Tools report it only when a side's taken-off count equals its came-on count, and otherwise mark it unknown.

## Goals

- `get_match`: one match, by its fixture or as a team's last match. Score, competition, stage and date; goals with names and minutes; each player's rating and xG.
- `get_player_matches`: every stored match one player played in, for any team (club, B or youth team, loan club, national team, friendlies), newest first; default the last 10, with optional from/to dates. Each row: date, team, opponent, competition, rating, xG, goals and assists (user's choice, 2026-11-11).
- `scout_opponent`: the next (or a given) opponent's recent results, goals, standout players by rating, and its last lineup.
- Team totals, cards and substitutions in `get_match` once decoded.

## Non-goals

- FM26 saves.
- Shot maps, pass maps, momentum, xG timelines and 2D-view data.
- Quick-simulated matches beyond their score (the save has nothing more).
- The per-player 16-match rating list in `player_stats.dat`: match objects carry ratings with the match attached, so the list is not needed (the two were not compared).

## Anti-goals

- Inventing data: a missing object or field reads as "not stored", never as 0.
- Labelling an unverified field as fact; the server instructions mark inferences, as elsewhere.

## Decided

- Match id = the `FIXTURE_RESULT` match id.
- Decode at load, not on demand: `fm.Load` reads every object's player-record fixed fields into a compact read-only index by match and by person (inference: about 1 MB; decompressing all 821 objects takes about 50 ms), passed in like fixtures. No decoding after load and no state on `Game` that changes later: tool calls run concurrently and `Game` has no Close for a decoder. Unreadable data becomes a load warning. A reload swaps the whole `Game`, so it needs nothing extra.
- Gating: the match id and goal events on `FIXTURE_RESULT` version `0x1f`; `match_stats_cfg.dat` on its inner file version 6. The `0x1f` check is per result inside `parseFixtures` (which also reads `0x1c`), so "not decodable for this build" is a per-result value from `savefile`, not an error; FM26 saves stay silent, as `rulesWarning` does for pre-27 saves. An unknown `match_stats_cfg.dat` version gives `ErrUnsupported` and a load warning ("match data unavailable"), never "no goals". The pack file's first bytes are a counter, not a magic.
- Match data joins on fixtures. The "match data unavailable" warning keys on the raw fixtures being nil (`hasFixtures`, which also covers `Load` skipping `ReadFixtures` without a warning when competition keys are missing), not on `fix_man.dat`'s read error; `resolveFixtures` never returns nil and drops unresolved fixtures. Objects with no resolved fixture are dropped and counted in the warning.
- Person to player: `NewGame` collects the person indices of goal events and match records together with the lineup persons, before its player loop (the `lineupPersons` pattern), fills them in the same loop, and keys each player's match list by UID. `fm.Player` gets no person-index field (it must stay within 512 bytes).
- Missing data is a status enum on each result, never merged by `omitempty`: #2 ships `quick_simulated`, `goals_only` (match id, no object read yet) and `unsupported_build`; #3 replaces `goals_only` with `stored` and adds `pruned` (the pack was read and has no object for the id). When the pack data is unreadable, results with an id keep `goals_only`, so `pruned` never stands for a read failure. A real zero is a value, not a status.
- `get_fixtures` rows gain `match_id` and that status in #2, and `get_match` accepts `match_id` as well as `team_index` + `date` or a team's last match, so the id the model sees is usable.
- Goal type: raw enum in `savefile`, labels in `fm`; type 3 (missed penalty) is marked as inference in the server instructions.
- Goal persons the save does not decode as players are reported as unnamed, like `TacticPlayer.Unnamed`.
- No fog of war on match data (results and match reports are public in game; inference).
- Layering as elsewhere: `savefile` parses (pack container, object fixed fields, events), `internal/fm` joins with fixtures and names, `internal/mcpserver` registers tools.

## PR breakdown (re-cut after recon; approved 2026-11-11)

| # | PR | Base | Est. diff |
|---|----|------|-----------|
| 1 | Recon docs: `matches.md`, goal events in `fixtures.md`, this spec | INDEPENDENT | docs only |
| 2 | `get_match` with goals: parse match id and events in `savefile`; tool takes `match_id`, `team_index` + `date`, or a team for its last match; `get_fixtures` rows gain `match_id` and the status | INDEPENDENT | ~400 |
| 3 | Match objects: pack reader, player-record fixed fields decoded at load into an index by match id (the index by player lands in #4, its first user); `get_match` adds player lines (shirt, played, started / came on, taken off where known, rating, xG) | STACKED-ON #2 | ~450 |
| 4 | `get_player_matches` from stored objects plus goal events | STACKED-ON #3 | ~350 |
| 5 | Team totals verified on the 07-15 report (see the decision below) in `get_match`; the rest of the team block, per-player cards, substitution times and minutes in a later PR against a second in-game reference | INDEPENDENT (after #4 merged) | ~300 |
| 6 | `scout_opponent` composed from #2, #3 and `get_last_lineup`; tool or MCP prompt (see below) | STACKED-ON #3 | ~300 (tool) or ~80 (prompt) |

Changes from the approved plan:

- Goals ship first, on their own, from `fix_man.dat`.
- Player lines come from match objects, not the 16-match list.
- Who played, started and came on ships in #3 (taken off only where known); cards and substitution times move to #5 with the team totals.
- #5 ships only team totals in `get_match`, so it no longer needs `get_player_matches`; #6 needs only #2 and #3.

Decided by the user (2026-11-11): `scout_opponent` is an MCP prompt like `squad_review` and `scout_position`, telling the model to combine `get_fixtures`, `get_match`, `get_player_matches` and `get_last_lineup`; no new fm API.

Decided by the user (2026-11-11): #5 first ships the 8 team totals matched against the 07-15 match report (passes attempted/completed and completed by third, tackles, headers, blocked shots, clear-cut chances, corners, fouls, yellow cards); shots, possession, distance, progressive passes, OPPDA, per-player cards, substitution times and minutes follow in a later PR once a fresh save and Advanced Stats / player stats screenshots are available.

## Left to the builder

- Tool parameter names and JSON shapes, following the existing tools (`get_fixtures`, `get_last_lineup`).
- How to find player records (they sit after variable-length strings, record length is not decoded, and 570 objects have no setup section). Acceptable: walk the header grammar if it decodes, or scan for the record shape (tag bytes 2 at offsets 0, 7 and 12, side 0 or 1, a known person index, a rating of 300–1000 at +136), checked by expecting exactly 11 starters per side. The shape tests alone find a 12th "starter" on 4 of 1 642 sides (objects with junk bitmasks), so the scan also needs the known-person test or a check on valid position bits. Team xG is anchored only in the 07-15 object, so it is not a general check until located.
- Caps on pack count, entry count and object size, as other parsers do.

## Calls made without the user (reversible)

- #5 starts with its own research step on a fresh save copy taken right after a competitive match, with the user's Advanced Stats and player stats screenshots of that match. It ships only fields verified in bytes and in game; anything not found stays in the docs' open questions, and if nothing is found #5 ends docs-only.

## Verification

- Unit tests on synthetic buffers for the pack container, events and record fields.
- Integration tests on `data/test_save_27_homegrown.fm`: the 07-15 match gives the five goals, 23 + 21 player lines, and both xG sums (1.98, 2.18).
- In-game: the 07-15 match report screenshots (2026-11-11) for #2 and #3. #5 needs the Advanced Stats tab and the player stats columns of one match, captured when #5 starts.
