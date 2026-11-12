package fm

import (
	"errors"
	"reflect"
	"slices"
	"strings"
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
		"2025-08-01": {&id, MatchDataPlayersUnreadable},
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
	if m.HomeTeam != "Home FC" || m.Score == nil || m.Score.Home != 2 || m.MatchData != MatchDataPlayersUnreadable {
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

// squad is a side of 11 starters (persons base..base+10; the last is taken
// off in minute 70), a substitute who came on in minute 70 (base+11) and an
// unused one (base+12).
func squad(side uint8, base int32) []savefile.MatchPlayer {
	out := make([]savefile.MatchPlayer, 0, 13)
	for i := range int32(11) {
		p := savefile.MatchPlayer{Person: base + i, Shirt: uint8(i + 1), Side: side, StartPositions: 0x10, EndPositions: 0x10, XG: 0.125, Rating: 690} //nolint:gosec // small test data
		if i == 10 {
			p.EndPositions, p.Stats.SubbedOffMinute = 0, 70
		}
		out = append(out, p)
	}
	return append(out,
		savefile.MatchPlayer{Person: base + 11, Shirt: 12, Side: side, EndPositions: 0x4000, XG: 0.5, Rating: 819, Stats: savefile.MatchPlayerStats{SubbedOnMinute: 70}},
		savefile.MatchPlayer{Person: base + 12, Shirt: 13, Side: side, Rating: 670})
}

// objectGame has detailedFixture (match 7), quickFixture and a result with
// match id 9 that has no object (pruned), with objs as the stored objects.
func objectGame(objs ...savefile.MatchObject) *Game {
	pruned := savefile.Fixture{Key: 0, Home: 2, Away: 1, Date: day(2025, 8, 22), Result: matchResult(1, 1, 9)}
	unreadable := savefile.Fixture{Key: 0, Home: 1, Away: 3, Date: day(2025, 8, 26), Result: matchResult(0, 0, 11)}
	return NewGame(Info{Date: gameDate, ManagerTeamIndex: 1}, tacticPlayers(), fixtureWorld(),
		PlayerExtras{Fixtures: []savefile.Fixture{detailedFixture, quickFixture, pruned, unreadable}, Matches: append(objs, savefile.MatchObject{MatchID: 11})})
}

func TestMatchStatusWithObjects(t *testing.T) {
	g := objectGame(savefile.MatchObject{MatchID: 7, Players: append(squad(0, 90), squad(1, 200)...)})

	got, err := g.View(false).Fixtures(FixtureFilter{})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]MatchDataStatus{"2025-08-01": MatchDataStored, "2025-08-08": MatchDataQuickSimulated, "2025-08-22": MatchDataPruned, "2025-08-26": MatchDataPlayersUnreadable}
	for _, f := range got.Results {
		if w := want[f.Date.Format(time.DateOnly)]; f.MatchData != w {
			t.Errorf("%s: match_data %q, want %q", f.Date.Format(time.DateOnly), f.MatchData, w)
		}
	}
	id := int64(9)
	m, err := g.View(false).Match(MatchSelector{MatchID: &id})
	if err != nil || m.Players != nil || m.MatchData != MatchDataPruned {
		t.Errorf("pruned match = %+v, %v; want no players", m, err)
	}
}

func TestMatchPlayers(t *testing.T) {
	home := squad(0, 92)
	home[8].Stats = savefile.MatchPlayerStats{
		Shots: 3, ShotsOnTarget: 2, ClearCutChances: 1, Fouls: 2, YellowCards: 1, Passes: 73, PassesCompleted: 65, KeyPasses: 4,
		Crosses: 7, CrossesCompleted: 3, Tackles: 4, TacklesWon: 3, KeyTackles: 2, Interceptions: 6, DistanceMetres: 9183.6,
		PossessionWon: 16, Clearances: 5, ProgressivePasses: 8, Sprints: 9,
	}
	g := objectGame(savefile.MatchObject{MatchID: 7, Players: append(home, squad(1, 200)...)})
	id := int64(7)
	m, err := g.View(false).Match(MatchSelector{MatchID: &id})
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Players) != 24 || len(m.UnusedSubstitutes) != 2 {
		t.Fatalf("%d players, %d unused; want 24 and 2", len(m.Players), len(m.UnusedSubstitutes))
	}
	// Home persons 92..104: 100 (Vicario) starts in shirt 9, 102 (Bergvall)
	// is taken off, 103 came on, 104 unused.
	vicario := m.Players[8]
	want := MatchPlayer{Player: TacticPlayer{UID: 1, Name: "Guglielmo Vicario", ShirtNumber: 9}, TeamIndex: 1, Team: "Home FC", Performance: Performance{
		Started: true, Rating: 6.9, XG: 0.13, Shots: 3, ShotsOnTarget: 2, ClearCutChances: 1, Passes: 73, PassesCompleted: 65, KeyPasses: 4,
		ProgressivePasses: 8, Crosses: 7, CrossesCompleted: 3, Tackles: 4, TacklesWon: 3, KeyTackles: 2, Interceptions: 6, Clearances: 5,
		PossessionWon: 16, Fouls: 2, YellowCards: 1, DistanceKm: 9.2, Sprints: 9,
	}}
	if !reflect.DeepEqual(vicario, want) {
		t.Errorf("Vicario = %+v\nwant      %+v", vicario, want)
	}
	if b := m.Players[10]; b.Player.Name != "Lucas Bergvall" || b.SubbedOffMinute == nil || *b.SubbedOffMinute != 70 || b.SubbedOnMinute != nil {
		t.Errorf("Bergvall = %+v, want taken off in minute 70", b)
	}
	if sub := m.Players[11]; sub.Started || sub.Rating != 8.19 || sub.XG != 0.5 || !sub.Player.Unnamed || sub.SubbedOnMinute == nil || *sub.SubbedOnMinute != 70 || sub.SubbedOffMinute != nil {
		t.Errorf("substitute = %+v, want on in minute 70", sub)
	}
	if u := m.UnusedSubstitutes; !u[0].Player.Unnamed || u[0].Player.ShirtNumber != 13 || u[0].TeamIndex != 1 || u[1].TeamIndex != 2 || u[1].Team != "Away FC" {
		t.Errorf("unused = %+v, want one per side with its team", u)
	}
}

