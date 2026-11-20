package savefile

import "testing"

// testSave271Path: a 27.1.0 save (database version 3756) of the KŽ career
// that also holds match objects written by 27.0.0.
const testSave271Path = "../../data/test_save_27_1.fm"

func TestTestSave271Clubs(t *testing.T) {
	db, err := ReadGameDB(openTestSaveAt(t, testSave271Path))
	if err != nil {
		t.Fatal(err)
	}
	if db.Header.DatabaseVersion != dbVersion27_1 || len(db.Clubs) != 30100 {
		t.Fatalf("database version %d, %d clubs; want %d and 30100", db.Header.DatabaseVersion, len(db.Clubs), dbVersion27_1)
	}
	// From the save's bytes; not checked in game.
	if kz := db.Clubs[15933]; kz.UID != 47040647 || kz.Balance != 12690015 {
		t.Errorf("club 15933 = uid %d, balance %d; want KŽ (47040647), balance 12690015", kz.UID, kz.Balance)
	}
	checkClubBudgets(t, db, 47040647, Budgets{Transfer: 1316073, WeeklyWage: 67381})
}

func TestTestSave271Tactics(t *testing.T) {
	tc, err := ReadTactics(openTestSaveAt(t, testSave271Path))
	if err != nil {
		t.Fatal(err)
	}
	if tc.WalkErr != nil || tc.TacticErr != nil || len(tc.Tactics) == 0 || len(tc.LastLineups) != 9108 {
		t.Errorf("walk %v, tactic %v, %d tactics, %d last lineups; want the whole file read and 9108", tc.WalkErr, tc.TacticErr, len(tc.Tactics), len(tc.LastLineups))
	}
}

// TestTestSave271MatchObjects checks that objects of both versions read and
// that their players add up to their team totals.
func TestTestSave271MatchObjects(t *testing.T) {
	objs, err := ReadMatchObjects(openTestSaveAt(t, testSave271Path))
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range objs {
		checkTeamSums(t, o)
	}
	if unread := unreadObjects(objs); len(objs) != 7551 || unread != 0 {
		t.Errorf("%d objects, %d unreadable; want 7551 and 0", len(objs), unread)
	}
}

// TestTestSave271ShiftedFields pins fields read after the 27.1.0 shift on
// match 51085 (KŽ 3–1 Wolfsberger, 2028-03-09). Values are the save's bytes,
// not checked in game.
func TestTestSave271ShiftedFields(t *testing.T) {
	objs, err := ReadMatchObjects(openTestSaveAt(t, testSave271Path))
	if err != nil {
		t.Fatal(err)
	}
	type fields struct {
		rating                                         uint16
		metres                                         int
		sprints, progressive, possessionWon, clearance uint8
	}
	want := map[int32]fields{
		18294: {653, 5466, 0, 1, 9, 0},
		21036: {813, 13661, 16, 12, 14, 1},
		39824: {757, 13681, 3, 8, 14, 4},
	}
	for _, o := range objs {
		if o.MatchID != 51085 {
			continue
		}
		for _, p := range o.Players {
			w, ok := want[p.Person]
			if !ok {
				continue
			}
			got := fields{p.Rating, int(p.Stats.DistanceMetres + 0.5), p.Stats.Sprints, p.Stats.ProgressivePasses, p.Stats.PossessionWon, p.Stats.Clearances}
			if got != w {
				t.Errorf("person %d = %+v, want %+v", p.Person, got, w)
			}
			delete(want, p.Person)
		}
	}
	if len(want) > 0 {
		t.Errorf("players not found in match 51085: %v", want)
	}
}
