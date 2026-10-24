package savefile

import (
	"encoding/binary"
	"reflect"
	"slices"
	"testing"
)

// appendStats encodes a 17-byte stat record: ref (u8 0, i32 -1), played
// twice, won, drawn, lost, shoot-out losses, u16 for, u16 against, i16 points.
func appendStats(b []byte, s TableStats) []byte {
	b = append(b, 0, 0xff, 0xff, 0xff, 0xff, s.Played, s.Played, s.Won, s.Drawn, s.Lost, s.ShootoutLosses)
	b = binary.LittleEndian.AppendUint16(b, s.GoalsFor)
	b = binary.LittleEndian.AppendUint16(b, s.GoalsAgainst)
	return binary.LittleEndian.AppendUint16(b, uint16(s.Points)) //nolint:gosec // signed field
}

// tableRowBytes encodes a table row in the documented grammar: the base
// (team ref, position and 13 more u8, i32, 5 stat records, no head-to-head,
// 12 bytes), 2 round positions, no m items, no sub-object, form, no nested row.
func tableRowBytes(row TableRow) []byte {
	b := append([]byte{0}, binary.LittleEndian.AppendUint32(nil, uint32(row.Team))...) //nolint:gosec // signed field
	b = append(b, row.Position)
	b = append(b, make([]byte, 13+4)...)
	for _, s := range []TableStats{row.Overall, row.Home, row.Away, row.Overall, {}} {
		b = appendStats(b, s)
	}
	b = append(b, 0)
	b = append(b, make([]byte, 12)...)
	b = append(b, 2, 0, row.Position, 0xff)
	b = append(b, 0, 0)
	b = append(b, row.Form[:]...)
	return append(b, 0)
}

// tableList encodes a u8-counted row list.
func tableList(rows ...TableRow) []byte {
	b := []byte{byte(len(rows))} //nolint:gosec // small test data
	for _, r := range rows {
		b = append(b, tableRowBytes(r)...)
	}
	return b
}

// stagePrefix is the 17 bytes before a table's count, starting with the
// i16 stage-name index.
func stagePrefix(stage int16) []byte {
	b := binary.LittleEndian.AppendUint16(nil, uint16(stage)) //nolint:gosec // signed field
	return append(b, 0x20, 0, 0, 0, 0xff, 0xff, 0xff, 0xff, 0xff, 0, 0, 0, 0, 0, 1)
}

var testTableRows = []TableRow{
	{Team: 685, Position: 0, Overall: TableStats{Played: 3, Won: 2, Drawn: 1, GoalsFor: 5, GoalsAgainst: 2, Points: 7},
		Home: TableStats{Played: 2, Won: 2, GoalsFor: 4, GoalsAgainst: 1, Points: 6}, Away: TableStats{Played: 1, Drawn: 1, GoalsFor: 1, GoalsAgainst: 1, Points: 1},
		Form: [10]uint8{3, 2, 3}},
	{Team: 652, Position: 1, Overall: TableStats{Played: 3, Won: 1, Drawn: 1, Lost: 1, GoalsFor: 3, GoalsAgainst: 3, Points: 4},
		Home: TableStats{Played: 1, Won: 1, GoalsFor: 2, Points: 3}, Away: TableStats{Played: 2, Drawn: 1, Lost: 1, GoalsFor: 1, GoalsAgainst: 3, Points: 1},
		Form: [10]uint8{1, 2, 3}},
	{Team: 660, Position: 2, Overall: TableStats{Played: 2, Lost: 1, ShootoutLosses: 1, GoalsFor: 1, GoalsAgainst: 4, Points: 1},
		Home: TableStats{Played: 1, Lost: 1, GoalsAgainst: 3}, Away: TableStats{Played: 1, ShootoutLosses: 1, GoalsFor: 1, GoalsAgainst: 1, Points: 1},
		Form: [10]uint8{2, 1}},
}

// tableFile is a comp file body holding one table with stage and its
// partner list in reverse order.
func tableFile(stage int16, rows []TableRow) []byte {
	b := append([]byte{0xaa, 0xbb, 0xcc}, stagePrefix(stage)...)
	b = append(b, tableList(rows...)...)
	partner := slices.Clone(rows)
	slices.Reverse(partner)
	b = append(b, tableList(partner...)...)
	return append(b, 0xdd, 0xee)
}

func TestParseLeagueTable(t *testing.T) {
	got, truncated := parseLeagueTables(tableFile(74, testTableRows), 16, 285)

	want := []LeagueTable{{CompetitionUID: 16, Stage: 74, Rows: testTableRows}}
	if !reflect.DeepEqual(got, want) || truncated {
		t.Errorf("tables = %+v, want %+v (the partner list skipped)", got, want)
	}
}

