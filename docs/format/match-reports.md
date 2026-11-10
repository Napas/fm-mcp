# FM27 retained match reports

## Evidence and support

Verified from bytes in a **copy** of a Caen career, game date
2026-07-23, build **27.0.1+2494687**. The save contains 163 live report
indexes; all 163 decode, and each report's date and home/away team indices
match a played fixture. No game binary was traced and the FM27 match screen
has not yet been compared. FM26 and report formats other than 60 are rejected.

The integration test `TestRealCaenMatchReports` uses `FM_MATCH_REPORT_SAVE`
(only point it at a copy of this particular save). Caen played one friendly,
Caen 1–1 Lille, on 2026-07-22. For the 32 players of that report with exactly
one friendly appearance, passes attempted/completed, shots/on target, goals
and assists equal their independent `player_stats.dat` scope-0 counters.
Ratings agree where scope 0 records a rated appearance. Botella (person
15411, uid 49045799): 33/28 passes, 2/1 shots, xG .16, xA .20, stored rating
673 → 6.7. Freddy Bomo (person 31448, uid 2000182073): 53/47 passes, 2/1
shots, 1 goal, stored rating 775 → 7.8. Additional byte comparisons of the
single-appearance scopes identify dribbles, fouls, key passes, tackles and
interceptions. Goalkeeper-specific meanings are not exposed as tackles.

Starting-XI correction, second copy dated **2026-08-03**, same build:
238 reports decode. Caen–Saint-Étienne (report **1128**, 2026-08-01, 0–0)
has zero at player offset 145 throughout, despite a recorded starting XI.
Across this copy, offset 145 ranges from 0 to 7 in initial-XI entries and
can be positive for substitutes. The earlier interpretation as a start
flag was wrong: the 1–1 Lille sample happened to give 1 to its starters.

The roster stores the **first 11 entries in starting tactic-slot order**,
then the bench. Independent comparison against `tactics_man.dat` last-lineup
records confirms all 11 person indices, slot by slot, for **382 teams**.
Only reports matching the team's unique latest played fixture date and
opponent were compared; no older report is attributed to the latest lineup.
Both reports 395 and 1128 give 11 starters per side, with Bomo starting.
The regression tests use `FM_MATCH_STARTERS_SAVE` pointing at a copy of this
second save and check both report and individual-history MCP responses.
This is independent save-data evidence, not yet an FM match-screen check.

## match_stats_cfg.dat — indexed PKS ring buffers

Despite its name, this is not just configuration. Inner-file version **6**:

```
u32 next_report_id, u8 1, u8 1, u32 group_count
group_count × group:
  u32 group_id, string name             pks_0 … pks_3 in the sample
  u32 configured_capacity, u8 1, u8 1
  u32 count, count × (u32 report_id, u32 offset, u32 compressed_size)
  u32 capacity, u32 write_position
  u8 3, u32 payload_length, payload_length bytes
  u8 trailer                           1 in the sample; meaning unknown
```

Offsets are relative to the group's payload. Each live entry is a complete
zstd frame, often without a declared uncompressed content size. **Use the
index, never scan for zstd magic:** the backing buffer can contain stale and
partial frames. The write position is not the buffer length. In `pks_3`,
capacity is 479232, write position 252558, backing buffer length 481693;
current entries occur on both sides of the write position. Four groups
contain 1, 2, 9 and 151 entries. Caen–Lille is report **395**, in `pks_0`.

Caps: 16 groups, 20000 total reports, 16 MiB decompressed per report,
256 MiB cumulative decompression per load. Invalid frame ranges/duplicate
IDs reject the index; an individual unsupported report is skipped and counted.

## Report header and team lists

Supported prefix: `01 00 01 3c 1f` (the names of these version bytes are
inferences). At offset **11**, a u32 stores minutes since 0001-01-01 00:00
with zero-based elapsed days. Caen–Lille: `f0 c8 7f 3f` = 1065339120 →
2026-07-22 20:00. All 163 dates match fixtures. Time zone semantics are
unknown: the API carries the stored wall-clock value with UTC as a neutral
encoding, not a claim about the real-world kick-off timezone.

The variable-length header embeds two adjacent, 23-byte team structures:
each starts `u8 3, u8 2, u32 team_index, u8 2`. The pair must be unique in
the first 1024 bytes, with distinct teams. Names/stadium/referee strings and
other header fields are not yet decoded.

A later team block starts `u8 2, u32 team_index, u8 roster_count`, then
tactical/summary data and the roster. In this sample, the first player
starts 425–476 bytes after the team reference (Caen 452, Lille 458).
The parser bounds the search to offsets 400–599 and validates the entire
declared roster (11–64), including side flags, counts and unique people.
Its first 11 entries are marked `started`; following entries belong to the
bench. This classification depends on roster slot, not shirt number, score
counters or entry-clock fields. Generated-player prefixes do not add a slot.
It rejects ambiguous candidates. This bounded scan is intentionally limited
to the observed layout, not a universal grammar for team tactics.

## Player records

Ordinary reference: `u8 2, u32 person_index`. A generated/grey player's
reference can have an 11-byte prefix starting with 1 before that reference;
the remaining prefix meanings are unknown. Such people may not resolve in
the database and are then returned as `unnamed_player`, without an invented
name/UID. Offsets below are relative to the ordinary `u8 2` reference:

| Offset | Type | Meaning |
|---|---|---|
| 1 | u32 | Person-table index, not player UID |
| 5 | u8 | Shirt number |
| 6 | u8 | Side: 0 home, 1 away |
| 20 | u8 | Goals |
| 25 | f32 | xG |
| 41, 42 | u8 | Shots, shots on target |
| 56 | u8 | Assists |
| 58, 60 | u8 | Dribbles, fouls |
| 86, 87, 88 | u8 | Passes attempted, completed, key passes |
| 92, 93 | u8 | Outfield tackles, tackles won; keepers not interpreted |
| 98 | u8 | Interceptions |
| 135 | u8 | Nonzero for a player who appeared; exact time/flag meaning unresolved |
| 136 | u16 | Engine rating ×100; round to a tenth for the API |
| 140 | f32 | Distance in metres; API rounds km to a tenth |
| 145 | u8 | Unresolved match counter; **not** a start flag (old interpretation disproved) |
| 191 | f32 | xA |
| 216 | u8 | Shot-group count |

Shot groups: `u16 unknown, u8 n, n × 15 bytes`. Five trailing bytes follow
the groups. A no-group record is 222 bytes. A group can contain more than
one shot: assuming a fixed 18 bytes per shot breaks Lille's records.
Detailed reports have groups whose total shots equals the shot count;
summary reports may have zero groups even with shots. The API does not
interpret shot-group contents.

**Engine ratings are not guaranteed screen ratings.** The save keeps a
rating for unused and briefly-used substitutes too. The API omits it for
unused players; `engine_rating` remains explicitly labelled for appearances.
Season counters can omit a short appearance's rating. Distance rounding
can differ from season counters, which accumulate integer tenths.

## API and open questions

`get_fixtures` adds `match_id` only when a retained decoded report uniquely
matches the date and ordered home/away team indices. `get_match_report`
returns the two rosters; `get_player_match_stats` lists appearances only.
IDs are save-local. The ring buffers are partial history; absent reports
are unavailable, not zero statistics. No season delta is attributed to a match.

Still unresolved: minutes/substitution clock flags, possession and other
team counters, cards/events, keeper counters, the exact FM screen-rating
eligibility rule, the full team-header grammar, time zone semantics, and
other versions/builds. Validate these against FM screens before expanding
the supported fields.
