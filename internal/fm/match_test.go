package fm

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/Napas/fm-mcp/internal/savefile"
)

// matchResult is a 27.0.0 result with a match id and events.
func matchResult(home, away uint8, id int32, events ...savefile.GoalEvent) *savefile.FixtureResult {
	r := played(home, away)
	r.Decoded, r.MatchID, r.Events = true, id, events
	return r
}

// matchGame has tacticPlayers (persons 100–102) and fixtures between teams
// 1, 2 and 3 of fixtureWorld, with team 1 the manager's.
func matchGame(fs ...savefile.Fixture) *Game {
	return NewGame(Info{Date: gameDate, ManagerTeamIndex: 1}, tacticPlayers(), fixtureWorld(), PlayerExtras{Fixtures: fs})
}

var (
	// detailedFixture: Home FC 2–1 Away FC with an assisted goal, an
	// own goal for the away side by a home player, a penalty taken by a
	// person who is not a player, and a missed penalty.
	detailedFixture = savefile.Fixture{Key: 0, Home: 1, Away: 2, Date: day(2025, 8, 1), Result: matchResult(2, 1, 7,
		savefile.GoalEvent{Scorer: 100, Assister: 101, Side: 0, Kind: savefile.EventGoal, Minute: 9},
		savefile.GoalEvent{Scorer: 102, Assister: savefile.NoPerson, Side: 1, Kind: savefile.EventOwnGoal, Minute: 30},
		savefile.GoalEvent{Scorer: 999, Assister: savefile.NoPerson, Side: 0, Kind: savefile.EventPenalty, Minute: 90, AddedMinutes: 3},
		savefile.GoalEvent{Scorer: 101, Assister: savefile.NoPerson, Side: 1, Kind: savefile.EventMissedPenalty, Minute: 60},
	)}
	quickFixture   = savefile.Fixture{Key: 0, Home: 2, Away: 1, Date: day(2025, 8, 8), Result: matchResult(0, 3, savefile.NoMatchID)}
	fm26Fixture    = savefile.Fixture{Key: 0, Home: 1, Away: 3, Date: day(2025, 8, 15), Result: played(1, 1)}
	upcomingMatchF = savefile.Fixture{Key: 0, Home: 3, Away: 1, Date: day(2025, 8, 29)}
)

func TestFixturesCarryMatchData(t *testing.T) {
	g := matchGame(detailedFixture, quickFixture, fm26Fixture, upcomingMatchF)

	got, err := g.View(false).Fixtures(FixtureFilter{})
	if err != nil {
		t.Fatal(err)
	}
	id := int64(7)
	want := map[string]struct {
		id     *int64
		status MatchDataStatus
	}{
		"2025-08-01": {&id, MatchDataGoals},
		"2025-08-08": {nil, MatchDataQuickSimulated},
		"2025-08-15": {nil, MatchDataUnsupported},
		"2025-08-29": {nil, ""},
	}
	for _, f := range append(got.Results, got.Upcoming...) {
		w := want[f.Date.Format(time.DateOnly)]
		if !reflect.DeepEqual(f.MatchID, w.id) || f.MatchData != w.status {
			t.Errorf("%s: match_id %v, match_data %q; want %v, %q", f.Date.Format(time.DateOnly), f.MatchID, f.MatchData, w.id, w.status)
		}
	}
}

func TestMatchByIDNamesEvents(t *testing.T) {
	id := int64(7)
	m, err := matchGame(detailedFixture, quickFixture).View(false).Match(MatchSelector{MatchID: &id})
	if err != nil {
		t.Fatal(err)
	}
	if m.HomeTeam != "Home FC" || m.Score == nil || m.Score.Home != 2 || m.MatchData != MatchDataGoals {
		t.Errorf("match = %+v", m.Fixture)
	}
	want := []MatchEvent{
		{Minute: 9, Type: "goal", TeamIndex: ptr(int64(1)), Team: "Home FC", Player: &TacticPlayer{UID: 1, Name: "Guglielmo Vicario"}, Assist: &TacticPlayer{UID: 2, Name: "Cristian Romero"}},
		{Minute: 30, Type: "own_goal", TeamIndex: ptr(int64(2)), Team: "Away FC", Player: &TacticPlayer{UID: 4, Name: "Lucas Bergvall"}},
		{Minute: 90, AddedMinutes: 3, Type: "penalty", TeamIndex: ptr(int64(1)), Team: "Home FC", Player: &TacticPlayer{Unnamed: true}},
		{Minute: 60, Type: "missed_penalty", TeamIndex: ptr(int64(2)), Team: "Away FC", Player: &TacticPlayer{UID: 2, Name: "Cristian Romero"}},
	}
	if !reflect.DeepEqual(m.Events, want) {
		t.Errorf("events = %+v\nwant %+v", m.Events, want)
	}
}

