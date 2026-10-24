# Manual test script

A hands-on session: ask Claude Code questions, compare its answers with the FM26 screens, and note every mismatch.

Expected values below come from the save `KŽ.fm` (FC Metz, 29 Nov 2035) as the server read it on 2026-10-20. If you play on and save again, the numbers will move; check against the game, not this file.

## 0. Setup (5 min)

1. `go build -o bin/fm-mcp ./cmd/fm-mcp`
2. Start `claude` in this folder. The `fm` server is registered for this project in local scope, so no approval is needed. `/mcp` should list `fm` as connected with 11 tools.
3. Open FM26 with the same save, so you can compare screens.

Log every problem in the table at the end: step, what Claude said, what the game shows.

## 1. Loading

| # | Ask | Expect |
|---|-----|--------|
| 1.1 | "give me a game overview" | Save `KŽ`, date 29 Nov 2035, manager Napas Mačiulaitis, club FC Metz, 139 914 players, build 26.3.2 |
| 1.2 | "list my saves" | 4 saves, newest first: `KŽ.fm`, `KŽ (v02).fm`, `KŽ (v03).fm`, `last save overwrite backup.fm` |
| 1.3 | "load KŽ (v03)" then "what's the game date?" | An earlier date than 1.1; reply comes in about a second |
| 1.4 | "load the newest save" | Back to 29 Nov 2035 |
| 1.5 | Play on a day, save in FM, then "what's the game date?" | The new in-game date, with no reload request |

## 2. One player against the game

Ask "show me Chris Ashby's full profile" and open his player screen in FM.

| Field | Server says | Game shows | Notes |
|-------|-------------|------------|-------|
| Age, born | 25, 27 Jan 2010 | | |
| Nation | England | | |
| Height | 178 cm | | |
| Positions | DC 20, DM 9, MC 9 | | position ratings are 1–20; the game shows dots or a colour scale |
| Value | €6.60M | | the game shows a range; the saved number is one value. Amounts assume euros and wages per year in FM's preferences, as on this machine |
| Wage | €921 516 p/a | | the game shows €922K p/a |
| Contract | 19 Jun 2034 – 30 Jun 2038 | | |
| Clauses | appearance fee €3 544, clean sheet bonus €2 658, unused sub fee €885, relegation release clause €3.44M, 10% sell-on | | **clause names are guesses**: check each one |
| Remaining loyalty bonus | €356 473 | | the game shows €350K (rounding to the nearest €25K above €100K is an inference) |
| CA / PA | 135 / 159 | | only visible with an editor or FM Scout; skip if you can't see it |
| Condition | 100% | | **guess** |
| Match sharpness | 89% | | **guess** |
| Reputation | home 5987, current 6562, world 3365 | | labels verified by code (editor field codes hr, cr, wr); the game shows stars only |
| Reputation level | National | | Ashby not checked; the rule matches 14 in-game samples (12 players) on 9 and 14 Dec (Continental and National only) |
| Personality | Fairly Professional | | seen in game on 2 Dec |
| Traits | Marks Opponent Tightly | | seen in game on 2 Dec |
| Favoured clubs | none | | none in the saves of 2–14 Dec; Matković shows R. Madrid (supporter) and Osijek (player), seen in game on 14 Dec |
| Media handling | Reserved | | Ashby not checked; the rule matches 8 in-game samples (6 players) on 9 and 14 Dec |

Attributes (compare all 54; these look odd for a DC rated 20, so check them first):

| Attribute | Server says | Game shows |
|-----------|-------------|------------|
| Jumping Reach | 7 | |
| Heading | 14 | |
| Communication | 3 | |
| Aggression | 20 | |
| Right / Left foot | 20 / 18 | |
| Marking, Positioning, Concentration | 15, 16, 16 | |

