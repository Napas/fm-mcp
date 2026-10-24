# fm-mcp

An MCP server that reads Football Manager 26 save files and gives an LLM the tools of an assistant manager: find and compare players, review your squad, and scout.

The server only reads saves and the game's files; a lint rule forbids file-writing calls in non-test code.

## Build

```sh
go build -o bin/fm-mcp ./cmd/fm-mcp
```

## Use

Claude Code picks up the server from `.mcp.json` in this folder. Approve it once when asked, then ask things like "who are my best centre backs?" or "find me a left back under 23 with potential above 160".

Claude Desktop: add this to `claude_desktop_config.json`:

```json
{ "mcpServers": { "fm": { "command": "/absolute/path/to/bin/fm-mcp" } } }
```

On first use the server loads `FM_SAVE`, or else the newest save in the FM26 save folders (`~/Library/Application Support/Sports Interactive/Football Manager 26/cloud/games` and `.../games`). Load times and memory: see [Loading and memory](#loading-and-memory). Load times and warnings are logged to stderr. Database versions other than 3727 (26.2.0) and 3733 (26.3.2) load with a warning, since their layouts are unchecked.

| Variable | Effect |
|----------|--------|
| `FM_SAVE` | Save to load on first use |
| `FM_SAVE_DIRS` | Folders to search for saves, separated like `PATH` |
| `FM_GAME_DB_DIR` | The game's database folder, the source of nation, competition and injury names (default: the Steam install) |
| `FM_FOG_OF_WAR` | `off`, `0` or `false` starts with fog of war off (default: on) |
| `FM_PREFS_DIR` | The game's preferences folder, the source of the currency and wage period (default: `~/Library/Application Support/Sports Interactive/Football Manager 26/Preferences/version 6`) |

## Loading and memory

The save loads on the first tool call that needs it (about 1.2 s for a 400 MB save). Before each call the server checks the loaded file. If it changed, the server waits until it has been unchanged for 2 s (about 10 s at most), reloads it and adds a note with the game date. The earlier copy stays, with a note, if the file is still being written (retried on a later call), fails to load (retried after it changes again) or is gone. The first load and `load_save` do not wait, so a save read mid-write fails to load (a first load is retried on the next call). Only the loaded file is watched: a save under another name needs `load_save`.

A loaded 140 000-player save uses about 450 MB (physical footprint, measured). Loading peaks at about 1.8 GB for a second; the server then returns the freed memory to the OS. A reload keeps the old copy until the new one is ready, so its peak is higher (not measured).

## Money

The save stores amounts in GBP, wages per week. The server converts them to the currency and wage period set in FM's preferences (re-read on each load), at the save's exchange rates. `game_overview` reports the `currency` and `wage_period` (`p/w`, `p/m` or `p/a`); `max_value` and `max_wage` use the same units. A missing setting file means the game's default (the UI language's currency, wages per week). Each setting falls back on its own, with a warning: an unreadable currency file to the UI language's currency (else GBP), a currency the save has no rate for to GBP, an unreadable wage period to per week. A missing preferences folder gives GBP per week with a warning.

## Fog of war

On by default, as in the game: the server hides what your scouts do not know. Each player gets `knowledge` (0–100) and `scouting`:

| `scouting` | Knowledge | Hidden |
|------------|-----------|--------|
| `full` | 100: your club's players (loanees out included) and fully scouted ones | nothing |
| `partial` | 80–99 | current and potential ability, wage, clauses, playing time, loyalty bonus, hidden attributes (consistency, dirtiness, important matches, injury proneness, versatility, and the personality ones: adaptability, ambition, loyalty, pressure, professionalism, sportsmanship, temperament, controversy), personality, media handling, favoured clubs |
| `required` | below 80 | also all attributes, traits and value |

Knowledge is the larger of the level stored in the save and a reputation baseline (current reputation 8000+ → 80, 6500+ → 50, 5000+ → 30). The game also counts nation and competition knowledge, which is not decoded, so the server can hide a little more than the game does. The 80 and 100 cut-offs are read off game screens. Hidden fields are left out. Other clubs' bank balances are hidden too, as in the game. Searches match and sort only on values you can see; players whose sort value is hidden come last, most reputable first. `search_players` always returns `total` (matches) and `needs_scouting`: players who pass every filter whose value you can see, but whose value for another filter is hidden, so they may match once scouted (0 with fog of war off). It is an upper bound: some of them, e.g. players whose value the save does not know, can never match. Free agents have no wage: `max_wage` leaves them out and wage sorts put them last. `set_fog_of_war` switches it, but the LLM is told to do so only when you ask.

## Tools

Prompts `squad_review`, `scout_position` and `contract_review` give ready-made assistant-manager requests.

