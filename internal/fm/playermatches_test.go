package fm

import (
	"errors"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/Napas/fm-mcp/internal/savefile"
)

// playerMatchesGame has two stored matches of Home FC (team 1): match 7
// (detailedFixture, 2025-08-01, at home to Away FC) and match 12
// (2025-08-15, away at Third FC), plus pruned match 9. Vicario (person 100,
// uid 1) starts both; Romero (101) plays match 7 only.
func playerMatchesGame() *Game {
	second := savefile.Fixture{Key: 0, Home: 3, Away: 1, Date: day(2025, 8, 15), Result: matchResult(0, 2, 12,
		savefile.GoalEvent{Scorer: 100, Assister: savefile.NoPerson, Side: 1, Kind: savefile.EventPenalty, Minute: 50},
		savefile.GoalEvent{Scorer: 300, Assister: 100, Side: 1, Kind: savefile.EventShootoutScored, Minute: 120})} // not an assist
	pruned := savefile.Fixture{Key: 0, Home: 2, Away: 1, Date: day(2025, 8, 22), Result: matchResult(1, 1, 9,
		savefile.GoalEvent{Scorer: 100, Assister: savefile.NoPerson, Side: 1, Kind: savefile.EventGoal, Minute: 5})}
	home7 := squad(0, 92, true) // persons 92–104: 100 starts in shirt 9, 101 starts, 102 taken off
	away12 := squad(1, 95, false)
	away12[6].Rating, away12[6].XG = 812, 0.704 // person 101 sits out: unused below
	away12[6].StartPositions, away12[6].EndPositions = 0, 0
	return NewGame(Info{Date: gameDate, ManagerTeamIndex: 1}, tacticPlayers(), fixtureWorld(), PlayerExtras{
		Fixtures: []savefile.Fixture{detailedFixture, second, pruned},
		Matches: []savefile.MatchObject{
			{MatchID: 7, Players: append(home7, squad(1, 200, false)...)},
			{MatchID: 12, Players: append(squad(0, 300, false), away12...)},
		},
	})
}

func TestPlayerMatchesNewestFirst(t *testing.T) {
	got, err := playerMatchesGame().View(false).PlayerMatches(1, PlayerMatchFilter{})
	if err != nil {
		t.Fatal(err)
	}
	no := false
	want := PlayerMatchList{
		Player: TacticPlayer{UID: 1, Name: "Guglielmo Vicario"},
		Matches: []PlayerMatch{
			{
				MatchID: 12, Date: time.Date(2025, 8, 15, 0, 0, 0, 0, time.UTC), CompetitionIndex: 8, Competition: "Sky Bet Championship", Stage: "Playoffs – Semi Final – First Leg",
				TeamIndex: 1, Team: "Home FC", OpponentIndex: 3, Opponent: "Third FC", Score: &Score{Home: 0, Away: 2}, ShirtNumber: 6,
				Performance: Performance{Started: true, Rating: 6.9, XG: 0.13}, Goals: 1, // a sub came on, nobody off: unknown
			},
			{
				MatchID: 7, Date: time.Date(2025, 8, 1, 0, 0, 0, 0, time.UTC), CompetitionIndex: 8, Competition: "Sky Bet Championship", Stage: "Playoffs – Semi Final – First Leg",
				TeamIndex: 1, Team: "Home FC", Home: true, OpponentIndex: 2, Opponent: "Away FC", Score: &Score{Home: 2, Away: 1}, ShirtNumber: 9,
				Performance: Performance{Started: true, TakenOff: &no, Rating: 6.9, XG: 0.13}, Goals: 1,
			},
		},
		Total: 2,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got  %+v\nwant %+v", got, want)
	}
}