func TestParseLeagueTableBadStage(t *testing.T) {
	for _, stage := range []int16{2025, -248, 285} {
		got, _ := parseLeagueTables(tableFile(stage, testTableRows), 16, 285)
		if len(got) != 1 || got[0].Stage != -1 {
			t.Errorf("stage %d: tables = %+v, want one with no stage", stage, got)
		}
	}
}

func TestParseLeagueTableRejects(t *testing.T) {
	for name, mutate := range map[string]func([]TableRow){
		"position not the index": func(rs []TableRow) { rs[1].Position = 2 },
		"overall not home+away":  func(rs []TableRow) { rs[0].Overall.GoalsFor = 6 },
		"played not W+D+L+X":     func(rs []TableRow) { rs[2].Overall.Played, rs[2].Home.Played = 3, 2 },
		"negative team":          func(rs []TableRow) { rs[0].Team = -5 },
	} {
		rows := slices.Clone(testTableRows)
		mutate(rows)
		body := append(stagePrefix(-1), tableList(rows...)...)
		if got, _ := parseLeagueTables(body, 16, 285); len(got) != 0 {
			t.Errorf("%s: tables = %+v, want none", name, got)
		}
	}
	one := append(stagePrefix(-1), tableList(testTableRows[0])...)
	if got, _ := parseLeagueTables(one, 16, 285); len(got) != 0 {
		t.Errorf("one-row table accepted: %+v", got)
	}
}

func TestParseLeagueTableRejectsSubObject(t *testing.T) {
	body := append(stagePrefix(-1), tableList(testTableRows...)...)
	// The second row's sub-object flag sits 12 bytes before its end.
	end := len(body)
	body[end-12] = 1
	if got, _ := parseLeagueTables(body, 16, 285); len(got) != 0 {
		t.Errorf("tables = %+v, want none", got)
	}
}

func TestTestSaveLeagueTables(t *testing.T) {
	a := openTestSave(t)
	tables, err := ReadLeagueTables(a, 285)
	if err != nil {
		t.Fatal(err)
	}
	if len(tables.Tables) != 238 || tables.Skipped != 0 {
		t.Errorf("%d tables, %d skipped files; want 238, 0", len(tables.Tables), tables.Skipped)
	}
	for _, tb := range tables.Tables {
		if tb.CompetitionUID != 11 { // Premier League
			continue
		}
		if len(tb.Rows) != 20 {
			t.Errorf("Premier League table has %d rows", len(tb.Rows))
		}
		for _, r := range tb.Rows {
			if r.Overall.Played != 0 {
				t.Errorf("row %+v has played", r)
			}
		}
		return
	}
	t.Error("no Premier League table")
}

// checkRealSaveTables checks the KŽ save of 2035-12-02: the table count and
// Ligue 1 (comp_16, COMP 11) against the decoded example and the fixtures.
func checkRealSaveTables(t *testing.T, a *Archive, db *GameDB) {
	t.Helper()
	tables, err := ReadLeagueTables(a, len(db.StageUIDs))
	if err != nil {
		t.Fatal(err)
	}
	if len(tables.Tables) != 603 || tables.Skipped != 0 {
		t.Errorf("%d tables, %d skipped files; want 603, 0", len(tables.Tables), tables.Skipped)
	}
	i := slices.IndexFunc(tables.Tables, func(tb LeagueTable) bool { return tb.CompetitionUID == 16 })
	if i < 0 {
		t.Fatal("no Ligue 1 table")
	}
	rows := tables.Tables[i].Rows
	if len(rows) != 18 {
		t.Fatalf("Ligue 1 has %d rows", len(rows))
	}
	if r := rows[0]; r.Team != 685 || r.Overall != (TableStats{Played: 15, Won: 10, Drawn: 3, Lost: 2, GoalsFor: 28, GoalsAgainst: 12, Points: 33}) {
		t.Errorf("1st = %+v", r)
	}
	if r := rows[1]; r.Team != 652 || r.Overall.Won != 7 || r.Overall.Drawn != 6 || r.Overall.Lost != 2 || r.Overall.GoalsFor != 34 || r.Overall.GoalsAgainst != 22 || r.Overall.Points != 27 ||
		r.Home.Won != 4 || r.Home.Drawn != 2 || r.Home.Lost != 1 || r.Away.Won != 3 || r.Away.Drawn != 4 || r.Away.Lost != 1 {
		t.Errorf("2nd = %+v", r)
	}
	checkFormMatchesFixtures(t, a, db, rows[0], 11)
}