| Tool | What it does |
|------|--------------|
| `list_saves` | Saves on disk, newest first |
| `load_save` | Load a save (the newest when no path is given) |
| `game_overview` | In-game date, manager, club, the club's transfer and wage budgets, player count, fog of war on or off |
| `set_fog_of_war` | Turn fog of war on or off |
| `find_players` | Players by name: each word must start a word of the name (each part of a hyphenated or apostrophe name), ignoring case and accents. |
| `get_player` | Full profile (`detail` `summary` keeps only the `club` row of `season_detail` and drops the interest factors): attributes (hidden ones included), personality, media handling, traits, favoured clubs, position ratings (10 and above), ability, reputation and reputation level, value, contract and clauses, squad number, playing time, remaining loyalty bonus, injuries, condition, season stats, best roles (`best_roles`), and `interest`: whether he would join your club on a transfer or a loan (an estimate) |
| `compare_players` | Several profiles side by side, `best_roles` and `interest` included; by default (`detail` `summary`) only the `club` row of `season_detail` and no interest factors, `detail` `full` for all |
| `find_clubs` | Clubs and teams (first, youth, women's, national) by name |
| `get_squad` | A team's details, its players best first, and players out on loan; your first team by default. A national team lists its called-up squad |
| `squad_depth` | Who can play each position |
| `get_tactic` | Your current first-team tactic: name, mentality, each slot's in- and out-of-possession position and role, the player in each slot with his `role_ability` and the bench. With no XI picked, the last match's lineup (`lineup_source`, `none` when there is neither); `unnamed_player` marks a person the save does not decode as a player |
| `get_last_lineup` | A team's most recent match: opponent, mentality, shape in and out of possession (e.g. 4-2-3-1), each slot's player with shirt number, positions and roles (no duties), and the bench; your first team by default. Persons the save does not decode as players show as `unnamed_player`. For scouting opponents (`get_fixtures` names the next one). The match date is not known |
| `get_fixtures` | A team's or a competition's fixtures: results (most recent first) and upcoming (soonest first), with competition, stage (e.g. "Playoffs – Semi Final – First Leg"), teams and the score after 90 minutes, extra time, penalties and aggregate; your first team by default. Filters: dates, played or upcoming, limit |
| `get_table` | Stored league tables in the game's order (groups, conferences and playoff tables included) with home and away splits and form; points as the game stores them. Your league table by default. Sub-competition tables (e.g. Liga MX Opening Stage) are labelled by matching their results or teams (`label_source`). Never computed from results |
| `search_players` | Scouting: position, age, ability, potential, team, club (club teams only), contracted team, nation, gender (defaults to your team's; `any` for both), value, wage, release clause (`max_release_clause`), expiring contracts, months at the current club (`min_months_at_club`), injuries, height, traits (all of), personality (any of), attribute minimums (preferred foot through `LeftFoot`/`RightFoot`); `role` (with `position`) ranks by role ability, `min_role_ability` sets a floor; sorts by this season's club stats (`season_goals`, `season_average_rating`, …) with `min_season_minutes`; `per90_<stat>` sorts and `min_per90` filters use club stats per 90 minutes, leaving out players under 450 minutes unless `min_season_minutes` is set; `interest` (`transfer`, `loan` or `transfer_or_loan`) leaves out players who certainly won't join your club and counts them in `interest_dropped`; returns the total match count and `needs_scouting` |

Player rows from `find_players`, `get_squad` and `search_players` carry season stats, `joined_club` and any national team call-up.
`role_ability` (`get_tactic`, `search_players` with `role`, `best_roles` in `get_player` and `compare_players`) is FM's role ability for an in-possession role at a position, on the current-ability scale. A player's best role usually equals his current ability; the game halves a role ability under the role's threshold (e.g. 100 for Wide Forward), the best role included. The game shows it as stars against its own assessment of the player, which the server cannot reproduce, so use it to rank, not to predict stars. One player's roles came out in the game's order in every sample checked, though ties may be listed in another order; across players it agrees with the game's order about 3 times in 4. `search_players` takes no side: Wide and Overlapping Centre-Back at DC use the better of DL and DR, while `get_tactic` uses the slot's side. `search_players` accepts only roles offered at the position. Fog of war hides it with current ability.
`get_player`, `compare_players` and `get_squad` rows carry `squad_number`, `playing_time` (FM's Playing Time: the agreed status, as squad lists and the contract screen's "Agreed" line show it), `actual_playing_time` (the contract screen's headline: how his game time currently rates) and `original_playing_time` (agreed at signing; FM does not show it). A loanee has his loan contract's. `actual_playing_time` is left out in some cases (see Limits). FM's "Expected" playing time is not decoded. `squad_number` is the number registered with the current contract's team. `get_squad` leaves it out for players contracted to another team (e.g. first-team players in a reserve squad) and in national-team squads; in its `out_on_loan` rows it is the number at the club whose squad it is (the main contract's). `get_last_lineup`'s `shirt_number` is the number worn in that match and can differ.
`get_player`, `compare_players` and `get_squad` rows carry `personality` (FM's label, such as "Fairly Professional") and `media_handling` (FM's styles, such as ["Evasive", "Unflappable"]), both computed from the hidden attributes, and `reputation_level` (the profile's reputation word: Worldwide, Continental, National, Regional, Local or Obscure; fog of war never hides it; left out for women's players and player-coaches). `get_player` and `compare_players` carry `traits` (player preferred moves, such as "Marks Opponent Tightly") and `favoured_clubs` (FM's Favoured Clubs: the club's short name, its first team's `team_index`, and "supporter" or "player").
A contract's `start` is when its current terms were signed (the contract screen's Start); `joined` is when the player joined that team. A player's `joined_club` is when he joined his current club, as his profile shows it; it can be later than his contract's `joined`.

`get_player` and `compare_players` carry `loyalty_bonus_remaining`: the main contract's unpaid loyalty bonus, paid weekly until the contract ends. FM shows it rounded (in the samples seen: to the nearest €1K below €100K, to the nearest €25K above).
Rows' `season` also has `minutes` and `player_of_the_match`. `get_player` and `compare_players` add `season_detail`: one row per competition group with stats (`club`, `league`, `domestic_cups`, `continental`, `friendlies`, `internationals`) with penalties, matches won and lost, the team's goals for and against in his matches, and match stats (passes, tackles, fouls and fouls against, pressures, `expected_goals` and `non_penalty_expected_goals`, distance_km…). `continental` counts all continental cups together; `internationals` is the senior national team, probably this international season (where the game resets it is not traced), and club rows exclude it. Goalkeepers (GK rating 15 or more) have `"goalkeeper": true` and get `goals_conceded`, `clean_sheets`, `saves_held`, `saves_parried`, `saves_tipped`, `penalties_faced`, `penalties_saved` and `expected_goals_prevented` instead of tackles and tackles won, headers and headers won, blocks and open-play crosses completed (the game stores both in the same places). Drawn matches are starts + sub_appearances − matches_won − matches_lost. FM shows many stats per 90 minutes: compute them as count × 90 / minutes.

## Limits

- Some fields are educated guesses: condition and sharpness, clause and bonus names, actual playing time (checked in game for 3 first-team outfield players; the others are predictions of the decoded rule. The field is left out away from a club's first team, where scores are not checked in game, for players 25 or younger whose result the youth test may change, and where a part of the rule not decoded may give Squad Player, Impact Sub, Breakthrough Prospect or Fringe Player. The game's adjustments for youth contracts, playing-time promises and team settings are not applied, so a served value can still differ), media handling (for some players the game may show Volatile, Short-tempered or Confrontational instead), the reputation words Regional and Obscure (Worldwide, Continental, National and Local are checked in game) and the reputation word of players without a team, the names of tactic roles FM26 renamed (listed in the server instructions), the internationals row's season. The season stats named from the game's code but not yet compared with its screens (team goals, matches lost, fouls against, pressures, open-play key passes and crosses, non-penalty xG, headers lost, shots and goals from outside the box, free kicks, blocks) are listed in `docs/format.md`. `docs/format.md` marks what is verified. The bank balance and the transfer and wage budgets are verified. `wage_budget` is the total; FM's "available" figure subtracts the current wage bill, which the server does not compute. The amount spent this season is not decoded.
- Favoured clubs and media handling: hiding them under partial scouting, and taking the main contract's club for Devoted, are the server's choices, not checked in game.
- A value at the game's cap (£300M in the save) means the club will not sell: `not_for_sale` is set and `value` left out.
- Not decoded yet: stadium names, loan terms, player history, women's national teams' nations, morale, team and player instructions, tactics of your other teams.
- Club names are the ones your game shows, so licence-free clubs keep their fake names.

## Develop

```sh
go test ./...                       # unit tests + data/test_save.fm integration tests
FM_REAL_SAVE=/tmp/kz-2035-12-02.fm FM_REAL_SAVE_9DEC=/tmp/kz-2035-12-09.fm FM_REAL_SAVE_14DEC=/tmp/kz-2035-12-14.fm FM_REAL_SAVE_1JAN=/tmp/kz-2036-01-01.fm go test ./...
go test ./internal/savefile -run '^$' -fuzz FuzzNewArchive -fuzztime 30s   # also FuzzParseDirectory, FuzzParsePlayer, FuzzReadTables
golangci-lint run ./...
```

Integration tests use `data/test_save.fm` (FM 26.2.0, gitignored). The real-save tests check ground truth from the KŽ career saves of 2035-12-02 (`FM_REAL_SAVE`), 2035-12-09 (`FM_REAL_SAVE_9DEC`), 2035-12-14 (`FM_REAL_SAVE_14DEC`) and 2036-01-01 (`FM_REAL_SAVE_1JAN`); they skip when the variable is unset and fail when the save has another game date. Point them only at copies, never at the game's save folder.

Layers: `internal/savefile` parses bytes, `internal/fm` is the domain model and queries, `internal/mcpserver` maps tools onto the domain, `cmd/fm-mcp` is the stdio entry point.
