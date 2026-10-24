package fm

import (
	"encoding/binary"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Napas/fm-mcp/internal/savefile"
)

func statsOf(p, w, d, l, gf, ga uint8, pts int16) savefile.TableStats {
	return savefile.TableStats{Played: p, Won: w, Drawn: d, Lost: l, GoalsFor: uint16(gf), GoalsAgainst: uint16(ga), Points: pts}
}

func rawRow(team int32, pos uint8, overall savefile.TableStats, form ...uint8) savefile.TableRow {
	r := savefile.TableRow{Team: team, Position: pos, Overall: overall, Home: overall}
	copy(r.Form[:], form)
	return r
}

// tableWorld is fixtureWorld with COMP uids: index 8 is uid 12, 11 is 16.
func tableWorld() World {
	w := fixtureWorld()
	w.CompetitionUIDs = make([]uint32, 13)
	w.CompetitionUIDs[8], w.CompetitionUIDs[11], w.CompetitionUIDs[12] = 12, 16, 17
	return w
}

func tableGame(manager int64, raw savefile.LeagueTables, leagueStages uint32) *Game {
	return NewGame(Info{Date: gameDate, ManagerTeamIndex: manager}, nil, tableWorld(), PlayerExtras{Tables: &raw, LeagueStages: leagueStages})
}

var championshipTable = savefile.LeagueTable{CompetitionUID: 12, Stage: -1, Rows: []savefile.TableRow{
	rawRow(1, 0, statsOf(3, 2, 1, 0, 5, 2, 7), 3, 2, 3, 0),
	rawRow(2, 1, statsOf(3, 0, 1, 2, 2, 5, 1), 1, 2, 1),
}}

var groupTable = savefile.LeagueTable{CompetitionUID: 16, Stage: 236, Rows: []savefile.TableRow{
	rawRow(3, 0, statsOf(1, 1, 0, 0, 2, 0, 3), 3),
	rawRow(1, 1, statsOf(1, 0, 0, 1, 0, 2, 0), 1),
}}

func TestTablesByCompetition(t *testing.T) {
	g := tableGame(1, savefile.LeagueTables{Tables: []savefile.LeagueTable{championshipTable, groupTable}}, 2)
	comp := int64(11)

	got, err := g.View(false).Tables(nil, &comp)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Competition != "Ligue 1" || got[0].Stage != "Playoffs" || got[0].LabelSource != labelFromFile {
		t.Fatalf("tables = %+v", got)
	}
	if r := got[0].Rows[0]; r.Position != 1 || r.TeamIndex != 3 || r.Team != "Third FC" || r.Points != 3 || r.GoalDifference != 2 || r.Form != "W" {
		t.Errorf("row = %+v", r)
	}
}

func TestTablesByTeamDivisionFirst(t *testing.T) {
	g := tableGame(1, savefile.LeagueTables{Tables: []savefile.LeagueTable{groupTable, championshipTable}}, 2)

	got, err := g.View(false).Tables(nil, nil) // the manager's team 1, division 8
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Competition != "Sky Bet Championship" {
		t.Fatalf("tables = %+v, want the division's table only", got)
	}
	r := got[0].Rows[0]
	if r.Played != 3 || r.Won != 2 || r.Drawn != 1 || r.GoalsFor != 5 || r.GoalsAgainst != 2 || r.Home.Played != 3 || r.Form != "WDW" || r.ShootoutLosses != 0 {
		t.Errorf("row = %+v", r)
	}

	three := int64(3) // division 11 has no table containing it except the group
	got, err = g.View(false).Tables(&three, nil)
	if err != nil || len(got) != 1 || got[0].Stage != "Playoffs" {
		t.Errorf("team 3: tables = %+v, %v", got, err)
	}
}

func TestTablesNoTable(t *testing.T) {
	g := tableGame(1, savefile.LeagueTables{Tables: []savefile.LeagueTable{championshipTable}}, 5)
	comp := int64(12)

	_, err := g.View(false).Tables(nil, &comp)
	if !errors.Is(err, ErrNoTable) || !strings.Contains(err.Error(), "no table") || !strings.Contains(err.Error(), "4 of 5 league stages") {
		t.Errorf("err = %v, want ErrNoTable naming the table warning", err)
	}
	three := int64(3)
	if _, err := g.View(false).Tables(&three, nil); !errors.Is(err, ErrNoTable) {
		t.Errorf("team without a table: err = %v", err)
	}
	missing := int64(99)
	if _, err := g.View(false).Tables(&missing, nil); !errors.Is(err, ErrInvalidFilter) {
		t.Errorf("unknown team: err = %v", err)
	}
}

