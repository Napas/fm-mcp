package savefile

import "testing"

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
