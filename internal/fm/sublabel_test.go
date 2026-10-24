package fm

import (
	"strings"
	"testing"
	"time"

	"github.com/Napas/fm-mcp/internal/savefile"
)

// subWorld has a parent competition (COMP 1, uid 100) with sub-competitions
// Opening (COMP 2) and Closing (COMP 3), keys 0–2 for them in order, and
// teams 1 and 2.
func subWorld() World {
	return World{
		Teams:           map[int64]*Team{1: {Index: 1, Name: "Club América", divisionIndex: 1}, 2: {Index: 2, Name: "Tigres", divisionIndex: 1}},
		Competitions:    map[int64]string{1: "Mexican First Division", 2: "Mexican First Division Opening Stage", 3: "Mexican First Division Closing Stage"},
		CompetitionUIDs: []uint32{0, 100, 101, 102},
		Keys: []savefile.CompetitionKey{
			{Comp: 1, Stage: -1, Group: -1, Leg: -1, SecondComp: -1},
			{Comp: 2, Stage: -1, Group: -1, Leg: -1, SecondComp: -1},
			{Comp: 3, Stage: -1, Group: -1, Leg: -1, SecondComp: -1},
		},
	}
}

func subTable(played bool) savefile.LeagueTable {
	if !played {
		return savefile.LeagueTable{CompetitionUID: 100, Stage: -1, Rows: []savefile.TableRow{{Team: 1}, {Team: 2, Position: 1}}}
	}
	return savefile.LeagueTable{CompetitionUID: 100, Stage: -1, Rows: []savefile.TableRow{
		{Team: 1, Overall: statsOf(2, 1, 1, 0, 3, 1, 4)},
		{Team: 2, Position: 1, Overall: statsOf(2, 0, 1, 1, 1, 3, 1)},
	}}
}

func subGame(tables []savefile.LeagueTable, fixtures ...savefile.Fixture) *Game {
	raw := savefile.LeagueTables{Tables: tables}
	return NewGame(Info{Date: gameDate, ManagerTeamIndex: 1}, nil, subWorld(), PlayerExtras{Fixtures: fixtures, Tables: &raw, LeagueStages: uint32(len(tables))}) //nolint:gosec // small test data
}

// openingFixtures: América 2–0 Tigres, Tigres 1–1 América in the Opening
// Stage (key 1), and the Closing Stage (key 2) scheduled.
func openingFixtures() []savefile.Fixture {
	return []savefile.Fixture{
		{Key: 1, Home: 1, Away: 2, Date: day(2025, 7, 20), Season: 2025, Result: played(2, 0)},
		{Key: 1, Home: 2, Away: 1, Date: day(2025, 7, 27), Season: 2025, Result: played(1, 1)},
		{Key: 2, Home: 1, Away: 2, Date: day(2026, 1, 10), Season: 2025},
		{Key: 2, Home: 2, Away: 1, Date: day(2026, 1, 17), Season: 2025},
	}
}

func TestRelabelByResults(t *testing.T) {
	g := subGame([]savefile.LeagueTable{subTable(true), subTable(false)}, openingFixtures()...)

	if got := g.tables[0]; got.Competition != "Mexican First Division Opening Stage" || got.CompetitionIndex != 2 || got.LabelSource != labelFromResults {
		t.Errorf("played table = %s (%d, %s), want the Opening Stage by results", got.Competition, got.CompetitionIndex, got.LabelSource)
	}
	// The Closing Stage has the same teams as the Opening Stage, so the
	// unplayed table cannot be told apart.
	if got := g.tables[1]; got.Competition != "Mexican First Division" || got.LabelSource != labelFromFile {
		t.Errorf("unplayed table = %s (%s), want the parent", got.Competition, got.LabelSource)
	}
}