func TestMatchByTeamAndDate(t *testing.T) {
	v := matchGame(detailedFixture, quickFixture, fm26Fixture, upcomingMatchF).View(false)
	team := int64(2)

	m, err := v.Match(MatchSelector{Team: &team, Date: time.Date(2025, 8, 8, 0, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	if m.HomeTeamIndex != 2 || m.MatchData != MatchDataQuickSimulated || m.Events != nil {
		t.Errorf("match = %+v, want the quick-simulated match without events", m)
	}
	if _, err := v.Match(MatchSelector{Team: &team, Date: time.Date(2025, 8, 2, 0, 0, 0, 0, time.UTC)}); !errors.Is(err, ErrNoMatch) {
		t.Errorf("no match that day: err = %v, want ErrNoMatch", err)
	}
	if _, err := v.Match(MatchSelector{Team: &team, Date: time.Date(2025, 8, 29, 0, 0, 0, 0, time.UTC)}); !errors.Is(err, ErrNoMatch) {
		t.Errorf("an unplayed match: err = %v, want ErrNoMatch", err)
	}
}

func TestMatchDefaultsToManagersLastResult(t *testing.T) {
	m, err := matchGame(detailedFixture, quickFixture, fm26Fixture, upcomingMatchF).View(false).Match(MatchSelector{})
	if err != nil {
		t.Fatal(err)
	}
	if m.Date.Format(time.DateOnly) != "2025-08-15" || m.MatchData != MatchDataUnsupported {
		t.Errorf("match = %+v, want team 1's last result (2025-08-15)", m.Fixture)
	}
}

func TestMatchErrors(t *testing.T) {
	for _, id := range []int64{8, savefile.NoMatchID, 0} { // fm26Fixture has no decoded id
		if _, err := matchGame(detailedFixture, quickFixture, fm26Fixture).View(false).Match(MatchSelector{MatchID: &id}); !errors.Is(err, ErrNoMatch) {
			t.Errorf("match id %d: err = %v, want ErrNoMatch", id, err)
		}
	}
	if _, err := NewGame(Info{ManagerTeamIndex: 1}, nil, fixtureWorld(), PlayerExtras{}).View(false).Match(MatchSelector{}); !errors.Is(err, ErrFixturesUnavailable) {
		t.Errorf("no fixtures: err = %v, want ErrFixturesUnavailable", err)
	}
	team := int64(99)
	if _, err := matchGame(detailedFixture).View(false).Match(MatchSelector{Team: &team}); !errors.Is(err, ErrInvalidFilter) {
		t.Errorf("unknown team: err = %v, want ErrInvalidFilter", err)
	}
}

func TestMatchUnknownEventKindAndSide(t *testing.T) {
	f := savefile.Fixture{Key: 0, Home: 1, Away: 2, Date: day(2025, 8, 1), Result: matchResult(1, 0, 3,
		savefile.GoalEvent{Scorer: 100, Assister: savefile.NoPerson, Side: 2, Kind: 9, Minute: 5})}
	id := int64(3)
	m, err := matchGame(f).View(false).Match(MatchSelector{MatchID: &id})
	if err != nil {
		t.Fatal(err)
	}
	if e := m.Events[0]; e.Type != "unknown_9" || e.TeamIndex != nil || e.Team != "" {
		t.Errorf("event = %+v, want type unknown_9 and no team", e)
	}
}
