# FM27 KŽ — transfer interest vs reputation: field observations for fm-mcp

Nov 7, 2026 · @Mindaugas Mačiulaitis

## Summary

The `interest` field over-predicts willingness: 16 of 21 players marked "uncertain" said no in-game.

- **Free agents** decide on the player's reputation, which the tool never checks. With a club reputation of 3200, the cutoff is between 4250 (yes) and 4700 (no).
- **Contracted players** decide on their current club's reputation, which the tool does check. Wesley (rep 5250, at a club rated 3000) agreed to talk.
- **Loans** seem to depend on playing time at the parent club. The only accepted loan was a "Future Prospect"; all four "Regular Starter" targets refused.

## Context

All tool data comes from the KŽ save as written on 2026-02-04 (in-game date). The in-game approaches were made a few days later (about 2026-02-09 to 2026-02-10); those outcomes come from screenshots and the user's reports.

| Item | Value |
| --- | --- |
| Game build | 27.0.0+2493426 |
| Manager club | FK Kauno Žalgiris (team\_index 16521), TOPLYGA |
| Club reputation | 3200 |
| Fog of war | Off (set via `set_fog_of_war`) |
| Transfer budget | €1.74M (tool and game agree) |
| Wage budget | Tool: €1,312,785 p/a. Game recruitment panel: €269K p/a. |
| Typical squad wage | €28–42K p/a (top earner Tanghe €41.6K) |

The wage mismatch looks like total budget (tool) vs remaining room (game). That is an inference, not checked against the save format.

## Observed outcomes

The tool returned "uncertain" for all 21 players approached. Only 5 said yes or agreed to talk: 2 free agents, 1 youth signing, 1 contracted player and 1 loan. Reputation values are `reputation.current` from the tool; interest is the channel that was used for the approach. Results are as reported by the user from in-game responses.

| Player | Approach | Status at approach | Tool interest | Player rep | Playing time at club | In-game result |
| --- | --- | --- | --- | --- | --- | --- |
| Vaidas Kazlauskas | Transfer | Free agent, age 15 | uncertain, no factors | 2050 (post-signing) | n/a | Signed 2026-02-12 |
| Chaïn Ghrissi | Transfer | Free agent | uncertain, no factors | 3900 | n/a | Agreed to talk. Asked €162–209K p/a to 2027-12-31 |
| Jhon Arcila | Transfer | Free agent | uncertain, no factors | 4250 | n/a | Agreed to talk ("not that interested"). Asked €252–322K p/a to 2026-12-31 |
| Tyrell Ashcroft | Transfer | Free agent | uncertain, no factors | 4700 | n/a | Refused |
| Marcel Wenig | Transfer | Free agent, ACL injury | uncertain, no factors | 5000 | n/a | Refused |
| Mehdi Chahiri | Transfer | Free agent | uncertain, no factors | 5350 | n/a | Refused |
| Carlos Vigaray | Transfer | Free agent | uncertain, no factors | 5450 | n/a | Refused |
| Marco Meyerhöfer | Transfer | Free agent | uncertain, no factors | 5500 | n/a | Refused |
| Sofyan Chader | Transfer | Free agent | uncertain, no factors | 5500 | n/a | Refused |
| Carlos Nieto | Transfer | Free agent | uncertain, no factors | 5600 | n/a | Refused |
| Javi Serrano | Transfer | Free agent | uncertain, no factors | 5700 | n/a | Refused |
| Luka Ilić | Transfer | Free agent | uncertain, no factors | 5750 | n/a | Refused |
| Ibrahima Niane | Transfer | Free agent | uncertain, no factors | 5900 | n/a | Refused |
| Dawid Kownacki | Transfer | Free agent | uncertain, no factors | 5900 | n/a | Joined Lech Poznań 2026-02-16 (not a clean refusal) |
| Wesley | Transfer | FC Hegelmann (club rep 3000) | uncertain, no factors | 5250 | Star Player | Agreed to talk. Fee €0.7–1.6M (rivalry markup), €34–44.5K p/a to 2027-12-31 |
| Lirjon Abdullahu | Transfer | SV Stuttgarter Kickers U19 | uncertain, no factors | 4000 | Breakthrough Prospect | Refused (player or club side unknown) |
| Arnas Voitinovičius | Loan | Benfica B (parent Benfica) | loan uncertain; transfer out (club\_reputation\_gap) | 5100 | Future Prospect | Accepted |
| Justas Lasickas | Loan | HNK Rijeka | loan uncertain; transfer out (club\_reputation\_gap) | 5600 | Regular Starter | Refused |
| Artemijus Tutyškinas | Loan | NK Celje | loan uncertain; transfer out (club\_reputation\_gap) | 5250 | Regular Starter | Refused |
| Armandas Kučys | Loan | NK Celje | loan uncertain; transfer out (club\_reputation\_gap) | 5250 | Regular Starter | Refused |
| Kipras Kažukolovas | Loan | FK Astana, ACL injury | loan uncertain; transfer out (club\_reputation\_gap) | 5500 | Regular Starter | Refused |