func TestMatchObjectWithoutFixtureWarns(t *testing.T) {
	// Match 55 is unreadable and has no fixture: counted once, as an orphan.
	// objectGame's own unreadable match 11 makes the "not read: 1".
	g := objectGame(savefile.MatchObject{MatchID: 55})
	for _, want := range []string{"match objects without a fixture: 1", "match objects not read: 1"} {
		if !slices.ContainsFunc(g.Info.Warnings, func(w string) bool { return strings.Contains(w, want) }) {
			t.Errorf("warnings = %q, want %q", g.Info.Warnings, want)
		}
	}
	dup := objectGame(savefile.MatchObject{MatchID: 7, Players: append(squad(0, 300), squad(1, 320)...)}, savefile.MatchObject{MatchID: 7, Players: squad(0, 400)})
	if !slices.ContainsFunc(dup.Info.Warnings, func(w string) bool { return strings.Contains(w, "match objects with a repeated match id: 1") }) {
		t.Errorf("warnings = %q, want the repeated id", dup.Info.Warnings)
	}
	id := int64(7)
	if m, err := dup.View(false).Match(MatchSelector{MatchID: &id}); err != nil || len(m.Players) != 24 {
		t.Errorf("repeated id: %d players, %v; want the first object's 24", len(m.Players), err)
	}
}