If the attributes are off, note whether they look shifted by one (each value matching the next attribute's in-game value). That points to a wrong attribute order rather than a wrong scale.

Repeat with two more players: your best striker and your goalkeeper (GK attributes such as Handling and Reflexes use a different block).

## 3. Your squad

| # | Ask | Expect |
|---|-----|--------|
| 3.1 | "show my squad" | FC Metz, Ligue 1 McDonald's, 24 players, top five by ability: Chris Ashby, Samal Edubatey, Louis Leroux, Stephen Ly, Anton Matković |
| 3.2 | "who is out on loan?" | 19 players, e.g. Aljaž Strajnar at NK Maribor, Luiz Flávio Pacheco at SC Bastia, Mamadou Diouf at Grenoble |
| 3.3 | "what's my bank balance and what are my budgets?" | Bank balance €34 293 296 (Finances → Overview). `game_overview` budgets on 2 Dec: transfer budget €9 618 594 (reported in game as €9.61M), wage budget €64 371 729 p/a. On 14 Dec: transfer budget €9 637 825 (game: €9.63M), wage budget unchanged (game: €16.43M available + €47.93M spending). Claude should call the wage budget the total, and not claim to know what is left of it, the original budget or what was spent this season |
| 3.4 | "how deep is my squad at each position?" | Each position lists players rated 15+; check that no position you rely on is empty |
| 3.5 | "who in my squad is injured?" | Matches the Medical Centre, knocks included: Danilo Veiga "bruised ankle, 1–2 days", Carlo Bernardi "tight groin, 1–3 days" |
| 3.6 | "which contracts expire by the end of next season?" | Matches Squad → Contracts filtered by expiry |
| 3.7 | "what's my current tactic?" | `get_tactic`: "ZaZ - Autumn 3.39 ST". On 2 Dec no XI is picked, so `lineup_source` is `last_match`: Sami, Veiga, Bernardi, Ashby, Leroux, Edubatey, Nduquidi, Matković, Reddy, Tacalfred, Ly. On the 9 Dec save it is `selected`: Sami in goal, Ly up front (ST in possession, AMC out of it), 9 on the bench. Checked in game on 9 Dec: positions, sides, roles, players, bench and mentality Attacking all match. The slots' `role_ability` on 9 Dec: Sami 125, Veiga 115, Nduquidi 109, Medina 107, Løvik 123, Tacalfred 127, Vassiljev 125, Matković 127, Reddy 124, Edubatey 133, Ly 127. Claude calls only renamed role names guesses |
| 3.8 | "find me a better striker for my tactic" | Claude reads the ST slot from `get_tactic` (Ly, Channel Forward, `role_ability` 127 on 9 Dec) and searches with `position` ST and `role` "Channel Forward", comparing `role_ability` with Ly's. Claude ranks by it and does not turn it into stars |
| 3.9 | "how did Reims line up last time?" / "who did we play last, and how did they line up?" | `get_last_lineup` (find Reims with `find_clubs`, team 1545; for the second question, Metz's own record names the opponent). 9 Dec save: Reims last played FC Metz, Balanced, 4-2-3-1 in possession (No Nonsense Goalkeeper; Wing-Back, two Ball-Playing Centre-Backs, Inside Full-Back; Deep Lying Playmaker and Defensive Midfielder; Inside Forward, Second Striker, Wide Forward; Poacher), 4-2-2-1-1 out of it. Metz's own (no team index) is against Stade de Reims, 3-2-2-2-1 / 5-2-3, the same XI and roles as `get_tactic`, with shirt numbers Sami 88 … Ly 1; on 2 Dec it is against Olympique de Marseille. Claude says roles have no duty and the match date is unknown. Asked about the next opponent, Claude says the next fixture is not known and asks which team. Compare with Reims' match report in game |
| 3.10 | "what squad numbers do my players wear?" | `get_squad` rows carry `squad_number`. 14 Dec save: Sami 88, Ly 1, Ashby 4, Bernardi 19, Assongo 28, Leroux 41, Reddy 16; `out_on_loan` rows carry the Metz number (Le Ven 69), while `get_player` on a loanee gives his loan club's (Le Ven 14); FC Metz 2's rows have no `squad_number` (its players are contracted to Metz). Compare with Squad → Squad Numbers |
| 3.11 | "is anyone unhappy with his playing time?" / "show Assongo's contract" | `playing_time` is the squad list's status and the contract screen's "Agreed:", `actual_playing_time` the contract screen's headline. 14 Dec save: Assongo agreed Squad Player, actual Surplus to Requirements; Bernardi agreed Regular Starter, actual Important Player (both checked in game); Ashby agreed Surplus, actual Regular Starter; Reddy agreed Star Player, actual Regular Starter (both rule predictions); Lobede and the FC Metz 2 players have no `actual_playing_time`. Claude says FM's "Expected" line is not available and does not present `original_playing_time` as something FM shows |
| 3.12 | "who in my squad is the best Overlapping Centre-Back?" / "what are Matković's best roles?" | `search_players` with `team_index` 652, `position` DC, `role` "Overlapping Centre-Back", ranked by `role_ability`; `get_player` `best_roles` for Matković. 9 Dec save: Vassiljev 120, Ashby 115, Bernardi 112, Vilas 110, Nduquidi 109, Medina 107; Matković's best roles Wide Forward (AMR) 129, Inside Forward (AMR) 127, Channel Forward (ST) 127. In game, each player's role list (Tactics → player → role) is in the same order as his role abilities, though tied roles may be listed in another order; stars across players may disagree. Claude says it cannot predict the stars. Asked for a role FM26 does not offer at the position (e.g. Regista at DM), Claude gets an error listing the roles offered there |
| 3.13 | "how are Sami and Matković doing this season?" (9 Dec save) | `get_player` `season_detail`. Sami, Ligue 1: `goalkeeper` true, conceded 22, clean sheets 3, saves held 27, parried 25, tipped 6, penalties 1 faced and 1 saved, `expected_goals_prevented` −9.62 (the game shows −0.60 per 90 over 1 440 minutes), 1 mistake leading to a goal; no tackles or headers. Matković, Ligue 1: 8 won and 1 lost, team goals 31 for and 16 against, fouls against 11, pressures 84 (20 completed), open-play key passes 31, open-play crosses 16 (10 completed), `non_penalty_expected_goals` 4.26, headers lost 38. Compare with Profile → Stats → Season Stats. Sami's keeper fields are checked in game (2035-12-28); Matković's are not. Esposito has an `internationals` row: 6 starts, 7 goals (the game's Euro Championship qualifiers row) |

Also check the 24: does the game's first-team squad have 24 players? Youth, reserve and B teams are separate teams on purpose.

## 4. Scouting and search

| # | Ask | Check |
|---|-----|-------|
| 4.1 | "find left backs under 23 with potential above 150 worth under €15M" | Results are left backs (DL or WBL), under 23, under €15M; it states the total number of matches. With fog of war on, only fully scouted players can match the potential filter |
| 4.2 | "find French strikers on free transfers" | Only free agents; nation France |
| 4.3 | "players with pace and acceleration of at least 16 who play AMR" | Every result has both attributes at 16 or higher |
| 4.4 | "who are the best players at PSG?" | Paris Saint-Germain players; the club filter also includes its youth and reserve teams. The save has no "PSG" name (short name "Paris SG"), so Claude should search "paris" |
| 4.5 | "search women's players for a DC" | Women only; the default is your team's gender (men) |
| 4.6 | "find Anton Matković" and "find anton matkovic" | The same player both times; accents and case don't matter |
| 4.7 | "compare Ashby, Edubatey and Leroux" | Side by side, consistent with section 2 |
| 4.8 | "show France's national team" | France (team 589), not France Women. Teams named "France (other team)" are national sides whose type isn't decoded |
| 4.9 | "show the England women's national team" | **Guess**: women's national teams map to nations by an inferred rule; check that the players are English women |
| 4.10 | "show Martin Straňák" | Fog of war: scouting `required`; no attributes, ability or value. Claude says he needs scouting |
| 4.11 | "show Tino Livramento" | Scouting `partial`: exact attributes, no ability or wage; not for sale |
| 4.12 | "show Idir Yahiaoui" (on loan from Metz) | Scouting `full`, like every Metz player |
| 4.13 | "turn fog of war off", "show Straňák", then "turn it back on" | Full profile while off; Claude changes the setting only because you asked |
| 4.14 | Change currency or wage period in FM's preferences, save in FM, then "show Ashby" | Value and wage in the new currency and period; `game_overview` names them |

## 5. Ready-made prompts

Type `/` in Claude Code; the server's prompts appear as `/mcp__fm__…`.

| # | Prompt | Check |
|---|--------|-------|
| 5.1 | `squad_review` | Strengths, weak positions and contract risks that you agree with |
| 5.2 | `scout_position` with position `DL`, max age 24, max value 10000000 (euros) | A shortlist that fits the limits |
| 5.3 | `contract_review` | Expiring contracts that match the game |

## 6. Bad input

| # | Ask | Expect |
|---|-----|--------|
| 6.1 | "show player 1" (a uid that doesn't exist) | A clear "not found", no crash |
| 6.2 | "search players at position XYZ" | The server returns `invalid filter: position "XYZ"`; Claude should explain or retry with a valid code |
| 6.2b | "search players at club totenham" | `invalid filter: no team matches club "totenham"; use find_clubs`; Claude should look the club up and retry |
| 6.3 | "load /tmp/nothing.fm" | A clear error; the current save stays loaded |
| 6.4 | Ask 10 questions in a row quickly | No slowdown after the first load |

## 7. Assistant quality

Open-ended; judge the answers as a manager would.

- "I have €5M and need a centre back to replace my weakest one. Who?"
- "Which of my loanees are ready to come back?"
- "Who should I sell this winter?"
- "Which young players at the club have the highest potential?"

Note answers that rely on guessed fields (condition, sharpness, finances, actual playing time, media handling, the reputation words Regional and Obscure, renamed tactic roles) and whether Claude says they are guesses.

## Results log

| Step | Claude said | Game shows | Severity (wrong / missing / odd) |
|------|-------------|------------|----------------------------------|
| | | | |