Kauno Žalgiris club reputation is 3200. Apart from Hegelmann, the current clubs' reputations were not pulled.

## Findings

The club-reputation check works for contracted transfers. The model misses two cases: free agents are never checked against reputation, and loans seem to depend on the player's playing-time status at his club. Everything below is inferred from the 21 cases above, not from FM's code.

1. **Free agents: the player's reputation decides, and the tool doesn't check it.** Accepted at 2050, 3900 and 4250; refused at 4700 and every value up to 5900. For a club at 3200, the cutoff sits between **4250 and 4700**, about 1.33–1.47× club reputation. The tool returns "uncertain" with no factors for all of them.
2. **Contracted transfers: the current club's reputation decides, and the tool gets this right.** Wesley (rep 5250, above the free-agent cutoff) agreed to talk. His club Hegelmann is rated 3000, below Kauno Žalgiris. The player's own reputation seems not to block when his current club is smaller.
3. **Loans: "uncertain" fails for regular starters.** Every loan target had `transfer: out` (club\_reputation\_gap) but `loan: uncertain` with no factors. The only acceptance (Voitinovičius) is a "Future Prospect" at a B team. All four refusals are "Regular Starter" at their parent club. Playing-time status looks like the deciding signal, and the model ignores it.
4. **Saying yes to talks is not the same as being affordable.** Willing free agents at rep 3900–4250 asked €162–322K p/a. That is 4–8× the club's top earner and 60–120% of the remaining €269K wage room. The tool has no way to show wage demands.
5. **`joined_club` 2026-01-19 is the save-start date.** Many players share it, including starters at the user's own club. That triggered `only_just_joined` for 634 of 2,116 dropped candidates in one search. In-game messages for some players do say "joined 16 days ago", so the flag may be correct, but signing dates and database placement can't be told apart.
6. **Wage budget field.** The tool reports €1,312,785 p/a. The game shows €269K p/a remaining (see Context).

## Suggested changes

The biggest win is adding a reputation factor for free agents, since free agents were 14 of the 21 approaches.

- **Free agents:** compare player `reputation.current` with the manager club's reputation. Above about 1.4× → `out` with a new reason such as `player_reputation_gap`. Between about 1.3× and 1.5× → `uncertain` with that factor attached. Expose the ratio so it can be tuned.
- **Loans:** add a factor from the player's `playing_time` at his parent club. "Regular Starter", "Important Player" or "Star Player" → `out` or a strong factor (`parent_club_needs_him`). "Future Prospect" or "Squad Player" → `uncertain`.
- **Loans while already on loan:** players such as Michelbrink (on loan at Cottbus until 2027-06-30) are still returned as `loan: uncertain`. They should come back `out` (`already_on_loan`), assuming FM blocks re-loaning. That assumption wasn't tested in-game.
- **Search filters:** add `min_value > 0` or `exclude_free_agents`. Free agents (value 0, no wage) currently crowd out contracted players when sorting by CA. Separately, a `max_reputation` filter would let you search below the cutoff directly.
- **Wage budget:** report remaining wage room, or both total and remaining, so the €269K constraint is visible.
- **Save-start `joined_club`:** consider tagging a `joined_club` equal to the save-start date as "database placement". That would avoid treating those players as recent signings in calibration.

## Open questions and caveats

The sample is small: 21 approaches, one club, one season start. Treat the thresholds as a first estimate.

- Were the refusals for free agents decided by reputation alone, or also by league reputation, ambition and expected playing time? The data can't separate these.
- For loans and for Abdullahu, the result doesn't show whether the player or the parent club said no.
- Is the free-agent cutoff a ratio to club reputation, or a fixed gap? Testing it needs a second club with a different reputation.
- Does a contracted player at a smaller club accept even with a much higher personal reputation? Wesley (5250 vs club 3200) is the only data point. Haydary (Sūduva, 5000), Sito (Intercity, 5285), Ezzarfani (Manresa, 5000) and Keïta (Grau-du-Roi, 5050) are untested candidates for checking.
- The save was reloaded mid-analysis (game date moved 2026-02-04 → 2026-02-25), so a few values may differ slightly from those seen in-game at approach time.
- The CA, PA, role-ability and reputation data come from the FM tool with fog of war off.