// TestReadMatchesSkipsSavesWithoutMatchIDs checks that FM26 saves, whose
// results have no decoded match id, never read the match pack.
func TestReadMatchesSkipsSavesWithoutMatchIDs(t *testing.T) {
	objs, warnings := readMatches(nil, []savefile.Fixture{fm26Fixture, upcomingMatchF})
	if objs != nil || warnings != nil {
		t.Errorf("readMatches = %v, %v; want nothing read", objs, warnings)
	}
	// An FM27 save before any match played in detail: read, and empty.
	objs, warnings = readMatches(nil, []savefile.Fixture{quickFixture, upcomingMatchF})
	if objs == nil || len(objs) != 0 || warnings != nil {
		t.Errorf("readMatches = %v, %v; want an empty, non-nil list", objs, warnings)
	}
}

// TestMatchByIDKeepsFirstFixture guards against two results sharing a match
// id (not seen in saves): the earlier one wins.
func TestMatchByIDKeepsFirstFixture(t *testing.T) {
	later := savefile.Fixture{Key: 0, Home: 3, Away: 1, Date: day(2025, 9, 1), Result: matchResult(4, 4, 7)}
	id := int64(7)
	m, err := matchGame(detailedFixture, later).View(false).Match(MatchSelector{MatchID: &id})
	if err != nil || m.Date.Format(time.DateOnly) != "2025-08-01" {
		t.Errorf("match = %v, %v; want the 2025-08-01 result", m.Date, err)
	}
}

func TestMatchTeamStats(t *testing.T) {
	teams := [2]savefile.TeamStats{
		{Passes: 468, PassesCompleted: 402, PassesCompletedByThird: &[3]uint16{118, 215, 69}, Tackles: 6, TacklesWon: 4, Headers: 26, HeadersWon: 16, ShotsBlocked: 1, ClearCutChances: 5, Corners: 6, Fouls: 13, YellowCards: 3},
		{Passes: 500, PassesCompleted: 435, Tackles: 12, TacklesWon: 7, Headers: 25, HeadersWon: 8, ShotsBlocked: 4, ClearCutChances: 3, Corners: 4, Fouls: 7, YellowCards: 2},
	}
	g := objectGame(savefile.MatchObject{MatchID: 7, Players: append(squad(0, 92), squad(1, 200)...), TeamStats: &teams})
	id := int64(7)
	m, err := g.View(false).Match(MatchSelector{MatchID: &id})
	if err != nil {
		t.Fatal(err)
	}
	want := []TeamMatchStats{
		{TeamIndex: 1, Team: "Home FC", Passes: 468, PassesCompleted: 402, PassesCompletedByThird: &ThirdPasses{Own: 118, Middle: 215, Final: 69},
			Tackles: 6, TacklesWon: 4, Headers: 26, HeadersWon: 16, ShotsBlocked: 1, ClearCutChances: 5, Corners: 6, Fouls: 13, YellowCards: 3},
		{TeamIndex: 2, Team: "Away FC", Passes: 500, PassesCompleted: 435, // no split: left out
			Tackles: 12, TacklesWon: 7, Headers: 25, HeadersWon: 8, ShotsBlocked: 4, ClearCutChances: 3, Corners: 4, Fouls: 7, YellowCards: 2},
	}
	if !reflect.DeepEqual(m.TeamStats, want) {
		t.Errorf("team stats = %+v\nwant %+v", m.TeamStats, want)
	}
	noTeams := objectGame(savefile.MatchObject{MatchID: 7, Players: append(squad(0, 92), squad(1, 200)...)})
	if m, _ := noTeams.View(false).Match(MatchSelector{MatchID: &id}); m.TeamStats != nil || len(m.Players) == 0 {
		t.Errorf("without team stats: %+v, want players and no team_stats", m.TeamStats)
	}
	if !slices.Contains(noTeams.Info.Warnings, "match team stats not read: 1") {
		t.Errorf("warnings = %q, want the missing team stats counted", noTeams.Info.Warnings)
	}
}