// checkFormMatchesFixtures compares the row's first 7 form values (3 win,
// 2 draw, 1 loss, most recent first) with the team's 7 latest played
// fixtures in competition comp.
func checkFormMatchesFixtures(t *testing.T, a *Archive, db *GameDB, row TableRow, comp int32) {
	t.Helper()
	fs, err := ReadFixtures(a)
	if err != nil {
		t.Fatal(err)
	}
	type result struct {
		date  Date
		value uint8
	}
	var results []result
	for _, f := range fs {
		if f.Result == nil || db.CompetitionKeys[f.Key].Comp != comp || f.Home != row.Team && f.Away != row.Team {
			continue
		}
		mine, theirs := f.Result.Home[0], f.Result.Away[0]
		if f.Away == row.Team {
			mine, theirs = theirs, mine
		}
		v := uint8(2)
		if mine > theirs {
			v = 3
		} else if mine < theirs {
			v = 1
		}
		results = append(results, result{f.Date, v})
	}
	if len(results) < 7 {
		t.Fatalf("%d results", len(results))
	}
	slices.SortFunc(results, func(x, y result) int { return y.date.Time().Compare(x.date.Time()) })
	for k := range 7 {
		if results[k].value != row.Form[k] {
			t.Errorf("form %v, latest results %v", row.Form, results[:7])
			return
		}
	}
}

// TestParseLeagueTableSkipsSameOrderPartner: before the season the partner
// list holds the rows in the same order, so it validates as a table too.
func TestParseLeagueTableSkipsSameOrderPartner(t *testing.T) {
	body := append(stagePrefix(-1), tableList(testTableRows...)...)
	body = append(body, tableList(testTableRows...)...)

	if got, _ := parseLeagueTables(body, 16, 285); len(got) != 1 {
		t.Errorf("%d tables, want 1 (the partner list skipped)", len(got))
	}
}

// otherTeams is testTableRows with other teams in reverse order: a list
// that parses but is neither the partner nor a candidate table.
func otherTeams() []TableRow {
	rows := slices.Clone(testTableRows)
	for i := range rows {
		rows[i].Team += 1000
	}
	slices.Reverse(rows)
	return rows
}

func TestScanLeagueTablesChargesEveryPartnerParse(t *testing.T) {
	table := tableList(testTableRows...)
	body := append(stagePrefix(-1), table...)
	body = append(body, tableList(otherTeams()...)...)

	// The budget covers the table only; reading the following list, which
	// is not its partner, must still be charged.
	if _, truncated := scanLeagueTables(body, 16, 285, len(table)+1); !truncated {
		t.Error("following-list parse not charged to the budget")
	}
	if got, truncated := scanLeagueTables(body, 16, 285, 3*len(body)); truncated || len(got) != 1 {
		t.Errorf("%d tables, truncated %v; want 1, false", len(got), truncated)
	}
}

func TestScanLeagueTablesStopsAtBudget(t *testing.T) {
	if got, truncated := scanLeagueTables(tableFile(74, testTableRows), 16, 285, 1); !truncated || len(got) != 0 {
		t.Errorf("%d tables, truncated %v; want 0, true", len(got), truncated)
	}
}

// chained sets row's chained-row flag and appends next after it.
func chained(row []byte, next []byte) []byte {
	row = slices.Clone(row)
	row[len(row)-1] = 1
	return append(row, next...)
}

func TestParseLeagueTableDropsChainedRows(t *testing.T) {
	extra := tableRowBytes(TableRow{Team: 9, Position: 5})
	body := append(stagePrefix(-1), 2)
	body = append(body, chained(tableRowBytes(testTableRows[0]), extra)...)
	body = append(body, tableRowBytes(testTableRows[1])...)

	got, _ := parseLeagueTables(body, 16, 285)
	if len(got) != 1 || !reflect.DeepEqual(got[0].Rows, testTableRows[:2]) {
		t.Errorf("tables = %+v, want the two rows without the chained one", got)
	}
}

func TestParseLeagueTableRejectsDeepChains(t *testing.T) {
	row := tableRowBytes(TableRow{Team: 9})
	for range maxTableRowChain + 1 {
		row = chained(tableRowBytes(TableRow{Team: 9}), row)
	}
	body := append(stagePrefix(-1), 2)
	body = append(body, chained(tableRowBytes(testTableRows[0]), row)...)
	body = append(body, tableRowBytes(testTableRows[1])...)

	if got, _ := parseLeagueTables(body, 16, 285); len(got) != 0 {
		t.Errorf("tables = %+v, want none", got)
	}
}

func TestScanLeagueTablesChargesRejectedCandidates(t *testing.T) {
	rows := slices.Clone(testTableRows)
	rows[1].Position = 2 // fails validation after a full parse
	list := tableList(rows...)
	body := append(stagePrefix(-1), list...)

	if _, truncated := scanLeagueTables(body, 16, 285, len(list)-1); !truncated {
		t.Error("a rejected candidate's parse is not charged to the budget")
	}
}