func TestPlayerMatchesCountsAssistsAndOwnGoals(t *testing.T) {
	v := playerMatchesGame().View(false)
	romero, err := v.PlayerMatches(2, PlayerMatchFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if romero.Total != 1 || romero.Matches[0].MatchID != 7 || romero.Matches[0].Assists != 1 || romero.Matches[0].Goals != 0 {
		t.Errorf("Romero = %+v, want match 7 only (unused in 12), one assist and no goal for his missed penalty", romero)
	}
	bergvall, err := v.PlayerMatches(4, PlayerMatchFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if m := bergvall.Matches[1]; bergvall.Total != 2 || m.MatchID != 7 || m.OwnGoals != 1 || m.Goals != 0 || m.TakenOff == nil || !*m.TakenOff {
		t.Errorf("Bergvall in match 7 = %+v, want one own goal and taken off", m)
	}
}

func TestPlayerMatchesFilters(t *testing.T) {
	v := playerMatchesGame().View(false)
	aug10 := time.Date(2025, 8, 10, 0, 0, 0, 0, time.UTC)
	for _, tt := range []struct {
		name  string
		f     PlayerMatchFilter
		total int
		ids   []int64
	}{
		{"limit 1 keeps the newest", PlayerMatchFilter{Limit: 1}, 2, []int64{12}},
		{"to 08-10", PlayerMatchFilter{To: aug10}, 1, []int64{7}},
		{"from 08-10", PlayerMatchFilter{From: aug10}, 1, []int64{12}},
	} {
		got, err := v.PlayerMatches(1, tt.f)
		if err != nil {
			t.Fatalf("%s: %v", tt.name, err)
		}
		var ids []int64
		for _, m := range got.Matches {
			ids = append(ids, m.MatchID)
		}
		if got.Total != tt.total || !slices.Equal(ids, tt.ids) {
			t.Errorf("%s: total %d, matches %v; want %d, %v", tt.name, got.Total, ids, tt.total, tt.ids)
		}
	}
}

func TestPlayerMatchesErrors(t *testing.T) {
	v := playerMatchesGame().View(false)
	if _, err := v.PlayerMatches(999, PlayerMatchFilter{}); !errors.Is(err, ErrInvalidFilter) {
		t.Errorf("unknown uid: err = %v, want ErrInvalidFilter", err)
	}
	if _, err := v.PlayerMatches(1, PlayerMatchFilter{From: time.Date(2025, 9, 1, 0, 0, 0, 0, time.UTC), To: time.Date(2025, 8, 1, 0, 0, 0, 0, time.UTC)}); !errors.Is(err, ErrInvalidFilter) {
		t.Errorf("reversed range: err = %v, want ErrInvalidFilter", err)
	}
	none := NewGame(Info{Date: gameDate, ManagerTeamIndex: 1}, tacticPlayers(), fixtureWorld(), PlayerExtras{Fixtures: []savefile.Fixture{detailedFixture}})
	if _, err := none.View(false).PlayerMatches(1, PlayerMatchFilter{}); !errors.Is(err, ErrMatchPlayersUnavailable) {
		t.Errorf("no match objects: err = %v, want ErrMatchPlayersUnavailable", err)
	}
}

// TestPlayerMatchesRepeatedMatchID guards against two results sharing a
// match id (not seen in saves): the match is listed once, for the earlier
// result, as get_match picks it.
func TestPlayerMatchesRepeatedMatchID(t *testing.T) {
	later := savefile.Fixture{Key: 0, Home: 3, Away: 1, Date: day(2025, 9, 1), Result: matchResult(4, 4, 7)}
	g := NewGame(Info{Date: gameDate, ManagerTeamIndex: 1}, tacticPlayers(), fixtureWorld(), PlayerExtras{
		Fixtures: []savefile.Fixture{detailedFixture, later},
		Matches:  []savefile.MatchObject{{MatchID: 7, Players: append(squad(0, 92, true), squad(1, 200, false)...)}},
	})
	got, err := g.View(false).PlayerMatches(1, PlayerMatchFilter{})
	if err != nil || got.Total != 1 || got.Matches[0].Date.Format(time.DateOnly) != "2025-08-01" {
		t.Errorf("got %+v, %v; want match 7 once, on 2025-08-01", got, err)
	}
}

// TestPlayerMatchesFM27BeforeDetailedMatches covers an FM27 save whose
// results all lack a match id: an empty log, not an error.
func TestPlayerMatchesFM27BeforeDetailedMatches(t *testing.T) {
	g := NewGame(Info{Date: gameDate, ManagerTeamIndex: 1}, tacticPlayers(), fixtureWorld(), PlayerExtras{
		Fixtures: []savefile.Fixture{quickFixture}, Matches: []savefile.MatchObject{},
	})
	got, err := g.View(false).PlayerMatches(1, PlayerMatchFilter{})
	if err != nil || got.Total != 0 || got.Matches == nil {
		t.Errorf("got %+v, %v; want an empty log", got, err)
	}
}
