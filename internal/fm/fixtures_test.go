package fm

import (
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Napas/fm-mcp/internal/savefile"
)

// day is a save date on the given day.
func day(year int, month time.Month, d int) savefile.Date {
	t := time.Date(year, month, d, 0, 0, 0, 0, time.UTC)
	return savefile.Date{Raw: uint16(t.YearDay()), Year: uint16(year)} //nolint:gosec // small test dates
}

func played(home, away uint8) *savefile.FixtureResult {
	return &savefile.FixtureResult{Home: [4]uint8{home, 0xff, 0xff, 0xff}, Away: [4]uint8{away, 0xff, 0xff, 0xff}}
}

// fixtureWorld is labelWorld with three teams.
func fixtureWorld() World {
	w := labelWorld()
	w.Teams = map[int64]*Team{
		1: {Index: 1, Name: "Home FC", divisionIndex: 8},
		2: {Index: 2, Name: "Away FC", divisionIndex: 8},
		3: {Index: 3, Name: "Third FC", divisionIndex: 11},
	}
	return w
}

func fixtureGame(manager int64, date time.Time, fs ...savefile.Fixture) *Game {
	return NewGame(Info{Date: date, ManagerTeamIndex: manager}, nil, fixtureWorld(), PlayerExtras{Fixtures: fs})
}

func fixtureDates(fs []Fixture) []string {
	out := make([]string, len(fs))
	for i, f := range fs {
		out[i] = f.Date.Format(time.DateOnly)
	}
	return out
}

