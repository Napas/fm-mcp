package savefile

import (
	"testing"
	"time"
)

const testSave27Path = "../../data/test_save_27.fm"

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
