package savefile

import (
	"reflect"
	"slices"
	"testing"
	"time"
)

const (
	testSave27Path  = "../../data/test_save_27.fm"
	testSave27bPath = "../../data/test_save_27b.fm" // after Kauno Žalgiris's first match
	kzHuman27       = 115841
	kzTeam27        = 16521
)

// kzTeamEntries27 are the human's team entries in both FM27 saves.
var kzTeamEntries27 = []int32{kzTeam27, 16546, 16573, 50315}

func TestTestSave27Clubs(t *testing.T) {
	a := openTestSaveAt(t, testSave27Path)

	db, err := ReadGameDB(a)
	if err != nil {
		t.Fatal(err)
	}

	if len(db.Clubs) != 30100 || len(db.Teams) != 66492 || len(db.CompetitionUIDs) != 8199 {
		t.Fatalf("got %d clubs, %d teams, %d competitions", len(db.Clubs), len(db.Teams), len(db.CompetitionUIDs))
	}
	arsenal := db.Teams[462]
	club := db.Clubs[arsenal.Container]
	if arsenal.UID != 602 || arsenal.Container != 331 || club.Name != "Arsenal" || club.Women || len(club.Teams) != 3 || club.Teams[0] != 462 {
		t.Errorf("team 462 = %+v, club = %+v", arsenal, club)
	}
	if bayern := db.Clubs[557]; bayern.ShortName != "FC Bayern" || len(bayern.AffiliatedTeams) != 1 || bayern.AffiliatedTeams[0] != 2259 {
		t.Errorf("club 557 = %+v, want FC Bayern with affiliated team 2259", bayern)
	}
	kz := db.Clubs[15933]
	if kz.UID != 47040647 || kz.Name != "FK Kauno Žalgiris" || kz.Balance != 3870421 {
		t.Errorf("club 15933 = %+v, want FK Kauno Žalgiris", kz)
	}
	checkClubBudgets(t, db, 47040647, Budgets{Transfer: 2078958, WeeklyWage: 21687})
	checkClubBudgets(t, db, 602, Budgets{Transfer: 152700000, WeeklyWage: 4940326})
}

// FM27 rates differ from FM26's (see "Currencies").
func TestTestSave27Currencies(t *testing.T) {
	for _, path := range []string{testSave27Path, testSave27bPath} {
		t.Run(path, func(t *testing.T) {
			db, err := ReadGameDB(openTestSaveAt(t, path))
			if err != nil {
				t.Fatal(err)
			}
			checkCurrencyTable(t, db, 1.164101, 1.347619)
		})
	}
}

func TestTestSave27GameDBHeader(t *testing.T) {
	a := openTestSaveAt(t, testSave27Path)
	f, err := a.ReadFile("game_db.dat")
	if err != nil {
		t.Fatal(err)
	}

	h, err := ParseGameDBHeader(f.Body)
	if err != nil {
		t.Fatal(err)
	}

	if h.DatabaseVersion != 3754 || h.OriginalDatabaseVersion != 3753 || h.Label != "2700" {
		t.Errorf("versions %d/%d, label %q", h.DatabaseVersion, h.OriginalDatabaseVersion, h.Label)
	}
	if h.Timestamp != 63927573184000 || h.Year != 2026 || h.Unknown808 != 701095 {
		t.Errorf("Timestamp %d, Year %d, Unknown808 %d", h.Timestamp, h.Year, h.Unknown808)
	}
	if got, want := h.GameDate.Time(), time.Date(2026, 1, 19, 0, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Errorf("GameDate = %s, want %s", got, want)
	}
}

// No match has been played in the FM27 save: only the internationals slot is
// kept, and it is empty.
func TestTestSave27SeasonStats(t *testing.T) {
	a := openTestSaveAt(t, testSave27Path)

	season, err := ReadSeasonStats(a)
	if err != nil {
		t.Fatal(err)
	}

	if len(season) != 937 {
		t.Errorf("%d players with season stats, want 937", len(season))
	}
	for person, rec := range season {
		if rec.Club != nil || rec.Friendlies != nil || rec.League != nil || rec.Cups != nil || rec.Continental != nil {
			t.Fatalf("person %d has club scopes", person)
		}
		if *rec.Internationals != (MatchStats{}) {
			t.Fatalf("person %d internationals = %+v, want empty", person, *rec.Internationals)
		}
	}
}

