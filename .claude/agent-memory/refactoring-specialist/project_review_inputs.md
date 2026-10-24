---
name: project-review-inputs
description: fm-mcp has no git history; review scope comes from a caller-supplied unified diff (e.g. /private/tmp/claude-502/fmwork/roundN.diff or .superpowers/sdd snapshots); real-save test env vars
metadata:
  type: project
---

fm-mcp has had no git commits (as of 2026-10-21), so `git diff` shows nothing. The caller hands over a snapshot diff (`.superpowers/sdd/<date-topic>/review-*.diff` or `branch.diff` against a `base/` copy beside it, or `/private/tmp/claude-502/fmwork/roundN.diff` with a `base-roundN` copy and implementation reports under `fmwork/research/`) and wants a report file written there.

**Why:** the repo is developed without commits; the caller forbids any git write. Several agents land features in parallel, so cross-feature duplicates (test helpers, name-table lookups, sorted team lists, per-uid real-save checks: `checkKnownPlayers` in savefile, `checkRealSavePlayers` in fm) are the main hotspot. `fm.Player` is packed to 512 bytes on purpose; never grow it.
**How to apply:** read the supplied diff instead of `git diff`; never commit. Snapshot files to the scratchpad before editing. Go commands need `GOCACHE=$TMPDIR/gocache`; lint needs `GOLANGCI_LINT_CACHE=$TMPDIR/lintcache`; real-save tests need `FM_REAL_SAVE=$PWD/data/kz-2035-12-02.fm FM_REAL_SAVE_9DEC=$PWD/data/kz-2035-12-09.fm FM_REAL_SAVE_14DEC=$PWD/data/kz-2035-12-14.fm FM_REAL_SAVE_1JAN=$PWD/data/kz-2036-01-01.fm FM_REAL_SAVE_16JAN=$PWD/data/kz-2036-01-16.fm` (16JAN added with player interest, 2026-10-23). Later rounds use per-item copies `fmwork/impl-<item>/{base,repo}` with rules in `fmwork/briefs/review-common.md`: diff base vs repo, edit only repo. Fuzz exec rates on this machine swing 10x with load; compare against a snapshot run side by side before blaming a change.
- Some reviews skip the report file and want a short SubagentHandback report instead (player-interest, 2026-10-23); follow the brief. savefile tests DeepEqual whole Contracts, so keep nil slices nil (no make() preallocation when empty).
- Sometimes there is no diff or pre-change copy at all (search filters round, 2026-10-23): the caller lists the changed spots; snapshot the repo to the scratchpad yourself and diff against that after editing.
- fm tool inputs sharing fields: embed a struct (jsonschema-go v0.4.3 flattens embedded fields via reflect.VisibleFields, same property order); verified by dumping jsonschema.For before/after in a throwaway test (fixtures/tables round, 2026-10-24).