func TestRelabelUnplayedByTeams(t *testing.T) {
	fixtures := []savefile.Fixture{
		{Key: 2, Home: 1, Away: 2, Date: day(2026, 1, 10), Season: 2025},
		{Key: 1, Home: 1, Away: 3, Date: day(2025, 7, 20), Season: 2025},
	}
	g := subGame([]savefile.LeagueTable{subTable(false)}, fixtures...)

	if got := g.tables[0]; got.Competition != "Mexican First Division Closing Stage" || got.LabelSource != labelFromResults {
		t.Errorf("table = %s (%s), want the Closing Stage by its teams", got.Competition, got.LabelSource)
	}
}

func TestRelabelAmbiguousKeepsFile(t *testing.T) {
	fixtures := openingFixtures()
	for _, f := range fixtures[:2] {
		f.Key = 2
		fixtures = append(fixtures, f) // the Closing Stage repeats the results
	}
	g := subGame([]savefile.LeagueTable{subTable(true)}, fixtures...)

	if got := g.tables[0]; got.Competition != "Mexican First Division" || got.LabelSource != labelFromFile {
		t.Errorf("table = %s (%s), want the parent when two match", got.Competition, got.LabelSource)
	}
}

func TestRelabelUsesExtraTimeAndLatestSeason(t *testing.T) {
	fixtures := []savefile.Fixture{
		// 1–1 after 90 minutes, 2–1 after extra time: a win.
		{Key: 1, Home: 1, Away: 2, Date: day(2025, 7, 20), Season: 2025, Result: &savefile.FixtureResult{Home: [4]uint8{1, 2, 0xff, 0xff}, Away: [4]uint8{1, 1, 0xff, 0xff}}},
		// A drawn group match lost on penalties counts as drawn (shoot-out loss).
		{Key: 1, Home: 2, Away: 1, Date: day(2025, 7, 27), Season: 2025, Result: &savefile.FixtureResult{Home: [4]uint8{0, 0, 4, 0xff}, Away: [4]uint8{0, 0, 3, 0xff}}},
		// Last season's result is ignored.
		{Key: 1, Home: 1, Away: 2, Date: day(2024, 7, 20), Season: 2024, Result: played(5, 0)},
	}
	table := savefile.LeagueTable{CompetitionUID: 100, Stage: -1, Rows: []savefile.TableRow{
		{Team: 1, Overall: savefile.TableStats{Played: 2, Won: 1, ShootoutLosses: 1, GoalsFor: 2, GoalsAgainst: 1, Points: 4}},
		{Team: 2, Position: 1, Overall: savefile.TableStats{Played: 2, Drawn: 1, Lost: 1, GoalsFor: 1, GoalsAgainst: 2, Points: 2}},
	}}
	g := subGame([]savefile.LeagueTable{table}, fixtures...)

	if got := g.tables[0]; got.CompetitionIndex != 2 || got.LabelSource != labelFromResults {
		t.Errorf("table = %s (%s), want the Opening Stage", got.Competition, got.LabelSource)
	}
}

func TestRealSaveSubCompetitions(t *testing.T) {
	g := loadRealSaveNamed(t, "FM_REAL_SAVE", time.Date(2035, 12, 2, 0, 0, 0, 0, time.UTC))
	labels := map[string]string{}
	relabelled := 0
	for _, tb := range g.tables {
		if tb.LabelSource == labelFromResults {
			relabelled++
		}
		labels[tb.Competition] = tb.LabelSource
	}
	t.Logf("%d of %d tables relabelled", relabelled, len(g.tables))
	noDuplicateRelabels(t, g.tables)
	for _, want := range []string{"Mexican First Division Opening Stage", "Spanish U19 Division 1 Group 1", "Spanish U19 Division 1 Group 7"} {
		if labels[want] != labelFromResults {
			t.Errorf("no table labelled %q by results", want)
		}
	}
	if labels["Mexican First Division"] != labelFromFile {
		t.Error("the unplayed Closing Stage table should keep the parent's name")
	}
	metz := int64(652)
	tables, err := g.View(false).Tables(&metz, nil)
	if err != nil || len(tables) != 1 || tables[0].LabelSource != labelFromFile || !strings.HasPrefix(tables[0].Competition, "Ligue 1") {
		t.Errorf("Ligue 1 = %+v, %v", tables, err)
	}
}