func TestFixturesDefaultView(t *testing.T) {
	g := fixtureGame(1, time.Date(2025, 8, 20, 0, 0, 0, 0, time.UTC),
		savefile.Fixture{Key: 0, Home: 1, Away: 2, Date: day(2025, 8, 1), Result: played(1, 0)},
		savefile.Fixture{Key: 0, Home: 2, Away: 1, Date: day(2025, 8, 15), Result: played(2, 2)},
		savefile.Fixture{Key: 0, Home: 1, Away: 3, Date: day(2025, 8, 8), Result: played(0, 1)},
		savefile.Fixture{Key: 0, Home: 1, Away: 2, Date: day(2025, 8, 29)},
		savefile.Fixture{Key: 0, Home: 3, Away: 1, Date: day(2025, 8, 22)},
		savefile.Fixture{Key: 0, Home: 2, Away: 3, Date: day(2025, 8, 23)},
	)

	got, err := g.View(false).Fixtures(FixtureFilter{Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if d := fixtureDates(got.Results); strings.Join(d, ",") != "2025-08-15,2025-08-08" {
		t.Errorf("results = %v, want newest two first", d)
	}
	if d := fixtureDates(got.Upcoming); strings.Join(d, ",") != "2025-08-22,2025-08-29" {
		t.Errorf("upcoming = %v, want soonest first", d)
	}
	if got.TotalResults != 3 || got.TotalUpcoming != 2 {
		t.Errorf("totals = %d, %d; want 3, 2", got.TotalResults, got.TotalUpcoming)
	}
	f := got.Results[0]
	if f.HomeTeam != "Away FC" || f.AwayTeam != "Home FC" || f.Competition != "Sky Bet Championship" || f.Stage != "Playoffs – Semi Final – First Leg" || f.Score == nil || f.Score.Home != 2 {
		t.Errorf("fixture = %+v", f)
	}
}

func TestFixturesCompetitionWithoutTeam(t *testing.T) {
	g := fixtureGame(1, gameDate,
		savefile.Fixture{Key: 0, Home: 1, Away: 2, Date: day(2025, 8, 1)},
		savefile.Fixture{Key: 2, Home: 3, Away: 2, Date: day(2025, 8, 2)},
	)
	ligue2 := int64(12)

	got, err := g.View(false).Fixtures(FixtureFilter{Competition: &ligue2})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Upcoming) != 1 || got.Upcoming[0].HomeTeamIndex != 3 || got.Upcoming[0].SecondCompetition != "Ligue 2" {
		t.Errorf("upcoming = %+v, want the two-division playoff only", got.Upcoming)
	}
}

func TestFixturesDateRangeAndStatus(t *testing.T) {
	g := fixtureGame(1, gameDate,
		savefile.Fixture{Key: 0, Home: 1, Away: 2, Date: day(2025, 8, 1), Result: played(1, 0)},
		savefile.Fixture{Key: 0, Home: 1, Away: 2, Date: day(2025, 8, 9), Result: played(1, 0)},
		savefile.Fixture{Key: 0, Home: 1, Away: 2, Date: day(2025, 8, 10)},
	)

	got, err := g.View(false).Fixtures(FixtureFilter{From: time.Date(2025, 8, 1, 0, 0, 0, 0, time.UTC), To: time.Date(2025, 8, 9, 0, 0, 0, 0, time.UTC), Status: "played"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Results) != 2 || len(got.Upcoming) != 0 {
		t.Errorf("got %d results, %d upcoming; want 2, 0", len(got.Results), len(got.Upcoming))
	}
}

func TestFixturesRejectsUnknownTeamAndStatus(t *testing.T) {
	g := fixtureGame(1, gameDate)
	missing := int64(99)
	if _, err := g.View(false).Fixtures(FixtureFilter{Team: &missing}); !errors.Is(err, ErrInvalidFilter) {
		t.Errorf("unknown team: err = %v", err)
	}
	if _, err := g.View(false).Fixtures(FixtureFilter{Status: "soon"}); !errors.Is(err, ErrInvalidFilter) {
		t.Errorf("unknown status: err = %v", err)
	}
}

func TestFixturesNeedATeamWhenManagerUnknown(t *testing.T) {
	g := fixtureGame(noTeam, gameDate)
	if _, err := g.View(false).Fixtures(FixtureFilter{}); !errors.Is(err, ErrInvalidFilter) || !strings.Contains(err.Error(), "team_index") {
		t.Errorf("err = %v, want one asking for team_index", err)
	}
}

func TestFixtureScoreParts(t *testing.T) {
	s := newScore(&savefile.FixtureResult{Home: [4]uint8{0, 0, 4, 1}, Away: [4]uint8{0, 0, 1, 1}})
	if s == nil || s.Home != 0 || s.Away != 0 || s.ExtraTime == nil || *s.ExtraTime != [2]int{0, 0} ||
		s.Penalties == nil || *s.Penalties != [2]int{4, 1} || s.Aggregate == nil || *s.Aggregate != [2]int{1, 1} {
		t.Errorf("score = %+v", s)
	}
	s = newScore(played(2, 1))
	if s == nil || s.Home != 2 || s.Away != 1 || s.ExtraTime != nil || s.Penalties != nil || s.Aggregate != nil {
		t.Errorf("score = %+v, want 90 minutes only", s)
	}
	if newScore(&savefile.FixtureResult{Home: [4]uint8{0xff, 0xff, 0xff, 0xff}, Away: [4]uint8{0xff, 0xff, 0xff, 0xff}}) != nil {
		t.Error("a result without a 90-minute score is not played")
	}
}

func TestUnresolvedFixtureKeysAreWarned(t *testing.T) {
	g := fixtureGame(1, gameDate,
		savefile.Fixture{Key: 0, Home: 1, Away: 2, Date: day(2025, 8, 1)},
		savefile.Fixture{Key: 99999, Home: 1, Away: 2, Date: day(2025, 8, 2)},
		savefile.Fixture{Key: 3, Home: 1, Away: 2, Date: day(2025, 8, 3)},
	)

	got, _ := g.View(false).Fixtures(FixtureFilter{})
	if got.TotalUpcoming != 1 {
		t.Errorf("%d fixtures served, want 1", got.TotalUpcoming)
	}
	if !slicesContain(g.Info.Warnings, "competition key does not resolve: 2") {
		t.Errorf("warnings = %q", g.Info.Warnings)
	}
}

func slicesContain(ss []string, sub string) bool {
	for _, s := range ss {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func TestFindCompetition(t *testing.T) {
	w := fixtureWorld()
	w.Competitions = map[int64]string{1: "Premier League", 2: "Scottish Premier League", 3: "Irish League", 8: "Sky Bet Championship"}
	g := NewGame(Info{Date: gameDate}, nil, w, PlayerExtras{})
	v := g.View(false)

	for query, want := range map[string]int64{"sky bet championship": 8, "Premier League": 1, "irish": 3} {
		if got, err := v.FindCompetition(query); err != nil || got != want {
			t.Errorf("FindCompetition(%q) = %d, %v; want %d", query, got, err, want)
		}
	}
	_, err := v.FindCompetition("league")
	if !errors.Is(err, ErrInvalidFilter) || !strings.Contains(err.Error(), "competition_index") || !strings.Contains(err.Error(), "Irish League") {
		t.Errorf("ambiguous: err = %v", err)
	}
	if _, err := v.FindCompetition("zzz"); !errors.Is(err, ErrInvalidFilter) {
		t.Errorf("no match: err = %v", err)
	}
}

func TestTestSaveTottenhamFirstLeagueFixture(t *testing.T) {
	if gameDBDir() == "" {
		t.Skip("game not installed")
	}
	g := loadTestSave(t)
	spurs := int64(547)
	league := g.world.Teams[spurs].divisionIndex

	got, err := g.View(false).Fixtures(FixtureFilter{Team: &spurs, Competition: &league, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Upcoming) != 1 {
		t.Fatalf("no upcoming league fixture")
	}
	f := got.Upcoming[0]
	if f.Date.Format(time.DateOnly) != "2025-08-16" || f.HomeTeam != "Tottenham Hotspur" || f.AwayTeam != "Burnley" || f.Score != nil || f.Competition != "Premier League" {
		t.Errorf("first league fixture = %+v", f)
	}
}

// fixtureLine renders a fixture as "date home h–a away (stage)".
func fixtureLine(f Fixture) string {
	s := f.Date.Format(time.DateOnly) + " " + f.HomeTeam
	if f.Score != nil {
		s += " " + strconv.Itoa(f.Score.Home) + "–" + strconv.Itoa(f.Score.Away)
	} else {
		s += " v"
	}
	s += " " + f.AwayTeam
	if f.Stage != "" {
		s += " (" + f.Stage + ")"
	}
	return s
}

func fixtureLines(fs []Fixture) []string {
	out := make([]string, len(fs))
	for i, f := range fs {
		out[i] = fixtureLine(f)
	}
	return out
}

// TestRealSaveFixtures checks fixtures of the KŽ save of 2035-12-02 decoded
// from bytes; Metz's are to be confirmed in game.
func TestRealSaveFixtures(t *testing.T) {
	g := loadRealSaveNamed(t, "FM_REAL_SAVE", time.Date(2035, 12, 2, 0, 0, 0, 0, time.UTC))
	v := g.View(false)

	metz := int64(652)
	got, err := v.Fixtures(FixtureFilter{Team: &metz, Limit: 3})
	if err != nil {
		t.Fatal(err)
	}
	wantResults := []string{
		"2035-12-02 Olympique de Marseille 1–3 FC Metz",
		"2035-11-29 Molde FK 1–6 FC Metz (League Phase)",
		"2035-11-25 FC Metz 5–1 Havre AC",
	}
	wantUpcoming := []string{
		"2035-12-09 FC Metz v Stade de Reims",
		"2035-12-13 FC Metz v Víkingur R. (League Phase)",
		"2036-01-06 ESTAC Troyes v FC Metz",
	}
	if l := fixtureLines(got.Results); strings.Join(l, "\n") != strings.Join(wantResults, "\n") {
		t.Errorf("Metz results:\n%s\nwant:\n%s", strings.Join(l, "\n"), strings.Join(wantResults, "\n"))
	}
	if l := fixtureLines(got.Upcoming); strings.Join(l, "\n") != strings.Join(wantUpcoming, "\n") {
		t.Errorf("Metz upcoming:\n%s\nwant:\n%s", strings.Join(l, "\n"), strings.Join(wantUpcoming, "\n"))
	}
	if len(got.Results) > 1 && got.Results[1].Competition != "UEFA Europa League" {
		t.Errorf("Molde fixture competition = %q", got.Results[1].Competition)
	}

	champ, err := v.FindCompetition("Sky Bet Championship")
	if err != nil {
		t.Fatal(err)
	}
	po, err := v.Fixtures(FixtureFilter{Competition: &champ, From: time.Date(2035, 5, 10, 0, 0, 0, 0, time.UTC), To: time.Date(2035, 5, 31, 0, 0, 0, 0, time.UTC), Status: "played"})
	if err != nil {
		t.Fatal(err)
	}
	wantPlayoffs := []string{
		"2035-05-28 Crystal Palace 2–1 Fulham (Playoffs – Final)",
		"2035-05-19 Blackburn Rovers 0–1 Crystal Palace (Playoffs – Semi Final – Second Leg)",
		"2035-05-18 Leeds United 0–2 Fulham (Playoffs – Semi Final – Second Leg)",
		"2035-05-14 Fulham 2–0 Leeds United (Playoffs – Semi Final – First Leg)",
		"2035-05-13 Crystal Palace 4–1 Blackburn Rovers (Playoffs – Semi Final – First Leg)",
	}
	if l := fixtureLines(po.Results); strings.Join(l, "\n") != strings.Join(wantPlayoffs, "\n") {
		t.Errorf("playoffs:\n%s\nwant:\n%s", strings.Join(l, "\n"), strings.Join(wantPlayoffs, "\n"))
	}
	if len(po.Results) > 1 && (po.Results[1].Score.Aggregate == nil || *po.Results[1].Score.Aggregate != [2]int{1, 5}) {
		t.Errorf("second-leg aggregate = %+v", po.Results[1].Score)
	}
}

func TestQueriesRejectUnknownCompetition(t *testing.T) {
	g := tableGame(1, savefile.LeagueTables{Tables: []savefile.LeagueTable{championshipTable}}, 1)
	g.fixtures, _ = resolveFixtures([]savefile.Fixture{{Key: 0, Home: 1, Away: 2, Date: day(2025, 8, 1)}}, tableWorld())
	g.hasFixtures = true
	for _, c := range []int64{-1, 13, 1 << 40} {
		if _, err := g.View(false).Fixtures(FixtureFilter{Competition: &c}); !errors.Is(err, ErrInvalidFilter) {
			t.Errorf("Fixtures(competition %d): err = %v, want ErrInvalidFilter", c, err)
		}
		if _, err := g.View(false).Tables(nil, &c); !errors.Is(err, ErrInvalidFilter) {
			t.Errorf("Tables(competition %d): err = %v, want ErrInvalidFilter", c, err)
		}
	}
}

func TestFixturesRejectReversedRange(t *testing.T) {
	g := fixtureGame(1, gameDate, savefile.Fixture{Key: 0, Home: 1, Away: 2, Date: day(2025, 8, 1)})
	f := FixtureFilter{From: time.Date(2025, 9, 1, 0, 0, 0, 0, time.UTC), To: time.Date(2025, 8, 1, 0, 0, 0, 0, time.UTC)}
	if _, err := g.View(false).Fixtures(f); !errors.Is(err, ErrInvalidFilter) {
		t.Errorf("err = %v, want ErrInvalidFilter", err)
	}
}

func TestFixturesUnavailable(t *testing.T) {
	g := NewGame(Info{Date: gameDate, ManagerTeamIndex: 1}, nil, fixtureWorld(), PlayerExtras{})
	if _, err := g.View(false).Fixtures(FixtureFilter{}); !errors.Is(err, ErrFixturesUnavailable) {
		t.Errorf("err = %v, want ErrFixturesUnavailable", err)
	}
	empty := NewGame(Info{Date: gameDate, ManagerTeamIndex: 1}, nil, fixtureWorld(), PlayerExtras{Fixtures: []savefile.Fixture{}})
	if got, err := empty.View(false).Fixtures(FixtureFilter{}); err != nil || got.TotalUpcoming != 0 {
		t.Errorf("a save without fixtures: %+v, %v", got, err)
	}
}

func TestFixtureSecondCompetitionIndexIsACopy(t *testing.T) {
	g := fixtureGame(1, gameDate,
		savefile.Fixture{Key: 2, Home: 1, Away: 2, Date: day(2025, 8, 1)},
		savefile.Fixture{Key: 2, Home: 2, Away: 1, Date: day(2025, 8, 2)},
	)
	got, err := g.View(false).Fixtures(FixtureFilter{})
	if err != nil || len(got.Upcoming) != 2 {
		t.Fatalf("%+v, %v", got, err)
	}
	*got.Upcoming[0].SecondCompetitionIndex = 99
	if *got.Upcoming[1].SecondCompetitionIndex != 12 {
		t.Error("fixtures share one SecondCompetitionIndex")
	}
}