// Kauno Žalgiris (team 16521) open the 2026 A Lyga (key 379) at home to team
// 6215 on 2026-02-28, in round 1 (bye in round 0). The first played fixture is
// a 2026-01-03 friendly (key 2895) lost 0–1.
func TestTestSave27Fixtures(t *testing.T) {
	fs, err := ReadFixtures(openTestSaveAt(t, testSave27Path))
	if err != nil {
		t.Fatal(err)
	}

	played := 0
	for _, f := range fs {
		if f.Result != nil {
			played++
		}
	}
	if len(fs) != 23460 || played != 209 {
		t.Errorf("%d fixtures, %d played; want 23460, 209", len(fs), played)
	}
	first := slices.IndexFunc(fs, func(f Fixture) bool { return f.Key == 379 && (f.Home == 16521 || f.Away == 16521) })
	if first < 0 {
		t.Fatal("no A Lyga fixture for Kauno Žalgiris")
	}
	got := fs[first]
	want := Fixture{Key: 379, Home: 16521, Away: 6215, Date: got.Date, Season: 2026, Round: 1}
	if !reflect.DeepEqual(got, want) || !got.Date.Time().Equal(time.Date(2026, 2, 28, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("first A Lyga fixture = %+v, want %+v on 2026-02-28", got, want)
	}
	i := slices.IndexFunc(fs, func(f Fixture) bool { return f.Result != nil })
	if i < 0 {
		t.Fatal("no played fixture")
	}
	friendly := fs[i]
	if friendly.Key != 2895 || friendly.Season != 0 || friendly.Round != 255 ||
		friendly.Result.Home != [4]uint8{0, 0xff, 0xff, 0xff} || friendly.Result.Away != [4]uint8{1, 0xff, 0xff, 0xff} ||
		!friendly.Date.Time().Equal(time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("first played fixture = %+v, result %+v", friendly, friendly.Result)
	}
}

// readTactics27 reads the tactics of an FM27 save and the tactics_man.dat body.
func readTactics27(t *testing.T, path string) (Tactics, []byte) {
	t.Helper()
	a := openTestSaveAt(t, path)
	tac, err := ReadTactics(a)
	if err != nil {
		t.Fatal(err)
	}
	f, err := a.ReadFile("tactics_man.dat")
	if err != nil {
		t.Fatal(err)
	}
	if tac.WalkErr != nil || tac.TacticErr != nil {
		t.Errorf("walk error %v, tactic error %v; want none", tac.WalkErr, tac.TacticErr)
	}
	return tac, f.Body
}

// No team has played: every last lineup is empty, and no team entry holds a
// tactic.
func TestTestSave27Tactics(t *testing.T) {
	tac, body := readTactics27(t, testSave27Path)

	if tac.Human != kzHuman27 || tac.Team != kzTeam27 || len(tac.Tactics) != 0 || tac.Selection != emptySelection() {
		t.Errorf("human %d, team %d, %d tactics, selection %v", tac.Human, tac.Team, len(tac.Tactics), tac.Selection)
	}
	walkedEntries(t, body, kzHuman27, kzTeamEntries27, []int32{-1, -1, -1, -1})
	checkAllFormations(t, tac, 1646)
	for team, l := range tac.LastLineups {
		if l.Opponent != -1 || l.Players != emptySelection() {
			t.Fatalf("team %d: last lineup %+v, want empty", team, l)
		}
	}
}

// kzSlots27 builds KŽ's 4-3-3 DM Wide slots: the preset in possession, the
// given positions and roles out of possession.
func kzSlots27(oopPos [11]uint32, oopRole [11]uint64) [tacticSlots]TacticSlot {
	ipPos := [11]uint32{1, 4, 0x200010, 0x100010, 8, 0x80, 0x200400, 0x100400, 0x800, 0x1000, 0x4000}
	ipRole := [11]uint64{0x200001, 0x400004, 0x200002, 0x200002, 0x400004, 0x200010, 0x400020, 0x800200, 0x400080, 0x400080, 0x800800}
	var s [tacticSlots]TacticSlot
	for i := range s {
		s[i] = TacticSlot{
			InPossession:    SlotRole{Position: ipPos[i], Role: ipRole[i]},
			OutOfPossession: SlotRole{Position: oopPos[i], Role: oopRole[i]},
		}
	}
	return s
}

// kzTactic27 is the preset with slots 5 to 10 changed out of possession. In
// game (2026-01-23) the CM (MC right) and AM (MC left) drop to CM, and the
// right winger to WMF on the right.
func kzTactic27() Tactic {
	return Tactic{Name: "4-3-3 DM Wide", Mentality: 4, Slots: kzSlots27(
		[11]uint32{1, 4, 0x200010, 0x100010, 8, 0x80, 0x200400, 0x100400, 0x100, 0x200, 0x4000},
		[11]uint64{0x400001, 0x200200, 0x200008, 0x200008, 0x200200, 0x208000, 0x500000, 0x500000, 0x800400000, 0x800400000, 0x40000400000},
	)}
}

// kzFormation27 is KŽ's last-match formation: out-of-possession positions in
// the tactic's stored order, no slot numbers.
func kzFormation27() [tacticSlots]TacticSlot {
	return kzSlots27(
		[11]uint32{1, 4, 0x200010, 0x100010, 8, 0x80, 0x100, 0x200400, 0x100400, 0x200, 0x4000},
		[11]uint64{0x400001, 0x200200, 0x200008, 0x200008, 0x200200, 0x208000, 0x800400000, 0x500000, 0x500000, 0x800400000, 0x40000400000},
	)
}

// Kauno Žalgiris played FK Banga (team 6215): the last lineup holds the XI
// in shirts 1 to 11 and the bench in 12 to 27, in the tactic's shape.
func TestTestSave27bTactics(t *testing.T) {
	tac, body := readTactics27(t, testSave27bPath)

	if tac.Human != kzHuman27 || tac.Team != kzTeam27 || tac.Active != 0 || len(tac.Tactics) != 1 {
		t.Fatalf("human %d, team %d, active %d, %d tactics", tac.Human, tac.Team, tac.Active, len(tac.Tactics))
	}
	if got, want := tac.Tactics[0], kzTactic27(); got != want {
		t.Errorf("tactic = %+v\nwant %+v", got, want)
	}
	for _, e := range walkedEntries(t, body, kzHuman27, kzTeamEntries27, []int32{0, 0, 0, 0}) {
		if len(e.tactics) != 1 || e.tactics[0] != kzTactic27() {
			t.Errorf("team %d: tactics %+v, want the KŽ tactic", e.team, e.tactics)
		}
	}
	checkAllFormations(t, tac, 2506)
	kz, banga := tac.LastLineups[kzTeam27], tac.LastLineups[6215]
	if kz.Opponent != 6215 || banga.Opponent != kzTeam27 || kz.Players[0] != 20613 {
		t.Errorf("KŽ opponent %d, Banga opponent %d, KŽ goalkeeper %d", kz.Opponent, banga.Opponent, kz.Players[0])
	}
	if want := [selectionSize]uint8{1, 2, 4, 5, 3, 6, 8, 7, 11, 10, 9, 12, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27}; kz.ShirtNumbers != want {
		t.Errorf("KŽ shirt numbers %v, want %v", kz.ShirtNumbers, want)
	}
	for i, s := range kzFormation27() {
		s.InPossession.Role &^= dutyMask
		s.OutOfPossession.Role &^= dutyMask
		if kz.Formation.Slots[i] != s {
			t.Errorf("KŽ formation slot %d = %+v, want %+v", i, kz.Formation.Slots[i], s)
		}
	}
}

// aLyga27 is the 2026 A Lyga's competition uid; its table lists nine teams
// in name order.
const aLyga27 = 2300046

var aLyga27Teams = []int32{6215, 6217, 24793, kzTeam27, 16532, 16547, 4208, 25153, 983}

// Every FM27 comp file reads. Group 41 of competition 150784 (six teams, six
// rounds played) matches its fixtures row for row.
func TestTestSave27LeagueTables(t *testing.T) {
	for _, tc := range []struct {
		path   string
		tables int
	}{{testSave27Path, 138}, {testSave27bPath, 141}} {
		t.Run(tc.path, func(t *testing.T) {
			a := openTestSaveAt(t, tc.path)
			db, err := ReadGameDB(a)
			if err != nil {
				t.Fatal(err)
			}
			tables, err := ReadLeagueTables(a, len(db.StageUIDs))
			if err != nil {
				t.Fatal(err)
			}
			if len(tables.Tables) != tc.tables || tables.Skipped != 0 {
				t.Errorf("%d tables, %d skipped files; want %d, 0", len(tables.Tables), tables.Skipped, tc.tables)
			}
			lyga := findTable(t, tables.Tables, aLyga27, -1)
			for i, r := range lyga.Rows {
				if i >= len(aLyga27Teams) || r.Team != aLyga27Teams[i] || r.Overall != (TableStats{}) {
					t.Errorf("A Lyga row %d = %+v", i, r)
				}
			}
			if len(lyga.Rows) != len(aLyga27Teams) {
				t.Errorf("A Lyga has %d rows", len(lyga.Rows))
			}
			checkTableMatchesFixtures(t, a, db, findTable(t, tables.Tables, 150784, 41))
		})
	}
}

func findTable(t *testing.T, tables []LeagueTable, uid uint32, stage int16) LeagueTable {
	t.Helper()
	i := slices.IndexFunc(tables, func(tb LeagueTable) bool { return tb.CompetitionUID == uid && tb.Stage == stage })
	if i < 0 {
		t.Fatalf("no table for competition %d stage %d", uid, stage)
	}
	return tables[i]
}

// checkTableMatchesFixtures compares each row's overall record, points
// aside, with the played fixtures of its competition and stage between the
// table's teams.
func checkTableMatchesFixtures(t *testing.T, a *Archive, db *GameDB, tb LeagueTable) {
	t.Helper()
	fs, err := ReadFixtures(a)
	if err != nil {
		t.Fatal(err)
	}
	comp := int32(slices.Index(db.CompetitionUIDs, tb.CompetitionUID)) //nolint:gosec // index of a slice
	if comp < 0 {
		t.Fatalf("no competition with uid %d", tb.CompetitionUID)
	}
	got := map[int32]*TableStats{}
	for _, r := range tb.Rows {
		got[r.Team] = &TableStats{}
	}
	for _, f := range fs {
		k := db.CompetitionKeys[f.Key]
		stage := k.Group
		if stage < 0 {
			stage = k.Stage
		}
		home, away := got[f.Home], got[f.Away]
		if f.Result == nil || k.Comp != comp || stage != int32(tb.Stage) || home == nil || away == nil {
			continue
		}
		addResult(home, f.Result.Home[0], f.Result.Away[0])
		addResult(away, f.Result.Away[0], f.Result.Home[0])
	}
	for _, r := range tb.Rows {
		want := r.Overall
		want.Points = 0
		if want.Played == 0 || *got[r.Team] != want {
			t.Errorf("team %d: table %+v, fixtures %+v", r.Team, want, *got[r.Team])
		}
	}
}

func addResult(s *TableStats, scored, conceded uint8) {
	s.Played++
	s.GoalsFor += uint16(scored)
	s.GoalsAgainst += uint16(conceded)
	switch {
	case scored > conceded:
		s.Won++
	case scored < conceded:
		s.Lost++
	default:
		s.Drawn++
	}
}

func TestTestSave27Players(t *testing.T) {
	// injured is the number of entries in the save's injury_manager.dat queues.
	for _, tt := range []struct {
		path    string
		injured int
	}{{testSave27Path, 1579}, {testSave27bPath, 1632}} {
		t.Run(tt.path, func(t *testing.T) {
			a := openTestSaveAt(t, tt.path)
			db, err := ReadGameDB(a)
			if err != nil {
				t.Fatal(err)
			}
			if db.SkippedPlayers != 0 {
				t.Errorf("%d players skipped", db.SkippedPlayers)
			}

			n := 0
			for _, p := range db.Players {
				if len(p.Injuries) > 0 {
					n++
				}
				if p.NationIndex < 0 || p.NationIndex >= int64(db.NationCount) {
					t.Fatalf("player %d (%s %s): nation %d of %d", p.UID, p.FirstName, p.LastName, p.NationIndex, db.NationCount)
				}
				for _, h := range p.Hidden {
					if h < 1 || h > 20 {
						t.Fatalf("player %d (%s %s): hidden attributes %v", p.UID, p.FirstName, p.LastName, p.Hidden)
					}
				}
			}
			if n != tt.injured {
				t.Errorf("%d injured players, want %d", n, tt.injured)
			}
			fox := Injury{ID: 118, Start: Date{Raw: 1, Year: 2026}, DaysMin: 213, DaysMax: 243}
			checkKnownPlayers(t, db.Players, map[uint32]Player{215502: {Injuries: []Injury{fox}}}, func(p, w Player) {
				if !reflect.DeepEqual(p.Injuries, w.Injuries) {
					t.Errorf("player %d (Ben Fox): injuries %+v, want %+v", p.UID, p.Injuries, w.Injuries)
				}
			})
			checkKnownPlayers(t, db.Players, map[uint32]Player{
				18096725: {NationIndex: 131, Hidden: [8]uint8{9, 12, 10, 10, 13, 11, 10, 6}, BirthplaceIndex: 32},   // Anton Tanghe, Belgium
				14192808: {NationIndex: 187, Hidden: [8]uint8{19, 12, 13, 10, 12, 11, 7, 8}, BirthplaceIndex: 1110}, // Franco Baldassarra, Argentina
			}, func(p, w Player) {
				if p.NationIndex != w.NationIndex || p.Hidden != w.Hidden || p.BirthplaceIndex != w.BirthplaceIndex {
					t.Errorf("player %d: nation %d, hidden %v, birthplace %d; want %d, %v, %d",
						p.UID, p.NationIndex, p.Hidden, p.BirthplaceIndex, w.NationIndex, w.Hidden, w.BirthplaceIndex)
				}
			})
		})
	}
}

func TestTestSave27Transfers(t *testing.T) {
	// records is the class 2 and 3 count (docs/format/transfers.md).
	for path, records := range map[string]int{testSave27Path: 33, testSave27bPath: 884} {
		t.Run(path, func(t *testing.T) {
			a := openTestSaveAt(t, path)
			ts, err := ReadTransfers(a)
			if err != nil {
				t.Fatal(err)
			}
			byID := make(map[uint32]Transfer, len(ts))
			arranged := 0
			for _, tr := range ts {
				byID[tr.ID] = tr
				if tr.Status == TransferArranged {
					arranged++
				}
			}
			if len(ts) != records {
				t.Errorf("%d records, want %d", len(ts), records)
			}
			if arranged != 33 {
				t.Errorf("%d arranged records, want 33", arranged)
			}
			for _, want := range []Transfer{
				{ID: 0, Person: 53232, Class: 2, Team: 538, Status: TransferArranged, Date: Date{Raw: 1, Year: 2028}}, // Cavan Sullivan
				{ID: 8, Person: 8353, Class: 2, Team: 463, Status: TransferArranged, Date: Date{Raw: 1, Year: 2027}},  // Ibrahim Dosso
			} {
				if got := byID[want.ID]; got != want {
					t.Errorf("id %d = %+v, want %+v", want.ID, got, want)
				}
			}
		})
	}
}