func TestTestSaveSubCompetitions(t *testing.T) {
	if gameDBDir() == "" {
		t.Skip("game not installed")
	}
	g := loadTestSave(t)
	found := map[string]bool{}
	for _, tb := range g.tables {
		if tb.LabelSource == labelFromResults {
			found[tb.Competition] = true
		}
	}
	t.Logf("%d sub-competition labels", len(found))
	noDuplicateRelabels(t, g.tables)
	if !slicesContain(g.Info.Warnings, "2 of 240 league stages") {
		t.Errorf("warnings = %q, want the 2 of 240 league-stage gap", g.Info.Warnings)
	}
	for _, want := range []string{"English U18 Division 1 North", "English U18 Division 1 South", "Spanish U19 Division 1 Group 1", "Spanish U19 Division 1 Group 7"} {
		if !found[want] {
			t.Errorf("no table labelled %q by results", want)
		}
	}
}

// TestRelabelKeepsFileForDuplicates: two different tables that would get the
// same sub-competition and stage cannot be told apart, so both keep the
// file's label.
func TestRelabelKeepsFileForDuplicates(t *testing.T) {
	fixtures := []savefile.Fixture{{Key: 2, Home: 1, Away: 2, Date: day(2026, 1, 10), Season: 2025}}
	other := savefile.LeagueTable{CompetitionUID: 100, Stage: -1, Rows: []savefile.TableRow{{Team: 2}, {Team: 1, Position: 1}}}
	g := subGame([]savefile.LeagueTable{subTable(false), other}, fixtures...)

	for i, got := range g.tables {
		if got.Competition != "Mexican First Division" || got.LabelSource != labelFromFile {
			t.Errorf("table %d = %s (%s), want the parent", i, got.Competition, got.LabelSource)
		}
	}
}

// noDuplicateRelabels fails when two tables were relabelled to the same
// competition and stage.
func noDuplicateRelabels(t *testing.T, tables []Table) {
	t.Helper()
	seen := map[[2]int64]bool{}
	for _, tb := range tables {
		if tb.LabelSource != labelFromResults {
			continue
		}
		k := [2]int64{tb.CompetitionIndex, int64(tb.stage)}
		if seen[k] {
			t.Errorf("two tables relabelled %s / %s", tb.Competition, tb.Stage)
		}
		seen[k] = true
	}
}

// TestIdenticalTablesServedOnce: an exact copy of a table in the same file
// is dropped, so the table keeps its sub-competition label (the test save
// stores Spanish U19 Division 1's groups twice).
func TestIdenticalTablesServedOnce(t *testing.T) {
	fixtures := []savefile.Fixture{{Key: 2, Home: 1, Away: 2, Date: day(2026, 1, 10), Season: 2025}}
	g := subGame([]savefile.LeagueTable{subTable(false), subTable(false)}, fixtures...)

	if len(g.tables) != 1 || g.tables[0].Competition != "Mexican First Division Closing Stage" {
		t.Errorf("tables = %+v, want one Closing Stage table", g.tables)
	}
	if len(g.Info.Warnings) != 0 {
		t.Errorf("warnings = %q: the dropped table was read", g.Info.Warnings)
	}
}

func TestEqualRowsInOtherStagesOrFilesAreKept(t *testing.T) {
	otherStage, otherFile := subTable(false), subTable(false)
	otherStage.Stage = 74
	otherFile.CompetitionUID = 101
	g := subGame([]savefile.LeagueTable{subTable(false), otherStage, otherFile})

	if len(g.tables) != 3 {
		t.Errorf("%d tables, want 3", len(g.tables))
	}
}
