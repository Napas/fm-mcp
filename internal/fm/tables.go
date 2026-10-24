package fm

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strings"

	"github.com/Napas/fm-mcp/internal/savefile"
)

var (
	// ErrNoTable means the save stores no table for a competition or team.
	ErrNoTable = errors.New("fm: no stored table")
	// ErrTablesUnavailable means the save's comp files could not be read.
	ErrTablesUnavailable = errors.New("fm: league tables unavailable")
)

// Label sources: the competition whose file holds the table, or the
// sub-competition its results or teams matched.
const (
	labelFromFile    = "file"
	labelFromResults = "results"
)

// formLetters maps stored form values to letters; others mean no result.
var formLetters = map[uint8]byte{3: 'W', 2: 'D', 1: 'L'}

// Table is a stored league table in the game's order. Fog of war hides
// nothing: tables are public.
type Table struct {
	CompetitionIndex int64      `json:"competition_index"`
	Competition      string     `json:"competition,omitempty"`
	Stage            string     `json:"stage,omitempty"` // e.g. "Group A"; empty for a whole league
	LabelSource      string     `json:"label_source"`    // "file" or "results"
	Rows             []TableRow `json:"rows"`
	stage            int32      // STAGE_NAME index; -1 = none
}

// TableRow is a team's line: its overall record, then the rest. Points are
// as stored, deductions and carried-over points included.
type TableRow struct {
	Position  int    `json:"position"` // 1-based
	TeamIndex int64  `json:"team_index"`
	Team      string `json:"team,omitempty"`
	TableRecord
	GoalDifference int         `json:"goal_difference"`
	Points         int         `json:"points"`
	ShootoutLosses int         `json:"shootout_losses,omitempty"` // groups where drawn matches go to penalties; drawn then counts shoot-out wins
	Home           TableRecord `json:"home"`
	Away           TableRecord `json:"away"`
	Form           string      `json:"form,omitempty"` // e.g. "WDLWW", most recent first
}

// TableRecord is a team's overall, home or away record.
type TableRecord struct {
	Played       int `json:"played"`
	Won          int `json:"won"`
	Drawn        int `json:"drawn"`
	Lost         int `json:"lost"`
	GoalsFor     int `json:"goals_for"`
	GoalsAgainst int `json:"goals_against"`
}

// newTables labels the tables by their file's competition and stage. Tables
// of a file whose uid is not a COMP are left out. It returns the tables and
// the warnings about tables that could not be read.
func newTables(raw *savefile.LeagueTables, leagueStages uint32, w World) ([]Table, []string) {
	if raw == nil {
		return nil, nil
	}
	byUID := make(map[uint32]int64, len(w.CompetitionUIDs))
	for i, uid := range w.CompetitionUIDs {
		byUID[uid] = int64(i)
	}
	out := make([]Table, 0, len(raw.Tables))
	seen := make(map[tableKey]bool, len(raw.Tables))
	found := uint32(0)
	for _, rt := range raw.Tables {
		comp, ok := byUID[rt.CompetitionUID]
		if !ok {
			continue
		}
		found++
		k := keyOf(rt)
		if seen[k] {
			continue
		}
		seen[k] = true
		t := Table{CompetitionIndex: comp, Competition: w.Competitions[comp], Stage: w.stageName(int64(rt.Stage)), LabelSource: labelFromFile, Rows: make([]TableRow, len(rt.Rows)), stage: int32(rt.Stage)}
		for i, r := range rt.Rows {
			t.Rows[i] = newTableRow(r, w)
		}
		out = append(out, t)
	}
	var warnings []string
	if found < leagueStages {
		warnings = append(warnings, fmt.Sprintf("%d of %d league stages have no readable table; their competitions show no table", leagueStages-found, leagueStages))
	}
	if raw.Skipped > 0 {
		warnings = append(warnings, fmt.Sprintf("league tables: %d comp files could not be fully read", raw.Skipped))
	}
	return out, warnings
}

