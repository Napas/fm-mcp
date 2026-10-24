# Backlog

Open work found in the 2026-10-20 testing session (save KŽ.fm, FC Metz, 2035-12-02). Ground truth from game screenshots is noted where we have it.

## Next up

1. **Season stats left open** (`docs/format.md`, "rgman/player_stats.dat"). The server serves every detail field named from code, the goalkeeper split and the `internationals` row. Still open:
   - d42–d44: a shot subtype, a subset of it and goals from it; no UI caller found.
   - Labels not checked in game. Screenshots that would verify most at once (Profile → Stats → Season Stats, 12-09 save):
     1. Done (2035-12-28): Sami's goalkeeping panel matches every keeper field; xG prevented = d26 / 100 − conceded (`docs/format.md`, "Ground truth 4"). Still open for keepers: Save % and Expected Save % columns (d32 as shots on target faced).
     2. Anton Matković (uid 2 000 301 456), Ligue 1 row, attacking and possession columns (not in the default panel; needs a view with these columns): Fouls Against 11, Pressures 84/20, Open Play Key Passes 31, Open Play Crosses 16/10, Headers Lost 38, npxG 4.26, xG Overperformance +2.74, Shots/Goals Outside Box 7/0, Free Kick Shots 0, Won 8 Lost 1, Team Goals 31.
     3. Francesco Pio Esposito (uid 2 000 146 883), international competitions and the season selector: which season the "International League" and "European League Div. A" rows belong to, and whether friendlies appear. Resolves slot 4 against slot 7 and when slot 4 is reset.
     4. Done (2036-01-01): Nduquidi's and Vassiljev's Ligue 1 panels match; d20 verified for outfield players. The panel's "Shots Blocked" is d58, so outfield d32 (blocks) stays unchecked. Vassiljev's slot 4 (6 matches) is not the visible Nations League rows; the top of his competition list would name its competition.
     5. Matković's career history, Ligue 1 2034/35 row: average rating 7.07 over 27 rated games would confirm the last-season snapshot (d18, totals offset 16).
   - The source of the in-game per-competition rows (Champions League apart from Europa League, Esposito's "International League"): not in `player_stats.dat`; candidates `rgman/comp_<uid>.dat` match records and `fix_man.dat`.

## Assistant feedback (2026-10-23)

Requests from the LLM assistant using the server. Its claims are marked observed or inference as it gave them.

### Cheap wins (data already decoded)

Done 2026-10-23: `joined_club`, per-90 sort and filter, `detail`, position ratings under 10 dropped, height, trait, personality, release clause and `min_role_ability` filters. Preferred foot is covered by `min_attributes` (`LeftFoot`/`RightFoot`); FM's foot labels are not derived.

- **Out-of-possession role ability in `get_tactic`**: only the in-possession role has a number (a DC shows 109 as Overlapping Centre-Back, nothing for his out-of-possession role). Needs the decoding listed under Tactics ("Role ability stars": out-of-possession role ability), so not purely server-side.

### New data domains (need reverse engineering)

- **Fixtures, results, league tables**: done 2026-10-24 (`get_fixtures`, `get_table`; `docs/format/fixtures.md`, `docs/format/competitions.md`). Still open:
  - The second row list of a LEAGUE_STAGE (same rows, other order) and the row fields marked guess.
  - Fixture events (16-byte items), the unknown FIXTURE base bytes, and the 14.5 MB after the fixture lists in KŽ.
  - The LEAGUE_STAGE gap: 42 of 645 (KŽ) and 2 of 240 (test) league stages have no table found; live leagues are covered except the South American U20 Championship.
  - Out-of-range stage values in some tables (2025, -24320, -248).
  - Score byte 3 outside second legs.
  - The match date of `get_last_lineup` from the fixture list.
  - Different tables served under the same label (38 pairs in KŽ: MLS conferences, Vertu Trophy north/south groups, Amazonas State Championship turns) need a disambiguator such as the section or turn.
  - In-range garbage in the stage-name field would pass unnoticed; cross-check it against the stage indices of the competition's fixture keys.
  - "The manager's team by default" lives in both `fm` (`Game.queryTeam`, fixtures and tables) and `mcpserver` (`squadIn.club`, other tools); move the others onto the `fm` helper.
- **Squad fitness and availability tool**: one view of injuries (type, return date), suspensions, yellow-card accumulation, condition and sharpness for the pre-match check. Condition, sharpness, current injuries (`injury`, `injured_until`) and season yellow cards are served; suspensions and per-competition card accumulation are not decoded.
- **Career history**: previous seasons' stats, transfer history and fees paid, for form trends and resale decisions. Only the current season (plus the last-season snapshot, d18) is served.
- **Staff**: no tools for coaches, scouts or physios. A staff search with attributes (Working with Youngsters, Judging Potential) for hiring. Non-player person records are not decoded (see "Favoured personnel").
- **Morale, happiness, promises**: only playing time is exposed. Decodability unknown.
- **Eligibility**: second nationality, EU status, homegrown status, for non-EU limits. FM26's current Ligue 1 rule not checked.

### Analytical layer

- **Diffing across saves**: index several saves and report CA/PA and attribute changes per player over time, for youth development. The assistant rates it high value, moderate effort (inference).
- **Squad summaries** (age profile, wage efficiency, expiring contracts): `search_players` already covers these; the only gain is fewer tokens. Low priority.

## Player data not decoded

- **Trait bits 48 and 53**: no label in the formatter; bit 48 is set on 271 players.
- **Reputation level for women and player-coaches**: the server leaves `reputation_level` out for them (`docs/format.md`, "Reputation level"). Women need the women's national-team container's factor (containers 251–501, not read); player-coaches need the staff rule `0x2f27b8c` and the ACTUAL_NON_PLAYER reputations (8 747 player-coaches on 2035-12-14).
- **Recalled loans**: Mamadou Diouf is shown by the game in the Metz squad ("Unr", FC Metz listed first) while the save's loan to Grenoble still stands.
- **Injuries**: check in game whether Aziz Diakhaté (Metz II, bruised ankle since 30 Nov, latest return 3 Dec) shows as injured on 2 Dec. The server reports him injured until 3 Dec, on the evidence in `docs/format.md` ("injury_manager.dat").
- **Favoured personnel**: the selection rule is decoded (`docs/format.md`, "Relationships and favoured clubs": kind 3, type 1, value ≥ 50; reason 6 manager, 10 friend, 8 staff), verified on 5 players. Serving it needs the names of non-player persons (managers, staff), whose records are not decoded. Open: Enzo Fernández's header says 5 while 4 items qualify.
- **Expected playing time**: the contract screen's "Expected:" line (Bernardi Important Player, Assongo Squad Player on 2035-12-14). Needs the squad-ability ranking `0x2f9e580` (`docs/format.md`, "Actual playing time"), not decoded.
- **Actual playing time, partial rule** (Lobede, 25, shows Surplus to Requirements on 2036-01-01: the youth test fails for him) (`docs/format.md`, "Actual playing time"). The server leaves the field out where these undecoded parts could change it: the level-7 match rule (Squad Player or Impact Sub); the `0x3780194` branches read from code but not checked in game (Breakthrough Prospect by an ability test at level 4, Impact Sub for players 33 and over at level 6, Fringe Player via a competition value); the youth rule up to age 25. It also leaves it out away from a club's first team, where scores are not checked in game. Not applied, so they can still change a served value: the youth-contract, promise and team clamps, and B Team Regular for players over 23 with a club-list (`+0x118`) entry that also caps results at Emergency Backup. In-game checks: Bassirou Sow (24, FC Metz 2: −100 score, Surplus by the rule), whether other reserve players with a −100 score (Vallet, Sanogo) show Surplus to Requirements, and a loanee's headline.
- **Star ratings** (scouted CA/PA stars), which would let ability searches work under fog of war.

## Club data not decoded

- **Original Budget**: check in game which CLUB_FINANCE field the Recruitment Budgets screen's "Original Budget" shows: offset 0 (£5 634 364 = €6 519 694 for Metz) or the field holding exactly €6 500 000 (`docs/format.md`, "Club budgets"). Compare on a club or date where the two differ by more than €0.1M. Not served until then.
- **Transfer spending this season**: the Recruitment Budgets screen shows "Spent this season" (€7.5M for Metz on 2035-12-14). Not decoded; offset 0 minus the current transfer budget is not it (£5.63M − £8.33M is negative).
- **Current wage bill including staff**: for Metz on 2035-12-14 FM shows €16.43M p/a available plus €47.93M p/a currently spending. The server reports only the total (`wage_budget`); staff contracts are not parsed, so it cannot compute what is left. Player wages alone are about 63 % of the budget (£672 499 of £1 069 817 a week on 2035-12-02).

## Tactics

`get_tactic` returns the human's current first-team tactic (`docs/format.md`, "tactics_man.dat"). Still open:

- **Other teams of the club** (FC Metz 2 and two more entries): the structural walk in `savefile` already reads every team entry of every human (19 023 and 38 394 hold a copy of the first team's tactic, 86 128 none), but keeps them private. Serving them needs a public accessor over the entries across humans; with it, pick the manager's first-team entry (across humans) instead of assuming the first human's first entry, and lift the multi-human `ErrUnsupported`.
- **Team and player instructions**: probably the tactic header's bitset and the per-role instruction bitsets.
- **Other tactic slots**: each team entry holds one tactic in every save seen; how many does the game show, and is the i32 before the list the active one?
- **Role ability stars**: role ability is decoded and served (`role_ability`; `docs/format.md`, "Role ability"), but not the stars the game shows for it. Within one player the order matches (27 of 27 roles); across players a higher value never has fewer stars in only 28 of 38 samples, and exact stars match 0 of 38. The stars follow a per-player, time-varying anchor (Ly's Channel Forward went from 3.5 to 4 stars between 2035-12-09 and 12-14 with the same attributes, current ability and role ability); hypothesis: the staff's assessed current-ability stars. Find where that assessment lives (cached star ranges at `0x3837ec`?) and the star mapping. Also open: out-of-possession role ability (position handling `0x48bec90`, best-role set), and why Rodríguez's MC list showed Wide Central Midfielder but not Central Midfielder (both are in the engine's MC set).
- **Last-match details**: `get_last_lineup` serves them. Open: the match date (not in the record; `get_fixtures` has it, not yet joined), and the record's 26 bytes after the shirt numbers (bitfield and unknown values).
- **Set-piece routines**: skipped structurally in every team entry (`docs/format.md`, "Rest of the team entry and human"), not mapped: which routine is which set piece, the prefix bytes, and the per-routine byte that rose by 24–25 in a week for Metz (familiarity?). Check in game, then serve them.

## Player interest — open

- **Relegation exemption**: `0x12ade1c` can exempt a player from `recently_left_your_club`; not decoded. The model assumes no exemption.
- **"Wanted by a club" (Wnt) bonus**: not decoded. Open cases: Stáňa and Maxsø.
- **Free-agent path** (`0x1284858`): not decoded enough to separate Musongi, David Silva and van Hattum; free agents are `uncertain` unless a gate applies.
- **Squad ranking** `0x2f9e580`: sets the offered level; not decoded.
- **Vega and Lau Jiménez**: in-memory inputs the save does not show (contract events beyond the ring, person `+0xa8` items, negotiation and scouting records) can exempt or block them; Vega passes FM's loan filter with a loan sum of -18 435.
- **Ages outside 24-25**: the model is tested on few players outside that range.

## Fog of war

- Reproduce the game's partial attribute ranges below knowledge 80 (Straňák: 8 of 36 attributes as ranges).
- Baseline knowledge from nation/competition knowledge; whether knowledge 94 is a separate tier.
- Whether the game hides condition, sharpness and injuries for unscouted players.

## Code follow-ups (from the 2026-10-20 second architecture review)

- `internal/mcpserver/server.go` mixes tool registration with the save session (loaded game, reload, warnings, fog). Move the session into its own type in `session.go` if it grows.
- `fm.NewMoney` takes an `fmprefs.Period`; give fm its own period type if more preference settings arrive.
- `fm.Profile` carries search-only fields (`role_ability`, `per90`), so `get_player`'s output schema lists fields it never fills. Return a search row type from `View.Search` instead.
- `max_release_clause` counts clauses limited to some buyers (foreign, domestic, higher-division or continental clubs). Decide per clause whether it applies to the manager's club.
- `internal/fm/view.go` is ~850 lines: move `query`, `compile*`, `judge` and the sort keys to `search.go`.
- `detail` (`get_player`, `compare_players`) is checked by the handler; its schema has no enum.
- `fm.Player` must stay within 512 bytes (`TestPlayerFitsA512ByteAllocation`); it is 456 bytes; playing times are stored as bytes and named on demand.

## Unverified guesses to check in game

- Wage period options per week (1) and per month (2), and per month = weekly × 52 / 12.
- Contract clause names: "minimum fee release clause" (Buganza), "optional contract extension by club" (Traoré-Barcola), "sell-on percentage" (Ashby: who receives it?).
- Units of "wage rise after league games" amounts (likely weekly).
- Team names "(other team)" and odd teams such as "Paris Saint-Germain 3" (reputation 1, no division).
- Does Bernardi's contract screen show a relegation release clause? The save has none for him.
- Remaining loyalty bonus on screen: tiered rounding? Nearest €25K fits Ashby, Sami, Bernardi and Leroux; Lobede's €92 513 shows as €93K. Not found in code.
- Favoured clubs and Devoted: does the game hide favoured clubs for a partly scouted player (the server does)? Does Yaser Yousefi (uid 2002168088) or Shane Goodwin (uid 2002181080) show "Devoted"? Which contract's club counts for a player without a team (the server takes the main contract)? Does the game add "Women" to a women's favoured club?
- Reputation level words Regional and Obscure (Worldwide, Continental, National and Local are seen in game): check one player of each.
- Reputation level of a free agent: the server uses his nationality (vt `+0x210` not traced).
- Media handling for a player without flag bit 3 whose rule gives a negative style: the server shows Outspoken (or Evasive) instead of Volatile, Short-tempered or Confrontational. Also whether the game hides media handling for a partly scouted player (the server hides it with the personality).
- Personality: Asier Castillo "Realist" and Mounir Haddad "Mercenary" (2035-12-09 predictions). `restrict = 1` is verified (Enzo Fernández shows Ambitious).
