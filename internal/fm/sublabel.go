package fm

import "slices"

// stageGroup is a competition's fixtures at one stage: the key's group when
// set, else its stage.
type stageGroup struct {
	comp, stage int32
}

// groupFixtures is a stage group's latest season: the teams of all its
// fixtures, and what each team's played ones add up to.
type groupFixtures struct {
	teams   map[int64]bool
	records map[int64]TableRecord
}

// relabel gives tables stored in a parent competition's file the
// sub-competition they belong to. Candidates are the competitions with
// fixtures at the table's stage. A table with matches played matches a
// candidate whose played fixtures (latest season) add up to every row's
// played, won, drawn + shoot-out losses, lost, for and against; a table with
// none played matches a candidate whose fixtures (latest season) have
// exactly its teams. Exactly one match other than the file's own
// competition relabels the table, unless another table would get the same
// competition and stage (e.g. a group stored twice): then all of them keep
// the file's label.
func relabel(tables []Table, fixtures []storedFixture, w World) {
	played, all := stageGroups(fixtures, w, true), stageGroups(fixtures, w, false)
	matched := make(map[int]stageGroup)
	claims := make(map[stageGroup]int)
	for i, t := range tables {
		groups, match := all, teamsMatch
		if slices.ContainsFunc(t.Rows, func(r TableRow) bool { return r.Played > 0 }) {
			groups, match = played, recordsMatch
		}
		var found []int32
		for g, fx := range groups {
			if g.stage == t.stage && match(t, fx) && !slices.Contains(found, g.comp) {
				found = append(found, g.comp)
			}
		}
		if len(found) == 1 && int64(found[0]) != t.CompetitionIndex {
			g := stageGroup{comp: found[0], stage: t.stage}
			matched[i] = g
			claims[g]++
		}
	}
	for i, g := range matched {
		if claims[g] == 1 {
			comp := int64(g.comp)
			tables[i].CompetitionIndex, tables[i].Competition, tables[i].LabelSource = comp, w.Competitions[comp], labelFromResults
		}
	}
}

// stageGroups groups fixtures by competition and stage, keeping each
// group's latest season; with onlyPlayed, of its played fixtures.
func stageGroups(fixtures []storedFixture, w World, onlyPlayed bool) map[stageGroup]*groupFixtures {
	latest := make(map[stageGroup]uint16)
	keep := func(f storedFixture) (stageGroup, bool) {
		if onlyPlayed && newScore(f.Result) == nil {
			return stageGroup{}, false
		}
		k := w.Keys[f.Key]
		stage := k.Group
		if stage < 0 {
			stage = k.Stage
		}
		return stageGroup{comp: k.Comp, stage: stage}, true
	}
	for _, f := range fixtures {
		if g, ok := keep(f); ok {
			latest[g] = max(latest[g], f.Season)
		}
	}
	out := make(map[stageGroup]*groupFixtures, len(latest))
	for _, f := range fixtures {
		g, ok := keep(f)
		if !ok || f.Season != latest[g] {
			continue
		}
		fx := out[g]
		if fx == nil {
			fx = &groupFixtures{teams: map[int64]bool{}, records: map[int64]TableRecord{}}
			out[g] = fx
		}
		home, away := int64(f.Home), int64(f.Away)
		fx.teams[home], fx.teams[away] = true, true
		if s := newScore(f.Result); s != nil {
			h, a := s.Home, s.Away
			if s.ExtraTime != nil {
				h, a = s.ExtraTime[0], s.ExtraTime[1]
			}
			fx.records[home] = fx.records[home].add(h, a)
			fx.records[away] = fx.records[away].add(a, h)
		}
	}
	return out
}

// add counts a match the team scored and conceded in.
func (r TableRecord) add(scored, conceded int) TableRecord {
	r.Played++
	r.GoalsFor += scored
	r.GoalsAgainst += conceded
	switch {
	case scored > conceded:
		r.Won++
	case scored < conceded:
		r.Lost++
	default:
		r.Drawn++
	}
	return r
}

func recordsMatch(t Table, fx *groupFixtures) bool {
	for _, r := range t.Rows {
		want := r.TableRecord
		want.Drawn += r.ShootoutLosses
		if fx.records[r.TeamIndex] != want {
			return false
		}
	}
	return true
}

func teamsMatch(t Table, fx *groupFixtures) bool {
	if len(fx.teams) != len(t.Rows) {
		return false
	}
	for _, r := range t.Rows {
		if !fx.teams[r.TeamIndex] {
			return false
		}
	}
	return true
}