// tableKey identifies a table's content: indistinguishable tables (same
// file, stage and rows) are served once. They can be different stages of a
// season: the test save stores Spanish U19 Division 1's groups twice before
// the season, the 2036-01-16 save Argentine Premier Division's groups A and B.
type tableKey struct {
	uid   uint32
	stage int16
	rows  string
}

// keyOf encodes the rows as fixed-size binary (TestTableRowIsFixedSize).
func keyOf(t savefile.LeagueTable) tableKey {
	rows, _ := binary.Append(nil, binary.LittleEndian, t.Rows)
	return tableKey{uid: t.CompetitionUID, stage: t.Stage, rows: string(rows)}
}

func newTableRow(r savefile.TableRow, w World) TableRow {
	o := r.Overall
	row := TableRow{
		Position: int(r.Position) + 1, TeamIndex: int64(r.Team), Team: w.teamName(int64(r.Team)),
		TableRecord: newTableRecord(o), GoalDifference: int(o.GoalsFor) - int(o.GoalsAgainst),
		Points: int(o.Points), ShootoutLosses: int(o.ShootoutLosses),
		Home: newTableRecord(r.Home), Away: newTableRecord(r.Away),
	}
	var form strings.Builder
	for _, v := range r.Form {
		if c, ok := formLetters[v]; ok {
			form.WriteByte(c)
		}
	}
	row.Form = form.String()
	return row
}

func newTableRecord(s savefile.TableStats) TableRecord {
	return TableRecord{Played: int(s.Played), Won: int(s.Won), Drawn: int(s.Drawn), Lost: int(s.Lost), GoalsFor: int(s.GoalsFor), GoalsAgainst: int(s.GoalsAgainst)}
}

// Tables returns the stored tables of a competition, or of a team: the
// tables of its division that hold it, else every table that holds it.
// Both set gives the competition's tables holding the team; neither gives
// the manager's team. It never computes a table: without one it returns
// ErrNoTable.
func (v View) Tables(team, competition *int64) ([]Table, error) {
	team, err := v.g.queryTeam(team, competition)
	if err != nil {
		return nil, err
	}
	if !v.g.hasTables {
		return nil, ErrTablesUnavailable
	}
	var out []Table
	switch {
	case competition != nil:
		out = v.g.tablesWhere(func(t Table) bool { return t.CompetitionIndex == *competition && (team == nil || holds(t, *team)) })
	default:
		div := v.g.world.Teams[*team].divisionIndex
		out = v.g.tablesWhere(func(t Table) bool { return t.CompetitionIndex == div && holds(t, *team) })
		if len(out) == 0 {
			out = v.g.tablesWhere(func(t Table) bool { return holds(t, *team) })
		}
	}
	if len(out) == 0 {
		return nil, v.g.noTable(team, competition)
	}
	return out, nil
}

func (g *Game) tablesWhere(keep func(Table) bool) []Table {
	var out []Table
	for _, t := range g.tables {
		if keep(t) {
			out = append(out, t)
		}
	}
	return out
}

func holds(t Table, team int64) bool {
	for _, r := range t.Rows {
		if r.TeamIndex == team {
			return true
		}
	}
	return false
}

// noTable is the ErrNoTable error for a query, naming the table warnings.
func (g *Game) noTable(team, competition *int64) error {
	var what []string
	if competition != nil {
		what = append(what, named("competition_index", *competition, g.world.Competitions[*competition]))
	}
	if team != nil {
		what = append(what, named("team_index", *team, g.world.teamName(*team)))
	}
	err := fmt.Errorf("%w: the save holds no table for %s; knockout rounds have none (use get_fixtures)", ErrNoTable, strings.Join(what, " and "))
	if len(g.tableWarnings) > 0 {
		err = fmt.Errorf("%w; %s", err, strings.Join(g.tableWarnings, "; "))
	}
	return err
}

// named renders "kind N (name)", or "kind N" without a name.
func named(kind string, index int64, name string) string {
	if name == "" {
		return fmt.Sprintf("%s %d", kind, index)
	}
	return fmt.Sprintf("%s %d (%s)", kind, index, name)
}