func TestTableWarnings(t *testing.T) {
	g := tableGame(1, savefile.LeagueTables{Tables: []savefile.LeagueTable{championshipTable, {CompetitionUID: 999, Rows: championshipTable.Rows}}, Skipped: 2}, 4)

	for _, want := range []string{"3 of 4 league stages have no readable table", "2 comp files"} {
		if !slicesContain(g.Info.Warnings, want) {
			t.Errorf("warnings %q lack %q", g.Info.Warnings, want)
		}
	}
}

func TestTableShootoutLosses(t *testing.T) {
	raw := championshipTable
	raw.Rows = []savefile.TableRow{rawRow(1, 0, savefile.TableStats{Played: 2, Won: 1, ShootoutLosses: 1, Points: 4}), rawRow(2, 1, statsOf(2, 0, 1, 1, 0, 0, 2))}
	g := tableGame(1, savefile.LeagueTables{Tables: []savefile.LeagueTable{raw}}, 1)

	got, err := g.View(false).Tables(nil, nil)
	if err != nil || got[0].Rows[0].ShootoutLosses != 1 || got[0].Rows[0].Points != 4 {
		t.Errorf("tables = %+v, %v", got, err)
	}
}

func TestTestSavePremierLeagueTable(t *testing.T) {
	if gameDBDir() == "" {
		t.Skip("game not installed")
	}
	g := loadTestSave(t)
	spurs := int64(547)

	got, err := g.View(false).Tables(&spurs, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Competition != "Premier League" || len(got[0].Rows) != 20 {
		t.Fatalf("tables = %d, first %q with %d rows", len(got), got[0].Competition, len(got[0].Rows))
	}
	for _, r := range got[0].Rows {
		if r.Played != 0 {
			t.Errorf("row %+v has played", r)
		}
	}
}

func TestRealSaveLigue1Table(t *testing.T) {
	g := loadRealSaveNamed(t, "FM_REAL_SAVE", time.Date(2035, 12, 2, 0, 0, 0, 0, time.UTC))
	metz := int64(652)

	got, err := g.View(false).Tables(&metz, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || len(got[0].Rows) != 18 {
		t.Fatalf("tables = %+v", got)
	}
	if r := got[0].Rows[0]; r.Team != "Stade Rennais FC" || r.Points != 33 || r.Position != 1 {
		t.Errorf("1st = %+v", r)
	}
	if r := got[0].Rows[1]; r.Team != "FC Metz" || r.Points != 27 || r.GoalDifference != 12 || r.Position != 2 {
		t.Errorf("2nd = %+v", r)
	}
}

func TestTablesUnavailable(t *testing.T) {
	g := NewGame(Info{Date: gameDate, ManagerTeamIndex: 1}, nil, tableWorld(), PlayerExtras{})
	if _, err := g.View(false).Tables(nil, nil); !errors.Is(err, ErrTablesUnavailable) {
		t.Errorf("err = %v, want ErrTablesUnavailable", err)
	}
}

func TestNoTableNamesUnnamedCompetitionCleanly(t *testing.T) {
	g := tableGame(1, savefile.LeagueTables{Tables: []savefile.LeagueTable{championshipTable}}, 1)
	comp := int64(5) // a COMP without a name
	_, err := g.View(false).Tables(nil, &comp)
	if !errors.Is(err, ErrNoTable) || strings.Contains(err.Error(), "()") {
		t.Errorf("err = %v", err)
	}
}

// TestTableRowIsFixedSize: keyOf encodes rows with encoding/binary, which
// needs fixed-size fields.
func TestTableRowIsFixedSize(t *testing.T) {
	if binary.Size(savefile.TableRow{}) <= 0 {
		t.Error("savefile.TableRow is not fixed-size; keyOf would drop distinct tables")
	}
}
